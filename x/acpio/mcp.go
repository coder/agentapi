package acpio

import (
	"github.com/coder/acp-go-sdk"
	"golang.org/x/xerrors"
)

// AgentapiMcpConfig represents the Claude MCP JSON format where mcpServers is a map
// with server names as keys.
type AgentapiMcpConfig struct {
	McpServers map[string]AgentapiMcpServer `json:"mcpServers"`
}

// AgentapiMcpServer represents a single MCP server in Claude's format.
type AgentapiMcpServer struct {
	// Type can be "stdio" or "http". Defaults to "stdio" if not specified.
	Type string `json:"type,omitempty"`
	// Stdio transport fields
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	// HTTP transport fields
	URL     string            `json:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

// convertAgentapiMcpToAcp converts a Claude MCP server config to the ACP format.
func (a *AgentapiMcpServer) convertAgentapiMcpToAcp(name string) (acp.McpServer, error) {
	serverType := a.Type
	if serverType == "" {
		// Default to stdio if no type specified and command is present
		if a.Command != "" {
			serverType = "stdio"
		} else if a.URL != "" {
			serverType = "http"
		}
	}

	switch serverType {
	case "stdio", "":
		if a.Command == "" {
			return acp.McpServer{}, xerrors.Errorf("stdio server %q missing command", name)
		}
		// Convert env map to []EnvVariable
		var envVars []acp.EnvVariable
		for key, value := range a.Env {
			envVars = append(envVars, acp.EnvVariable{
				Name:  key,
				Value: value,
			})
		}
		return acp.McpServer{
			Stdio: &acp.McpServerStdio{
				Name:    name,
				Command: a.Command,
				Args:    a.Args,
				Env:     envVars,
			},
		}, nil

	case "http":
		if a.URL == "" {
			return acp.McpServer{}, xerrors.Errorf("http server %q missing url", name)
		}
		// Convert headers map to []HttpHeader
		var headers []acp.HttpHeader
		for key, value := range a.Headers {
			headers = append(headers, acp.HttpHeader{
				Name:  key,
				Value: value,
			})
		}
		return acp.McpServer{
			Http: &acp.McpServerHttp{
				Name:    name,
				Type:    "http",
				Url:     a.URL,
				Headers: headers,
			},
		}, nil

	default:
		return acp.McpServer{}, xerrors.Errorf("unsupported server type %q for server %q", serverType, name)
	}
}
