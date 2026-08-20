// Package claudeagent is a from-scratch Go client for the same protocol
// @anthropic-ai/claude-agent-sdk's query() speaks: it spawns the `claude` CLI
// with --input-format stream-json --output-format stream-json and drives it
// as a persistent, multi-turn subprocess.
//
// This is not a wrapper or a port of the npm package's code — that package
// is closed-source (shipped as a minified sdk.mjs) under Anthropic's
// Commercial Terms of Service, not something to vendor or decompile. What
// this package implements instead is the wire protocol, which is Claude
// Code's own documented headless/programmatic mode (`claude -p
// --output-format stream-json`) — confirmed by reading the npm package's
// public .d.ts files (Options.pathToClaudeCodeExecutable,
// Options.spawnClaudeCodeProcess) and by exercising the protocol directly:
// see the sibling go-observer-spike project, which validated persistent
// multi-turn sessions, error surfaces, real transcript ingestion, SQLite
// persistence, and PostToolUse hook wiring (including discovering that an
// async hook's child process does not outlive its parent `claude` process —
// the reason a real port needs a detached worker, not a hook that does the
// work inline) before this package was extracted from it.
//
// # Scope
//
// This package covers the one thing claude-mem's real ClaudeProvider.ts
// actually uses from the npm SDK: the query() function (it imports nothing
// else — see src/services/worker/ClaudeProvider.ts:27). Everything below is
// deliberately NOT here, because it's a policy/application decision rather
// than a protocol fact, and belongs in the caller (e.g. a claude-mem-go
// project built on top of this package):
//
//   - Error classification (retryable vs. fatal vs. needs-new-auth): Result
//     carries the CLI's raw IsError/APIErrorStatus/Subtype fields.
//     Deciding what's transient vs. unrecoverable is exactly what
//     claude-mem's classifyClaudeError does on top of the SDK, not inside
//     it — so a caller's own classify.go is where that belongs, not here.
//   - Concurrency limits across multiple Sessions (claude-mem-go's Pool /
//     the real waitForSlot()/CLAUDE_MEM_MAX_CONCURRENT_AGENTS): a Session
//     is one subprocess; how many run at once is entirely up to the caller.
//   - Transcript-file parsing (claude-mem's src/shared/transcript-parser.ts):
//     unrelated to the query() protocol — belongs in the caller.
//   - Subagents, MCP server management, permission-prompt callbacks,
//     session fork/resume/rename: real capabilities of the npm SDK (see its
//     sdk.d.ts), but not exercised by claude-mem's actual usage, so not
//     built here yet. The HookInput/HookEvent types are the one piece of
//     that wider surface included, because claude-mem-go's PostToolUse
//     wiring needs them.
package claudeagent
