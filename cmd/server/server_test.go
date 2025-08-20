package server

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type nullWriter struct{}

func (w *nullWriter) Write(p []byte) (int, error) {
	return len(p), nil
}

// setupCommandOutput configures a cobra command to use a null writer for output capture.
func setupCommandOutput(t *testing.T, cmd *cobra.Command) {
	t.Helper()

	cmd.SetOut(&nullWriter{})
	cmd.SetErr(&nullWriter{})
}

func TestParseAgentType(t *testing.T) {
	tests := []struct {
		firstArg     string
		agentTypeVar string
		want         AgentType
	}{
		{
			firstArg:     "",
			agentTypeVar: "",
			want:         AgentTypeCustom,
		},
		{
			firstArg:     "claude",
			agentTypeVar: "",
			want:         AgentTypeClaude,
		},
		{
			firstArg:     "gemini",
			agentTypeVar: "",
			want:         AgentTypeGemini,
		},
		{
			firstArg:     "cursor-agent",
			agentTypeVar: "",
			want:         AgentTypeCursorAgent,
		},
		{
			firstArg:     "cursor",
			agentTypeVar: "",
			want:         AgentTypeCursor,
		},
		{
			firstArg:     "auggie",
			agentTypeVar: "",
			want:         AgentTypeAuggie,
		},
		{
			firstArg:     "amp",
			agentTypeVar: "",
			want:         AgentTypeAmp,
		},
		{
			firstArg:     "goose",
			agentTypeVar: "",
			want:         AgentTypeGoose,
		},
		{
			firstArg:     "aider",
			agentTypeVar: "",
			want:         AgentTypeAider,
		},
		{
			firstArg:     "whatever",
			agentTypeVar: "",
			want:         AgentTypeCustom,
		},
		{
			firstArg:     "claude",
			agentTypeVar: "goose",
			want:         AgentTypeGoose,
		},
		{
			firstArg:     "goose",
			agentTypeVar: "claude",
			want:         AgentTypeClaude,
		},
		{
			firstArg:     "claude",
			agentTypeVar: "gemini",
			want:         AgentTypeGemini,
		},
		{
			firstArg:     "claude",
			agentTypeVar: "cursor-agent",
			want:         AgentTypeCursorAgent,
		},
		{
			firstArg:     "claude",
			agentTypeVar: "cursor",
			want:         AgentTypeCursor,
		},
		{
			firstArg:     "claude",
			agentTypeVar: "auggie",
			want:         AgentTypeAuggie,
		},
		{
			firstArg:     "aider",
			agentTypeVar: "claude",
			want:         AgentTypeClaude,
		},
		{
			firstArg:     "aider",
			agentTypeVar: "custom",
			want:         AgentTypeCustom,
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("%s-%s-%s", test.firstArg, test.agentTypeVar, test.want), func(t *testing.T) {
			got, err := parseAgentType(test.firstArg, test.agentTypeVar)
			require.NoError(t, err)
			require.Equal(t, test.want, got)
		})
	}

	t.Run("invalid agent type", func(t *testing.T) {
		_, err := parseAgentType("claude", "invalid")
		require.Error(t, err)
	})
}

// Test helper to isolate viper config between tests
func isolateViper(t *testing.T) {
	// Save current state
	oldConfig := viper.AllSettings()

	// Reset viper
	viper.Reset()

	// Clear AGENTAPI_ env vars
	var agentapiEnvs []string
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "AGENTAPI_") {
			parts := strings.SplitN(env, "=", 2)
			agentapiEnvs = append(agentapiEnvs, parts[0])
			if err := os.Unsetenv(parts[0]); err != nil {
				t.Fatalf("Failed to unset env var %s: %v", parts[0], err)
			}
		}
	}

	t.Cleanup(func() {
		// Restore state
		viper.Reset()
		for key, value := range oldConfig {
			viper.Set(key, value)
		}

		// Restore env vars
		for _, key := range agentapiEnvs {
			if val := os.Getenv(key); val != "" {
				if err := os.Setenv(key, val); err != nil {
					t.Fatalf("Failed to set env var %s: %v", key, err)
				}
			}
		}
	})
}

