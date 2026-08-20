package claudeagent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
)

// TestMain intercepts helper-process mode: when GO_WANT_HELPER_PROCESS=1 is
// set, this test binary acts as a fake `claude` CLI instead of running as
// `go test` — the same technique the standard library's os/exec tests use
// (TestHelperProcess) to test subprocess-spawning code without depending
// on any real external binary.
//
// This is what makes NewSession/Send testable at all. Before this, neither
// this package nor claude-mem-go unit-tested the actual subprocess/
// stream-json protocol layer — it was verified by hand against the real
// claude CLI (documented as a known gap in claude-mem-go's README). The
// fake CLI below speaks the real wire shape (stdinMessage in, resultLine
// out) rather than mocking Session's own methods, so what's under test is
// the real NewSession/Send code path: process spawn, stdin encoding,
// stdout scanning, and result-line routing.
func TestMain(m *testing.M) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") == "1" {
		fakeClaudeCLI()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// fakeClaudeCLI reads newline-delimited stdinMessage JSON (the exact shape
// Session.Send writes) and replies with one resultLine JSON per turn on
// stdout. Behavior is configurable via env vars so different tests can
// drive different scenarios through the same helper process:
//
//   - FAKE_CLI_SESSION_ID: session_id every result line reports (default
//     "fake-session-1") — fixed per process, matching a real CLI's actual
//     behavior of keeping one session_id for the process's whole lifetime.
//   - FAKE_CLI_EXIT_AFTER: if set, the process exits (without answering)
//     once it has received this many turns — simulates the subprocess
//     dying mid-conversation.
func fakeClaudeCLI() {
	sessionID := os.Getenv("FAKE_CLI_SESSION_ID")
	if sessionID == "" {
		sessionID = "fake-session-1"
	}
	exitAfter := -1
	if v := os.Getenv("FAKE_CLI_EXIT_AFTER"); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil {
			exitAfter = n
		}
	}

	turn := 0
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	enc := json.NewEncoder(os.Stdout)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(strings.TrimSpace(string(line))) == 0 {
			continue
		}
		var in stdinMessage
		if err := json.Unmarshal(line, &in); err != nil {
			continue
		}
		turn++

		if exitAfter >= 0 && turn > exitAfter {
			// Simulate a dead subprocess: exit without ever answering this
			// turn, instead of writing a result line for it.
			return
		}

		out := resultLine{
			Type:         "result",
			Subtype:      "success",
			Result:       "echo: " + in.Message.Content,
			SessionID:    sessionID,
			TotalCostUSD: 0.001 * float64(turn),
		}
		out.Usage.CacheReadInputTokens = turn * 100

		if strings.Contains(in.Message.Content, "TRIGGER_ERROR") {
			out.IsError = true
			status := 529
			out.APIErrorStatus = &status
			out.Result = "overloaded"
		}

		if err := enc.Encode(out); err != nil {
			fmt.Fprintln(os.Stderr, "fake cli: encode result:", err)
			os.Exit(1)
		}
	}
}

// newFakeSession spawns a Session whose subprocess is this same test
// binary re-invoked in helper-process mode (see TestMain) — a real
// exec.Cmd, real stdin/stdout pipes, real stream-json encoding/decoding,
// just not the real claude CLI on the other end.
func newFakeSession(t *testing.T, extraEnv ...string) *Session {
	t.Helper()
	env := append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
	env = append(env, extraEnv...)
	s, err := NewSession(context.Background(), Options{
		ClaudeExecutable: os.Args[0],
		Env:              env,
	})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestSessionSendReturnsResultLine(t *testing.T) {
	s := newFakeSession(t)
	res, err := s.Send("hello")
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if res.Text != "echo: hello" {
		t.Errorf("Text = %q, want %q", res.Text, "echo: hello")
	}
	if res.IsError {
		t.Error("IsError = true on a normal turn")
	}
	if res.Subtype != "success" {
		t.Errorf("Subtype = %q, want success", res.Subtype)
	}
}

// TestSessionMultiTurnSharesSessionIDAndAccumulatesCache is the automated
// version of the manual check this project's own history relies on to
// confirm a Session is genuinely one persistent subprocess handling
// multiple turns in order, not something that could accidentally respawn
// per call: the same session_id across turns, and cache_read_input_tokens
// actually rising.
func TestSessionMultiTurnSharesSessionIDAndAccumulatesCache(t *testing.T) {
	s := newFakeSession(t, "FAKE_CLI_SESSION_ID=abc-123")

	first, err := s.Send("turn one")
	if err != nil {
		t.Fatalf("Send (first): %v", err)
	}
	second, err := s.Send("turn two")
	if err != nil {
		t.Fatalf("Send (second): %v", err)
	}

	if first.SessionID != "abc-123" || second.SessionID != "abc-123" {
		t.Fatalf("SessionID = %q then %q, want both to equal abc-123 (same persistent subprocess)", first.SessionID, second.SessionID)
	}
	if second.CacheReadInputTokens <= first.CacheReadInputTokens {
		t.Fatalf("CacheReadInputTokens did not rise across turns: %d then %d", first.CacheReadInputTokens, second.CacheReadInputTokens)
	}
	if second.Text != "echo: turn two" {
		t.Fatalf("second turn's Text = %q, want it to reflect the SECOND prompt, not stale/replayed state", second.Text)
	}
}

func TestSessionErrorTurnReportsAPIErrorStatus(t *testing.T) {
	s := newFakeSession(t)
	res, err := s.Send("please TRIGGER_ERROR now")
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !res.IsError {
		t.Fatal("IsError = false, want true for a turn the CLI reported as an error")
	}
	if res.APIErrorStatus == nil || *res.APIErrorStatus != 529 {
		t.Fatalf("APIErrorStatus = %v, want 529", res.APIErrorStatus)
	}
}

// TestSessionSendAfterProcessDiesReturnsClearError is the automated version
// of the real failure mode this project's whole worker/hook architecture
// exists because of: a subprocess that ends mid-conversation must surface
// as a clear error from Send, not a hang or a silent zero-value Result.
func TestSessionSendAfterProcessDiesReturnsClearError(t *testing.T) {
	s := newFakeSession(t, "FAKE_CLI_EXIT_AFTER=1")

	if _, err := s.Send("first turn, answered normally"); err != nil {
		t.Fatalf("Send (first, should succeed): %v", err)
	}

	_, err := s.Send("second turn, the fake CLI exits instead of answering")
	if err == nil {
		t.Fatal("Send after the subprocess died: want an error, got nil")
	}
}

func TestSessionCloseWaitsForCleanExit(t *testing.T) {
	s := newFakeSession(t)
	if _, err := s.Send("hello"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v, want nil (the fake CLI exits cleanly once stdin closes)", err)
	}
}
