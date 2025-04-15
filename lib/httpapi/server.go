package httpapi

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/coder/agentapi/lib/logctx"
	mf "github.com/coder/agentapi/lib/msgfmt"
	st "github.com/coder/agentapi/lib/screentracker"
	"github.com/coder/agentapi/lib/termexec"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/danielgtaylor/huma/v2/sse"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"golang.org/x/xerrors"
)

// Server represents the HTTP server
type Server struct {
	router       chi.Router
	api          huma.API
	port         int
	srv          *http.Server
	mu           sync.RWMutex
	logger       *slog.Logger
	conversation *st.Conversation
	agentio      *termexec.Process
	agentType    mf.AgentType
	emitter      *EventEmitter
}

// NewServer creates a new server instance
func NewServer(ctx context.Context, agentType mf.AgentType, process *termexec.Process, port int) *Server {
	router := chi.NewMux()

	// Setup CORS middleware
	corsMiddleware := cors.New(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000", "*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300, // Maximum value not ignored by any of major browsers
	})
	router.Use(corsMiddleware.Handler)

	api := humachi.New(router, huma.DefaultConfig("AgentAPI API", "0.1.0"))
	formatMessage := func(message string, userInput string) string {
		return mf.FormatAgentMessage(agentType, message, userInput)
	}
	// That's about 40 frames per second. It's slightly less
	// because the action of taking a snapshot takes time too.
	snapshotInterval := 25 * time.Millisecond
	conversation := st.NewConversation(ctx, st.ConversationConfig{
		AgentIO: process,
		GetTime: func() time.Time {
			return time.Now()
		},
		SnapshotInterval:      snapshotInterval,
		ScreenStabilityLength: 2 * time.Second,
		FormatMessage:         formatMessage,
	})
	conversation.StartSnapshotLoop(ctx)
	emitter := NewEventEmitter(1024)
	go func() {
		for {
			emitter.UpdateStatusAndEmitChanges(conversation.Status())
			emitter.UpdateMessagesAndEmitChanges(conversation.Messages())
			emitter.UpdateScreenAndEmitChanges(conversation.Screen())
			time.Sleep(snapshotInterval)
		}
	}()

	s := &Server{
		router:       router,
		api:          api,
		port:         port,
		conversation: conversation,
		logger:       logctx.From(ctx),
		agentio:      process,
		agentType:    agentType,
		emitter:      emitter,
	}

	// Register API routes
	s.registerRoutes()

	return s
}

// registerRoutes sets up all API endpoints
func (s *Server) registerRoutes() {
	// GET /status endpoint
	huma.Get(s.api, "/status", s.getStatus)

	// GET /messages endpoint
	huma.Get(s.api, "/messages", s.getMessages)

	// POST /message endpoint
	huma.Post(s.api, "/message", s.createMessage)

	// GET /events endpoint
	sse.Register(s.api, huma.Operation{
		OperationID: "subscribeEvents",
		Method:      http.MethodGet,
		Path:        "/events",
		Summary:     "Subscribe to events",
	}, map[string]any{
		// Mapping of event type name to Go struct for that event.
		"message_update": MessageUpdateBody{},
		"status_change":  StatusChangeBody{},
	}, s.subscribeEvents)

	sse.Register(s.api, huma.Operation{
		OperationID: "subscribeScreen",
		Method:      http.MethodGet,
		Path:        "/internal/screen",
		Summary:     "Subscribe to screen",
		Hidden:      true,
	}, map[string]any{
		"screen": ScreenUpdateBody{},
	}, s.subscribeScreen)
}

// getStatus handles GET /status
func (s *Server) getStatus(ctx context.Context, input *struct{}) (*StatusResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status := s.conversation.Status()
	agentStatus := convertStatus(status)

	resp := &StatusResponse{}
	resp.Body.Status = string(agentStatus)

	return resp, nil
}

// getMessages handles GET /messages
func (s *Server) getMessages(ctx context.Context, input *struct{}) (*MessagesResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	resp := &MessagesResponse{}
	resp.Body.Messages = make([]Message, len(s.conversation.Messages()))
	for i, msg := range s.conversation.Messages() {
		resp.Body.Messages[i] = Message{
			Role:    string(msg.Role),
			Content: msg.Message,
		}
	}

	return resp, nil
}

// createMessage handles POST /message
func (s *Server) createMessage(ctx context.Context, input *MessageRequest) (*MessageResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch input.Body.Type {
	case MessageTypeUser:
		if err := s.conversation.SendMessage(FormatMessage(s.agentType, input.Body.Content)...); err != nil {
			return nil, xerrors.Errorf("failed to send message: %w", err)
		}
	case MessageTypeRaw:
		if _, err := s.agentio.Write([]byte(input.Body.Content)); err != nil {
			return nil, xerrors.Errorf("failed to send message: %w", err)
		}
	}

	resp := &MessageResponse{}
	resp.Body.Ok = true

	return resp, nil
}

// subscribeEvents is an SSE endpoint that sends events to the client
func (s *Server) subscribeEvents(ctx context.Context, input *struct{}, send sse.Sender) {
	subscriberId, ch, stateEvents := s.emitter.Subscribe()
	defer s.emitter.Unsubscribe(subscriberId)
	s.logger.Info("New subscriber", "subscriberId", subscriberId)
	for _, event := range stateEvents {
		if event.Type == EventTypeScreenUpdate {
			continue
		}
		if err := send.Data(event.Payload); err != nil {
			s.logger.Error("Failed to send event", "subscriberId", subscriberId, "error", err)
			return
		}
	}
	for {
		select {
		case event, ok := <-ch:
			if !ok {
				s.logger.Info("Channel closed", "subscriberId", subscriberId)
				return
			}
			if event.Type == EventTypeScreenUpdate {
				continue
			}
			if err := send.Data(event.Payload); err != nil {
				s.logger.Error("Failed to send event", "subscriberId", subscriberId, "error", err)
				return
			}
		case <-ctx.Done():
			s.logger.Info("Context done", "subscriberId", subscriberId)
			return
		}
	}
}

func (s *Server) subscribeScreen(ctx context.Context, input *struct{}, send sse.Sender) {
	subscriberId, ch, stateEvents := s.emitter.Subscribe()
	defer s.emitter.Unsubscribe(subscriberId)
	s.logger.Info("New screen subscriber", "subscriberId", subscriberId)
	for _, event := range stateEvents {
		if event.Type != EventTypeScreenUpdate {
			continue
		}
		if err := send.Data(event.Payload); err != nil {
			s.logger.Error("Failed to send screen event", "subscriberId", subscriberId, "error", err)
			return
		}
	}
	for {
		select {
		case event, ok := <-ch:
			if !ok {
				s.logger.Info("Screen channel closed", "subscriberId", subscriberId)
				return
			}
			if event.Type != EventTypeScreenUpdate {
				continue
			}
			if err := send.Data(event.Payload); err != nil {
				s.logger.Error("Failed to send screen event", "subscriberId", subscriberId, "error", err)
				return
			}
		case <-ctx.Done():
			s.logger.Info("Screen context done", "subscriberId", subscriberId)
			return
		}
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.port)
	s.srv = &http.Server{
		Addr:    addr,
		Handler: s.router,
	}

	return s.srv.ListenAndServe()
}

// Stop gracefully stops the HTTP server
func (s *Server) Stop(ctx context.Context) error {
	if s.srv != nil {
		return s.srv.Shutdown(ctx)
	}
	return nil
}
