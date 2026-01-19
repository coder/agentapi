package screentracker

import (
	"context"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"
)

// ACPConversation tracks conversations with ACP-based agents.
// Unlike PTY-based Conversation, ACP has blocking writes where the
// response is complete when Write() returns.
type ACPConversation struct {
	mu        sync.Mutex
	agentIO   AgentIO
	messages  []ConversationMessage
	prompting bool // true while Write() is in progress
	logger    *slog.Logger

	// Initial prompt handling
	initialPrompt           string
	initialPromptSent       bool
	readyForInitialPrompt   bool
}

// NewACPConversation creates a new ACPConversation.
func NewACPConversation(agentIO AgentIO, logger *slog.Logger, initialPrompt string) *ACPConversation {
	if logger == nil {
		logger = slog.Default()
	}
	return &ACPConversation{
		agentIO:           agentIO,
		logger:            logger,
		initialPrompt:     initialPrompt,
		initialPromptSent: len(initialPrompt) == 0,
		// ACP agents are ready immediately (no screen detection needed)
		readyForInitialPrompt: true,
	}
}

// SendMessage sends a message to the agent asynchronously.
// It returns immediately after recording the user message and starts
// the agent request in a background goroutine. Returns an error if
// a message is already being processed.
func (c *ACPConversation) SendMessage(messageParts ...MessagePart) error {
	message := messageParts[0].String()
	for _, part := range messageParts[1:] {
		message += part.String()
	}
	message = strings.TrimSpace(message)

	if message == "" {
		return MessageValidationErrorEmpty
	}

	// Check if already prompting and set state atomically
	c.mu.Lock()
	if c.prompting {
		c.mu.Unlock()
		return MessageValidationErrorChanging
	}
	c.messages = append(c.messages, ConversationMessage{
		Id:      len(c.messages),
		Role:    ConversationRoleUser,
		Message: message,
		Time:    time.Now(),
	})
	c.prompting = true
	c.mu.Unlock()

	c.logger.Debug("ACPConversation sending message", "message", message)

	// Run the blocking write in a goroutine so HTTP returns immediately
	go c.executePrompt(messageParts)

	return nil
}

// executePrompt runs the actual agent request in background
func (c *ACPConversation) executePrompt(messageParts []MessagePart) {
	err := ExecuteParts(c.agentIO, messageParts...)

	c.mu.Lock()
	defer c.mu.Unlock()

	c.prompting = false

	if err != nil {
		c.logger.Error("ACPConversation message failed", "error", err)
		return
	}

	// Agent response is now in ReadScreen()
	response := strings.TrimSpace(c.agentIO.ReadScreen())
	if response != "" {
		c.messages = append(c.messages, ConversationMessage{
			Id:      len(c.messages),
			Role:    ConversationRoleAgent,
			Message: response,
			Time:    time.Now(),
		})
	}

	c.logger.Debug("ACPConversation message complete", "responseLen", len(response))
}

// Status returns the current conversation status.
func (c *ACPConversation) Status() ConversationStatus {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.prompting {
		return ConversationStatusChanging // agent is processing
	}
	return ConversationStatusStable
}

// Messages returns the conversation history.
func (c *ACPConversation) Messages() []ConversationMessage {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.messages)
}

// Screen returns the current screen content.
func (c *ACPConversation) Screen() string {
	return c.agentIO.ReadScreen()
}

// StartSnapshotLoop is a no-op for ACP since responses are captured synchronously.
func (c *ACPConversation) StartSnapshotLoop(ctx context.Context) {
	// No-op for ACP - we don't need to poll for screen changes
}

// Initial prompt handling methods

func (c *ACPConversation) GetInitialPrompt() string {
	return c.initialPrompt
}

func (c *ACPConversation) IsInitialPromptSent() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.initialPromptSent
}

func (c *ACPConversation) SetInitialPromptSent(sent bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.initialPromptSent = sent
}

func (c *ACPConversation) IsReadyForInitialPrompt() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.readyForInitialPrompt
}

func (c *ACPConversation) SetReadyForInitialPrompt(ready bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.readyForInitialPrompt = ready
}

// Ensure ACPConversation implements ConversationTracker
var _ ConversationTracker = (*ACPConversation)(nil)
