package harness

// runner.go — baseRunner: the shared subprocess execution engine.
//
// Ported from thegent src/thegent/agents/direct_agents.py:
//   DirectAgentRunner._run_capture / _run_live / _filter_noisy_stderr
//
// baseRunner is not exported; concrete harnesses embed a pointer to it.

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
)

// noisyStderrPatterns mirrors the _NOISY_STDERR_PATTERNS list in thegent's
// direct_agents.py.  Lines matching any of these are stripped from stderr.
var noisyStderrPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\(node:\d+\) \[DEP0040\].*punycode`),
	regexp.MustCompile(`Session cleanup disabled:`),
	regexp.MustCompile(`Hook registry initialized with \d+ hook entries`),
	regexp.MustCompile(`Error executing tool run_shell_command: Tool "run_shell_command" not found`),
	regexp.MustCompile(`Use \x60node --trace-deprecation`),
	regexp.MustCompile(`^Loaded cached credentials\.$`),
	regexp.MustCompile(`^\[OK\] `),
	regexp.MustCompile(`^\[INFO\] `),
	regexp.MustCompile(`^Total usage est:`),
	regexp.MustCompile(`^Total duration `),
	regexp.MustCompile(`^Total code changes:`),
	regexp.MustCompile(`^Usage by model:`),
	regexp.MustCompile(`^Copilot CLI available`),
	regexp.MustCompile(`^Commit:`),
}

func filterNoisyStderr(text string) string {
	if text == "" {
		return text
	}
	var kept []string
	for _, line := range strings.Split(text, "\n") {
		noisy := false
		for _, re := range noisyStderrPatterns {
			if re.MatchString(line) {
				noisy = true
				break
			}
		}
		if !noisy {
			kept = append(kept, line)
		}
	}
	return strings.TrimRight(strings.Join(kept, "\n"), "\n")
}

// stripANSI removes ANSI escape sequences from s.
var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func stripANSI(s string) string { return ansiRe.ReplaceAllString(s, "") }

// baseRunner holds the shared execution logic used by all concrete harnesses.
type baseRunner struct {
	agentName string
	cliPath   string
	usesStdin bool
}

// execute runs cmd as a subprocess, honouring opts.LiveOutput, opts.Timeout,
// and the context cancellation.  stdinPayload is written to stdin when
// baseRunner.usesStdin is true.
func (b *baseRunner) execute(
	ctx context.Context,
	opts RunOptions,
	cmd []string,
	stdinPayload string,
) (RunResult, error) {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	//nolint:gosec // cmd is constructed internally, not from user input directly
	proc := exec.CommandContext(ctx, cmd[0], cmd[1:]...)

	if opts.WorkDir != "" {
		proc.Dir = opts.WorkDir
	}

	// Merge additional env vars on top of the current process environment.
	if len(opts.Env) > 0 {
		env := os.Environ()
		for k, v := range opts.Env {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
		proc.Env = env
	}

	start := time.Now()

	var result RunResult
	var err error

	if opts.LiveOutput {
		result, err = b.runLive(ctx, proc, stdinPayload, opts.OnStdout, opts.OnStderr)
	} else {
		result, err = b.runCapture(ctx, proc, stdinPayload)
	}

	result.Duration = time.Since(start)

	// Parse token/cost metrics from combined output.
	combined := result.Stdout + "\n" + result.Stderr
	result.PromptTokens, result.CompletionTokens = parseTokens(combined)
	result.CostUSD = parseCost(combined)

	return result, err
}

// runCapture collects stdout/stderr without streaming.
func (b *baseRunner) runCapture(
	ctx context.Context,
	proc *exec.Cmd,
	stdinPayload string,
) (RunResult, error) {
	if b.usesStdin && stdinPayload != "" {
		proc.Stdin = strings.NewReader(stdinPayload)
	}

	var stdout, stderr bytes.Buffer
	proc.Stdout = &stdout
	proc.Stderr = &stderr

	runErr := proc.Run()

	stdoutStr := stripANSI(stdout.String())
	stderrStr := filterNoisyStderr(stripANSI(stderr.String()))

	timedOut := isContextTimeout(ctx, runErr)
	exitCode := exitCodeFrom(proc, runErr, timedOut)

	return RunResult{
		ExitCode: exitCode,
		Stdout:   stdoutStr,
		Stderr:   stderrStr,
		TimedOut: timedOut,
	}, nil
}

// runLive streams stdout/stderr lines to the provided callbacks.
func (b *baseRunner) runLive(
	ctx context.Context,
	proc *exec.Cmd,
	stdinPayload string,
	onStdout func(string),
	onStderr func(string),
) (RunResult, error) {
	if b.usesStdin && stdinPayload != "" {
		proc.Stdin = strings.NewReader(stdinPayload)
	}

	stdoutPipe, err := proc.StdoutPipe()
	if err != nil {
		return RunResult{ExitCode: 1, Stderr: err.Error()}, err
	}
	stderrPipe, err := proc.StderrPipe()
	if err != nil {
		return RunResult{ExitCode: 1, Stderr: err.Error()}, err
	}

	if startErr := proc.Start(); startErr != nil {
		return RunResult{ExitCode: 1, Stderr: startErr.Error()}, startErr
	}

	var (
		outLines []string
		errLines []string
		mu       sync.Mutex
	)

	drain := func(pipe io.Reader, collector *[]string, cb func(string)) {
		scanner := bufio.NewScanner(pipe)
		for scanner.Scan() {
			line := stripANSI(scanner.Text())
			mu.Lock()
			*collector = append(*collector, line)
			mu.Unlock()
			if cb != nil {
				cb(line)
			}
		}
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); drain(stdoutPipe, &outLines, onStdout) }()
	go func() { defer wg.Done(); drain(stderrPipe, &errLines, onStderr) }()
	wg.Wait()

	runErr := proc.Wait()
	timedOut := isContextTimeout(ctx, runErr)
	exitCode := exitCodeFrom(proc, runErr, timedOut)

	return RunResult{
		ExitCode: exitCode,
		Stdout:   strings.Join(outLines, "\n"),
		Stderr:   filterNoisyStderr(strings.Join(errLines, "\n")),
		TimedOut: timedOut,
	}, nil
}

// isContextTimeout returns true when ctx has expired (deadline exceeded or
// cancelled) and the process error is non-nil.
func isContextTimeout(ctx context.Context, procErr error) bool {
	if procErr == nil {
		return false
	}
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

// exitCodeFrom extracts the numeric exit code from the completed process.
func exitCodeFrom(proc *exec.Cmd, procErr error, timedOut bool) int {
	if timedOut {
		return 124
	}
	if procErr == nil {
		return 0
	}
	if proc.ProcessState != nil {
		return proc.ProcessState.ExitCode()
	}
	return 1
}
