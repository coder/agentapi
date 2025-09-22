package cli

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	mf "github.com/coder/agentapi/lib/cli/msgfmt"
	st "github.com/coder/agentapi/lib/cli/screentracker"
	"github.com/coder/agentapi/lib/cli/termexec"
	"github.com/coder/agentapi/lib/types"
	"github.com/danielgtaylor/huma/v2/sse"
	"golang.org/x/xerrors"
)

type CLIHandler struct {
	emitter      *EventEmitter
	conversation *st.Conversation
	agentio      *termexec.Process
	mu           sync.RWMutex
	agentType    mf.AgentType
	logger       *slog.Logger
}

const snapshotInterval = 25 * time.Millisecond

func convertStatus(status st.ConversationStatus) types.AgentStatus {
	switch status {
	case st.ConversationStatusInitializing:
		return types.AgentStatusRunning
	case st.ConversationStatusStable:
		return types.AgentStatusStable
	case st.ConversationStatusChanging:
		return types.AgentStatusRunning
	default:
		panic(fmt.Sprintf("unknown conversation status: %s", status))
	}
}

func NewCLIHandler(ctx context.Context, logger *slog.Logger, agentio *termexec.Process, agentType mf.AgentType) *CLIHandler {
	formatMessage := func(message string, userInput string) string {
		return mf.FormatAgentMessage(agentType, message, userInput)
	}

	conversation := st.NewConversation(ctx, st.ConversationConfig{
		AgentIO: agentio,
		GetTime: func() time.Time {
			return time.Now()
		},
		SnapshotInterval:      25 * time.Millisecond,
		ScreenStabilityLength: 2 * time.Second,
		FormatMessage:         formatMessage,
	})

	emitter := NewEventEmitter(1024)

	handler := &CLIHandler{
		emitter:      emitter,
		conversation: conversation,
		agentio:      agentio,
		agentType:    agentType,
		logger:       logger,
	}

	return handler
}

// GetStatus handles GET /status
func (c *CLIHandler) GetStatus(ctx context.Context, input *struct{}) (*types.StatusResponse, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	status := c.conversation.Status()
	agentStatus := convertStatus(status)

	resp := &types.StatusResponse{}
	resp.Body.Status = agentStatus

	return resp, nil
}

// GetMessages handles GET /messages
func (c *CLIHandler) GetMessages(ctx context.Context, input *struct{}) (*types.MessagesResponse, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	resp := &types.MessagesResponse{}
	resp.Body.Messages = make([]types.Message, len(c.conversation.Messages()))
	for i, msg := range c.conversation.Messages() {
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
func (c *CLIHandler) CreateMessage(ctx context.Context, input *types.MessageRequest) (*types.MessageResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	switch input.Body.Type {
	case types.MessageTypeUser:
		if err := c.conversation.SendMessage(FormatMessage(c.agentType, input.Body.Content)...); err != nil {
			return nil, xerrors.Errorf("failed to send message: %w", err)
		}
	case types.MessageTypeRaw:
		if _, err := c.agentio.Write([]byte(input.Body.Content)); err != nil {
			return nil, xerrors.Errorf("failed to send message: %w", err)
		}
	}

	resp := &types.MessageResponse{}
	resp.Body.Ok = true

	return resp, nil
}

// SubscribeEvents is an SSE endpoint that sends events to the client
func (c *CLIHandler) SubscribeEvents(ctx context.Context, input *struct{}, send sse.Sender) {
	subscriberId, ch, stateEvents := c.emitter.Subscribe()
	defer c.emitter.Unsubscribe(subscriberId)
	c.logger.Info("New subscriber", "subscriberId", subscriberId)
	for _, event := range stateEvents {
		if event.Type == EventTypeScreenUpdate {
			continue
		}
		if err := send.Data(event.Payload); err != nil {
			c.logger.Error("Failed to send event", "subscriberId", subscriberId, "error", err)
			return
		}
	}
	for {
		select {
		case event, ok := <-ch:
			if !ok {
				c.logger.Info("Channel closed", "subscriberId", subscriberId)
				return
			}
			if event.Type == EventTypeScreenUpdate {
				continue
			}
			if err := send.Data(event.Payload); err != nil {
				c.logger.Error("Failed to send event", "subscriberId", subscriberId, "error", err)
				return
			}
		case <-ctx.Done():
			c.logger.Info("Context done", "subscriberId", subscriberId)
			return
		}
	}
}

func (c *CLIHandler) SubscribeConversations(ctx context.Context, input *struct{}, send sse.Sender) {
	subscriberId, ch, stateEvents := c.emitter.Subscribe()
	defer c.emitter.Unsubscribe(subscriberId)
	c.logger.Info("New screen subscriber", "subscriberId", subscriberId)
	for _, event := range stateEvents {
		if event.Type != EventTypeScreenUpdate {
			continue
		}
		if err := send.Data(event.Payload); err != nil {
			c.logger.Error("Failed to send screen event", "subscriberId", subscriberId, "error", err)
			return
		}
	}
	for {
		select {
		case event, ok := <-ch:
			if !ok {
				c.logger.Info("Screen channel closed", "subscriberId", subscriberId)
				return
			}
			if event.Type != EventTypeScreenUpdate {
				continue
			}
			if err := send.Data(event.Payload); err != nil {
				c.logger.Error("Failed to send screen event", "subscriberId", subscriberId, "error", err)
				return
			}
		case <-ctx.Done():
			c.logger.Info("Screen context done", "subscriberId", subscriberId)
			return
		}
	}
}

func (c *CLIHandler) StartSnapshotLoop(ctx context.Context) {
	c.conversation.StartSnapshotLoop(ctx)
	go func() {
		for {
			c.emitter.UpdateStatusAndEmitChanges(c.conversation.Status())
			c.emitter.UpdateMessagesAndEmitChanges(c.conversation.Messages())
			c.emitter.UpdateScreenAndEmitChanges(c.conversation.Screen())
			time.Sleep(snapshotInterval)
		}
	}()
}
