package claudeagent

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

// capabilityProbeArgs are the flags every Session spawn always passes.
// Probing with them instead of a bare --version is deliberate: claude-mem's
// find-claude-executable.ts documents CLIs older than the 2.1.x line
// rejecting --permission-mode dontAsk with "argument 'dontAsk' is invalid"
// and exiting 1 before doing any work — a binary that answers --version but
// fails this probe would still die instantly at spawn time.
var capabilityProbeArgs = []string{"--permission-mode", "dontAsk", "--version"}

const probeTimeout = 10 * time.Second

// FindClaudeExecutable resolves a usable `claude` CLI on PATH and probes it
// with the same flags every Session always passes, so an incompatible
// binary is rejected here with a clear error instead of failing obscurely on
// first spawn.
//
// This is a narrowed port of find-claude-executable.ts: the real one also
// scans several install locations (npm-global, the native auto-updating
// installer, desktop-app bundles) and picks the newest of several capable
// candidates, because a user can plausibly have more than one `claude` on
// disk. This version only checks PATH — worth revisiting if claude-mem-go
// needs to run somewhere PATH resolution isn't enough.
func FindClaudeExecutable() (string, error) {
	path, err := exec.LookPath("claude")
	if err != nil {
		return "", fmt.Errorf("claude executable not found on PATH: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()

	if err := exec.CommandContext(ctx, path, capabilityProbeArgs...).Run(); err != nil {
		return "", fmt.Errorf("claude at %s failed the capability probe %v: %w — "+
			"this Claude CLI is too old", path, capabilityProbeArgs, err)
	}
	return path, nil
}
