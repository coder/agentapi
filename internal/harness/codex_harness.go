package harness

// codex_harness.go — OpenAI Codex CLI harness.
//
// Ported from thegent src/thegent/agents/direct_agents.py (DirectAgentRunner,
// agent_name=="codex" branch of _build_cmd).
//
// The Codex CLI is invoked as:
//
//	codex exec - --skip-git-repo-check [--cd <cwd>] --json [--model <model>]
//	             [--sandbox workspace-write | --full-auto] < <prompt>
//
// Prompt is delivered via stdin (uses_stdin=true in the Python implementation).

import (
	"context"
	"os/exec"
)

const codexAgentName = "codex"

// CodexHarness invokes the `codex` CLI as a subprocess.
type CodexHarness struct {
	base *baseRunner
}

// NewCodexHarness constructs a CodexHarness.
// cliPath is the path to the `codex` binary; leave empty to resolve via PATH.
func NewCodexHarness(cliPath string) *CodexHarness {
	if cliPath == "" {
		if p, err := exec.LookPath("codex"); err == nil {
			cliPath = p
		} else {
			cliPath = "codex"
		}
	}
	return &CodexHarness{
		base: &baseRunner{
			agentName: codexAgentName,
			cliPath:   cliPath,
			usesStdin: true,
		},
	}
}

// AgentName implements Runner.
func (h *CodexHarness) AgentName() string { return codexAgentName }

// Run implements Runner.
func (h *CodexHarness) Run(ctx context.Context, opts RunOptions) (RunResult, error) {
	cmd := h.buildCmd(opts)
	return h.base.execute(ctx, opts, cmd, opts.Prompt)
}

// buildCmd assembles the codex CLI argument list.
func (h *CodexHarness) buildCmd(opts RunOptions) []string {
	cmd := []string{h.base.cliPath, "exec", "-", "--skip-git-repo-check"}

	if opts.WorkDir != "" {
		cmd = append(cmd, "--cd", opts.WorkDir)
	}
	// Always emit JSON output for structured parsing.
	cmd = append(cmd, "--json")
	if opts.Model != "" {
		cmd = append(cmd, "--model", opts.Model)
	}
	switch opts.Mode {
	case ModeWrite:
		cmd = append(cmd, "--sandbox", "workspace-write")
	case ModeFull:
		cmd = append(cmd, "--full-auto")
	}
	return cmd
}
