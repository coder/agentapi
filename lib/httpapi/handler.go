package httpapi

import (
	"context"

	"github.com/coder/agentapi/lib/types"
	"github.com/danielgtaylor/huma/v2/sse"
)

// AgentHandler defines the interface that all interaction modes must implement
type AgentHandler interface {
	GetStatus(ctx context.Context, input *struct{}) (*types.StatusResponse, error)
	CreateMessage(ctx context.Context, input *types.MessageRequest) (*types.MessageResponse, error)
	GetMessages(ctx context.Context, input *struct{}) (*types.MessagesResponse, error)
	SubscribeEvents(ctx context.Context, input *struct{}, send sse.Sender)
	// SubscribeConversations Was Initially SubscribeScreen, tbd whether we want to expose this in SDK mode TODO 1
	SubscribeConversations(ctx context.Context, input *struct{}, send sse.Sender)
}
