package screentracker

import (
	"context"
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
	MessageValidationErrorWhitespace = xerrors.New("message must be trimmed of leading and trailing whitespace")
	MessageValidationErrorEmpty      = xerrors.New("message must not be empty")
	MessageValidationErrorChanging   = xerrors.New("message can only be sent when the agent is waiting for user input")
)

type AgentIO interface {
	Write(data []byte) (int, error)
	ReadScreen() string
}

type MessagePart interface {
	Do(writer AgentIO) error
	String() string
}

// Conversation allows tracking of a conversation between a user and an agent.
type Conversation interface {
	Messages() []ConversationMessage
	Snapshot(string)
	Start(context.Context)
	Status() ConversationStatus
	String() string
}

type ConversationMessage struct {
	Id      int
	Message string
	Role    ConversationRole
	Time    time.Time
}
