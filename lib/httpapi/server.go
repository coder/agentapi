package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/coder/agentapi/internal/version"
	"github.com/coder/agentapi/lib/logctx"
	mf "github.com/coder/agentapi/lib/msgfmt"
	st "github.com/coder/agentapi/lib/screentracker"
	"github.com/coder/agentapi/lib/termexec"
	"github.com/coder/quartz"
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
	conversation st.Conversation
	agentio      *termexec.Process
	agentType    mf.AgentType
	emitter      *EventEmitter
	chatBasePath string
	tempDir      string
	clock        quartz.Clock
}

func (s *Server) NormalizeSchema(schema any) any {
	switch val := (schema).(type) {
	case *any:
		s.NormalizeSchema(*val)
	case []any:
		for i := range val {
			s.NormalizeSchema(&val[i])
		}
		sort.SliceStable(val, func(i, j int) bool {
			return fmt.Sprintf("%v", val[i]) < fmt.Sprintf("%v", val[j])
		})
	case map[string]any:
		for k := range val {
			valUnderKey := val[k]
			s.NormalizeSchema(&valUnderKey)
			val[k] = valUnderKey
		}
	}
	return schema
}

func (s *Server) GetOpenAPI() string {
	jsonBytes, err := s.api.OpenAPI().Downgrade()
	if err != nil {
		return ""
	}
	// unmarshal the json and pretty print it
	var jsonObj any
	if err := json.Unmarshal(jsonBytes, &jsonObj); err != nil {
		return ""
	}

	// Normalize
	normalized := s.NormalizeSchema(jsonObj)

	prettyJSON, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return ""
	}
	return string(prettyJSON)
}

// That's about 40 frames per second. It's slightly less
// because the action of taking a snapshot takes time too.
const snapshotInterval = 25 * time.Millisecond

type ServerConfig struct {
	AgentType      mf.AgentType
	Process        *termexec.Process
	Port           int
	ChatBasePath   string
	AllowedHosts   []string
	AllowedOrigins []string
	InitialPrompt  string
	Clock          quartz.Clock
}

// NewServer creates a new server instance
func NewServer(ctx context.Context, config ServerConfig) (*Server, error) {
	router := chi.NewMux()

	logger := logctx.From(ctx)

	if config.Clock == nil {
		config.Clock = quartz.NewReal()
	}

	allowedHosts, err := parseAllowedHosts(config.AllowedHosts)
	if err != nil {
		return nil, xerrors.Errorf("failed to parse allowed hosts: %w", err)
	}
	allowedOrigins, err := parseAllowedOrigins(config.AllowedOrigins)
	if err != nil {
		return nil, xerrors.Errorf("failed to parse allowed origins: %w", err)
	}

	logger.Info(fmt.Sprintf("Allowed hosts: %s", strings.Join(allowedHosts, ", ")))
	logger.Info(fmt.Sprintf("Allowed origins: %s", strings.Join(allowedOrigins, ", ")))

	// Enforce allowed hosts in a custom middleware that ignores the port during matching.
	badHostHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Invalid host header. Allowed hosts: "+strings.Join(allowedHosts, ", "), http.StatusBadRequest)
	})
	router.Use(hostAuthorizationMiddleware(allowedHosts, badHostHandler))

	corsMiddleware := cors.New(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300, // Maximum value not ignored by any of major browsers
	})
	router.Use(corsMiddleware.Handler)

	humaConfig := huma.DefaultConfig("AgentAPI", version.Version)
	humaConfig.Info.Description = "HTTP API for Claude Code, Goose, and Aider.\n\nhttps://github.com/coder/agentapi"
	api := humachi.New(router, humaConfig)
	formatMessage := func(message string, userInput string) string {
		return mf.FormatAgentMessage(config.AgentType, message, userInput)
	}

	isAgentReadyForInitialPrompt := func(message string) bool {
		return mf.IsAgentReadyForInitialPrompt(config.AgentType, message)
	}

	formatToolCall := func(message string) (string, []string) {
		return mf.FormatToolCall(config.AgentType, message)
	}

	emitter := NewEventEmitter(WithAgentType(config.AgentType))

	// Format initial prompt into message parts if provided
	var initialPrompt []st.MessagePart
	if config.InitialPrompt != "" {
		initialPrompt = FormatMessage(config.AgentType, config.InitialPrompt)
	}

	conversation := st.NewPTY(ctx, st.PTYConversationConfig{
		AgentType:             config.AgentType,
		AgentIO:               config.Process,
		Clock:                 config.Clock,
		SnapshotInterval:      snapshotInterval,
		ScreenStabilityLength: 2 * time.Second,
		FormatMessage:         formatMessage,
		ReadyForInitialPrompt: isAgentReadyForInitialPrompt,
		FormatToolCall:        formatToolCall,
		InitialPrompt:         initialPrompt,
		Logger:                logger,
	}, emitter)

	// Create temporary directory for uploads
	tempDir, err := os.MkdirTemp("", "agentapi-uploads-")
	if err != nil {
		return nil, xerrors.Errorf("failed to create temporary directory: %w", err)
	}
	logger.Info("Created temporary directory for uploads", "tempDir", tempDir)

	s := &Server{
		router:       router,
		api:          api,
		port:         config.Port,
		conversation: conversation,
		logger:       logger,
		agentio:      config.Process,
		agentType:    config.AgentType,
		emitter:      emitter,
		chatBasePath: strings.TrimSuffix(config.ChatBasePath, "/"),
		tempDir:      tempDir,
		clock:        config.Clock,
	}

	// Register API routes
	s.registerRoutes()

	// Start the conversation polling loop if we have a process.
	// Process is nil only when --print-openapi is used (no agent runs).
	// The process is already running at this point - termexec.StartProcess()
	// blocks until the PTY is created and the process is active. Agent
	// readiness (waiting for the prompt) is handled asynchronously inside
	// conversation.Start() via ReadyForInitialPrompt.
	if config.Process != nil {
		s.conversation.Start(ctx)
	}

	return s, nil
}

