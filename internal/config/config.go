// Package config provides configuration management for AgentAPI using phenotype-go-kit.
package config

import (
	"os"

	"github.com/spf13/viper"
)

// ServerConfig represents the server configuration for AgentAPI.
type ServerConfig struct {
	Port           int      `mapstructure:"port"`
	Host           string   `mapstructure:"host"`
	ChatBasePath   string   `mapstructure:"chat_base_path"`
	AllowedHosts   []string `mapstructure:"allowed_hosts"`
	AllowedOrigins []string `mapstructure:"allowed_origins"`
	TermWidth      uint16   `mapstructure:"term_width"`
	TermHeight     uint16   `mapstructure:"term_height"`
	PrintOpenAPI   bool     `mapstructure:"print_openapi"`
}

// AgentAPIConfig represents the complete configuration for AgentAPI.
type AgentAPIConfig struct {
	Server ServerConfig `mapstructure:"server"`
	Agent  AgentConfig  `mapstructure:"agent"`
}

// AgentConfig represents agent-related configuration.
type AgentConfig struct {
	Type           string `mapstructure:"type"`
	InitialPrompt  string `mapstructure:"initial_prompt"`
}

// LoadConfig loads the configuration from a file and environment variables.
func LoadConfig(filePath string) (*AgentAPIConfig, error) {
	defaults := map[string]any{
		"server.port":             3284,
		"server.host":             "localhost",
		"server.chat_base_path":   "/chat",
		"server.allowed_hosts":    []string{"localhost", "127.0.0.1", "[::1]"},
		"server.allowed_origins":  []string{"http://localhost:3284", "http://localhost:3000", "http://localhost:3001"},
		"server.term_width":       uint16(80),
		"server.term_height":      uint16(1000),
		"server.print_openapi":    false,
		"agent.type":              "",
		"agent.initial_prompt":    "",
	}

	viper.SetEnvPrefix("AGENTAPI")
	viper.AutomaticEnv()
	for key, value := range defaults {
		viper.SetDefault(key, value)
	}

	if filePath != "" {
		viper.SetConfigFile(filePath)
		if _, statErr := os.Stat(filePath); statErr == nil {
			if err := viper.ReadInConfig(); err != nil {
				return nil, err
			}
		}
	}

	// Unmarshal into config struct
	var cfg AgentAPIConfig
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// LoadConfigWithEnv loads configuration from environment variables and a config file.
// Environment variables take precedence over config file values.
func LoadConfigWithEnv(filePath string) (*AgentAPIConfig, error) {
	// First load from file
	cfg, err := LoadConfig(filePath)
	if err != nil && filePath != "" {
		return nil, err
	}

	// Then override with environment variables
	viper.SetEnvPrefix("AGENTAPI")
	viper.AutomaticEnv()

	// Re-unmarshal with environment overrides
	if err := viper.Unmarshal(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// BindEnvVars binds specific environment variables to configuration keys.
func BindEnvVars() error {
	envBindings := map[string]string{
		"server.port":             "AGENTAPI_PORT",
		"server.host":             "AGENTAPI_HOST",
		"server.chat_base_path":   "AGENTAPI_CHAT_BASE_PATH",
		"server.allowed_hosts":    "AGENTAPI_ALLOWED_HOSTS",
		"server.allowed_origins":  "AGENTAPI_ALLOWED_ORIGINS",
		"server.term_width":       "AGENTAPI_TERM_WIDTH",
		"server.term_height":      "AGENTAPI_TERM_HEIGHT",
		"agent.type":              "AGENTAPI_AGENT_TYPE",
		"agent.initial_prompt":    "AGENTAPI_INITIAL_PROMPT",
	}

	for key, envVar := range envBindings {
		if err := viper.BindEnv(key, envVar); err != nil {
			return err
		}
	}

	return nil
}
