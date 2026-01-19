package acpio

import (
	"testing"
	"time"

	st "github.com/coder/agentapi/lib/screentracker"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func textMsg(s string) st.MessagePartText {
	return st.MessagePartText{Content: s}
}

// streamingMockAgentIO wraps MockAgentIO and implements StreamingAgentIO
type streamingMockAgentIO struct {
	*st.MockAgentIO
	onChunk  func(string)
	response string // response to send as chunks when Write is called
}

func (s *streamingMockAgentIO) SetOnChunk(fn func(string)) {
	s.onChunk = fn
}

func (s *streamingMockAgentIO) triggerChunks() {
	if s.onChunk != nil && s.response != "" {
		s.onChunk(s.response)
	}
}

func TestACPConversation_SendMessage(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	mock := st.NewMockAgentIO(ctrl)
	agent := &streamingMockAgentIO{MockAgentIO: mock, response: "Hello! How can I help you?"}

	mock.EXPECT().Write([]byte("Hi there")).DoAndReturn(func(data []byte) (int, error) {
		agent.triggerChunks()
		return len(data), nil
	})

	conv := NewConversation(agent, nil, "", nil)

	err := conv.SendMessage(textMsg("Hi there"))
	require.NoError(t, err)

	// SendMessage returns immediately, wait for background goroutine
	require.Eventually(t, func() bool {
		return conv.Status() == st.ConversationStatusStable
	}, time.Second, 10*time.Millisecond)

	// Verify messages tracked
	msgs := conv.Messages()
	require.Len(t, msgs, 2)

	assert.Equal(t, st.ConversationRoleUser, msgs[0].Role)
	assert.Equal(t, "Hi there", msgs[0].Message)

	assert.Equal(t, st.ConversationRoleAgent, msgs[1].Role)
	assert.Equal(t, "Hello! How can I help you?", msgs[1].Message)
}

func TestACPConversation_SendMessage_EmptyRejected(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	agent := st.NewMockAgentIO(ctrl)
	// No Write() expected - empty message should be rejected before calling agent

	conv := NewConversation(agent, nil, "", nil)

	err := conv.SendMessage(textMsg(""))
	assert.ErrorIs(t, err, st.MessageValidationErrorEmpty)
}

func TestACPConversation_SendMessage_RejectsWhilePrompting(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	mock := st.NewMockAgentIO(ctrl)
	agent := &streamingMockAgentIO{MockAgentIO: mock, response: "Response"}

	// First write blocks for 100ms
	mock.EXPECT().Write(gomock.Any()).DoAndReturn(func(data []byte) (int, error) {
		time.Sleep(100 * time.Millisecond)
		agent.triggerChunks()
		return len(data), nil
	})

	conv := NewConversation(agent, nil, "", nil)

	// First message starts processing
	err := conv.SendMessage(textMsg("First"))
	require.NoError(t, err)

	// Status should be changing
	assert.Equal(t, st.ConversationStatusChanging, conv.Status())

	// Second message should be rejected while first is processing
	err = conv.SendMessage(textMsg("Second"))
	assert.ErrorIs(t, err, st.MessageValidationErrorChanging)

	// Wait for first to complete
	require.Eventually(t, func() bool {
		return conv.Status() == st.ConversationStatusStable
	}, time.Second, 10*time.Millisecond)

	// Now a new message should work
	agent.response = "Response 2"
	mock.EXPECT().Write(gomock.Any()).DoAndReturn(func(data []byte) (int, error) {
		agent.triggerChunks()
		return len(data), nil
	})

	err = conv.SendMessage(textMsg("Third"))
	require.NoError(t, err)

	// Wait for third message to complete
	require.Eventually(t, func() bool {
		return conv.Status() == st.ConversationStatusStable
	}, time.Second, 10*time.Millisecond)
}

func TestACPConversation_Status(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	mock := st.NewMockAgentIO(ctrl)
	agent := &streamingMockAgentIO{MockAgentIO: mock, response: "Response"}

	// Write blocks for 100ms to simulate agent processing
	mock.EXPECT().Write(gomock.Any()).DoAndReturn(func(data []byte) (int, error) {
		time.Sleep(100 * time.Millisecond)
		agent.triggerChunks()
		return len(data), nil
	})

	conv := NewConversation(agent, nil, "", nil)

	// Initially stable
	assert.Equal(t, st.ConversationStatusStable, conv.Status())

	// Send message (returns immediately, runs in background)
	err := conv.SendMessage(textMsg("Hello"))
	require.NoError(t, err)

	// Should be "changing" immediately after SendMessage returns
	assert.Equal(t, st.ConversationStatusChanging, conv.Status())

	// Wait for completion
	require.Eventually(t, func() bool {
		return conv.Status() == st.ConversationStatusStable
	}, time.Second, 10*time.Millisecond)
}

func TestACPConversation_Messages_ThreadSafe(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	mock := st.NewMockAgentIO(ctrl)
	agent := &streamingMockAgentIO{MockAgentIO: mock, response: "Response"}

	// Single write since concurrent sends are now rejected
	mock.EXPECT().Write(gomock.Any()).DoAndReturn(func(data []byte) (int, error) {
		time.Sleep(50 * time.Millisecond)
		agent.triggerChunks()
		return len(data), nil
	})

	conv := NewConversation(agent, nil, "", nil)

	// Send one message
	err := conv.SendMessage(textMsg("Message"))
	require.NoError(t, err)

	// Read messages while send is in progress - should not panic
	for i := 0; i < 5; i++ {
		_ = conv.Messages()
		_ = conv.Status()
		time.Sleep(10 * time.Millisecond)
	}

	// Wait for completion
	require.Eventually(t, func() bool {
		return conv.Status() == st.ConversationStatusStable
	}, time.Second, 10*time.Millisecond)

	// Should have 2 messages (1 user + 1 agent)
	msgs := conv.Messages()
	assert.Len(t, msgs, 2)
}

func TestACPConversation_InitialPrompt(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	agent := st.NewMockAgentIO(ctrl)
	conv := NewConversation(agent, nil, "Hello, world!", nil)

	assert.Equal(t, "Hello, world!", conv.GetInitialPrompt())
	assert.False(t, conv.IsInitialPromptSent())
	assert.True(t, conv.IsReadyForInitialPrompt()) // ACP is always ready

	conv.SetInitialPromptSent(true)
	assert.True(t, conv.IsInitialPromptSent())
}

func TestACPConversation_Screen(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	agent := st.NewMockAgentIO(ctrl)
	agent.EXPECT().ReadScreen().Return("Current screen content")

	conv := NewConversation(agent, nil, "", nil)

	assert.Equal(t, "Current screen content", conv.Screen())
}