// Test configuration values via ServerCmd execution
func TestServerCmd_AllArgs_Defaults(t *testing.T) {
	tests := []struct {
		name     string
		flag     string
		expected any
		getter   func() any
	}{
		{"type default", FlagType, "", func() any { return viper.GetString(FlagType) }},
		{"port default", FlagPort, 3284, func() any { return viper.GetInt(FlagPort) }},
		{"print-openapi default", FlagPrintOpenAPI, false, func() any { return viper.GetBool(FlagPrintOpenAPI) }},
		{"chat-base-path default", FlagChatBasePath, "/chat", func() any { return viper.GetString(FlagChatBasePath) }},
		{"term-width default", FlagTermWidth, uint16(80), func() any { return viper.GetUint16(FlagTermWidth) }},
		{"term-height default", FlagTermHeight, uint16(1000), func() any { return viper.GetUint16(FlagTermHeight) }},
		{"allowed-hosts default", FlagAllowedHosts, []string{"localhost", "127.0.0.1", "[::1]"}, func() any { return viper.GetStringSlice(FlagAllowedHosts) }},
		{"allowed-origins default", FlagAllowedOrigins, []string{"http://localhost:3284", "http://localhost:3000", "http://localhost:3001"}, func() any { return viper.GetStringSlice(FlagAllowedOrigins) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolateViper(t)
			serverCmd := CreateServerCmd()
			setupCommandOutput(t, serverCmd)
			// Execute with --exit to get defaults
			serverCmd.SetArgs([]string{"--exit", "dummy-command"})
			if err := serverCmd.Execute(); err != nil {
				t.Fatalf("Failed to execute server command: %v", err)
			}

			assert.Equal(t, tt.expected, tt.getter())
		})
	}
}

func TestServerCmd_AllEnvVars(t *testing.T) {
	tests := []struct {
		name     string
		envVar   string
		envValue string
		expected any
		getter   func() any
	}{
		{"AGENTAPI_TYPE", "AGENTAPI_TYPE", "claude", "claude", func() any { return viper.GetString(FlagType) }},
		{"AGENTAPI_PORT", "AGENTAPI_PORT", "8080", 8080, func() any { return viper.GetInt(FlagPort) }},
		{"AGENTAPI_PRINT_OPENAPI", "AGENTAPI_PRINT_OPENAPI", "true", true, func() any { return viper.GetBool(FlagPrintOpenAPI) }},
		{"AGENTAPI_CHAT_BASE_PATH", "AGENTAPI_CHAT_BASE_PATH", "/api", "/api", func() any { return viper.GetString(FlagChatBasePath) }},
		{"AGENTAPI_TERM_WIDTH", "AGENTAPI_TERM_WIDTH", "120", uint16(120), func() any { return viper.GetUint16(FlagTermWidth) }},
		{"AGENTAPI_TERM_HEIGHT", "AGENTAPI_TERM_HEIGHT", "500", uint16(500), func() any { return viper.GetUint16(FlagTermHeight) }},
		{"AGENTAPI_ALLOWED_HOSTS", "AGENTAPI_ALLOWED_HOSTS", "localhost example.com", []string{"localhost", "example.com"}, func() any { return viper.GetStringSlice(FlagAllowedHosts) }},
		{"AGENTAPI_ALLOWED_ORIGINS", "AGENTAPI_ALLOWED_ORIGINS", "https://example.com http://localhost:3000", []string{"https://example.com", "http://localhost:3000"}, func() any { return viper.GetStringSlice(FlagAllowedOrigins) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolateViper(t)
			t.Setenv(tt.envVar, tt.envValue)

			serverCmd := CreateServerCmd()
			setupCommandOutput(t, serverCmd)
			serverCmd.SetArgs([]string{"--exit", "dummy-command"})
			if err := serverCmd.Execute(); err != nil {
				t.Fatalf("Failed to execute server command: %v", err)
			}

			assert.Equal(t, tt.expected, tt.getter())
		})
	}
}

