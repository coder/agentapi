package acpio_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/coder/quartz"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coder/agentapi/lib/screentracker"
	"github.com/coder/agentapi/x/acpio"
)

// mockAgentIO implements acpio.ChunkableAgentIO for testing.
// It provides a channel-based synchronization mechanism to test ACPConversation
// without relying on time.Sleep.
type mockAgentIO struct {
	mu            sync.Mutex
	written       []byte
	screenContent string
	onChunkFn     func(chunk string)

	// Control behavior
	writeErr error
	// writeBlock is a channel that, if non-nil, will cause Write to block until closed.
	// This allows tests to control when the write completes.
	writeBlock chan struct{}
	// writeStarted is closed when Write begins (before blocking on writeBlock).
	// This allows tests to synchronize on the write starting.
	writeStarted chan struct{}
}

// mockEmitter implements screentracker.Emitter for testing.
type mockEmitter struct {
	mu              sync.Mutex
	messagesCalls   int
	statusCalls     int
	screenCalls     int
	lastMessages    []screentracker.ConversationMessage
	lastStatus      screentracker.ConversationStatus
	lastScreen      string
}

func newMockEmitter() *mockEmitter {
	return &mockEmitter{}
}

func (m *mockEmitter) EmitMessages(messages []screentracker.ConversationMessage) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messagesCalls++
	m.lastMessages = messages
}

func (m *mockEmitter) EmitStatus(status screentracker.ConversationStatus) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.statusCalls++
	m.lastStatus = status
}

func (m *mockEmitter) EmitScreen(screen string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.screenCalls++
	m.lastScreen = screen
}

func (m *mockEmitter) TotalCalls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.messagesCalls + m.statusCalls + m.screenCalls
}

func newMockAgentIO() *mockAgentIO {
	return &mockAgentIO{}
}

func (m *mockAgentIO) Write(data []byte) (int, error) {
	// Signal that write has started
	m.mu.Lock()
	started := m.writeStarted
	block := m.writeBlock
	m.mu.Unlock()

	if started != nil {
		close(started)
	}

	// Block if configured to do so (for testing concurrent sends)
	if block != nil {
		<-block
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.writeErr != nil {
		return 0, m.writeErr
	}
	m.written = append(m.written, data...)
	return len(data), nil
}

func (m *mockAgentIO) ReadScreen() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.screenContent
}

func (m *mockAgentIO) SetOnChunk(fn func(chunk string)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onChunkFn = fn
}

// SimulateChunks simulates the agent sending streaming chunks.
// This triggers the onChunk callback as if the agent was responding.
func (m *mockAgentIO) SimulateChunks(chunks ...string) {
	m.mu.Lock()
	fn := m.onChunkFn
	m.mu.Unlock()
	for _, chunk := range chunks {
		if fn != nil {
			fn(chunk)
		}
	}
}

// GetWritten returns all data written to the agent.
func (m *mockAgentIO) GetWritten() []byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]byte(nil), m.written...)
}

