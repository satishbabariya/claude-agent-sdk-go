package claudeagent

// PermissionMode maps to claude's --permission-mode. PermissionModeDontAsk
// is what claude-mem's hardened Observer/KnowledgeAgent sessions use: "deny
// unless pre-approved," with nothing pre-approved (see Options.Tools).
type PermissionMode string

const (
	PermissionModeDefault     PermissionMode = ""
	PermissionModeDontAsk     PermissionMode = "dontAsk"
	PermissionModeAcceptEdits PermissionMode = "acceptEdits"
	PermissionModeBypassAll   PermissionMode = "bypassPermissions"
	PermissionModePlan        PermissionMode = "plan"
	PermissionModeManual      PermissionMode = "manual"
	PermissionModeAuto        PermissionMode = "auto"
)

// SettingSource maps to one entry of --setting-sources. An empty non-nil
// Options.SettingSources loads none of them — the mechanism that isolates an
// observer session from the caller's own hooks/skills/plugins, which
// matters a lot for a session whose whole point is running with no tool
// access: without it, the caller's own PostToolUse hook could fire on
// nothing (no tools = no tool calls), but its SessionStart/UserPromptSubmit
// hooks and skill-injection would still leak in and pollute the prompt.
type SettingSource string

const (
	SettingSourceUser    SettingSource = "user"
	SettingSourceProject SettingSource = "project"
	SettingSourceLocal   SettingSource = "local"
)

// Options configures one Session's spawned claude subprocess. Field names
// and semantics are narrowed from the npm package's Options type (sdk.d.ts)
// to the subset claude-mem's ClaudeProvider.ts and hardened-options.ts
// actually set — see the package doc for what's deliberately absent.
type Options struct {
	// ClaudeExecutable overrides which `claude` binary to spawn. Empty
	// resolves via FindClaudeExecutable().
	ClaudeExecutable string

	Model string

	// SystemPrompt REPLACES the CLI's default system prompt (--system-prompt).
	// There is no "append" variant yet (the npm SDK's appendSystemPrompt) —
	// not something claude-mem's Observer/KnowledgeAgent paths use, since
	// they want a from-scratch identity ("you do not have access to tools"),
	// not the default coding-agent prompt plus an addendum.
	SystemPrompt string

	// Cwd defaults to the OBSERVER_SESSIONS_DIR-equivalent the caller
	// chooses. Never silently falls back to the calling process's own cwd —
	// hardened-options.ts calls this out explicitly as a jail boundary.
	Cwd string

	// Env, when non-nil, REPLACES the subprocess environment entirely — the
	// npm SDK's own documented behavior for this field, not a Go-specific
	// choice. A caller that wants to add one variable to its own environment
	// must build off os.Environ() itself; this package won't merge for you,
	// so an accidental partial-env spawn (missing PATH/HOME) can't happen by
	// omission.
	Env []string

	PermissionMode PermissionMode

	// Tools, when non-nil, maps to --tools; a non-nil empty slice disables
	// every built-in tool (claude-mem's Observer "belt" layer — see
	// hardened-options.ts's threat-model comment: this is the ONE option
	// that actually disables all built-ins, not just avoids
	// auto-approving them; DisallowedTools alone is not sufficient).
	// Nil leaves the CLI's default tool set untouched.
	Tools []string

	// DisallowedTools is an explicit per-tool deny list — hardened-options.ts's
	// "suspenders" layer, redundant with an empty Tools by design (removing
	// either one must not reopen the gap).
	DisallowedTools []string

	// SettingSources maps to --setting-sources. Nil leaves the CLI's default
	// (load everything) untouched; see SettingSource's doc for why an
	// observer-style session usually wants this set to an empty slice.
	SettingSources []SettingSource

	// StrictMCPConfig maps to --strict-mcp-config: only use MCP servers
	// passed via an (currently unsupported) MCP config, ignoring all other
	// MCP configuration sources. Combined with passing none, this is how a
	// session gets zero MCP servers instead of whatever the caller's normal
	// environment has configured.
	StrictMCPConfig bool
}