func TestServerCmd_ArgsPrecedenceOverEnv(t *testing.T) {
	tests := []struct {
		name     string
		envVar   string
		envValue string
		args     []string
		expected any
		getter   func() any
	}{
		{
			"type: CLI overrides env",
			"AGENTAPI_TYPE", "goose",
			[]string{"--type", "claude"},
			"claude",
			func() any { return viper.GetString(FlagType) },
		},
		{
			"port: CLI overrides env",
			"AGENTAPI_PORT", "8080",
			[]string{"--port", "9090"},
			9090,
			func() any { return viper.GetInt(FlagPort) },
		},
		{
			"print-openapi: CLI overrides env",
			"AGENTAPI_PRINT_OPENAPI", "false",
			[]string{"--print-openapi"},
			true,
			func() any { return viper.GetBool(FlagPrintOpenAPI) },
		},
		{
			"chat-base-path: CLI overrides env",
			"AGENTAPI_CHAT_BASE_PATH", "/env-path",
			[]string{"--chat-base-path", "/cli-path"},
			"/cli-path",
			func() any { return viper.GetString(FlagChatBasePath) },
		},
		{
			"term-width: CLI overrides env",
			"AGENTAPI_TERM_WIDTH", "100",
			[]string{"--term-width", "150"},
			uint16(150),
			func() any { return viper.GetUint16(FlagTermWidth) },
		},
		{
			"term-height: CLI overrides env",
			"AGENTAPI_TERM_HEIGHT", "500",
			[]string{"--term-height", "600"},
			uint16(600),
			func() any { return viper.GetUint16(FlagTermHeight) },
		},
		{
			"allowed-origins: CLI overrides env",
			"AGENTAPI_ALLOWED_ORIGINS", "https://env-example.com http://localhost:3000",
			[]string{"--allowed-origins", "https://cli-example.com"},
			[]string{"https://cli-example.com"},
			func() any { return viper.GetStringSlice(FlagAllowedOrigins) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolateViper(t)
			t.Setenv(tt.envVar, tt.envValue)

			args := append(tt.args, "--exit", "dummy-command")
			serverCmd := CreateServerCmd()
			setupCommandOutput(t, serverCmd)
			serverCmd.SetArgs(args)
			if err := serverCmd.Execute(); err != nil {
				t.Fatalf("Failed to execute server command: %v", err)
			}

			assert.Equal(t, tt.expected, tt.getter())
		})
	}
}

func TestMixed_ConfigurationScenarios(t *testing.T) {
	t.Run("some env, some cli, some defaults", func(t *testing.T) {
		isolateViper(t)

		// Set some env vars
		t.Setenv("AGENTAPI_TYPE", "goose")
		t.Setenv("AGENTAPI_TERM_WIDTH", "120")

		// Set some CLI args
		serverCmd := CreateServerCmd()
		setupCommandOutput(t, serverCmd)
		serverCmd.SetArgs([]string{"--port", "9999", "--print-openapi", "--exit", "dummy-command"})
		if err := serverCmd.Execute(); err != nil {
			t.Fatalf("Failed to execute server command: %v", err)
		}

		// Verify mixed configuration
		assert.Equal(t, "goose", viper.GetString(FlagType))            // from env
		assert.Equal(t, 9999, viper.GetInt(FlagPort))                  // from CLI
		assert.Equal(t, true, viper.GetBool(FlagPrintOpenAPI))         // from CLI
		assert.Equal(t, "/chat", viper.GetString(FlagChatBasePath))    // default
		assert.Equal(t, uint16(120), viper.GetUint16(FlagTermWidth))   // from env
		assert.Equal(t, uint16(1000), viper.GetUint16(FlagTermHeight)) // default
	})
}

func TestServerCmd_AllowedHosts(t *testing.T) {
	tests := []struct {
		name        string
		env         map[string]string
		args        []string
		expectedErr string
		expected    []string // only checked if expectedErr is empty
	}{
		// Environment variable scenarios (space-separated format)
		{
			name:     "env: single valid host",
			env:      map[string]string{"AGENTAPI_ALLOWED_HOSTS": "localhost"},
			args:     []string{},
			expected: []string{"localhost"},
		},
		{
			name:     "env: multiple valid hosts space-separated",
			env:      map[string]string{"AGENTAPI_ALLOWED_HOSTS": "localhost example.com 192.168.1.1"},
			args:     []string{},
			expected: []string{"localhost", "example.com", "192.168.1.1"},
		},
		{
			name:     "env: host with tab",
			env:      map[string]string{"AGENTAPI_ALLOWED_HOSTS": "localhost\texample.com"},
			args:     []string{},
			expected: []string{"localhost", "example.com"},
		},
		// CLI flag scenarios (comma-separated format)
		{
			name:     "flag: single valid host",
			args:     []string{"--allowed-hosts", "localhost"},
			expected: []string{"localhost"},
		},
		{
			name:     "flag: multiple valid hosts comma-separated",
			args:     []string{"--allowed-hosts", "localhost,example.com,192.168.1.1"},
			expected: []string{"localhost", "example.com", "192.168.1.1"},
		},
		{
			name:     "flag: multiple valid hosts with multiple flags",
			args:     []string{"--allowed-hosts", "localhost", "--allowed-hosts", "example.com"},
			expected: []string{"localhost", "example.com"},
		},
		{
			name:     "flag: host with newline",
			args:     []string{"--allowed-hosts", "localhost\n"},
			expected: []string{"localhost"},
		},
		{
			name:     "flag: ipv6 bracketed literal",
			args:     []string{"--allowed-hosts", "[2001:db8::1]"},
			expected: []string{"[2001:db8::1]"},
		},

		// Mixed scenarios (env + flag precedence)
		{
			name:     "mixed: flag overrides env",
			env:      map[string]string{"AGENTAPI_ALLOWED_HOSTS": "localhost"},
			args:     []string{"--allowed-hosts", "override.com"},
			expected: []string{"override.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolateViper(t)

			// Set environment variables if provided
			for key, value := range tt.env {
				t.Setenv(key, value)
			}

			serverCmd := CreateServerCmd()
			setupCommandOutput(t, serverCmd)
			serverCmd.SetArgs(append(tt.args, "--exit", "dummy-command"))
			err := serverCmd.Execute()

			if tt.expectedErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, viper.GetStringSlice(FlagAllowedHosts))
			}
		})
	}
}

