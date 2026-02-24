package acpio

import (
	"context"
	"log/slog"
	"slices"
	"strings"
	"sync"

	st "github.com/coder/agentapi/lib/screentracker"
	"github.com/coder/quartz"
)

// Compile-time assertion that ACPConversation implements st.Conversation
var _ st.Conversation = (*ACPConversation)(nil)

// ChunkableAgentIO extends AgentIO with chunk callback support for streaming responses.
// This interface is what ACPConversation needs from its AgentIO implementation.
type ChunkableAgentIO interface {
	st.AgentIO
	SetOnChunk(fn func(chunk string))
}

// ACPConversation tracks conversations with ACP-based agents.
// Unlike PTY-based Conversation, ACP has blocking writes where the
// response is complete when Write() returns.
type ACPConversation struct {
	mu                sync.Mutex
	ctx               context.Context
	cancel            context.CancelFunc
	agentIO           ChunkableAgentIO
	messages          []st.ConversationMessage
	prompting         bool // true while agent is processing
	streamingResponse strings.Builder
	logger            *slog.Logger
	emitter           st.Emitter
	initialPrompt     []st.MessagePart
	clock             quartz.Clock
}

// noopEmitter is a no-op implementation of Emitter for when no emitter is provided.
type noopEmitter struct{}

func (noopEmitter) EmitMessages([]st.ConversationMessage) {}
func (noopEmitter) EmitStatus(st.ConversationStatus)      {}
func (noopEmitter) EmitScreen(string)                     {}

// NewACPConversation creates a new ACPConversation.
// If emitter is provided, it will receive events when messages/status/screen change.
// If clock is nil, a real clock will be used.
func NewACPConversation(ctx context.Context, agentIO ChunkableAgentIO, logger *slog.Logger, initialPrompt []st.MessagePart, emitter st.Emitter, clock quartz.Clock) *ACPConversation {
	if logger == nil {
		logger = slog.Default()
	}
	if clock == nil {
		clock = quartz.NewReal()
	}
	if emitter == nil {
		emitter = noopEmitter{}
	}
	ctx, cancel := context.WithCancel(ctx)
	c := &ACPConversation{
		ctx:           ctx,
		cancel:        cancel,
		agentIO:       agentIO,
		logger:        logger,
		initialPrompt: initialPrompt,
		emitter:       emitter,
		clock:         clock,
	}
	return c
}

// Messages returns the conversation history.
func (c *ACPConversation) Messages() []st.ConversationMessage {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.messages)
}

// Send sends a message to the agent asynchronously.
// It returns immediately after recording the user message and starts
// the agent request in a background goroutine. Returns an error if
// a message is already being processed.
func (c *ACPConversation) Send(messageParts ...st.MessagePart) error {
	message := ""
	for _, part := range messageParts {
		message += part.String()
	}

	// Validate whitespace BEFORE trimming (match PTY behavior)
	if message != strings.TrimSpace(message) {
		return st.ErrMessageValidationWhitespace
	}

	if message == "" {
		return st.ErrMessageValidationEmpty
	}

	// Check if already prompting and set state atomically
	c.mu.Lock()
	if c.prompting {
		c.mu.Unlock()
		return st.ErrMessageValidationChanging
	}
	c.messages = append(c.messages, st.ConversationMessage{
		Id:      len(c.messages),
		Role:    st.ConversationRoleUser,
		Message: message,
		Time:    c.clock.Now(),
	})
	// Add placeholder for streaming agent response
	c.messages = append(c.messages, st.ConversationMessage{
		Id:      len(c.messages),
		Role:    st.ConversationRoleAgent,
		Message: "",
		Time:    c.clock.Now(),
	})
	c.streamingResponse.Reset()
	c.prompting = true
	status := c.statusLocked()
	c.mu.Unlock()

	// Emit status change to "running" before starting the prompt
	c.emitter.EmitStatus(status)

	c.logger.Debug("ACPConversation sending message", "message", message)

	// Run the blocking write in a goroutine so HTTP returns immediately
	go c.executePrompt(messageParts)

	return nil
}