// Handler returns the underlying chi.Router for testing purposes.
func (s *Server) Handler() http.Handler {
	return s.router
}

// registerRoutes sets up all API endpoints
func (s *Server) registerRoutes() {
	// GET /ready endpoint - readiness probe
	huma.Get(s.api, "/ready", s.getReady, func(o *huma.Operation) {
		o.Description = "Readiness probe for Kubernetes."
	})

	// GET /logs endpoint
	huma.Get(s.api, "/logs", s.getLogs, func(o *huma.Operation) {
		o.Description = "Returns server logs."
	})

	// GET /rate-limit endpoint
	huma.Get(s.api, "/rate-limit", s.getRateLimit, func(o *huma.Operation) {
		o.Description = "Returns rate limit status."
	})

	// GET /config endpoint
	huma.Get(s.api, "/config", s.getConfig, func(o *huma.Operation) {
		o.Description = "Returns the server configuration."
	})

	// GET /health endpoint - liveness probe for load balancers
	huma.Get(s.api, "/health", s.getHealth, func(o *huma.Operation) {
		o.Description = "Health check endpoint for load balancers."
	})
	// GET /version endpoint
	huma.Get(s.api, "/version", s.getVersion, func(o *huma.Operation) {
		o.Description = "Returns the server version."
	})

	// GET /status endpoint
	huma.Get(s.api, "/status", s.getStatus, func(o *huma.Operation) {
		o.Description = "Returns the current status of the agent."
	})
	// GET /info endpoint - returns agent and server info
	huma.Get(s.api, "/info", s.getInfo, func(o *huma.Operation) {
		o.Description = "Returns information about the server and agent."
	})

	// GET /messages endpoint
	// Query params: after (int) - return messages after this ID, limit (int) - limit results
	huma.Get(s.api, "/messages", s.getMessages, func(o *huma.Operation) {
		o.Description = "Returns a list of messages representing the conversation history with the agent. Supports ?after=<id> and ?limit=<n> query parameters for pagination."
	})

	// DELETE /messages endpoint - clear all messages
	huma.Delete(s.api, "/messages", s.clearMessages, func(o *huma.Operation) {
		o.Description = "Clear all messages from conversation history."
	})
	// GET /messages/count endpoint
	huma.Get(s.api, "/messages/count", s.getMessagesCount, func(o *huma.Operation) {
		o.Description = "Returns the count of messages in the conversation."
	})

	// POST /message endpoint
	huma.Post(s.api, "/message", s.createMessage, func(o *huma.Operation) {
		o.Description = "Send a message to the agent. For messages of type 'user', the agent's status must be 'stable' for the operation to complete successfully. Otherwise, this endpoint will return an error."
	})

	huma.Post(s.api, "/upload", s.uploadFiles, func(o *huma.Operation) {
		o.Description = "Upload files to the specified upload path."
	})

	// GET /events endpoint
	sse.Register(s.api, huma.Operation{
		OperationID: "subscribeEvents",
		Method:      http.MethodGet,
		Path:        "/events",
		Summary:     "Subscribe to events",
		Description: "The events are sent as Server-Sent Events (SSE). Initially, the endpoint returns a list of events needed to reconstruct the current state of the conversation and the agent's status. After that, it only returns events that have occurred since the last event was sent.\n\nNote: When an agent is running, the last message in the conversation history is updated frequently, and the endpoint sends a new message update event each time.",
		Middlewares: []func(huma.Context, func(huma.Context)){sseMiddleware},
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
		Middlewares: []func(huma.Context, func(huma.Context)){sseMiddleware},
	}, map[string]any{
		"screen": ScreenUpdateBody{},
	}, s.subscribeScreen)

	s.router.Handle("/", http.HandlerFunc(s.redirectToChat))

	// Serve static files for the chat interface under /chat
	s.registerStaticFileRoutes()
}

