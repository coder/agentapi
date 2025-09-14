package httpapi

import (
	"context"

	"github.com/coder/agentapi/lib/types"
	"github.com/danielgtaylor/huma/v2/sse"
)

// InteractionHandler defines the interface that all interaction modes must implement
type InteractionHandler interface {
	GetStatus(ctx context.Context, input *struct{}) (*types.StatusResponse, error)
	CreateMessage(ctx context.Context, input *types.MessageRequest) (*types.MessageResponse, error)
	GetMessages(ctx context.Context, input *struct{}) (*types.MessagesResponse, error)
	SubscribeConversations(ctx context.Context, input *struct{}, send sse.Sender)
	SubscribeEvents(ctx context.Context, input *struct{}, send sse.Sender)
	//StartSnapshotLoop(ctx context.Context)
}