// Start sets up chunk handling and sends the initial prompt if provided.
func (c *ACPConversation) Start(ctx context.Context) {
	// Wire up the chunk callback for streaming
	c.agentIO.SetOnChunk(c.handleChunk)

	// Send initial prompt if provided
	if len(c.initialPrompt) > 0 {
		err := c.Send(c.initialPrompt...)
		if err != nil {
			c.logger.Error("ACPConversation failed to send initial prompt", "error", err)
		}
	} else {
		// No initial prompt means we start in stable state
		c.emitter.EmitStatus(c.Status())
	}
}

// Status returns the current conversation status.
func (c *ACPConversation) Status() st.ConversationStatus {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.statusLocked()
}

// statusLocked returns the status without acquiring the lock (caller must hold lock).
func (c *ACPConversation) statusLocked() st.ConversationStatus {
	if c.prompting {
		return st.ConversationStatusChanging // agent is processing
	}
	return st.ConversationStatusStable
}

// Stop cancels any in-progress operations.
func (c *ACPConversation) Stop() {
	c.cancel()
}

// Text returns the current streaming response text.
func (c *ACPConversation) Text() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.streamingResponse.String()
}

// handleChunk is called for each streaming chunk from the agent.
func (c *ACPConversation) handleChunk(chunk string) {
	c.mu.Lock()
	// Log and discard chunks that arrive after the prompt has completed or errored.
	// This should not happen under normal operation — if it does, it indicates a
	// bug in the ACP SDK or a race in the connection teardown.
	if !c.prompting {
		c.mu.Unlock()
		c.logger.Error("received chunk while not prompting (late/unexpected chunk discarded)",
			"chunkLen", len(chunk))
		return
	}
	c.streamingResponse.WriteString(chunk)
	// Only update the last message if it's the agent placeholder (defense-in-depth)
	if len(c.messages) > 0 && c.messages[len(c.messages)-1].Role == st.ConversationRoleAgent {
		c.messages[len(c.messages)-1].Message = c.streamingResponse.String()
	}
	messages := slices.Clone(c.messages)
	status := c.statusLocked()
	screen := c.streamingResponse.String()
	c.mu.Unlock()

	c.emitter.EmitMessages(messages)
	c.emitter.EmitStatus(status)
	c.emitter.EmitScreen(screen)
}

// executePrompt runs the actual agent request in background
func (c *ACPConversation) executePrompt(messageParts []st.MessagePart) {
	var err error
	for _, part := range messageParts {
		if c.ctx.Err() != nil {
			err = c.ctx.Err()
			break
		}
		if partErr := part.Do(c.agentIO); partErr != nil {
			err = partErr
			break
		}
	}

	c.mu.Lock()
	c.prompting = false

	if err != nil {
		c.logger.Error("ACPConversation message failed", "error", err)
		// Remove the agent's streaming message on error (may be empty or partial)
		if len(c.messages) > 0 && c.messages[len(c.messages)-1].Role == st.ConversationRoleAgent {
			c.messages = c.messages[:len(c.messages)-1]
		}
		messages := slices.Clone(c.messages)
		status := c.statusLocked()
		screen := c.streamingResponse.String()
		c.mu.Unlock()

		c.emitter.EmitMessages(messages)
		c.emitter.EmitStatus(status)
		c.emitter.EmitScreen(screen)
		return
	}

	// Final response should already be in the last message via streaming
	// but ensure it's finalized
	response := c.streamingResponse.String()
	if len(c.messages) > 0 && c.messages[len(c.messages)-1].Role == st.ConversationRoleAgent {
		c.messages[len(c.messages)-1].Message = strings.TrimSpace(response)
	}
	messages := slices.Clone(c.messages)
	status := c.statusLocked()
	screen := c.streamingResponse.String()
	c.mu.Unlock()

	c.emitter.EmitMessages(messages)
	c.emitter.EmitStatus(status)
	c.emitter.EmitScreen(screen)

	c.logger.Debug("ACPConversation message complete", "responseLen", len(response))
}
