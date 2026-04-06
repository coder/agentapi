package acpio

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	acp "github.com/coder/acp-go-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetSupportedMCPConfig(t *testing.T) {
	logger := slog.Default()

	t.Run("empty file path returns empty slice", func(t *testing.T) {
		initResp := &acp.InitializeResponse{}
		result, err := getSupportedMCPConfig("", logger, initResp)
		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("file not found returns error", func(t *testing.T) {
		initResp := &acp.InitializeResponse{}
		_, err := getSupportedMCPConfig("/nonexistent/path/mcp.json", logger, initResp)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to open mcp file")
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		tmpDir := t.TempDir()
		mcpFile := filepath.Join(tmpDir, "invalid.json")
		err := os.WriteFile(mcpFile, []byte("not valid json"), 0o644)
		require.NoError(t, err)

		initResp := &acp.InitializeResponse{}
		_, err = getSupportedMCPConfig(mcpFile, logger, initResp)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to decode mcp file")
	})

	t.Run("empty file returns warning and empty slice", func(t *testing.T) {
		tmpDir := t.TempDir()
		mcpFile := filepath.Join(tmpDir, "empty.json")
		err := os.WriteFile(mcpFile, []byte(""), 0o644)
		require.NoError(t, err)

		initResp := &acp.InitializeResponse{}
		result, err := getSupportedMCPConfig(mcpFile, logger, initResp)
		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("servers filtered correctly", func(t *testing.T) {
		tmpDir := t.TempDir()
		mcpFile := filepath.Join(tmpDir, "mcp.json")
		mcpContent := `{
			"mcpServers": {
				"stdio-server": {
					"type": "stdio",
					"command": "/usr/bin/stdio-mcp",
					"args": []
				},
				"http-server": {
					"type": "http",
					"url": "https://example.com/mcp",
					"headers": {}
				}
			}
		}`
		err := os.WriteFile(mcpFile, []byte(mcpContent), 0o644)
		require.NoError(t, err)

		// With HTTP capability disabled, only stdio should be included
		initResp := &acp.InitializeResponse{
			AgentCapabilities: acp.AgentCapabilities{
				McpCapabilities: acp.McpCapabilities{
					Http: false,
					Sse:  false,
				},
			},
		}
		result, err := getSupportedMCPConfig(mcpFile, logger, initResp)
		require.NoError(t, err)
		assert.Len(t, result, 1)
		assert.NotNil(t, result[0].Stdio)
		assert.Equal(t, "stdio-server", result[0].Stdio.Name)

		// With HTTP capability enabled, both should be included
		initResp.AgentCapabilities.McpCapabilities.Http = true
		result, err = getSupportedMCPConfig(mcpFile, logger, initResp)
		require.NoError(t, err)
		assert.Len(t, result, 2)
	})

	t.Run("empty mcpServers object returns empty slice", func(t *testing.T) {
		tmpDir := t.TempDir()
		mcpFile := filepath.Join(tmpDir, "mcp.json")
		mcpContent := `{"mcpServers": {}}`
		err := os.WriteFile(mcpFile, []byte(mcpContent), 0o644)
		require.NoError(t, err)

		initResp := &acp.InitializeResponse{}
		result, err := getSupportedMCPConfig(mcpFile, logger, initResp)
		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("server without command or url is skipped", func(t *testing.T) {
		tmpDir := t.TempDir()
		mcpFile := filepath.Join(tmpDir, "mcp.json")
		mcpContent := `{
			"mcpServers": {
				"invalid-server": {
					"type": "stdio",
					"args": ["--foo"]
				}
			}
		}`
		err := os.WriteFile(mcpFile, []byte(mcpContent), 0o644)
		require.NoError(t, err)

		initResp := &acp.InitializeResponse{}
		result, err := getSupportedMCPConfig(mcpFile, logger, initResp)
		require.NoError(t, err)
		// Invalid servers are skipped with a warning, not an error
		assert.Empty(t, result)
	})

	t.Run("server without type is skipped", func(t *testing.T) {
		tmpDir := t.TempDir()
		mcpFile := filepath.Join(tmpDir, "mcp.json")
		mcpContent := `{
			"mcpServers": {
				"no-type-server": {
					"command": "/usr/bin/test"
				}
			}
		}`
		err := os.WriteFile(mcpFile, []byte(mcpContent), 0o644)
		require.NoError(t, err)

		initResp := &acp.InitializeResponse{}
		result, err := getSupportedMCPConfig(mcpFile, logger, initResp)
		require.NoError(t, err)
		// Servers without type are skipped with a warning
		assert.Empty(t, result)
	})
}

func TestConvertAgentapiMcpToAcp(t *testing.T) {
	t.Run("converts stdio server correctly", func(t *testing.T) {
		server := AgentapiMcpServer{
			Type:    "stdio",
			Command: "/usr/bin/mcp-server",
			Args:    []string{"--arg1", "--arg2"},
			Env: map[string]string{
				"KEY1": "value1",
				"KEY2": "value2",
			},
		}

		result, err := server.convertAgentapiMcpToAcp("my-server")
		require.NoError(t, err)
		require.NotNil(t, result.Stdio)
		assert.Equal(t, "my-server", result.Stdio.Name)
		assert.Equal(t, "/usr/bin/mcp-server", result.Stdio.Command)
		assert.Equal(t, []string{"--arg1", "--arg2"}, result.Stdio.Args)
		assert.Len(t, result.Stdio.Env, 2)
	})

	t.Run("converts http server correctly", func(t *testing.T) {
		server := AgentapiMcpServer{
			Type: "http",
			URL:  "https://api.example.com/mcp",
			Headers: map[string]string{
				"Authorization": "Bearer token",
				"X-Custom":      "value",
			},
		}

		result, err := server.convertAgentapiMcpToAcp("api-server")
		require.NoError(t, err)
		require.NotNil(t, result.Http)
		assert.Equal(t, "api-server", result.Http.Name)
		assert.Equal(t, "https://api.example.com/mcp", result.Http.Url)
		assert.Len(t, result.Http.Headers, 2)
	})

	t.Run("returns error for stdio without command", func(t *testing.T) {
		server := AgentapiMcpServer{
			Type: "stdio",
			Args: []string{"--arg"},
		}

		_, err := server.convertAgentapiMcpToAcp("bad-server")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing command")
	})

	t.Run("returns error for http without url", func(t *testing.T) {
		server := AgentapiMcpServer{
			Type: "http",
		}

		_, err := server.convertAgentapiMcpToAcp("bad-server")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing url")
	})

	t.Run("returns error for unsupported type", func(t *testing.T) {
		server := AgentapiMcpServer{
			Type: "websocket",
		}

		_, err := server.convertAgentapiMcpToAcp("bad-server")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported server type")
	})

	t.Run("returns error for missing type", func(t *testing.T) {
		server := AgentapiMcpServer{
			Command: "/usr/bin/test",
		}

		_, err := server.convertAgentapiMcpToAcp("no-type-server")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported server type")
	})
}
