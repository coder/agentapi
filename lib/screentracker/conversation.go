package screentracker

import (
	"context"
	"strings"
	"time"

	"github.com/coder/agentapi/lib/util"
	"github.com/danielgtaylor/huma/v2"
	"golang.org/x/xerrors"
)

type ConversationStatus string

const (
	ConversationStatusChanging     ConversationStatus = "changing"
	ConversationStatusStable       ConversationStatus = "stable"
	ConversationStatusInitializing ConversationStatus = "initializing"
)

type ConversationRole string

func (c ConversationRole) Schema(r huma.Registry) *huma.Schema {
	return util.OpenAPISchema(r, "ConversationRole", ConversationRoleValues)
}

const (
	ConversationRoleUser  ConversationRole = "user"
	ConversationRoleAgent ConversationRole = "agent"
)

var ConversationRoleValues = []ConversationRole{
	ConversationRoleUser,
	ConversationRoleAgent,
}

var (
	ErrMessageValidationWhitespace = xerrors.New("message must be trimmed of leading and trailing whitespace")
	ErrMessageValidationEmpty      = xerrors.New("message must not be empty")
	ErrMessageValidationChanging   = xerrors.New("message can only be sent when the agent is waiting for user input")
)

type AgentIO interface {
	Write(data []byte) (int, error)
	ReadScreen() string
}

type MessagePart interface {
	Do(writer AgentIO) error
	String() string
}

func buildStringFromMessageParts(parts []MessagePart) string {
	var sb strings.Builder
	for _, part := range parts {
		sb.WriteString(part.String())
	}
	return sb.String()
}

// Conversation represents a conversation between a user and an agent.
// It is intended as the primary interface for interacting with a session.
// Implementations must support the following capabilities:
//   - Fetching all messages between the user and agent,
//   - Sending a message to the agent,
//   - Starting a background loop to update the conversation state, if required,
//   - Fetching the status of the conversation,
//   - Returning a textual representation of the conversation "screen" (used for notifying subscribers of updates to the conversation).
type Conversation interface {
	Messages() []ConversationMessage
	Send(...MessagePart) error
	Start(context.Context)
	Status() ConversationStatus
	Text() string
	SaveState() error
}

// Emitter receives conversation state updates.
type Emitter interface {
	EmitMessages([]ConversationMessage)
	EmitStatus(ConversationStatus)
	EmitScreen(string)
}

type ConversationMessage struct {
	Id      int
	Message string
	Role    ConversationRole
	Time    time.Time
}

type StatePersistenceConfig struct {
	StateFile string
	LoadState bool
	SaveState bool
}
