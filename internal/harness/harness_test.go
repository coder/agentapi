package harness_test

import (
	"context"
	"testing"
	"time"

	"github.com/coder/agentapi/internal/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- parse helpers ----------------------------------------------------------

func TestParseTokens_InOutPattern(t *testing.T) {
	t.Parallel()
	p, c := harness.ParseTokens("Tokens: 1234 input, 567 output")
	assert.Equal(t, 1234, p)
	assert.Equal(t, 567, c)
}

func TestParseTokens_EqualsPattern(t *testing.T) {
	t.Parallel()
	p, c := harness.ParseTokens("input_tokens=800, output_tokens=200")
	assert.Equal(t, 800, p)
	assert.Equal(t, 200, c)
}

func TestParseTokens_LabelPattern(t *testing.T) {
	t.Parallel()
	p, c := harness.ParseTokens("Prompt tokens: 500, Completion tokens: 100")
	assert.Equal(t, 500, p)
	assert.Equal(t, 100, c)
}

func TestParseTokens_SumPattern(t *testing.T) {
	t.Parallel()
	p, c := harness.ParseTokens("Usage: 400 + 150 = 550 total")
	assert.Equal(t, 400, p)
	assert.Equal(t, 150, c)
}

func TestParseTokens_NoMatch(t *testing.T) {
	t.Parallel()
	p, c := harness.ParseTokens("no token info here")
	assert.Equal(t, 0, p)
	assert.Equal(t, 0, c)
}

func TestParseCost_EqualsPattern(t *testing.T) {
	t.Parallel()
	cost := harness.ParseCost("cost_usd=0.0123")
	assert.InDelta(t, 0.0123, cost, 1e-9)
}

func TestParseCost_LabelPattern(t *testing.T) {
	t.Parallel()
	cost := harness.ParseCost("Total cost: $0.05")
	assert.InDelta(t, 0.05, cost, 1e-9)
}

func TestParseCost_NoMatch(t *testing.T) {
	t.Parallel()
	assert.InDelta(t, 0.0, harness.ParseCost("nothing here"), 1e-9)
}

// ---- RunHarness (integration-style with echo) --------------------------------

func TestRunHarness_UnknownAgent(t *testing.T) {
	t.Parallel()
	reg := harness.NewHarnessRegistry()
	result := harness.RunHarness(context.Background(), reg, harness.RunHarnessOptions{
		AgentID: "nonexistent",
		Prompt:  "hello",
		TaskID:  "t1",
	})
	assert.False(t, result.Success)
	assert.Contains(t, result.Err, "unknown agent")
	assert.Equal(t, "t1", result.TaskID)
}

// echoRunner is a test double that returns a fixed RunResult.
type echoRunner struct {
	name   string
	result harness.RunResult
	err    error
}

func (e *echoRunner) AgentName() string { return e.name }
func (e *echoRunner) Run(_ context.Context, _ harness.RunOptions) (harness.RunResult, error) {
	return e.result, e.err
}

func TestRunHarness_SuccessPath(t *testing.T) {
	t.Parallel()
	reg := harness.NewHarnessRegistry()
	reg.Register("mock", &echoRunner{
		name: "mock",
		result: harness.RunResult{
			ExitCode: 0,
			Stdout:   "hello world",
			Stderr:   "",
			TimedOut: false,
			Duration: time.Second,
		},
	})

	result := harness.RunHarness(context.Background(), reg, harness.RunHarnessOptions{
		AgentID: "mock",
		Prompt:  "say hello",
		TaskID:  "t2",
	})

	require.True(t, result.Success)
	assert.Equal(t, "hello world", result.Output)
	assert.Empty(t, result.Err)
	assert.Equal(t, "t2", result.TaskID)
}

func TestRunHarness_FailurePath(t *testing.T) {
	t.Parallel()
	reg := harness.NewHarnessRegistry()
	reg.Register("mock", &echoRunner{
		name: "mock",
		result: harness.RunResult{
			ExitCode: 1,
			Stdout:   "",
			Stderr:   "something went wrong",
			TimedOut: false,
		},
	})

	result := harness.RunHarness(context.Background(), reg, harness.RunHarnessOptions{
		AgentID: "mock",
		Prompt:  "fail please",
	})

	assert.False(t, result.Success)
	assert.Equal(t, "something went wrong", result.Err)
}

func TestRunHarness_AgentAliases(t *testing.T) {
	t.Parallel()
	reg := harness.NewHarnessRegistry()
	reg.Register("opencode", &echoRunner{name: "opencode", result: harness.RunResult{ExitCode: 0, Stdout: "ok"}})

	// "droid" is an alias for "opencode"
	result := harness.RunHarness(context.Background(), reg, harness.RunHarnessOptions{
		AgentID: "droid",
		Prompt:  "do something",
	})
	assert.True(t, result.Success)
}

// ---- harness construction smoke-tests ---------------------------------------

func TestNewClaudeHarness_AgentName(t *testing.T) {
	t.Parallel()
	h := harness.NewClaudeHarness("/nonexistent/claude")
	assert.Equal(t, "claude", h.AgentName())
}

func TestNewCodexHarness_AgentName(t *testing.T) {
	t.Parallel()
	h := harness.NewCodexHarness("/nonexistent/codex")
	assert.Equal(t, "codex", h.AgentName())
}

func TestNewCursorAgentHarness_AgentName(t *testing.T) {
	t.Parallel()
	h := harness.NewCursorAgentHarness("/nonexistent/cursor-agent")
	assert.Equal(t, "cursor-agent", h.AgentName())
}

func TestNewCopilotHarness_AgentName(t *testing.T) {
	t.Parallel()
	h := harness.NewCopilotHarness("/nonexistent/copilot")
	assert.Equal(t, "copilot", h.AgentName())
}

func TestNewGeminiHarness_AgentName(t *testing.T) {
	t.Parallel()
	h := harness.NewGeminiHarness("/nonexistent/gemini")
	assert.Equal(t, "gemini", h.AgentName())
}

func TestNewOpencodeHarness_AgentName(t *testing.T) {
	t.Parallel()
	h := harness.NewOpencodeHarness("/nonexistent/opencode")
	assert.Equal(t, "opencode", h.AgentName())
}
