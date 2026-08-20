package claudeagent

import (
	"strings"
	"testing"
)

// This payload shape is not guessed — it matches a real PostToolUse
// invocation captured while validating this package against actual Claude
// Code hooks (see claude-mem-go's project history).
const realPostToolUsePayload = `{
	"session_id": "86108f04-c501-4c56-b332-0ab2ba2a067d",
	"transcript_path": "/Users/x/.claude/projects/-Users-x-proj/86108f04.jsonl",
	"cwd": "/Users/x/proj",
	"hook_event_name": "PostToolUse",
	"tool_name": "Bash",
	"tool_input": {"command": "echo hi"},
	"tool_response": {"stdout": "hi\n", "stderr": ""}
}`

func TestParseHookInputRealShape(t *testing.T) {
	in, err := ParseHookInput(strings.NewReader(realPostToolUsePayload))
	if err != nil {
		t.Fatalf("ParseHookInput: %v", err)
	}
	if in.SessionID != "86108f04-c501-4c56-b332-0ab2ba2a067d" {
		t.Errorf("SessionID = %q", in.SessionID)
	}
	if in.Event != HookEventPostToolUse {
		t.Errorf("Event = %q, want %q", in.Event, HookEventPostToolUse)
	}
	if in.ToolName != "Bash" {
		t.Errorf("ToolName = %q, want Bash", in.ToolName)
	}
	if string(in.ToolInput) != `{"command": "echo hi"}` {
		t.Errorf("ToolInput = %s, want the raw JSON preserved verbatim (no reshaping)", in.ToolInput)
	}
	if !strings.Contains(string(in.ToolResponse), "hi\\n") {
		t.Errorf("ToolResponse = %s, want raw JSON containing the stdout", in.ToolResponse)
	}
}

func TestParseHookInputMalformed(t *testing.T) {
	if _, err := ParseHookInput(strings.NewReader("not json")); err == nil {
		t.Fatal("ParseHookInput on invalid JSON: want an error, got nil")
	}
}

func TestParseHookInputSessionStartHasNoToolFields(t *testing.T) {
	in, err := ParseHookInput(strings.NewReader(`{"session_id":"s1","hook_event_name":"SessionStart"}`))
	if err != nil {
		t.Fatalf("ParseHookInput: %v", err)
	}
	if in.ToolName != "" {
		t.Errorf("ToolName = %q, want empty for a SessionStart payload", in.ToolName)
	}
	if in.Event != HookEventSessionStart {
		t.Errorf("Event = %q, want %q", in.Event, HookEventSessionStart)
	}
}
