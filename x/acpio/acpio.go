package acpio

import (
	"context"
	"io"
	"strings"
	"sync"

	acp "github.com/coder/acp-go-sdk"
)

// ACPAgentIO implements screentracker.AgentIO using the ACP protocol
type ACPAgentIO struct {
	ctx       context.Context
	conn      *acp.ClientSideConnection
	sessionID acp.SessionId
	mu        sync.RWMutex
	response  strings.Builder
}

// acpClient implements acp.Client to handle callbacks from the agent
type acpClient struct {
	agentIO *ACPAgentIO
}

var _ acp.Client = (*acpClient)(nil)

func (c *acpClient) SessionUpdate(ctx context.Context, params acp.SessionNotification) error {
	if params.Update.AgentMessageChunk != nil {
		if text := params.Update.AgentMessageChunk.Content.Text; text != nil {
			c.agentIO.mu.Lock()
			c.agentIO.response.WriteString(text.Text)
			c.agentIO.mu.Unlock()
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

// NewWithPipes creates an ACPAgentIO connected via the provided pipes (for testing)
func NewWithPipes(ctx context.Context, toAgent io.Writer, fromAgent io.Reader) (*ACPAgentIO, error) {
	agentIO := &ACPAgentIO{ctx: ctx}
	client := &acpClient{agentIO: agentIO}

	conn := acp.NewClientSideConnection(client, toAgent, fromAgent)
	agentIO.conn = conn

	// Initialize the connection
	_, err := conn.Initialize(ctx, acp.InitializeRequest{
		ProtocolVersion:    acp.ProtocolVersionNumber,
		ClientCapabilities: acp.ClientCapabilities{},
	})
	if err != nil {
		return nil, err
	}

	// Create a session
	sessResp, err := conn.NewSession(ctx, acp.NewSessionRequest{
		Cwd:        "/tmp",
		McpServers: []acp.McpServer{},
	})
	if err != nil {
		return nil, err
	}
	agentIO.sessionID = sessResp.SessionId

	return agentIO, nil
}

// Write sends a message to the agent via ACP prompt
func (a *ACPAgentIO) Write(data []byte) (int, error) {
	// Clear previous response
	a.mu.Lock()
	a.response.Reset()
	a.mu.Unlock()

	text := string(data)

	// Strip bracketed paste escape sequences if present
	text = strings.TrimPrefix(text, "\x1b[200~")
	text = strings.TrimSuffix(text, "\x1b[201~")

	_, err := a.conn.Prompt(a.ctx, acp.PromptRequest{
		SessionId: a.sessionID,
		Prompt:    []acp.ContentBlock{acp.TextBlock(text)},
	})
	if err != nil {
		return 0, err
	}

	return len(data), nil
}

// ReadScreen returns the accumulated agent response
func (a *ACPAgentIO) ReadScreen() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.response.String()
}