// BlockWrite sets up blocking for the next Write call and returns:
// - started: a channel that is closed when Write begins
// - done: a channel to close to unblock the Write
func (m *mockAgentIO) BlockWrite() (started chan struct{}, done chan struct{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.writeStarted = make(chan struct{})
	m.writeBlock = make(chan struct{})
	return m.writeStarted, m.writeBlock
}

func Test_NewACPConversation(t *testing.T) {
	mClock := quartz.NewMock(t)
	mock := newMockAgentIO()

	conv := acpio.NewACPConversation(mock, nil, nil, nil, mClock)

	require.NotNil(t, conv)
}

func Test_Messages_InitiallyEmpty(t *testing.T) {
	mClock := quartz.NewMock(t)
	mock := newMockAgentIO()
	conv := acpio.NewACPConversation(mock, nil, nil, nil, mClock)

	messages := conv.Messages()

	assert.Empty(t, messages)
}

func Test_Status_InitiallyStable(t *testing.T) {
	mClock := quartz.NewMock(t)
	mock := newMockAgentIO()
	conv := acpio.NewACPConversation(mock, nil, nil, nil, mClock)

	status := conv.Status()

	assert.Equal(t, screentracker.ConversationStatusStable, status)
}

func Test_Send_AddsUserMessage(t *testing.T) {
	mClock := quartz.NewMock(t)
	mock := newMockAgentIO()
	// Set up blocking to synchronize with the goroutine
	started, done := mock.BlockWrite()

	conv := acpio.NewACPConversation(mock, nil, nil, nil, mClock)
	conv.Start(context.Background())

	err := conv.Send(screentracker.MessagePartText{Content: "hello"})
	require.NoError(t, err)

	// Wait for the write goroutine to start
	<-started

	messages := conv.Messages()
	require.Len(t, messages, 2) // user message + placeholder agent message

	assert.Equal(t, screentracker.ConversationRoleUser, messages[0].Role)
	assert.Equal(t, "hello", messages[0].Message)
	assert.Equal(t, screentracker.ConversationRoleAgent, messages[1].Role)

	// Unblock the write to let the test complete cleanly
	close(done)
}

func Test_Send_RejectsEmptyMessage(t *testing.T) {
	mClock := quartz.NewMock(t)
	mock := newMockAgentIO()
	conv := acpio.NewACPConversation(mock, nil, nil, nil, mClock)

	err := conv.Send(screentracker.MessagePartText{Content: ""})

	assert.ErrorIs(t, err, screentracker.ErrMessageValidationEmpty)
}

func Test_Send_RejectsWhitespace(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{"leading space", " hello"},
		{"trailing space", "hello "},
		{"leading newline", "\nhello"},
		{"trailing newline", "hello\n"},
		{"both sides", " hello "},
		{"newlines both sides", "\nhello\n"},
		{"leading tab", "\thello"},
		{"trailing tab", "hello\t"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mClock := quartz.NewMock(t)
			mock := newMockAgentIO()
			conv := acpio.NewACPConversation(mock, nil, nil, nil, mClock)

			err := conv.Send(screentracker.MessagePartText{Content: tt.content})

			assert.ErrorIs(t, err, screentracker.ErrMessageValidationWhitespace)
		})
	}
}

func Test_Send_RejectsDuplicateSend(t *testing.T) {
	mClock := quartz.NewMock(t)
	mock := newMockAgentIO()
	// Block the write so it doesn't complete immediately
	started, done := mock.BlockWrite()

	conv := acpio.NewACPConversation(mock, nil, nil, nil, mClock)
	conv.Start(context.Background())

	// First send should succeed
	err := conv.Send(screentracker.MessagePartText{Content: "first"})
	require.NoError(t, err)

	// Wait for the write to start (ensuring we're in "prompting" state)
	<-started

	// Second send while first is processing should fail
	err = conv.Send(screentracker.MessagePartText{Content: "second"})
	assert.ErrorIs(t, err, screentracker.ErrMessageValidationChanging)

	// Unblock the write to let the test complete cleanly
	close(done)
}

func Test_Status_ChangesWhileProcessing(t *testing.T) {
	mClock := quartz.NewMock(t)
	mock := newMockAgentIO()
	// Block the write so we can observe status changes
	started, done := mock.BlockWrite()

	conv := acpio.NewACPConversation(mock, nil, nil, nil, mClock)
	conv.Start(context.Background())

	// Initially stable
	assert.Equal(t, screentracker.ConversationStatusStable, conv.Status())

	// Send a message
	err := conv.Send(screentracker.MessagePartText{Content: "test"})
	require.NoError(t, err)

	// Wait for write to start
	<-started

	// Status should be changing while processing
	assert.Equal(t, screentracker.ConversationStatusChanging, conv.Status())

	// Unblock the write
	close(done)

	// Give the goroutine a chance to complete (status update happens after Write returns)
	require.Eventually(t, func() bool {
		return conv.Status() == screentracker.ConversationStatusStable
	}, 100*time.Millisecond, 5*time.Millisecond, "status should return to stable")
}

