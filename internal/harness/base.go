// Package harness provides a unified agent execution harness for agentapi-plusplus.
//
// This package is a Go port of the harness abstractions that originated in thegent
// (src/thegent/agents/base.py, direct_agents.py, agents/crew/harness.py).
// It defines the core interfaces and types for running AI agent CLIs (claude, codex,
// gemini, copilot, cursor-agent, opencode) in a subprocess-managed, observable way.
//
// Architecture:
//
//	Runner (interface) — the abstract harness contract
//	  └─ BaseRunner        — shared helpers (token/cost parsing, output filtering)
//	       ├─ ClaudeHarness
//	       ├─ CodexHarness
//	       └─ GenericHarness (cursor-agent, copilot, gemini, opencode)
//
// Orchestration is provided by RunHarness (run_harness.go) which accepts an
// agent identifier, prompt, and context and returns a RunResult.
package harness

import (
	"context"
	"time"
)

// Mode controls the permission scope given to the agent process.
type Mode string

const (
	// ModeReadOnly restricts the agent to read-only operations.
	ModeReadOnly Mode = "read-only"
	// ModeWrite allows the agent to write files in its working directory.
	ModeWrite Mode = "write"
	// ModeFull grants full autonomous permissions (dangerous).
	ModeFull Mode = "full"
)

// RunOptions parameterises a single agent invocation.
type RunOptions struct {
	// Prompt is the task description sent to the agent.
	Prompt string
	// WorkDir is the working directory for the subprocess (optional).
	WorkDir string
	// Mode controls the permission scope.
	Mode Mode
	// Timeout caps the subprocess wall-clock runtime.
	Timeout time.Duration
	// Model overrides the agent's default model selection.
	Model string
	// Env holds additional environment variables merged into the subprocess env.
	Env map[string]string
	// LiveOutput, when true, streams stdout/stderr lines via OnStdout/OnStderr.
	LiveOutput bool
	// OnStdout receives each stdout line when LiveOutput is true.
	OnStdout func(line string)
	// OnStderr receives each stderr line when LiveOutput is true.
	OnStderr func(line string)
}

// RunResult is the outcome of a completed agent invocation.
type RunResult struct {
	// ExitCode is the subprocess exit code (0 = success).
	ExitCode int
	// Stdout is the captured standard output (ANSI stripped).
	Stdout string
	// Stderr is the captured standard error (ANSI and noise stripped).
	Stderr string
	// TimedOut is true when the process was killed due to exceeding Timeout.
	TimedOut bool
	// Duration is the wall-clock time elapsed for the run.
	Duration time.Duration
	// PromptTokens is the number of prompt/input tokens (parsed from output).
	PromptTokens int
	// CompletionTokens is the number of completion/output tokens (parsed from output).
	CompletionTokens int
	// CostUSD is the estimated cost in USD (parsed from output when available).
	CostUSD float64
}

// Runner is the abstract harness contract.  Each agent CLI implements this
// interface so that orchestration code (RunHarness) is agent-agnostic.
type Runner interface {
	// AgentName returns the canonical lowercase name of this runner (e.g. "claude").
	AgentName() string
	// Run executes the agent with the given options and returns the result.
	// Implementations MUST honour opts.Timeout and cancel the subprocess
	// when the context is cancelled.
	Run(ctx context.Context, opts RunOptions) (RunResult, error)
}

// ExecutionResult is the harness-level outcome used by the orchestration layer
// (RunHarness).  It mirrors the Python ExecutionResult dataclass from
// thegent_agents/agents/crew/executor.py.
type ExecutionResult struct {
	// TaskID is an opaque identifier propagated from the caller context.
	TaskID string
	// Success is true when ExitCode == 0 and the run did not time out.
	Success bool
	// Output contains the agent's stdout on success, or empty on failure.
	Output string
	// Err contains the error description on failure, or empty on success.
	Err string
	// Duration is the wall-clock execution time.
	Duration time.Duration
	// TokensUsed is the total token count (prompt + completion).
	TokensUsed int
	// CostUSD is the estimated cost in USD.
	CostUSD float64
}
