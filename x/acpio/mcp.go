package acpio

import (
	"slices"

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
	// Type can be "stdio", "sse" or "http"
	Type string `json:"type"`
	// Stdio transport fields
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	// HTTP | SSE transport fields
	URL     string            `json:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

// convertAgentapiMcpToAcp converts a Claude MCP server config to the ACP format.
func (a *AgentapiMcpServer) convertAgentapiMcpToAcp(name string) (acp.McpServer, error) {
	serverType := a.Type
	acpMCPServer := acp.McpServer{}

	if serverType == "stdio" {
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

		acpMCPServer.Stdio = &acp.McpServerStdio{
			Name:    name,
			Command: a.Command,
			Args:    a.Args,
			Env:     envVars,
		}
	} else if slices.Contains([]string{"http", "sse"}, serverType) {
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

		if serverType == "sse" {
			acpMCPServer.Sse = &acp.McpServerSse{
				Name:    name,
				Type:    "sse",
				Url:     a.URL,
				Headers: headers,
			}
		} else {
			acpMCPServer.Http = &acp.McpServerHttp{
				Name:    name,
				Type:    "http",
				Url:     a.URL,
				Headers: headers,
			}
		}
	} else {
		return acp.McpServer{}, xerrors.Errorf("unsupported server type %q for server %q", serverType, name)
	}
	return acpMCPServer, nil
}