// getLogs handles GET /logs
func (s *Server) getLogs(ctx context.Context, input *struct{}) (*LogsResponse, error) {
	resp := &LogsResponse{}
	resp.Body.Logs = []string{}
	return resp, nil
}

// getRateLimit handles GET /rate-limit
func (s *Server) getRateLimit(ctx context.Context, input *struct{}) (*RateLimitResponse, error) {
	resp := &RateLimitResponse{}
	resp.Body.Enabled = false
	resp.Body.Requests = 100
	return resp, nil
}

// getConfig handles GET /config
func (s *Server) getConfig(ctx context.Context, input *struct{}) (*ConfigResponse, error) {
	resp := &ConfigResponse{}
	resp.Body.AgentType = string(s.agentType)
	resp.Body.Port = s.port
	return resp, nil
}

// getHealth handles GET /health
func (s *Server) getHealth(ctx context.Context, input *struct{}) (*HealthResponse, error) {
	resp := &HealthResponse{}
	resp.Body.Status = "ok"
	return resp, nil
}

// getVersion handles GET /version
func (s *Server) getVersion(ctx context.Context, input *struct{}) (*VersionResponse, error) {
	resp := &VersionResponse{}
	resp.Body.Version = version.Version
	return resp, nil
}

// getInfo handles GET /info
func (s *Server) getInfo(ctx context.Context, input *struct{}) (*InfoResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	resp := &InfoResponse{}
	resp.Body.Version = version.Version
	resp.Body.AgentType = s.agentType
	resp.Body.Features = map[string]bool{
		"messages":    true,
		"events":      true,
		"upload":      true,
		"pagination":  true,
		"slashCmd":    true,
	}
	return resp, nil
}

// getReady handles GET /ready
func (s *Server) getReady(ctx context.Context, input *struct{}) (*ReadyResponse, error) {
	resp := &ReadyResponse{}
	resp.Body.Ready = true
	return resp, nil
}

// getStatus handles GET /status
func (s *Server) getStatus(ctx context.Context, input *struct{}) (*StatusResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status := s.conversation.Status()
	agentStatus, err := convertStatus(status)
	if err != nil {
		return nil, xerrors.Errorf("failed to convert status: %w", err)
	}

	resp := &StatusResponse{}
	resp.Body.Status = agentStatus
	resp.Body.AgentType = s.agentType

	return resp, nil
}

// getMessages handles GET /messages
//
//	@param after (query) int "Return messages after this ID"
//	@param limit (query) int "Limit number of messages returned"
func (s *Server) getMessages(ctx context.Context, input *struct {
	After *int `json:"after,optional"`
	Limit *int `json:"limit,optional"`
}) (*MessagesResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	allMessages := s.conversation.Messages()

	// Filter by 'after' parameter
	messages := allMessages
	if input.After != nil {
		afterID := *input.After
		filtered := make([]st.ConversationMessage, 0)
		for _, msg := range allMessages {
			if msg.Id > afterID {
				filtered = append(filtered, msg)
			}
		}
		messages = filtered
	}

	// Apply limit
	if input.Limit != nil && *input.Limit > 0 {
		limit := *input.Limit
		if len(messages) > limit {
			messages = messages[:limit]
		}
	}

	resp := &MessagesResponse{}
	resp.Body.Messages = make([]Message, len(messages))
	for i, msg := range messages {
		resp.Body.Messages[i] = Message{
			Id:      msg.Id,
			Role:    msg.Role,
			Content: msg.Message,
			Time:    msg.Time,
		}
	}

	return resp, nil
}

