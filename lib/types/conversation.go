package types

import (
	"time"

	"github.com/coder/agentapi/lib/util"
	"github.com/danielgtaylor/huma/v2"
)

type ConversationRole string

const (
	ConversationRoleUser  ConversationRole = "user"
	ConversationRoleAgent ConversationRole = "agent"
)

var ConversationRoleValues = []ConversationRole{
	ConversationRoleUser,
	ConversationRoleAgent,
}

type ConversationMessage struct {
	Id      int
	Message string
	Role    ConversationRole
	Time    time.Time
}

func (c ConversationRole) Schema(r huma.Registry) *huma.Schema {
	return util.OpenAPISchema(r, "ConversationRole", ConversationRoleValues)
}

type InteractionType string

const (
	SDKInteractionType InteractionType = "sdk"
	CLIInteractionType InteractionType = "cli"
)

type MessageType string

const (
	MessageTypeUser MessageType = "user"
	MessageTypeRaw  MessageType = "raw"
)

var MessageTypeValues = []MessageType{
	MessageTypeUser,
	MessageTypeRaw,
}

func (m MessageType) Schema(r huma.Registry) *huma.Schema {
	return util.OpenAPISchema(r, "MessageType", MessageTypeValues)
}

type AgentStatus string

const (
	AgentStatusRunning AgentStatus = "running"
	AgentStatusStable  AgentStatus = "stable"
)

var AgentStatusValues = []AgentStatus{
	AgentStatusStable,
	AgentStatusRunning,
}

func (a AgentStatus) Schema(r huma.Registry) *huma.Schema {
	return util.OpenAPISchema(r, "AgentStatus", AgentStatusValues)
}
