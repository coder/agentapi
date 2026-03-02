package cast_test

import (
	"bufio"
	"bytes"
	"context"
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
	castTestTimeout        = 30 * time.Second
	castOperationTimeout   = 10 * time.Second
	castHealthCheckTimeout = 10 * time.Second
	castFile               = "testdata/claude.cast"
	scriptFile             = "testdata/claude.txt"
)

type scriptEntry struct {
	Role    string // "user" or "agent"
	Message string
}

func loadScript(t testing.TB, path string) []scriptEntry {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var entries []scriptEntry
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			continue
		}
		role, msg, ok := strings.Cut(line, "\t")
		require.True(t, ok, "malformed script line: %q", line)
		require.Contains(t, []string{"user", "agent"}, role,
			"unexpected role %q in script line: %q", role, line)
		entries = append(entries, scriptEntry{Role: role, Message: msg})
	}
	require.NoError(t, sc.Err())
	return entries
}

func TestE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	script := loadScript(t, scriptFile)
	require.GreaterOrEqual(t, len(script), 4, "claude.txt must have at least 4 entries")
	require.Equal(t, "user", script[0].Role)  // initial prompt (startup)
	require.Equal(t, "agent", script[1].Role) // reply to initial prompt
	require.Equal(t, "user", script[2].Role)  // test message sent via API
	require.Equal(t, "agent", script[3].Role) // reply to test message
	// Note: The API merges the startup sequence (initial prompt + initial reply)
	// into the first agent message, so we expect 3 messages from the API, not 4.
	initialPromptReply := script[1].Message
	userMessage := script[2].Message
	agentReply := script[3].Message

	ctx, cancel := context.WithTimeout(context.Background(), castTestTimeout)
	defer cancel()

	apiClient := setup(ctx, t)

	// Agent should be running while processing the initial greeting.
	statusResp, err := apiClient.GetStatus(ctx)
	require.NoError(t, err)
	require.Equal(t, agentapisdk.StatusRunning, statusResp.Status)

	require.NoError(t, waitAgentAPIStable(ctx, t, apiClient, castOperationTimeout, "initial stable"))

	_, err = apiClient.PostMessage(ctx, agentapisdk.PostMessageParams{
		Content: userMessage,
		Type:    agentapisdk.MessageTypeUser,
	})
	require.NoError(t, err, "failed to send message")

	// Agent should be running while processing the reply.
	statusResp, err = apiClient.GetStatus(ctx)
	require.NoError(t, err)
	require.Equal(t, agentapisdk.StatusRunning, statusResp.Status)

	require.NoError(t, waitAgentAPIStable(ctx, t, apiClient, castOperationTimeout, "post message"))

	msgResp, err := apiClient.GetMessages(ctx)
	require.NoError(t, err, "failed to get messages")
	require.Len(t, msgResp.Messages, 3)
	// First message is agent greeting (contains startup screen + initial prompt reply)
	require.Contains(t, msgResp.Messages[0].Content, initialPromptReply)
	require.Contains(t, msgResp.Messages[1].Content, userMessage)
	require.Contains(t, msgResp.Messages[2].Content, agentReply)
}

func defaultCmdFn(ctx context.Context, t testing.TB, serverPort int, binaryPath, cwd string) (string, []string) {
	// Terminal dimensions must match the cast file (80x1000) for correct ANSI escape sequence positioning.
	return binaryPath, []string{"server", fmt.Sprintf("--port=%d", serverPort), "--term-width=80", "--term-height=1000", "--", "go", "run", filepath.Join(cwd, "cmd", "cast_agent.go"), castFile}
}

func setup(ctx context.Context, t testing.TB) *agentapisdk.Client {
	t.Helper()

	binaryPath := os.Getenv("AGENTAPI_BINARY_PATH")
	if binaryPath == "" {
		cwd, err := os.Getwd()
		require.NoError(t, err, "Failed to get current working directory")
		// We're in e2e/cast, so go up two levels to reach the repo root
		binaryPath = filepath.Join(cwd, "..", "..", "out", "agentapi")
		t.Logf("Building binary at %s", binaryPath)
		buildCmd := exec.CommandContext(ctx, "go", "build", "-o", binaryPath, ".")
		buildCmd.Dir = filepath.Join(cwd, "..", "..")
		t.Logf("run: %s", buildCmd.String())
		require.NoError(t, buildCmd.Run(), "Failed to build binary")
	}

	serverPort, err := getFreePort()
	require.NoError(t, err, "Failed to get free port for server")

	cwd, err := os.Getwd()
	require.NoError(t, err, "Failed to get current working directory")

	bin, args := defaultCmdFn(ctx, t, serverPort, binaryPath, cwd)
	t.Logf("Running command: %s %s", bin, strings.Join(args, " "))
	cmd := exec.CommandContext(ctx, bin, args...)

	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err, "Failed to create stdout pipe")

	stderr, err := cmd.StderrPipe()
	require.NoError(t, err, "Failed to create stderr pipe")

	err = cmd.Start()
	require.NoError(t, err, "Failed to start agentapi server")

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

	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
		wg.Wait()
	})

	serverURL := fmt.Sprintf("http://localhost:%d", serverPort)
	require.NoError(t, waitForServer(ctx, t, serverURL, castHealthCheckTimeout), "Server not ready")
	apiClient, err := agentapisdk.NewClient(serverURL)
	require.NoError(t, err, "Failed to create agentapi SDK client")

	return apiClient
}

func logOutput(t testing.TB, prefix string, r io.Reader) {
	t.Helper()
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		t.Logf("[%s] %s", prefix, scanner.Text())
	}
}

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
	var lastMessage string
	defer func() {
		elapsed := time.Since(start)
		t.Logf("%s: agent API status: %s (elapsed: %s)", msg, currStatus, elapsed.Round(100*time.Millisecond))
		if t.Failed() && lastMessage != "" {
			fmt.Fprintf(os.Stderr, "\n=== Last agent message ===\n%s\n=== End last agent message ===\n", lastMessage)
		}
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
			} else if emc, ok := evt.(agentapisdk.EventMessageUpdate); ok {
				lastMessage = emc.Message
				t.Logf("Got message event: id=%d role=%s len=%d", emc.Id, emc.Role, len(emc.Message))
			}
		case err := <-errs:
			return fmt.Errorf("read events: %w", err)
		}
	}
}

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
