package main_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	agentapisdk "github.com/coder/agentapi-sdk-go"
	"github.com/stretchr/testify/require"
)

const (
	testTimeout        = 30 * time.Second
	operationTimeout   = 10 * time.Second
	healthCheckTimeout = 10 * time.Second
)

type ScriptEntry struct {
	ExpectMessage   string `json:"expectMessage"`
	ThinkDurationMS int64  `json:"thinkDurationMS"`
	ResponseMessage string `json:"responseMessage"`
}

func TestE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Run("basic", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()
		script, apiClient, cleanup := setup(ctx, t, nil, true)
		defer cleanup()
		messageReq := agentapisdk.PostMessageParams{
			Content: "This is a test message.",
			Type:    agentapisdk.MessageTypeUser,
		}
		_, err := apiClient.PostMessage(ctx, messageReq)
		require.NoError(t, err, "Failed to send message via SDK")
		require.NoError(t, waitAgentAPIStable(ctx, t, apiClient, operationTimeout, "post message"))
		msgResp, err := apiClient.GetMessages(ctx)
		require.NoError(t, err, "Failed to get messages via SDK")
		require.Len(t, msgResp.Messages, 3)
		require.Equal(t, script[0].ResponseMessage, strings.TrimSpace(msgResp.Messages[0].Content))
		require.Equal(t, script[1].ExpectMessage, strings.TrimSpace(msgResp.Messages[1].Content))
		require.Equal(t, script[1].ResponseMessage, strings.TrimSpace(msgResp.Messages[2].Content))
	})

	t.Run("thinking", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		script, apiClient, cleanup := setup(ctx, t, nil, true)
		defer cleanup()
		messageReq := agentapisdk.PostMessageParams{
			Content: "What is the answer to life, the universe, and everything?",
			Type:    agentapisdk.MessageTypeUser,
		}
		_, err := apiClient.PostMessage(ctx, messageReq)
		require.NoError(t, err, "Failed to send message via SDK")
		statusResp, err := apiClient.GetStatus(ctx)
		require.NoError(t, err)
		require.Equal(t, agentapisdk.StatusRunning, statusResp.Status)
		require.NoError(t, waitAgentAPIStable(ctx, t, apiClient, 5*time.Second, "post message"))
		msgResp, err := apiClient.GetMessages(ctx)
		require.NoError(t, err, "Failed to get messages via SDK")
		require.Len(t, msgResp.Messages, 3)
		require.Equal(t, script[0].ResponseMessage, strings.TrimSpace(msgResp.Messages[0].Content))
		require.Equal(t, script[1].ExpectMessage, strings.TrimSpace(msgResp.Messages[1].Content))
		parts := strings.Split(msgResp.Messages[2].Content, "\n")
		require.Len(t, parts, 2)
		require.Equal(t, script[1].ResponseMessage, strings.TrimSpace(parts[0]))
		require.Equal(t, script[2].ResponseMessage, strings.TrimSpace(parts[1]))
	})

	t.Run("stdin", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		script, apiClient, cleanup := setup(ctx, t, &params{
			cmdFn: func(ctx context.Context, t testing.TB, serverPort int, binaryPath, cwd, scriptFilePath string) (string, []string) {
				defCmd, defArgs := defaultCmdFn(ctx, t, serverPort, binaryPath, cwd, scriptFilePath)
				script := fmt.Sprintf(`echo "hello agent" | %s %s`, defCmd, strings.Join(defArgs, " "))
				return "/bin/sh", []string{"-c", script}
			},
		}, true)
		defer cleanup()
		require.NoError(t, waitAgentAPIStable(ctx, t, apiClient, 5*time.Second, "stdin"))
		msgResp, err := apiClient.GetMessages(ctx)
		require.NoError(t, err, "Failed to get messages via SDK")
		require.Len(t, msgResp.Messages, 3)
		require.Equal(t, script[0].ExpectMessage, strings.TrimSpace(msgResp.Messages[1].Content))
		require.Equal(t, script[0].ResponseMessage, strings.TrimSpace(msgResp.Messages[2].Content))
	})

	t.Run("state_persistence", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		// Create a temporary state file
		stateFile := filepath.Join(t.TempDir(), "state.json")
		scriptFilePath := filepath.Join("testdata", "state_persistence.json")

		// Step 1: Start server with state persistence enabled and send first message
		script, apiClient, cleanup := setup(ctx, t, &params{
			stateFile:      stateFile,
			scriptFilePath: scriptFilePath,
		}, true)

		// Send first message
		messageReq := agentapisdk.PostMessageParams{
			Content: "First message before state save.",
			Type:    agentapisdk.MessageTypeUser,
		}
		_, err := apiClient.PostMessage(ctx, messageReq)
		require.NoError(t, err, "Failed to send first message")
		require.NoError(t, waitAgentAPIStable(ctx, t, apiClient, operationTimeout, "first message"))

		// Verify messages before shutdown
		msgResp, err := apiClient.GetMessages(ctx)
		require.NoError(t, err, "Failed to get messages before shutdown")
		require.Len(t, msgResp.Messages, 3, "Expected 3 messages before shutdown")
		require.Equal(t, script[0].ResponseMessage, strings.TrimSpace(msgResp.Messages[0].Content))
		require.Equal(t, script[1].ExpectMessage, strings.TrimSpace(msgResp.Messages[1].Content))
		require.Equal(t, script[1].ResponseMessage, strings.TrimSpace(msgResp.Messages[2].Content))

		// Step 2: Stop server (triggers state save)
		cleanup()

		// Verify state file was created
		require.FileExists(t, stateFile, "State file should exist after shutdown")

		// Step 3: Start new server instance and load state
		// Note: We don't wait for stable here because the echo agent will try to replay
		// from the beginning, which conflicts with restored state. We just verify the
		// state was loaded and messages are present.
		_, apiClient2, cleanup2 := setup(ctx, t, &params{
			stateFile:      stateFile,
			scriptFilePath: scriptFilePath,
		}, false)
		defer cleanup2()

		// Step 4: Wait for state to be restored by retrying until we get expected messages
		msgResp2, err := waitForMessagesWithCount(ctx, t, apiClient2, 3, operationTimeout, "state restore")
		require.NoError(t, err, "Failed to get messages after state restore")
		require.Len(t, msgResp2.Messages, 3, "Expected 3 messages after state restore")

		// Verify all messages match the state before shutdown
		require.Equal(t, script[0].ResponseMessage, strings.TrimSpace(msgResp2.Messages[0].Content))
		require.Equal(t, script[1].ExpectMessage, strings.TrimSpace(msgResp2.Messages[1].Content))
		require.Equal(t, script[1].ResponseMessage, strings.TrimSpace(msgResp2.Messages[2].Content))
	})

	t.Run("state_persistence_initial_prompt", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		// Create a temporary state file
		stateFile := filepath.Join(t.TempDir(), "state.json")
		scriptFilePath := filepath.Join("testdata", "state_persistence_initial_prompt.json")

		// Step 1: Start server with initial prompt
		initialPrompt1 := "Test initial prompt"
		_, apiClient, cleanup := setup(ctx, t, &params{
			stateFile:      stateFile,
			scriptFilePath: scriptFilePath,
			initialPrompt:  initialPrompt1,
		}, true)

		// Verify initial prompt was sent (should have 3 messages: agent greeting + initial prompt + response)
		msgResp, err := apiClient.GetMessages(ctx)
		require.NoError(t, err, "Failed to get messages after initial prompt")
		require.Len(t, msgResp.Messages, 3, "Expected 3 messages after initial prompt")
		require.Equal(t, "Hello! I'm ready to help you.", strings.TrimSpace(msgResp.Messages[0].Content))
		require.Equal(t, initialPrompt1, strings.TrimSpace(msgResp.Messages[1].Content))
		require.Equal(t, "Echo: Test initial prompt", strings.TrimSpace(msgResp.Messages[2].Content))

		// Step 2: Close server
		cleanup()
		require.FileExists(t, stateFile, "State file should exist after shutdown")

		// Step 3: Restart WITHOUT an initial prompt
		_, apiClient2, cleanup2 := setup(ctx, t, &params{
			stateFile:      stateFile,
			scriptFilePath: scriptFilePath,
		}, false)
		defer cleanup2()

		// Step 4: Wait for state to be restored and verify initial prompt was NOT sent again
		msgResp2, err := waitForMessagesWithCount(ctx, t, apiClient2, 3, operationTimeout, "restart without initial prompt")
		require.NoError(t, err, "Failed to get messages after restart without initial prompt")
		require.Len(t, msgResp2.Messages, 3, "Expected 3 messages (initial prompt should not be sent again)")
		require.Equal(t, initialPrompt1, strings.TrimSpace(msgResp2.Messages[1].Content))

		// Step 5: Close server
		cleanup2()

		// Step 6: Restart with same initial prompt
		_, apiClient3, cleanup3 := setup(ctx, t, &params{
			stateFile:      stateFile,
			scriptFilePath: scriptFilePath,
			initialPrompt:  initialPrompt1,
		}, false)
		defer cleanup3()

		// Step 7: Wait for state to be restored and verify same initial prompt was NOT sent again
		msgResp3, err := waitForMessagesWithCount(ctx, t, apiClient3, 3, operationTimeout, "restart with same initial prompt")
		require.NoError(t, err, "Failed to get messages after restart with same initial prompt")
		require.Len(t, msgResp3.Messages, 3, "Expected 3 messages (same initial prompt should not be sent again)")

	})

	t.Run("state_persistence_different_initial_prompt", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		// Create a temporary state file
		stateFile := filepath.Join(t.TempDir(), "state.json")

		// Step 1: Start server with initial prompt "Test initial prompt" using phase1 script
		initialPrompt1 := "Test initial prompt"
		_, apiClient, cleanup := setup(ctx, t, &params{
			stateFile:      stateFile,
			scriptFilePath: filepath.Join("testdata", "state_persistence_different_initial_prompt_phase1.json"),
			initialPrompt:  initialPrompt1,
		}, true)

		// Verify initial prompt was sent (3 messages: greeting + prompt + response)
		msgResp, err := apiClient.GetMessages(ctx)
		require.NoError(t, err, "Failed to get messages after initial prompt")
		require.Len(t, msgResp.Messages, 3, "Expected 3 messages after initial prompt")
		require.Equal(t, "Hello! I'm ready to help you.", strings.TrimSpace(msgResp.Messages[0].Content))
		require.Equal(t, initialPrompt1, strings.TrimSpace(msgResp.Messages[1].Content))
		require.Equal(t, "Echo: Test initial prompt", strings.TrimSpace(msgResp.Messages[2].Content))

		// Step 2: Close server
		cleanup()
		require.FileExists(t, stateFile, "State file should exist after shutdown")

		// Step 3: Restart with DIFFERENT initial prompt using a different script
		initialPrompt2 := "Different initial prompt"
		_, apiClient2, cleanup2 := setup(ctx, t, &params{
			stateFile:      stateFile,
			scriptFilePath: filepath.Join("testdata", "state_persistence_different_initial_prompt.json"),
			initialPrompt:  initialPrompt2,
		}, false)
		defer cleanup2()

		// Wait for initial prompt to be processed and state to stabilize
		require.NoError(t, waitAgentAPIStable(ctx, t, apiClient2, operationTimeout, "after different initial prompt"))

		// Step 4: Verify new initial prompt WAS sent (5 messages: 3 previous + 2 new)
		msgResp2, err := waitForMessagesWithCount(ctx, t, apiClient2, 5, operationTimeout, "different initial prompt processed")
		require.NoError(t, err, "Failed to get messages after different initial prompt")
		require.Len(t, msgResp2.Messages, 5, "Expected 5 messages after different initial prompt (3 previous + 2 new)")
		// Verify the new initial prompt and response were added
		require.Equal(t, initialPrompt2, strings.TrimSpace(msgResp2.Messages[3].Content))
		require.Equal(t, "Echo: Different initial prompt", strings.TrimSpace(msgResp2.Messages[4].Content))

	})

	t.Run("acp_basic", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		script, apiClient := setup(ctx, t, &params{
			cmdFn: func(ctx context.Context, t testing.TB, serverPort int, binaryPath, cwd, scriptFilePath string) (string, []string) {
				return binaryPath, []string{
					"server",
					fmt.Sprintf("--port=%d", serverPort),
					"--experimental-acp",
					"--", "go", "run", filepath.Join(cwd, "acp_echo.go"), scriptFilePath,
				}
			},
		})
		messageReq := agentapisdk.PostMessageParams{
			Content: "This is a test message.",
			Type:    agentapisdk.MessageTypeUser,
		}
		_, err := apiClient.PostMessage(ctx, messageReq)
		require.NoError(t, err, "Failed to send message via SDK")
		require.NoError(t, waitAgentAPIStable(ctx, t, apiClient, operationTimeout, "post message"))
		msgResp, err := apiClient.GetMessages(ctx)
		require.NoError(t, err, "Failed to get messages via SDK")
		require.Len(t, msgResp.Messages, 2)
		require.Equal(t, script[0].ExpectMessage, strings.TrimSpace(msgResp.Messages[0].Content))
		require.Equal(t, script[0].ResponseMessage, strings.TrimSpace(msgResp.Messages[1].Content))
	})
}

