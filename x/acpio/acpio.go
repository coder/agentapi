package acpio

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"

	acp "github.com/coder/acp-go-sdk"
	st "github.com/coder/agentapi/lib/screentracker"
	"golang.org/x/xerrors"
)

// Compile-time assertion that ACPAgentIO implements st.AgentIO
var _ st.AgentIO = (*ACPAgentIO)(nil)

// ACPAgentIO implements screentracker.AgentIO using the ACP protocol
type ACPAgentIO struct {
	ctx       context.Context
	conn      *acp.ClientSideConnection
	sessionID acp.SessionId
	mu        sync.RWMutex
	response  strings.Builder
	logger    *slog.Logger
	onChunk   func(chunk string) // called on each streaming chunk
}

// acpClient implements acp.Client to handle callbacks from the agent
type acpClient struct {
	agentIO *ACPAgentIO
}

var _ acp.Client = (*acpClient)(nil)

func (c *acpClient) SessionUpdate(ctx context.Context, params acp.SessionNotification) error {
	c.agentIO.logger.Debug("SessionUpdate received",
		"sessionId", params.SessionId,
		"hasAgentMessageChunk", params.Update.AgentMessageChunk != nil)

	if params.Update.AgentMessageChunk != nil {
		if text := params.Update.AgentMessageChunk.Content.Text; text != nil {
			c.agentIO.logger.Debug("AgentMessageChunk text",
				"text", text.Text,
				"textLen", len(text.Text))
			c.agentIO.mu.Lock()
			c.agentIO.response.WriteString(text.Text)
			onChunk := c.agentIO.onChunk
			c.agentIO.mu.Unlock()
			if onChunk != nil {
				onChunk(text.Text)
			}
		}
	}

	// Handle tool calls - format as text and append to response
	if params.Update.ToolCall != nil {
		tc := params.Update.ToolCall
		formatted := fmt.Sprintf("\n[Tool: %s] %s\n", tc.Kind, tc.Title)
		c.agentIO.mu.Lock()
		c.agentIO.response.WriteString(formatted)
		onChunk := c.agentIO.onChunk
		c.agentIO.mu.Unlock()
		if onChunk != nil {
			onChunk(formatted)
		}
	}

	if params.Update.ToolCallUpdate != nil {
		tcu := params.Update.ToolCallUpdate
		var formatted string
		if tcu.Status != nil {
			formatted = fmt.Sprintf("[Tool Status: %s]\n", *tcu.Status)
		}
		if formatted != "" {
			c.agentIO.mu.Lock()
			c.agentIO.response.WriteString(formatted)
			onChunk := c.agentIO.onChunk
			c.agentIO.mu.Unlock()
			if onChunk != nil {
				onChunk(formatted)
			}
		}
	}

	return nil
}

func (c *acpClient) RequestPermission(ctx context.Context, params acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
	// Auto-approve all permissions for Phase 1
	return acp.RequestPermissionResponse{
		Outcome: acp.RequestPermissionOutcome{
			Selected: &acp.RequestPermissionOutcomeSelected{OptionId: "allow"},
		},
	}, nil
}

func (c *acpClient) ReadTextFile(ctx context.Context, params acp.ReadTextFileRequest) (acp.ReadTextFileResponse, error) {
	return acp.ReadTextFileResponse{}, nil
}

func (c *acpClient) WriteTextFile(ctx context.Context, params acp.WriteTextFileRequest) (acp.WriteTextFileResponse, error) {
	return acp.WriteTextFileResponse{}, nil
}

func (c *acpClient) CreateTerminal(ctx context.Context, params acp.CreateTerminalRequest) (acp.CreateTerminalResponse, error) {
	return acp.CreateTerminalResponse{}, nil
}

func (c *acpClient) KillTerminalCommand(ctx context.Context, params acp.KillTerminalCommandRequest) (acp.KillTerminalCommandResponse, error) {
	return acp.KillTerminalCommandResponse{}, nil
}

func (c *acpClient) TerminalOutput(ctx context.Context, params acp.TerminalOutputRequest) (acp.TerminalOutputResponse, error) {
	return acp.TerminalOutputResponse{}, nil
}

func (c *acpClient) ReleaseTerminal(ctx context.Context, params acp.ReleaseTerminalRequest) (acp.ReleaseTerminalResponse, error) {
	return acp.ReleaseTerminalResponse{}, nil
}

func (c *acpClient) WaitForTerminalExit(ctx context.Context, params acp.WaitForTerminalExitRequest) (acp.WaitForTerminalExitResponse, error) {
	return acp.WaitForTerminalExitResponse{}, nil
}

// SetOnChunk sets a callback that will be called for each streaming chunk.
func (a *ACPAgentIO) SetOnChunk(fn func(chunk string)) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.onChunk = fn
}

