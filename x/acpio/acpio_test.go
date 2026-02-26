package acpio_test

import (
	"context"
	"io"
	"os"
	"sync"
	"testing"

	acp "github.com/coder/acp-go-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coder/agentapi/x/acpio"
)

// testAgent implements acp.Agent for testing.
type testAgent struct {
	conn     *acp.AgentSideConnection
	onPrompt func(ctx context.Context, conn *acp.AgentSideConnection, p acp.PromptRequest) (acp.PromptResponse, error)
}

var _ acp.Agent = (*testAgent)(nil)

func (a *testAgent) SetAgentConnection(c *acp.AgentSideConnection) { a.conn = c }

func (a *testAgent) Authenticate(context.Context, acp.AuthenticateRequest) (acp.AuthenticateResponse, error) {
	return acp.AuthenticateResponse{}, nil
}

func (a *testAgent) Initialize(context.Context, acp.InitializeRequest) (acp.InitializeResponse, error) {
	return acp.InitializeResponse{
		ProtocolVersion:   acp.ProtocolVersionNumber,
		AgentCapabilities: acp.AgentCapabilities{},
	}, nil
}

func (a *testAgent) Cancel(context.Context, acp.CancelNotification) error { return nil }

func (a *testAgent) NewSession(context.Context, acp.NewSessionRequest) (acp.NewSessionResponse, error) {
	return acp.NewSessionResponse{SessionId: "test-session"}, nil
}

func (a *testAgent) SetSessionMode(context.Context, acp.SetSessionModeRequest) (acp.SetSessionModeResponse, error) {
	return acp.SetSessionModeResponse{}, nil
}

func (a *testAgent) Prompt(ctx context.Context, p acp.PromptRequest) (acp.PromptResponse, error) {
	if a.onPrompt != nil {
		return a.onPrompt(ctx, a.conn, p)
	}
	return acp.PromptResponse{StopReason: acp.StopReasonEndTurn}, nil
}

// newTestPair creates an ACPAgentIO connected to a testAgent via pipes.
func newTestPair(t *testing.T, agent *testAgent) *acpio.ACPAgentIO {
	t.Helper()

	// Two pipe pairs: client writes → agent reads, agent writes → client reads.
	clientToAgentR, clientToAgentW := io.Pipe()
	agentToClientR, agentToClientW := io.Pipe()

	// Client side: peerInput=clientToAgentW (writes to agent), peerOutput=agentToClientR (reads from agent)
	// Agent side:  peerInput=agentToClientW (writes to client), peerOutput=clientToAgentR (reads from client)
	asc := acp.NewAgentSideConnection(agent, agentToClientW, clientToAgentR)
	agent.SetAgentConnection(asc)

	agentIO, err := acpio.NewWithPipes(
		context.Background(),
		clientToAgentW, agentToClientR,
		nil,
		func() (string, error) { return os.TempDir(), nil },
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = clientToAgentW.Close()
		_ = agentToClientW.Close()
	})

	return agentIO
}

// chunkCollector collects chunks from SetOnChunk in a thread-safe way
// and provides a method to wait for a specific number of chunks.
type chunkCollector struct {
	mu     sync.Mutex
	cond   *sync.Cond
	chunks []string
}

func newChunkCollector() *chunkCollector {
	c := &chunkCollector{}
	c.cond = sync.NewCond(&c.mu)
	return c
}

func (c *chunkCollector) callback(chunk string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.chunks = append(c.chunks, chunk)
	c.cond.Broadcast()
}

func (c *chunkCollector) waitForN(t *testing.T, n int) []string {
	t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()
	for len(c.chunks) < n {
		c.cond.Wait()
	}
	return append([]string(nil), c.chunks...)
}

func Test_ACPAgentIO_WriteAndReadScreen(t *testing.T) {
	collector := newChunkCollector()
	agent := &testAgent{
		onPrompt: func(ctx context.Context, conn *acp.AgentSideConnection, p acp.PromptRequest) (acp.PromptResponse, error) {
			_ = conn.SessionUpdate(ctx, acp.SessionNotification{
				SessionId: p.SessionId,
				Update:    acp.UpdateAgentMessageText("Hello from agent!"),
			})
			return acp.PromptResponse{StopReason: acp.StopReasonEndTurn}, nil
		},
	}
	agentIO := newTestPair(t, agent)
	agentIO.SetOnChunk(collector.callback)

	n, err := agentIO.Write([]byte("test prompt"))
	require.NoError(t, err)
	assert.Equal(t, len("test prompt"), n)

	// SessionUpdate notifications are async — wait for the chunk to arrive.
	collector.waitForN(t, 1)
	assert.Equal(t, "Hello from agent!", agentIO.ReadScreen())
}