type params struct {
	cmdFn          func(ctx context.Context, t testing.TB, serverPort int, binaryPath, cwd, scriptFilePath string) (string, []string)
	stateFile      string
	scriptFilePath string
	initialPrompt  string
}

func defaultCmdFn(ctx context.Context, t testing.TB, serverPort int, binaryPath, cwd, scriptFilePath string) (string, []string) {
	return binaryPath, []string{"server", fmt.Sprintf("--port=%d", serverPort), "--", "go", "run", filepath.Join(cwd, "echo.go"), scriptFilePath}
}

func stateCmdFn(stateFile, initialPrompt string) func(ctx context.Context, t testing.TB, serverPort int, binaryPath, cwd, scriptFilePath string) (string, []string) {
	return func(ctx context.Context, t testing.TB, serverPort int, binaryPath, cwd, scriptFilePath string) (string, []string) {
		args := []string{
			"server",
			fmt.Sprintf("--port=%d", serverPort),
			fmt.Sprintf("--state-file=%s", stateFile),
		}
		if initialPrompt != "" {
			args = append(args, fmt.Sprintf("--initial-prompt=%s", initialPrompt))
		}
		args = append(args, "--", "go", "run", filepath.Join(cwd, "echo.go"), scriptFilePath)
		return binaryPath, args
	}
}

