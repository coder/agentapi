package sdk

import (
	"context"
	"sync"

	mf "github.com/coder/agentapi/lib/cli/msgfmt"
	"github.com/coder/agentapi/lib/types"
	"github.com/danielgtaylor/huma/v2/sse"
)

type SDKHandler struct {
	agentType    mf.AgentType
	agentSDK     SDK
	conversation []types.ConversationMessage
	mu           sync.RWMutex
}

func NewSDKHandler(ctx context.Context) *SDKHandler {
	return &SDKHandler{}
}

func (s *SDKHandler) GetStatus(ctx context.Context, input *struct{}) (*types.StatusResponse, error) {
	return s.agentSDK.GetStatus()
}

func (s *SDKHandler) GetMessages(ctx context.Context, input *struct{}) (*types.MessagesResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	resp := &types.MessagesResponse{}
	resp.Body.Messages = make([]types.Message, len(s.conversation))
	for i, msg := range s.conversation {
		resp.Body.Messages[i] = types.Message{
			Id:      msg.Id,
			Role:    msg.Role,
			Content: msg.Message,
			Time:    msg.Time,
		}
	}

	return resp, nil
}

// CreateMessage handles POST /message
func (s *SDKHandler) CreateMessage(ctx context.Context, input *types.MessageRequest) (*types.MessageResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.agentSDK.QueryAgent(input.Body.Content)
	if err != nil {
		return nil, err
	}
	resp := &types.MessageResponse{}
	resp.Body.Ok = true
	return resp, nil
}

// SubscribeEvents is an SSE endpoint that sends events to the client
func (s *SDKHandler) SubscribeEvents(ctx context.Context, input *struct{}, send sse.Sender) {

}

func (s *SDKHandler) SubscribeConversations(ctx context.Context, input *struct{}, send sse.Sender) {
}

func (s *SDKHandler) StartSnapshotLoop(ctx context.Context) {
}