func Test_ACPAgentIO_StreamingChunks(t *testing.T) {
	collector := newChunkCollector()
	agent := &testAgent{
		onPrompt: func(ctx context.Context, conn *acp.AgentSideConnection, p acp.PromptRequest) (acp.PromptResponse, error) {
			for _, text := range []string{"Hello", " ", "world!"} {
				_ = conn.SessionUpdate(ctx, acp.SessionNotification{
					SessionId: p.SessionId,
					Update:    acp.UpdateAgentMessageText(text),
				})
			}
			return acp.PromptResponse{StopReason: acp.StopReasonEndTurn}, nil
		},
	}
	agentIO := newTestPair(t, agent)
	agentIO.SetOnChunk(collector.callback)

	_, err := agentIO.Write([]byte("test"))
	require.NoError(t, err)

	// All three chunks should arrive (order may vary due to async notification handling).
	chunks := collector.waitForN(t, 3)
	assert.Len(t, chunks, 3)
	assert.ElementsMatch(t, []string{"Hello", " ", "world!"}, chunks)
}

func Test_ACPAgentIO_StripsEscapeSequences(t *testing.T) {
	received := make(chan string, 1)
	agent := &testAgent{
		onPrompt: func(ctx context.Context, conn *acp.AgentSideConnection, p acp.PromptRequest) (acp.PromptResponse, error) {
			defer close(received)
			for _, block := range p.Prompt {
				if block.Text != nil {
					received <- block.Text.Text
				}
			}
			return acp.PromptResponse{StopReason: acp.StopReasonEndTurn}, nil
		},
	}
	agentIO := newTestPair(t, agent)

	// Bracketed paste sequences should be stripped
	_, err := agentIO.Write([]byte("\x1b[200~hello world\x1b[201~"))
	require.NoError(t, err)
	assert.Equal(t, "hello world", <-received)
}

func Test_ACPAgentIO_IgnoresEmptyPrompt(t *testing.T) {
	agent := &testAgent{
		onPrompt: func(ctx context.Context, conn *acp.AgentSideConnection, p acp.PromptRequest) (acp.PromptResponse, error) {
			assert.Fail(t, "empty prompt should not reach the agent")
			return acp.PromptResponse{StopReason: acp.StopReasonEndTurn}, nil
		},
	}
	agentIO := newTestPair(t, agent)

	// Empty after stripping should be a no-op
	n, err := agentIO.Write([]byte("  \t\n  "))
	require.NoError(t, err)
	assert.Equal(t, len("  \t\n  "), n)
}

func Test_ACPAgentIO_ToolCallFormattedAsText(t *testing.T) {
	collector := newChunkCollector()
	agent := &testAgent{
		onPrompt: func(ctx context.Context, conn *acp.AgentSideConnection, p acp.PromptRequest) (acp.PromptResponse, error) {
			_ = conn.SessionUpdate(ctx, acp.SessionNotification{
				SessionId: p.SessionId,
				Update: acp.StartToolCall(
					"call_1",
					"Reading file",
					acp.WithStartKind(acp.ToolKindRead),
				),
			})
			return acp.PromptResponse{StopReason: acp.StopReasonEndTurn}, nil
		},
	}
	agentIO := newTestPair(t, agent)
	agentIO.SetOnChunk(collector.callback)

	_, err := agentIO.Write([]byte("do something"))
	require.NoError(t, err)

	collector.waitForN(t, 1)
	assert.Contains(t, agentIO.ReadScreen(), "[Tool: read]")
	assert.Contains(t, agentIO.ReadScreen(), "Reading file")
}

func Test_ACPAgentIO_ResetsResponseBetweenWrites(t *testing.T) {
	collector := newChunkCollector()
	callCount := 0
	agent := &testAgent{
		onPrompt: func(ctx context.Context, conn *acp.AgentSideConnection, p acp.PromptRequest) (acp.PromptResponse, error) {
			callCount++
			_ = conn.SessionUpdate(ctx, acp.SessionNotification{
				SessionId: p.SessionId,
				Update:    acp.UpdateAgentMessageText("response " + string(rune('0'+callCount))),
			})
			return acp.PromptResponse{StopReason: acp.StopReasonEndTurn}, nil
		},
	}
	agentIO := newTestPair(t, agent)
	agentIO.SetOnChunk(collector.callback)

	_, err := agentIO.Write([]byte("first"))
	require.NoError(t, err)
	collector.waitForN(t, 1)
	assert.Equal(t, "response 1", agentIO.ReadScreen())

	_, err = agentIO.Write([]byte("second"))
	require.NoError(t, err)
	collector.waitForN(t, 2)
	// Response should be reset, not accumulated
	assert.Equal(t, "response 2", agentIO.ReadScreen())
}