func setup(ctx context.Context, t testing.TB, p *params, waitForStable bool) ([]ScriptEntry, *agentapisdk.Client, func()) {
	t.Helper()

	if p == nil {
		p = &params{}
	}
	if p.cmdFn == nil {
		if p.stateFile != "" {
			p.cmdFn = stateCmdFn(p.stateFile, p.initialPrompt)
		} else {
			p.cmdFn = defaultCmdFn
		}
	}

	scriptFilePath := p.scriptFilePath
	if scriptFilePath == "" {
		scriptFilePath = filepath.Join("testdata", filepath.Base(t.Name())+".json")
	}
	data, err := os.ReadFile(scriptFilePath)
	require.NoError(t, err, "Failed to read test script file: %s", scriptFilePath)

	var script []ScriptEntry
	err = json.Unmarshal(data, &script)
	require.NoError(t, err, "Failed to unmarshal script from %s", scriptFilePath)

	binaryPath := os.Getenv("AGENTAPI_BINARY_PATH")
	if binaryPath == "" {
		cwd, err := os.Getwd()
		require.NoError(t, err, "Failed to get current working directory")
		binaryPath = filepath.Join(cwd, "..", "out", "agentapi")
		t.Logf("Building binary at %s", binaryPath)
		buildCmd := exec.CommandContext(ctx, "go", "build", "-o", binaryPath, ".")
		buildCmd.Dir = filepath.Join(cwd, "..")
		t.Logf("run: %s", buildCmd.String())
		require.NoError(t, buildCmd.Run(), "Failed to build binary")
	}

	serverPort, err := getFreePort()
	require.NoError(t, err, "Failed to get free port for server")

	cwd, err := os.Getwd()
	require.NoError(t, err, "Failed to get current working directory")

	bin, args := p.cmdFn(ctx, t, serverPort, binaryPath, cwd, scriptFilePath)
	t.Logf("Running command: %s %s", bin, strings.Join(args, " "))
	cmd := exec.CommandContext(ctx, bin, args...)

	// Capture output for debugging
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err, "Failed to create stdout pipe")

	stderr, err := cmd.StderrPipe()
	require.NoError(t, err, "Failed to create stderr pipe")

	// Start process
	err = cmd.Start()
	require.NoError(t, err, "Failed to start agentapi server")

	// Log output in background
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		logOutput(t, "SERVER-STDOUT", stdout)
	}()

	go func() {
		defer wg.Done()
		logOutput(t, "SERVER-STDERR", stderr)
	}()

	// Create cleanup function
	cleanup := func() {
		if cmd.Process != nil {
			// Send SIGINT to allow graceful shutdown and state save
			_ = cmd.Process.Signal(os.Interrupt)
			// Wait for process to exit gracefully (with timeout)
			done := make(chan error, 1)
			go func() {
				done <- cmd.Wait()
			}()
			select {
			case <-done:
				// Process exited gracefully
			case <-time.After(10 * time.Second):
				// Timeout, force kill
				_ = cmd.Process.Kill()
				<-done
			}
		}
		wg.Wait()
	}

	serverURL := fmt.Sprintf("http://localhost:%d", serverPort)
	require.NoError(t, waitForServer(ctx, t, serverURL, healthCheckTimeout), "Server not ready")
	apiClient, err := agentapisdk.NewClient(serverURL)
	require.NoError(t, err, "Failed to create agentapi SDK client")

	if waitForStable {
		require.NoError(t, waitAgentAPIStable(ctx, t, apiClient, operationTimeout, "setup"))
	}
	return script, apiClient, cleanup
}