func TestServerCmd_AllowedOrigins(t *testing.T) {
	tests := []struct {
		name        string
		env         map[string]string
		args        []string
		expectedErr string
		expected    []string // only checked if expectedErr is empty
	}{
		// Environment variable scenarios (space-separated format)
		{
			name:     "env: single valid origin",
			env:      map[string]string{"AGENTAPI_ALLOWED_ORIGINS": "https://example.com"},
			args:     []string{},
			expected: []string{"https://example.com"},
		},
		{
			name:     "env: multiple valid origins space-separated",
			env:      map[string]string{"AGENTAPI_ALLOWED_ORIGINS": "https://example.com http://localhost:3000 https://app.example.com"},
			args:     []string{},
			expected: []string{"https://example.com", "http://localhost:3000", "https://app.example.com"},
		},
		{
			name:     "env: wildcard origin",
			env:      map[string]string{"AGENTAPI_ALLOWED_ORIGINS": "*"},
			args:     []string{},
			expected: []string{"*"},
		},
		{
			name:     "env: origin with tab",
			env:      map[string]string{"AGENTAPI_ALLOWED_ORIGINS": "https://example.com\thttp://localhost:3000"},
			args:     []string{},
			expected: []string{"https://example.com", "http://localhost:3000"},
		},

		// CLI flag scenarios (comma-separated format)
		{
			name:     "flag: single valid origin",
			args:     []string{"--allowed-origins", "https://example.com"},
			expected: []string{"https://example.com"},
		},
		{
			name:     "flag: multiple valid origins comma-separated",
			args:     []string{"--allowed-origins", "https://example.com,http://localhost:3000,https://app.example.com"},
			expected: []string{"https://example.com", "http://localhost:3000", "https://app.example.com"},
		},
		{
			name:     "flag: multiple valid origins with multiple flags",
			args:     []string{"--allowed-origins", "https://example.com", "--allowed-origins", "http://localhost:3000"},
			expected: []string{"https://example.com", "http://localhost:3000"},
		},
		{
			name:     "flag: wildcard origin",
			args:     []string{"--allowed-origins", "*"},
			expected: []string{"*"},
		},
		{
			name:     "flag: origin with newline",
			args:     []string{"--allowed-origins", "https://example.com\n"},
			expected: []string{"https://example.com"},
		},

		// Mixed scenarios (env + flag precedence)
		{
			name:     "mixed: flag overrides env",
			env:      map[string]string{"AGENTAPI_ALLOWED_ORIGINS": "https://env-example.com"},
			args:     []string{"--allowed-origins", "https://override.com"},
			expected: []string{"https://override.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolateViper(t)

			// Set environment variables if provided
			for key, value := range tt.env {
				t.Setenv(key, value)
			}

			serverCmd := CreateServerCmd()
			setupCommandOutput(t, serverCmd)
			serverCmd.SetArgs(append(tt.args, "--exit", "dummy-command"))
			err := serverCmd.Execute()

			if tt.expectedErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, viper.GetStringSlice(FlagAllowedOrigins))
			}
		})
	}
}
