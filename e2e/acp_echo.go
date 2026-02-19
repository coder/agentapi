//go:build ignore

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"

	acp "github.com/coder/acp-go-sdk"
)

// ScriptEntry defines a single entry in the test script.
type ScriptEntry struct {
	ExpectMessage   string `json:"expectMessage"`
	ThinkDurationMS int64  `json:"thinkDurationMS"`
	ResponseMessage string `json:"responseMessage"`
}

// acpEchoAgent implements the ACP Agent interface for testing.
type acpEchoAgent struct {
	script      []ScriptEntry
	scriptIndex int
	conn        *acp.AgentSideConnection
	sessionID   acp.SessionId
}

var _ acp.Agent = (*acpEchoAgent)(nil)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "Usage: acp_echo <script.json>")
		os.Exit(1)
	}

	script, err := loadScript(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading script: %v\n", err)
		os.Exit(1)
	}

	if len(script) == 0 {
		fmt.Fprintln(os.Stderr, "Script is empty")
		os.Exit(1)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	go func() {
		<-sigCh
		os.Exit(0)
	}()

	agent := &acpEchoAgent{
		script: script,
	}

	conn := acp.NewAgentSideConnection(agent, os.Stdout, os.Stdin)
	agent.conn = conn

	<-conn.Done()
}

func (a *acpEchoAgent) Initialize(_ context.Context, _ acp.InitializeRequest) (acp.InitializeResponse, error) {
	return acp.InitializeResponse{
		ProtocolVersion:   acp.ProtocolVersionNumber,
		AgentCapabilities: acp.AgentCapabilities{},
	}, nil
}

func (a *acpEchoAgent) Authenticate(_ context.Context, _ acp.AuthenticateRequest) (acp.AuthenticateResponse, error) {
	return acp.AuthenticateResponse{}, nil
}

func (a *acpEchoAgent) Cancel(_ context.Context, _ acp.CancelNotification) error {
	return nil
}

func (a *acpEchoAgent) NewSession(_ context.Context, _ acp.NewSessionRequest) (acp.NewSessionResponse, error) {
	a.sessionID = "test-session"
	return acp.NewSessionResponse{
		SessionId: a.sessionID,
	}, nil
}

func (a *acpEchoAgent) Prompt(ctx context.Context, params acp.PromptRequest) (acp.PromptResponse, error) {
	// Extract text from prompt
	var promptText string
	for _, block := range params.Prompt {
		if block.Text != nil {
			promptText = block.Text.Text
			break
		}
	}
	promptText = strings.TrimSpace(promptText)

	if a.scriptIndex >= len(a.script) {
		return acp.PromptResponse{
			StopReason: acp.StopReasonEndTurn,
		}, nil
	}

	entry := a.script[a.scriptIndex]
	expected := strings.TrimSpace(entry.ExpectMessage)

	// Empty ExpectMessage matches any prompt
	if expected != "" && expected != promptText {
		return acp.PromptResponse{}, fmt.Errorf("expected message %q but got %q", expected, promptText)
	}

	a.scriptIndex++

	// Send response via session update
	if err := a.conn.SessionUpdate(ctx, acp.SessionNotification{
		SessionId: params.SessionId,
		Update:    acp.UpdateAgentMessageText(entry.ResponseMessage),
	}); err != nil {
		return acp.PromptResponse{}, err
	}

	return acp.PromptResponse{
		StopReason: acp.StopReasonEndTurn,
	}, nil
}

func (a *acpEchoAgent) SetSessionMode(_ context.Context, _ acp.SetSessionModeRequest) (acp.SetSessionModeResponse, error) {
	return acp.SetSessionModeResponse{}, nil
}

func loadScript(scriptPath string) ([]ScriptEntry, error) {
	data, err := os.ReadFile(scriptPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read script file: %w", err)
	}

	var script []ScriptEntry
	if err := json.Unmarshal(data, &script); err != nil {
		return nil, fmt.Errorf("failed to parse script JSON: %w", err)
	}

	return script, nil
}