// logOutput logs process output with prefix
func logOutput(t testing.TB, prefix string, r io.Reader) {
	t.Helper()
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		t.Logf("[%s] %s", prefix, scanner.Text())
	}
}

// waitForServer waits for a server to be ready
func waitForServer(ctx context.Context, t testing.TB, url string, timeout time.Duration) error {
	t.Helper()
	client := &http.Client{Timeout: time.Second}
	healthCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-healthCtx.Done():
			require.Failf(t, "failed to start server", "server at %s not ready within timeout: %w", url, healthCtx.Err())
		case <-ticker.C:
			resp, err := client.Get(url)
			if err == nil {
				_ = resp.Body.Close()
				return nil
			}
			t.Logf("Server not ready yet: %s", err)
		}
	}
}

func waitAgentAPIStable(ctx context.Context, t testing.TB, apiClient *agentapisdk.Client, waitFor time.Duration, msg string) error {
	t.Helper()
	waitCtx, waitCancel := context.WithTimeout(ctx, waitFor)
	defer waitCancel()

	start := time.Now()
	var currStatus agentapisdk.AgentStatus
	defer func() {
		elapsed := time.Since(start)
		t.Logf("%s: agent API status: %s (elapsed: %s)", msg, currStatus, elapsed.Round(100*time.Millisecond))
	}()
	evts, errs, err := apiClient.SubscribeEvents(ctx)
	require.NoError(t, err, "failed to subscribe to events")
	for {
		select {
		case <-waitCtx.Done():
			return waitCtx.Err()
		case evt := <-evts:
			if esc, ok := evt.(agentapisdk.EventStatusChange); ok {
				currStatus = esc.Status
				if currStatus == agentapisdk.StatusStable {
					return nil
				}
			} else {
				var sb strings.Builder
				if err := json.NewEncoder(&sb).Encode(evt); err != nil {
					t.Logf("Failed to encode event: %v", err)
				}
				t.Logf("Got event: %s", sb.String())
			}
		case err := <-errs:
			return fmt.Errorf("read events: %w", err)
		}
	}
}

