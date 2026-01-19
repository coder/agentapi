package screentracker

//go:generate mockgen -destination=mock_conversation_tracker_test.go -package=screentracker . ConversationTracker,AgentIO

import "context"

// Conversation is the interface for tracking conversations with agents.
// It has two implementations:
// - PTYConversation: for PTY-based agents (polling, screen diffing)
// - ACPConversation: for ACP-based agents (blocking writes, explicit responses)
type Conversation interface {
	// SendMessage sends a message to the agent
	SendMessage(messageParts ...MessagePart) error

	// Messages returns the conversation history
	Messages() []ConversationMessage

	// Status returns the current conversation status
	Status() ConversationStatus

	// Screen returns the current screen content
	Screen() string

	// StartSnapshotLoop starts the background loop that polls for screen changes.
	// For ACP, this is a no-op since responses are captured synchronously.
	StartSnapshotLoop(ctx context.Context)

	// InitialPrompt handling - these are needed for the server's initial prompt logic
	GetInitialPrompt() string
	IsInitialPromptSent() bool
	SetInitialPromptSent(sent bool)
	IsReadyForInitialPrompt() bool
	SetReadyForInitialPrompt(ready bool)
}
