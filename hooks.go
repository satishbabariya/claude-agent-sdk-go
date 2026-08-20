package claudeagent

import (
	"encoding/json"
	"io"
)

// HookEvent identifies which Claude Code hook a payload came from. Only
// PostToolUse has actually been exercised against a real payload so far
// (see go-observer-spike's hook.go); the rest are named from the npm SDK's
// HOOK_EVENTS list (sdk.d.ts) so a caller can switch on HookInput.Event
// without inventing string constants, even for events this package hasn't
// been tested against yet.
type HookEvent string

const (
	HookEventPreToolUse       HookEvent = "PreToolUse"
	HookEventPostToolUse      HookEvent = "PostToolUse"
	HookEventNotification     HookEvent = "Notification"
	HookEventUserPromptSubmit HookEvent = "UserPromptSubmit"
	HookEventSessionStart     HookEvent = "SessionStart"
	HookEventSessionEnd       HookEvent = "SessionEnd"
	HookEventStop             HookEvent = "Stop"
	HookEventSubagentStart    HookEvent = "SubagentStart"
	HookEventSubagentStop     HookEvent = "SubagentStop"
	HookEventPreCompact       HookEvent = "PreCompact"
)

// HookInput is the subset of a Claude Code hook payload this package reads,
// verified against a real PostToolUse payload (see go-observer-spike's
// simulated and then genuinely Claude-Code-triggered hook tests) and against
// claude-mem's own src/cli/adapters/claude-code.ts normalizeInput, which
// reads the same field names off the same payload.
//
// ToolInput and ToolResponse are kept as raw JSON deliberately: claude-mem's
// real observer prompt is built from json.Stringify(tool_input) /
// json.Stringify(tool_response) verbatim (ClaudeProvider.ts's
// createMessageGenerator), not a reshaped extraction, so RawMessage is the
// faithful type here, not a parsed Go struct guessing at every tool's shape.
type HookInput struct {
	SessionID      string          `json:"session_id"`
	TranscriptPath string          `json:"transcript_path"`
	Cwd            string          `json:"cwd"`
	Event          HookEvent       `json:"hook_event_name"`
	ToolName       string          `json:"tool_name"`
	ToolInput      json.RawMessage `json:"tool_input"`
	ToolResponse   json.RawMessage `json:"tool_response"`
	// Prompt is UserPromptSubmit's payload field: the user's submitted text,
	// verbatim. Confirmed against a real captured payload (a stub hook
	// command dumping stdin to a file, the same technique this package's
	// other fields were verified with) — a real session's UserPromptSubmit
	// payload also carries prompt_id and permission_mode, which nothing in
	// this package or its callers has needed yet, so they're not modeled
	// here; add them if a real caller needs them, not preemptively.
	Prompt string `json:"prompt"`
}

// ParseHookInput reads and decodes one hook payload from r (typically
// os.Stdin inside a hook command). A payload that isn't valid JSON, or is
// for an event this type doesn't model any fields for, decodes to a
// zero-ish HookInput rather than erroring where possible — callers should
// still check the returned error, but a hook process crashing on an
// unexpected payload shape is exactly the failure mode claude-mem's own
// tolerant JSONL parsing (transcript-parser.ts) avoids on purpose.
func ParseHookInput(r io.Reader) (HookInput, error) {
	var in HookInput
	dec := json.NewDecoder(r)
	err := dec.Decode(&in)
	return in, err
}