// waitForMessagesWithCount retries GetMessages until it returns the expected number of messages or the timeout is reached.
func waitForMessagesWithCount(ctx context.Context, t testing.TB, apiClient *agentapisdk.Client, expectedCount int, timeout time.Duration, msg string) (*agentapisdk.GetMessagesResponse, error) {
	t.Helper()
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	start := time.Now()
	var lastErr error
	var lastCount int

	for {
		select {
		case <-waitCtx.Done():
			if lastErr != nil {
				return nil, fmt.Errorf("%s: failed to get %d messages after %v (last error: %w, last count: %d)",
					msg, expectedCount, time.Since(start).Round(100*time.Millisecond), lastErr, lastCount)
			}
			return nil, fmt.Errorf("%s: timeout waiting for %d messages after %v (last count: %d)",
				msg, expectedCount, time.Since(start).Round(100*time.Millisecond), lastCount)
		case <-ticker.C:
			resp, err := apiClient.GetMessages(ctx)
			if err != nil {
				lastErr = err
				t.Logf("%s: GetMessages failed (will retry): %v", msg, err)
				continue
			}
			lastCount = len(resp.Messages)
			if lastCount == expectedCount {
				elapsed := time.Since(start)
				t.Logf("%s: got expected %d messages (elapsed: %s)", msg, expectedCount, elapsed.Round(100*time.Millisecond))
				return resp, nil
			}
			t.Logf("%s: got %d messages, expecting %d (will retry)", msg, lastCount, expectedCount)
		}
	}
}

// getFreePort returns a free TCP port
func getFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer func() { _ = l.Close() }()

	return l.Addr().(*net.TCPAddr).Port, nil
}