// NewWithPipes creates an ACPAgentIO connected via the provided pipes
func NewWithPipes(ctx context.Context, toAgent io.Writer, fromAgent io.Reader, logger *slog.Logger, getwd func() (string, error), mcpFilePath string) (*ACPAgentIO, error) {
	if logger == nil {
		logger = slog.Default()
	}
	agentIO := &ACPAgentIO{ctx: ctx, logger: logger}
	client := &acpClient{agentIO: agentIO}

	conn := acp.NewClientSideConnection(client, toAgent, fromAgent)
	agentIO.conn = conn

	logger.Debug("Initializing ACP connection")

	// Initialize the connection
	initResp, err := conn.Initialize(ctx, acp.InitializeRequest{
		ProtocolVersion:    acp.ProtocolVersionNumber,
		ClientCapabilities: acp.ClientCapabilities{},
	})
	if err != nil {
		logger.Error("Failed to initialize ACP connection", "error", err)
		return nil, err
	}
	logger.Debug("ACP initialized", "protocolVersion", initResp.ProtocolVersion)

	// Prepare the MCPs for the session
	supportedMCPList, err := getSupportedMCPConfig(mcpFilePath, logger, &initResp)
	if err != nil {
		return nil, err
	}

	// Create a session
	cwd, err := getwd()
	if err != nil {
		logger.Error("Failed to get working directory", "error", err)
		return nil, err
	}
	sessResp, err := conn.NewSession(ctx, acp.NewSessionRequest{
		Cwd:        cwd,
		McpServers: supportedMCPList,
	})
	if err != nil {
		logger.Error("Failed to create ACP session", "error", err)
		return nil, err
	}
	agentIO.sessionID = sessResp.SessionId
	logger.Debug("ACP session created", "sessionId", sessResp.SessionId)

	return agentIO, nil
}

func getSupportedMCPConfig(mcpFilePath string, logger *slog.Logger, initResp *acp.InitializeResponse) ([]acp.McpServer, error) {
	if mcpFilePath == "" {
		return []acp.McpServer{}, nil
	}

	mcpFile, err := os.Open(mcpFilePath)
	if err != nil {
		return nil, xerrors.Errorf("failed to open mcp file: %w", err)
	}

	defer func() {
		if closeErr := mcpFile.Close(); closeErr != nil {
			logger.Error("Failed to close mcp file", "path", mcpFilePath, "error", closeErr)
		}
	}()

	var claudeConfig AgentapiMcpConfig
	decoder := json.NewDecoder(mcpFile)

	if err = decoder.Decode(&claudeConfig); err != nil {
		return nil, xerrors.Errorf("failed to decode mcp file: %w", err)
	}

	// Convert MCP format to ACP format and filter by agent capabilities
	var supportedMCPList []acp.McpServer
	for name, server := range claudeConfig.McpServers {
		mcpServer, err := server.convertAgentapiMcpToAcp(name)
		if err != nil {
			logger.Warn("Skipping invalid MCP server", "name", name, "error", err)
			continue
		}

		// Filter based on agent capabilities
		if mcpServer.Http != nil && !initResp.AgentCapabilities.McpCapabilities.Http {
			logger.Debug("Skipping HTTP MCP server (agent doesn't support HTTP)", "name", name)
			continue
		}
		if mcpServer.Sse != nil && !initResp.AgentCapabilities.McpCapabilities.Sse {
			logger.Debug("Skipping SSE MCP server (agent doesn't support SSE)", "name", name)
			continue
		}

		supportedMCPList = append(supportedMCPList, mcpServer)
	}
	return supportedMCPList, nil
}

// Write sends a message to the agent via ACP prompt
func (a *ACPAgentIO) Write(data []byte) (int, error) {
	text := string(data)

	// Strip bracketed paste escape sequences if present
	text = strings.TrimPrefix(text, "\x1b[200~")
	text = strings.TrimSuffix(text, "\x1b[201~")

	// Strip terminal hack sequences (x\b pattern used for Claude Code compatibility)
	text = strings.TrimPrefix(text, "x\b")

	text = strings.TrimSpace(text)

	// Don't send empty prompts
	if text == "" {
		a.logger.Debug("Ignoring empty prompt", "rawDataLen", len(data))
		return len(data), nil
	}

	// Clear previous response
	a.mu.Lock()
	a.response.Reset()
	a.mu.Unlock()

	a.logger.Debug("Sending prompt",
		"sessionId", a.sessionID,
		"text", text,
		"textLen", len(text),
		"rawDataLen", len(data))

	// Ensure the context has not been cancelled before writing prompt
	if err := a.ctx.Err(); err != nil {
		a.logger.Debug("Aborting write", "error", err)
		return 0, err
	}

	resp, err := a.conn.Prompt(a.ctx, acp.PromptRequest{
		SessionId: a.sessionID,
		Prompt:    []acp.ContentBlock{acp.TextBlock(text)},
	})
	if err != nil {
		a.logger.Error("Prompt failed", "error", err)
		return 0, err
	}

	a.logger.Debug("Prompt completed", "stopReason", resp.StopReason)

	return len(data), nil
}

// ReadScreen returns the accumulated agent response
func (a *ACPAgentIO) ReadScreen() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.response.String()
}
