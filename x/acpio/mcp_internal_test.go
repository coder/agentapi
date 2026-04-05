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
		assert.Contains(t, err.Error(), "Failed to open mcp file")
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		tmpDir := t.TempDir()
		mcpFile := filepath.Join(tmpDir, "invalid.json")
		err := os.WriteFile(mcpFile, []byte("not valid json"), 0o644)
		require.NoError(t, err)

		initResp := &acp.InitializeResponse{}
		_, err = getSupportedMCPConfig(mcpFile, logger, initResp)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Failed to decode mcp file")
	})

	t.Run("stdio servers always included", func(t *testing.T) {
		tmpDir := t.TempDir()
		mcpFile := filepath.Join(tmpDir, "mcp.json")
		mcpContent := `{
			"mcpServers": [
				{
					"name": "test-stdio",
					"command": "/usr/bin/test",
					"args": ["--stdio"],
					"env": []
				}
			]
		}`
		err := os.WriteFile(mcpFile, []byte(mcpContent), 0o644)
		require.NoError(t, err)

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
		assert.Equal(t, "test-stdio", result[0].Stdio.Name)
	})

	t.Run("http servers filtered when capability is false", func(t *testing.T) {
		tmpDir := t.TempDir()
		mcpFile := filepath.Join(tmpDir, "mcp.json")
		mcpContent := `{
			"mcpServers": [
				{
					"type": "http",
					"name": "test-http",
					"url": "https://example.com/mcp",
					"headers": []
				}
			]
		}`
		err := os.WriteFile(mcpFile, []byte(mcpContent), 0o644)
		require.NoError(t, err)

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
		assert.Empty(t, result)
	})

	t.Run("http servers included when capability is true", func(t *testing.T) {
		tmpDir := t.TempDir()
		mcpFile := filepath.Join(tmpDir, "mcp.json")
		mcpContent := `{
			"mcpServers": [
				{
					"type": "http",
					"name": "test-http",
					"url": "https://example.com/mcp",
					"headers": []
				}
			]
		}`
		err := os.WriteFile(mcpFile, []byte(mcpContent), 0o644)
		require.NoError(t, err)

		initResp := &acp.InitializeResponse{
			AgentCapabilities: acp.AgentCapabilities{
				McpCapabilities: acp.McpCapabilities{
					Http: true,
					Sse:  false,
				},
			},
		}
		result, err := getSupportedMCPConfig(mcpFile, logger, initResp)
		require.NoError(t, err)
		assert.Len(t, result, 1)
		assert.NotNil(t, result[0].Http)
		assert.Equal(t, "test-http", result[0].Http.Name)
	})

	t.Run("mixed servers filtered correctly", func(t *testing.T) {
		tmpDir := t.TempDir()
		mcpFile := filepath.Join(tmpDir, "mcp.json")
		mcpContent := `{
			"mcpServers": [
				{
					"name": "stdio-server",
					"command": "/usr/bin/stdio-mcp",
					"args": [],
					"env": []
				},
				{
					"type": "http",
					"name": "http-server",
					"url": "https://example.com/mcp",
					"headers": []
				}
			]
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

	t.Run("empty mcpServers array returns empty slice", func(t *testing.T) {
		tmpDir := t.TempDir()
		mcpFile := filepath.Join(tmpDir, "mcp.json")
		mcpContent := `{"mcpServers": []}`
		err := os.WriteFile(mcpFile, []byte(mcpContent), 0o644)
		require.NoError(t, err)

		initResp := &acp.InitializeResponse{}
		result, err := getSupportedMCPConfig(mcpFile, logger, initResp)
		require.NoError(t, err)
		assert.Empty(t, result)
	})
}
