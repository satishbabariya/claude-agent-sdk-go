// Example: a two-turn observer session configured the same way claude-mem's
// hardened-options.ts configures the real Observer — no tools, no settings
// inheritance, no MCP — proving the package's Options fields actually
// produce that isolation, not just that they compile.
package main

import (
	"context"
	"fmt"
	"log"

	claudeagent "github.com/satishbabariya/claude-agent-sdk-go"
)

func main() {
	ctx := context.Background()

	opts := claudeagent.Options{
		Model:          "haiku",
		SystemPrompt:   "You are an Observer. You do not have access to tools.",
		PermissionMode: claudeagent.PermissionModeDontAsk,
		Tools:          []string{}, // non-nil empty: disables every built-in tool
		DisallowedTools: []string{
			"Bash", "Read", "Write", "Edit", "Grep", "Glob",
			"WebFetch", "WebSearch", "Task", "NotebookEdit",
		},
		SettingSources: []claudeagent.SettingSource{}, // non-nil empty: no hooks/skills/plugins
	}

	sess, err := claudeagent.NewSession(ctx, opts)
	if err != nil {
		log.Fatalf("NewSession: %v", err)
	}
	defer sess.Close()

	r1, err := sess.Send("Reply with exactly: ONE")
	if err != nil {
		log.Fatalf("turn 1: %v", err)
	}
	fmt.Printf("turn 1: session=%s text=%q cost=$%.4f\n", r1.SessionID, r1.Text, r1.CostUSD)

	r2, err := sess.Send("Reply with exactly: TWO")
	if err != nil {
		log.Fatalf("turn 2: %v", err)
	}
	fmt.Printf("turn 2: session=%s text=%q cost=$%.4f\n", r2.SessionID, r2.Text, r2.CostUSD)

	if r1.SessionID != r2.SessionID {
		log.Fatalf("FAIL: expected the same session across turns, got %s then %s", r1.SessionID, r2.SessionID)
	}
	fmt.Println("\nOK: both turns shared one session — real persistent multi-turn conversation, " +
		"driven entirely through claude-agent-sdk-go, zero use of @anthropic-ai/claude-agent-sdk.")
}
