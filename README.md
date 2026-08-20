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

`session_test.go` covers the actual subprocess/protocol layer —
`NewSession`/`Send` — via a real subprocess, not a mock: `TestMain`
re-execs the test binary itself as a stand-in `claude` CLI (the same
`TestHelperProcess` technique the standard library's `os/exec` tests use),
driven through `Options.ClaudeExecutable`/`Env`. What's under test is the
real code path — process spawn, stdin encoding, stdout scanning, result-
line routing — against a process that speaks the real wire shape
(`stdinMessage` in, `resultLine` out), not Session's own methods mocked
out. Covers a single-turn round trip, multiple turns sharing one
`session_id` with `cache_read_input_tokens` rising (the automated version
of the manual signal this project's own history cites as evidence of real
context accumulation), API error status propagation, a subprocess dying
mid-conversation surfacing a clear `Send` error instead of a hang, and
clean `Close()`. This runs in CI (see `examples/observer`'s separate,
still-manual verification note below for what remains outside `go test`).

What's still verified by hand, not by `go test ./...`: the real `claude`
CLI's actual behavior (argument acceptance, real model responses,
multi-turn context genuinely carrying real tokens) — see
`examples/observer`, a runnable proof against the real binary, not a
mock. The fake-CLI tests above prove this package's OWN code is correct
against the documented wire protocol; they can't prove the real CLI still
speaks that protocol the same way in some future version. A real (not
mocked) integration test — spawn `claude` for real, assert on `Result` —
gated behind a build tag so `go test ./...` stays fast and CLI-free by
default, is the next thing worth adding here.
