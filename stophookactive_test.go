package claudeagent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestParseHookInputReadsStopHookActiveFromARealPayload parses a payload
// captured from a live Claude Code Stop hook (a stub hook command dumping
// stdin, the same technique the AgentID and Prompt fields were verified
// with) rather than a hand-written fixture. Only the local filesystem
// paths and machine-specific UUIDs were replaced; every field name, type,
// and presence/absence is exactly as Claude Code emitted it — including
// the fields this struct deliberately does not model, which is itself
// worth keeping visible.
func TestParseHookInputReadsStopHookActiveFromARealPayload(t *testing.T) {
	f, err := os.Open(filepath.Join("testdata", "stop-hook-payload.json"))
	if err != nil {
		t.Fatalf("open captured payload: %v", err)
	}
	defer f.Close()

	in, err := ParseHookInput(f)
	if err != nil {
		t.Fatalf("ParseHookInput on a real payload: %v", err)
	}
	if in.Event != "Stop" {
		t.Fatalf("Event = %q, want Stop", in.Event)
	}
	if in.SessionID == "" {
		t.Fatal("SessionID empty — the real payload definitely has one")
	}
	// The captured turn was not a retry, so false is the correct value.
	// What is being asserted is that the field DECODES, which a missing or
	// misspelled struct tag would silently fail at while still yielding
	// false — so this assertion is paired with the true case below, which
	// is the one that actually catches a wrong tag.
	if in.StopHookActive {
		t.Fatalf("StopHookActive = true, want false for this captured non-retry turn")
	}
}

func TestParseHookInputDecodesStopHookActiveTrue(t *testing.T) {
	in, err := ParseHookInput(strings.NewReader(
		`{"session_id":"s","hook_event_name":"Stop","stop_hook_active":true}`))
	if err != nil {
		t.Fatalf("ParseHookInput: %v", err)
	}
	if !in.StopHookActive {
		t.Fatal("StopHookActive = false for a payload with stop_hook_active:true — " +
			"the json tag is wrong, and every Stop-hook re-entry would look like a fresh turn")
	}
}

func TestParseHookInputStopHookActiveDefaultsFalseWhenAbsent(t *testing.T) {
	in, err := ParseHookInput(strings.NewReader(`{"session_id":"s","hook_event_name":"Stop"}`))
	if err != nil {
		t.Fatalf("ParseHookInput: %v", err)
	}
	if in.StopHookActive {
		t.Fatal("StopHookActive = true when the field is absent — absent must mean 'not a retry'")
	}
}
