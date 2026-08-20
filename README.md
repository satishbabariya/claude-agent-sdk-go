# claude-agent-sdk-go

A from-scratch Go client for the protocol `@anthropic-ai/claude-agent-sdk`'s
`query()` speaks: spawn the `claude` CLI with
`--input-format stream-json --output-format stream-json` and drive it as a
persistent, multi-turn subprocess.

This is not a wrapper or a port of the npm package's code — that package is
closed-source (shipped as a minified `sdk.mjs`) under Anthropic's Commercial
Terms of Service. What's implemented here is the wire protocol instead,
which is Claude Code's own documented headless/programmatic mode. That it's
"just" a subprocess wrapper isn't a guess: it's confirmed by the npm
package's public `.d.ts` types (`Options.pathToClaudeCodeExecutable`,
`Options.spawnClaudeCodeProcess`) and by exercising the protocol directly —
persistent multi-turn sessions (same `session_id` across turns, rising
`cache_read_input_tokens`), real error surfaces (a bad model name really
does come back as `api_error_status: 404`), and a genuine PostToolUse hook
flow, all against the actual `claude` CLI, not a mock of it.

## Install

```sh
go get claude-agent-sdk-go
```

(Requires the `claude` CLI on `PATH`; nothing else.)

## Usage

```go
sess, err := claudeagent.NewSession(ctx, claudeagent.Options{
    Model:          "haiku",
    SystemPrompt:   "You are an Observer. You do not have access to tools.",
    PermissionMode: claudeagent.PermissionModeDontAsk,
    Tools:          []string{}, // disables every built-in tool
    SettingSources: []claudeagent.SettingSource{}, // no inherited hooks/skills/plugins
})
defer sess.Close()

r1, _ := sess.Send("Reply with exactly: ONE")
r2, _ := sess.Send("Reply with exactly: TWO")
// r1.SessionID == r2.SessionID — same conversation, real context accumulation
```

See `examples/observer` for a runnable version of the above.

## Scope

Covers the one thing claude-mem's real `ClaudeProvider.ts` actually imports
from the npm SDK: `query()`. Deliberately **not** included, because these
are policy/application decisions rather than protocol facts (see
[claude-mem-go](../claude-mem-go), which builds them on top of this):

- Error classification (retryable vs. fatal) — `Result` exposes the raw
  `IsError`/`APIErrorStatus`/`Subtype` fields for a caller's own classifier.
- Concurrency limits across multiple `Session`s.
- Transcript-file parsing.
- Subagents, MCP server management, permission-prompt callbacks, session
  fork/resume/rename — real capabilities of the npm SDK, not exercised by
  claude-mem's actual usage, so not built here yet.

## Testing

```sh
go test ./...
```

`hooks_test.go` covers `ParseHookInput` against a real captured
`PostToolUse` payload shape (pure JSON decoding, no subprocess needed).
`Session`/`Query` — the actual subprocess/protocol layer — don't have unit
tests yet: every claim above about them was verified by hand against the
real `claude` CLI instead (see `examples/observer`, a runnable proof, not a
mock). A package this thin is arguably better served by integration-style
checks against the real binary than by mocking the subprocess, but a
real (not mocked) integration test — spawn `claude`, assert on `Result` —
gated behind a build tag so `go test ./...` stays fast and hook-free by
default, is the next thing worth adding here.