// clearMessages handles DELETE /messages
func (s *Server) clearMessages(ctx context.Context, input *struct{}) (*MessagesClearResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	resp := &MessagesClearResponse{}
	count := len(s.conversation.Messages())
	if clearer, ok := any(s.conversation).(interface{ ClearMessages() }); ok {
		clearer.ClearMessages()
	}
	resp.Body.Ok = true
	resp.Body.Count = count
	return resp, nil
}

// getMessagesCount handles GET /messages/count
func (s *Server) getMessagesCount(ctx context.Context, input *struct{}) (*MessagesCountResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	resp := &MessagesCountResponse{}
	resp.Body.Count = len(s.conversation.Messages())
	return resp, nil
}

// createMessage handles POST /message
func (s *Server) createMessage(ctx context.Context, input *MessageRequest) (*MessageResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch input.Body.Type {
	case MessageTypeUser:
		if err := s.conversation.Send(FormatMessage(s.agentType, input.Body.Content)...); err != nil {
			return nil, xerrors.Errorf("failed to send message: %w", err)
		}
	case MessageTypeRaw:
		if _, err := s.agentio.Write([]byte(input.Body.Content)); err != nil {
			return nil, xerrors.Errorf("failed to send message: %w", err)
		}
	case MessageTypeCommand:
		// Send slash command directly - add enter at the end
		content := input.Body.Content
		if !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		if _, err := s.agentio.Write([]byte(content)); err != nil {
			return nil, xerrors.Errorf("failed to send command: %w", err)
		}
	}

	resp := &MessageResponse{}
	resp.Body.Ok = true

	return resp, nil
}

// uploadFiles handles POST /upload
func (s *Server) uploadFiles(ctx context.Context, input *struct {
	RawBody huma.MultipartFormFiles[UploadRequest]
},
) (*UploadResponse, error) {
	formData := input.RawBody.Data()

	file := formData.File.File

	// Limit file size to 10MB
	const maxFileSize = 10 << 20 // 10MB
	buf, err := io.ReadAll(io.LimitReader(file, maxFileSize+1))
	if err != nil {
		return nil, xerrors.Errorf("failed to upload file: %w", err)
	}
	if len(buf) > maxFileSize {
		return nil, huma.Error400BadRequest("file size exceeds 10MB limit")
	}

	// Calculate checksum of the uploaded file to create unique subdirectory
	hash := sha256.Sum256(buf)
	checksum := hex.EncodeToString(hash[:8]) // Use first 8 bytes (16 hex chars)

	// Create checksum-based subdirectory in tempDir
	uploadDir := filepath.Join(s.tempDir, checksum)
	err = os.MkdirAll(uploadDir, 0o755)
	if err != nil {
		return nil, xerrors.Errorf("failed to create upload directory: %w", err)
	}

	// Save individual file with original filename (extract just the base filename for security)
	filename := filepath.Base(formData.File.Filename)

	outPath := filepath.Join(uploadDir, filename)
	err = os.WriteFile(outPath, buf, 0o644)
	if err != nil {
		return nil, xerrors.Errorf("failed to write file: %w", err)
	}

	resp := &UploadResponse{}
	resp.Body.Ok = true
	resp.Body.FilePath = outPath
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
	// Clean up temporary directory
	s.cleanupTempDir()

	if s.srv != nil {
		return s.srv.Shutdown(ctx)
	}
	return nil
}

// cleanupTempDir removes the temporary directory and all its contents
func (s *Server) cleanupTempDir() {
	if err := os.RemoveAll(s.tempDir); err != nil {
		s.logger.Error("Failed to clean up temporary directory", "tempDir", s.tempDir, "error", err)
	} else {
		s.logger.Info("Cleaned up temporary directory", "tempDir", s.tempDir)
	}
}

// registerStaticFileRoutes sets up routes for serving static files
func (s *Server) registerStaticFileRoutes() {
	chatHandler := FileServerWithIndexFallback(s.chatBasePath)

	// Mount the file server at /chat
	s.router.Handle("/chat", http.StripPrefix("/chat", chatHandler))
	s.router.Handle("/chat/*", http.StripPrefix("/chat", chatHandler))
}
