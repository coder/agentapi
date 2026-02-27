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
	operationTimeout   = 5 * time.Second
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
		script, apiClient := setup(ctx, t)
		require.NoError(t, waitAgentAPIStable(ctx, apiClient, operationTimeout))
		messageReq := agentapisdk.PostMessageParams{
			Content: "This is a test message.",
			Type:    agentapisdk.MessageTypeUser,
		}
		_, err := apiClient.PostMessage(ctx, messageReq)
		require.NoError(t, err, "Failed to send message via SDK")
		require.NoError(t, waitAgentAPIStable(ctx, apiClient, operationTimeout))
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

		script, apiClient := setup(ctx, t)
		messageReq := agentapisdk.PostMessageParams{
			Content: "What is the answer to life, the universe, and everything?",
			Type:    agentapisdk.MessageTypeUser,
		}
		_, err := apiClient.PostMessage(ctx, messageReq)
		require.NoError(t, err, "Failed to send message via SDK")
		statusResp, err := apiClient.GetStatus(ctx)
		require.NoError(t, err)
		require.Equal(t, agentapisdk.StatusRunning, statusResp.Status)
		require.NoError(t, waitAgentAPIStable(ctx, apiClient, 5*time.Second))
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
}

func setup(ctx context.Context, t testing.TB) ([]ScriptEntry, *agentapisdk.Client) {
	t.Helper()

	scriptFilePath := filepath.Join("testdata", filepath.Base(t.Name())+".json")
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
		_, err = os.Stat(binaryPath)
		if err != nil {
			t.Logf("Building binary at %s", binaryPath)
			buildCmd := exec.CommandContext(ctx, "go", "build", "-o", binaryPath, ".")
			buildCmd.Dir = filepath.Join(cwd, "..")
			t.Logf("run: %s", buildCmd.String())
			require.NoError(t, buildCmd.Run(), "Failed to build binary")
		}
	}

	serverPort, err := getFreePort()
	require.NoError(t, err, "Failed to get free port for server")

	cwd, err := os.Getwd()
	require.NoError(t, err, "Failed to get current working directory")

	cmd := exec.CommandContext(ctx, binaryPath, "server",
		fmt.Sprintf("--port=%d", serverPort),
		"--",
		"go", "run", filepath.Join(cwd, "echo.go"), scriptFilePath)

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

	// Clean up process
	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
		wg.Wait()
	})

	serverURL := fmt.Sprintf("http://localhost:%d", serverPort)
	require.NoError(t, waitForServer(ctx, t, serverURL, healthCheckTimeout), "Server not ready")
	apiClient, err := agentapisdk.NewClient(serverURL)
	require.NoError(t, err, "Failed to create agentapi SDK client")

	require.NoError(t, waitAgentAPIStable(ctx, apiClient, operationTimeout))
	return script, apiClient
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

func waitAgentAPIStable(ctx context.Context, apiClient *agentapisdk.Client, waitFor time.Duration) error {
	waitCtx, waitCancel := context.WithTimeout(ctx, waitFor)
	defer waitCancel()

	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-waitCtx.Done():
			return waitCtx.Err()
		case <-tick.C:
			sr, err := apiClient.GetStatus(ctx)
			if err != nil {
				continue
			}
			if sr.Status == agentapisdk.StatusStable {
				return nil
			}
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
