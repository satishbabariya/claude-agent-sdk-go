package claudeagent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// Result is one turn's outcome, straight off the CLI's stream-json "result"
// line. Deliberately not classified into retryable/fatal here — see the
// package doc: that's the caller's policy decision, not a protocol fact.
type Result struct {
	// Text is the model's final reply text for this turn.
	Text string
	// IsError mirrors the CLI's own is_error field.
	IsError bool
	// APIErrorStatus is the upstream HTTP-equivalent status when IsError is
	// true and the CLI reported one (e.g. 401, 404, 429, 529) — nil
	// otherwise. A caller classifying errors reads this, not Text.
	APIErrorStatus *int
	// Subtype is the CLI's own subtype field (e.g. "success").
	Subtype string
	// SessionID is the CLI's own session id — every turn sent to the same
	// Session returns the same value, which is how a caller confirms a
	// multi-turn conversation actually shared context rather than each Send
	// spawning something independent.
	SessionID string
	CostUSD   float64
	// CacheReadInputTokens, when it rises across turns on the same Session,
	// is further confirmation of real context accumulation.
	CacheReadInputTokens int
}

// resultLine is the subset of one stream-json line this package parses.
// Every other line shape (system/assistant/etc.) is read and discarded —
// only "result" closes out a turn.
type resultLine struct {
	Type           string  `json:"type"`
	Subtype        string  `json:"subtype"`
	IsError        bool    `json:"is_error"`
	Result         string  `json:"result"`
	TotalCostUSD   float64 `json:"total_cost_usd"`
	APIErrorStatus *int    `json:"api_error_status"`
	SessionID      string  `json:"session_id"`
	Usage          struct {
		CacheReadInputTokens int `json:"cache_read_input_tokens"`
	} `json:"usage"`
}

type stdinMessage struct {
	Type    string `json:"type"`
	Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`
}

func newStdinMessage(content string) stdinMessage {
	var m stdinMessage
	m.Type = "user"
	m.Message.Role = "user"
	m.Message.Content = content
	return m
}

// Session is one long-lived `claude` subprocess in --input-format
// stream-json --output-format stream-json mode: Send feeds it one user turn
// at a time and blocks for that turn's result. This is the entire mechanism
// the npm SDK's query() wraps — see the package doc.
//
// Turns are processed strictly in order by the CLI, which is what makes
// concurrent-looking Send/read safe despite there being no per-turn
// correlation id in the protocol: the next "result" line read always
// belongs to the most recently written turn.
type Session struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	lines   chan resultLine
	scanErr chan error
}

// NewSession spawns the subprocess described by opts. The caller must Close
// the returned Session even if it errors before any Send.
func NewSession(ctx context.Context, opts Options) (*Session, error) {
	bin := opts.ClaudeExecutable
	if bin == "" {
		found, err := FindClaudeExecutable()
		if err != nil {
			return nil, err
		}
		bin = found
	}

	args := []string{
		"-p",
		"--input-format", "stream-json",
		"--output-format", "stream-json",
		"--verbose",
	}
	if opts.Model != "" {
		args = append(args, "--model", opts.Model)
	}
	if opts.SystemPrompt != "" {
		args = append(args, "--system-prompt", opts.SystemPrompt)
	}
	if opts.PermissionMode != PermissionModeDefault {
		args = append(args, "--permission-mode", string(opts.PermissionMode))
	}
	if opts.Tools != nil {
		args = append(args, "--tools", strings.Join(opts.Tools, ","))
	}
	if len(opts.DisallowedTools) > 0 {
		args = append(args, "--disallowed-tools", strings.Join(opts.DisallowedTools, ","))
	}
	if opts.SettingSources != nil {
		strs := make([]string, len(opts.SettingSources))
		for i, s := range opts.SettingSources {
			strs[i] = string(s)
		}
		args = append(args, "--setting-sources", strings.Join(strs, ","))
	}
	if opts.StrictMCPConfig {
		args = append(args, "--strict-mcp-config")
	}

	cmd := exec.CommandContext(ctx, bin, args...)
	if opts.Cwd != "" {
		cmd.Dir = opts.Cwd
	}
	if opts.Env != nil {
		cmd.Env = opts.Env
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("spawn %s: %w", bin, err)
	}

	s := &Session{
		cmd:     cmd,
		stdin:   stdin,
		lines:   make(chan resultLine, 8),
		scanErr: make(chan error, 1),
	}

	go func() {
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
		for scanner.Scan() {
			var l resultLine
			if json.Unmarshal(scanner.Bytes(), &l) != nil {
				continue
			}
			if l.Type == "result" {
				s.lines <- l
			}
		}
		close(s.lines)
		s.scanErr <- scanner.Err()
	}()

	return s, nil
}

// Send writes one prompt as a user turn and blocks for its result.
func (s *Session) Send(prompt string) (Result, error) {
	enc := json.NewEncoder(s.stdin)
	if err := enc.Encode(newStdinMessage(prompt)); err != nil {
		return Result{}, fmt.Errorf("write turn: %w", err)
	}

	l, ok := <-s.lines
	if !ok {
		err := <-s.scanErr
		if err == nil {
			err = fmt.Errorf("claude process ended before answering this turn")
		}
		return Result{}, err
	}

	return Result{
		Text:                 l.Result,
		IsError:              l.IsError,
		APIErrorStatus:       l.APIErrorStatus,
		Subtype:              l.Subtype,
		SessionID:            l.SessionID,
		CostUSD:              l.TotalCostUSD,
		CacheReadInputTokens: l.Usage.CacheReadInputTokens,
	}, nil
}

// Close signals end-of-input and waits for the subprocess to exit.
func (s *Session) Close() error {
	s.stdin.Close()
	return s.cmd.Wait()
}