func Test_Text_ReturnsStreamingContent(t *testing.T) {
	mClock := quartz.NewMock(t)
	mock := newMockAgentIO()
	// Block the write so we can simulate streaming during processing
	started, done := mock.BlockWrite()

	conv := acpio.NewACPConversation(mock, nil, nil, nil, mClock)
	conv.Start(context.Background())

	// Initially empty
	assert.Equal(t, "", conv.Text())

	// Send a message
	err := conv.Send(screentracker.MessagePartText{Content: "question"})
	require.NoError(t, err)

	// Wait for write to start
	<-started

	// Simulate streaming chunks from agent
	mock.SimulateChunks("Hello", " ", "world!")

	// Text should contain the streamed content
	assert.Equal(t, "Hello world!", conv.Text())

	// The last message should also be updated
	messages := conv.Messages()
	require.Len(t, messages, 2)
	assert.Equal(t, "Hello world!", messages[1].Message)

	// Unblock the write to let the test complete cleanly
	close(done)
}

func Test_Emitter_CalledOnChanges(t *testing.T) {
	mClock := quartz.NewMock(t)
	mock := newMockAgentIO()
	// Block the write so we can simulate chunks during processing
	started, done := mock.BlockWrite()

	emitter := newMockEmitter()

	conv := acpio.NewACPConversation(mock, nil, nil, emitter, mClock)
	conv.Start(context.Background())

	// Send a message
	err := conv.Send(screentracker.MessagePartText{Content: "test"})
	require.NoError(t, err)

	// Wait for write to start
	<-started

	// Simulate chunks - each should trigger emitter calls
	mock.SimulateChunks("chunk1")
	mock.SimulateChunks("chunk2")

	emitter.mu.Lock()
	messagesCallsBeforeComplete := emitter.messagesCalls
	emitter.mu.Unlock()

	// Should have emit calls from chunks (each chunk emits messages, status, and screen)
	assert.Equal(t, 2, messagesCallsBeforeComplete)

	// Unblock the write to complete processing
	close(done)

	// Wait for completion emit
	require.Eventually(t, func() bool {
		emitter.mu.Lock()
		c := emitter.messagesCalls
		emitter.mu.Unlock()
		return c >= 3 // 2 from chunks + 1 from completion
	}, 100*time.Millisecond, 5*time.Millisecond, "should receive completion emit")
}

func Test_InitialPrompt_SentOnStart(t *testing.T) {
	mClock := quartz.NewMock(t)
	mock := newMockAgentIO()
	// Set up blocking to synchronize with the initial prompt send
	started, done := mock.BlockWrite()

	initialPrompt := []screentracker.MessagePart{
		screentracker.MessagePartText{Content: "initial prompt"},
	}

	conv := acpio.NewACPConversation(mock, nil, initialPrompt, nil, mClock)
	conv.Start(context.Background())

	// Wait for write to start (initial prompt is being sent)
	<-started

	// Should have user message from initial prompt
	messages := conv.Messages()
	require.GreaterOrEqual(t, len(messages), 1)
	assert.Equal(t, screentracker.ConversationRoleUser, messages[0].Role)
	assert.Equal(t, "initial prompt", messages[0].Message)

	// Unblock the write to let the test complete cleanly
	close(done)
}

func Test_Messages_AreCopied(t *testing.T) {
	mClock := quartz.NewMock(t)
	mock := newMockAgentIO()
	// Set up blocking to synchronize
	started, done := mock.BlockWrite()

	conv := acpio.NewACPConversation(mock, nil, nil, nil, mClock)
	conv.Start(context.Background())

	err := conv.Send(screentracker.MessagePartText{Content: "test"})
	require.NoError(t, err)

	// Wait for write to start
	<-started

	// Get messages and modify
	messages := conv.Messages()
	require.Len(t, messages, 2)
	messages[0].Message = "modified"

	// Original should be unchanged
	originalMessages := conv.Messages()
	assert.Equal(t, "test", originalMessages[0].Message)

	// Unblock the write to let the test complete cleanly
	close(done)
}
