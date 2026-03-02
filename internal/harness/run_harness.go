package harness

// run_harness.go — RunHarness: the orchestration entry-point.
//
// Ported from thegent packages/thegent-agents/src/thegent_agents/agents/crew/harness.py
// (create_agent_executor / agent_executor).
//
// RunHarness is the agentapi-plusplus equivalent of the Python
// create_agent_executor() factory.  It:
//
//  1. Maps a human-readable agent identifier to the correct Runner implementation.
//  2. Executes the runner with the caller-supplied prompt and options.
//  3. Returns an ExecutionResult that the server layer (agent_handler.go) can use.

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// agentAliases maps alternative identifiers to canonical names.
// Ported from the agent_name_map dict in thegent's create_agent_executor.
var agentAliases = map[string]string{
	"codex":        "codex",
	"cursor":       "cursor-agent",
	"cursor-agent": "cursor-agent",
	"claude":       "claude",
	"copilot":      "copilot",
	"gemini":       "gemini",
	"droid":        "opencode", // droid is the internal alias for opencode
	"opencode":     "opencode",
}

// RunHarnessOptions configures a single orchestrated agent run.
type RunHarnessOptions struct {
	// AgentID is the agent identifier (e.g. "claude", "codex", "cursor", "droid").
	AgentID string
	// Prompt is the task description.
	Prompt string
	// TaskID is an opaque identifier propagated to ExecutionResult.
	TaskID string
	// WorkDir is the working directory passed to the agent subprocess.
	WorkDir string
	// Mode controls the permission scope (defaults to ModeWrite).
	Mode Mode
	// Timeout caps the subprocess wall-clock runtime (defaults to 5 minutes).
	Timeout time.Duration
	// Model overrides the agent's default model.
	Model string
	// Env holds additional environment variables merged into the subprocess env.
	Env map[string]string
}

// HarnessRegistry is a lightweight registry that maps canonical agent names to
// their Runner instances.  Callers can register custom runners or rely on the
// defaults provided by DefaultRegistry.
type HarnessRegistry struct {
	runners map[string]Runner
}

// NewHarnessRegistry creates an empty registry.
func NewHarnessRegistry() *HarnessRegistry {
	return &HarnessRegistry{runners: make(map[string]Runner)}
}

// Register adds (or replaces) a runner for the given canonical agent name.
func (r *HarnessRegistry) Register(name string, runner Runner) {
	r.runners[strings.ToLower(name)] = runner
}

// Get returns the runner for a canonical agent name.
func (r *HarnessRegistry) Get(name string) (Runner, bool) {
	runner, ok := r.runners[strings.ToLower(name)]
	return runner, ok
}

// DefaultRegistry returns a HarnessRegistry pre-populated with the default
// harnesses for all supported agents (paths resolved via PATH).
func DefaultRegistry() *HarnessRegistry {
	reg := NewHarnessRegistry()
	reg.Register(claudeAgentName, NewClaudeHarness(""))
	reg.Register(codexAgentName, NewCodexHarness(""))
	reg.Register(cursorAgentName, NewCursorAgentHarness(""))
	reg.Register(copilotAgentName, NewCopilotHarness(""))
	reg.Register(geminiAgentName, NewGeminiHarness(""))
	reg.Register(opencodeAgentName, NewOpencodeHarness(""))
	return reg
}

// RunHarness executes the agent identified by opts.AgentID and returns an
// ExecutionResult.  It is the Go equivalent of the Python agent_executor
// closure returned by create_agent_executor.
func RunHarness(ctx context.Context, registry *HarnessRegistry, opts RunHarnessOptions) ExecutionResult {
	// Resolve alias → canonical name.
	canonicalName, ok := agentAliases[strings.ToLower(opts.AgentID)]
	if !ok {
		canonicalName = strings.ToLower(opts.AgentID)
	}

	runner, ok := registry.Get(canonicalName)
	if !ok {
		return ExecutionResult{
			TaskID:  opts.TaskID,
			Success: false,
			Err:     fmt.Sprintf("unknown agent: %q", opts.AgentID),
		}
	}

	mode := opts.Mode
	if mode == "" {
		mode = ModeWrite
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}

	start := time.Now()
	result, err := runner.Run(ctx, RunOptions{
		Prompt:  opts.Prompt,
		WorkDir: opts.WorkDir,
		Mode:    mode,
		Timeout: timeout,
		Model:   opts.Model,
		Env:     opts.Env,
	})
	elapsed := time.Since(start)

	if err != nil {
		return ExecutionResult{
			TaskID:   opts.TaskID,
			Success:  false,
			Err:      err.Error(),
			Duration: elapsed,
		}
	}

	success := result.ExitCode == 0 && !result.TimedOut

	var output, errMsg string
	if success {
		output = result.Stdout
	} else {
		if result.TimedOut {
			errMsg = fmt.Sprintf("agent timed out after %s", timeout)
		} else {
			errMsg = result.Stderr
		}
	}

	return ExecutionResult{
		TaskID:     opts.TaskID,
		Success:    success,
		Output:     output,
		Err:        errMsg,
		Duration:   result.Duration,
		TokensUsed: result.PromptTokens + result.CompletionTokens,
		CostUSD:    result.CostUSD,
	}
}
