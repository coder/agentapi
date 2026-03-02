package harness

// generic_harness.go — harnesses for cursor-agent, copilot, gemini, and opencode.
//
// Ported from thegent src/thegent/agents/direct_agents.py (DirectAgentRunner,
// _build_cmd branches for cursor-agent, copilot, gemini, opencode).
//
// Unlike Claude and Codex, these agents receive the prompt as a CLI argument
// rather than via stdin.

import (
	"context"
	"os/exec"
)

// ---- cursor-agent -------------------------------------------------------

const cursorAgentName = "cursor-agent"

// CursorAgentHarness invokes the `cursor-agent` (or `cursor`) CLI.
type CursorAgentHarness struct {
	base *baseRunner
}

// NewCursorAgentHarness constructs a CursorAgentHarness.
func NewCursorAgentHarness(cliPath string) *CursorAgentHarness {
	if cliPath == "" {
		for _, name := range []string{"cursor-agent", "cursor"} {
			if p, err := exec.LookPath(name); err == nil {
				cliPath = p
				break
			}
		}
		if cliPath == "" {
			cliPath = "cursor-agent"
		}
	}
	return &CursorAgentHarness{
		base: &baseRunner{
			agentName: cursorAgentName,
			cliPath:   cliPath,
			usesStdin: false,
		},
	}
}

// AgentName implements Runner.
func (h *CursorAgentHarness) AgentName() string { return cursorAgentName }

// Run implements Runner.
func (h *CursorAgentHarness) Run(ctx context.Context, opts RunOptions) (RunResult, error) {
	cmd := h.buildCmd(opts)
	cmd = append(cmd, opts.Prompt)
	return h.base.execute(ctx, opts, cmd, "")
}

func (h *CursorAgentHarness) buildCmd(opts RunOptions) []string {
	cmd := []string{h.base.cliPath, "--print"}
	if opts.Mode != ModeReadOnly {
		cmd = append(cmd, "--trust")
	}
	if opts.WorkDir != "" {
		cmd = append(cmd, "--workspace", opts.WorkDir)
	}
	if opts.Model != "" {
		cmd = append(cmd, "--model", opts.Model)
	}
	return cmd
}

// ---- copilot ------------------------------------------------------------

const copilotAgentName = "copilot"

// CopilotHarness invokes the `copilot` CLI.
type CopilotHarness struct {
	base *baseRunner
}

// NewCopilotHarness constructs a CopilotHarness.
func NewCopilotHarness(cliPath string) *CopilotHarness {
	if cliPath == "" {
		if p, err := exec.LookPath("copilot"); err == nil {
			cliPath = p
		} else {
			cliPath = "copilot"
		}
	}
	return &CopilotHarness{
		base: &baseRunner{
			agentName: copilotAgentName,
			cliPath:   cliPath,
			usesStdin: false,
		},
	}
}

// AgentName implements Runner.
func (h *CopilotHarness) AgentName() string { return copilotAgentName }

// Run implements Runner.
func (h *CopilotHarness) Run(ctx context.Context, opts RunOptions) (RunResult, error) {
	cmd := h.buildCmd(opts)
	cmd = append(cmd, "-p", opts.Prompt)
	return h.base.execute(ctx, opts, cmd, "")
}

func (h *CopilotHarness) buildCmd(opts RunOptions) []string {
	cmd := []string{h.base.cliPath}
	if opts.WorkDir != "" {
		cmd = append(cmd, "--add-dir", opts.WorkDir)
	}
	if opts.Mode != ModeReadOnly {
		cmd = append(cmd, "--allow-all-tools")
	}
	cmd = append(cmd, "--stream", "on")
	if opts.Model != "" {
		cmd = append(cmd, "--model", opts.Model)
	}
	return cmd
}

// ---- gemini -------------------------------------------------------------

const geminiAgentName = "gemini"

// GeminiHarness invokes the `gemini` CLI.
type GeminiHarness struct {
	base *baseRunner
}

// NewGeminiHarness constructs a GeminiHarness.
func NewGeminiHarness(cliPath string) *GeminiHarness {
	if cliPath == "" {
		if p, err := exec.LookPath("gemini"); err == nil {
			cliPath = p
		} else {
			cliPath = "gemini"
		}
	}
	return &GeminiHarness{
		base: &baseRunner{
			agentName: geminiAgentName,
			cliPath:   cliPath,
			usesStdin: false,
		},
	}
}

// AgentName implements Runner.
func (h *GeminiHarness) AgentName() string { return geminiAgentName }

// Run implements Runner.
func (h *GeminiHarness) Run(ctx context.Context, opts RunOptions) (RunResult, error) {
	cmd := h.buildCmd(opts)
	cmd = append(cmd, "-p", opts.Prompt)
	return h.base.execute(ctx, opts, cmd, "")
}

func (h *GeminiHarness) buildCmd(opts RunOptions) []string {
	cmd := []string{h.base.cliPath}
	if opts.WorkDir != "" {
		cmd = append(cmd, "--include-directories", opts.WorkDir)
	}
	cmd = append(cmd, "-o", "stream-json")
	if opts.Model != "" {
		cmd = append(cmd, "-m", opts.Model)
	}
	return cmd
}

// ---- opencode -----------------------------------------------------------

const opencodeAgentName = "opencode"

// OpencodeHarness invokes the `opencode` CLI.
type OpencodeHarness struct {
	base *baseRunner
}

// NewOpencodeHarness constructs an OpencodeHarness.
func NewOpencodeHarness(cliPath string) *OpencodeHarness {
	if cliPath == "" {
		if p, err := exec.LookPath("opencode"); err == nil {
			cliPath = p
		} else {
			cliPath = "opencode"
		}
	}
	return &OpencodeHarness{
		base: &baseRunner{
			agentName: opencodeAgentName,
			cliPath:   cliPath,
			usesStdin: false,
		},
	}
}

// AgentName implements Runner.
func (h *OpencodeHarness) AgentName() string { return opencodeAgentName }

// Run implements Runner.
func (h *OpencodeHarness) Run(ctx context.Context, opts RunOptions) (RunResult, error) {
	cmd := h.buildCmd(opts)
	cmd = append(cmd, opts.Prompt)
	return h.base.execute(ctx, opts, cmd, "")
}

func (h *OpencodeHarness) buildCmd(opts RunOptions) []string {
	cmd := []string{h.base.cliPath, "run"}
	if opts.Model != "" {
		cmd = append(cmd, "-m", opts.Model)
	}
	return cmd
}
