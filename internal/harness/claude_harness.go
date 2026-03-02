package harness

// claude_harness.go — Claude Code CLI harness.
//
// Ported from thegent src/thegent/agents/direct_agents.py (DirectAgentRunner,
// agent_name=="claude" branch of _build_cmd).
//
// The Claude CLI is invoked as:
//
//	claude --print [--dangerously-skip-permissions] [--add-dir <cwd>]
//	       --output-format stream-json --verbose [--model <model>] < <prompt>
//
// Prompt is delivered via stdin (uses_stdin=true in the Python implementation).

import (
	"context"
	"os/exec"
)

const claudeAgentName = "claude"

// ClaudeHarness invokes the `claude` CLI (Claude Code) as a subprocess.
type ClaudeHarness struct {
	base *baseRunner
}

// NewClaudeHarness constructs a ClaudeHarness.
// cliPath is the path to the `claude` binary; leave empty to resolve via PATH.
func NewClaudeHarness(cliPath string) *ClaudeHarness {
	if cliPath == "" {
		if p, err := exec.LookPath("claude"); err == nil {
			cliPath = p
		} else {
			cliPath = "claude"
		}
	}
	return &ClaudeHarness{
		base: &baseRunner{
			agentName: claudeAgentName,
			cliPath:   cliPath,
			usesStdin: true,
		},
	}
}

// AgentName implements Runner.
func (h *ClaudeHarness) AgentName() string { return claudeAgentName }

// Run implements Runner.
func (h *ClaudeHarness) Run(ctx context.Context, opts RunOptions) (RunResult, error) {
	cmd := h.buildCmd(opts)
	return h.base.execute(ctx, opts, cmd, opts.Prompt)
}

// buildCmd assembles the claude CLI argument list (without the prompt; that is
// written to stdin by baseRunner.execute).
func (h *ClaudeHarness) buildCmd(opts RunOptions) []string {
	cmd := []string{h.base.cliPath, "--print"}

	if opts.Mode != ModeReadOnly {
		cmd = append(cmd, "--dangerously-skip-permissions")
	}
	if opts.WorkDir != "" {
		cmd = append(cmd, "--add-dir", opts.WorkDir)
	}
	// Always request stream-json for structured output.
	cmd = append(cmd, "--output-format", "stream-json", "--verbose")
	if opts.Model != "" {
		cmd = append(cmd, "--model", opts.Model)
	}
	return cmd
}
