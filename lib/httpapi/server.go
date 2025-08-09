package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
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
	chatBasePath string
}

func (s *Server) GetOpenAPI() string {
	jsonBytes, err := s.api.OpenAPI().MarshalJSON()
	if err != nil {
		return ""
	}
	// unmarshal the json and pretty print it
	var jsonObj any
	if err := json.Unmarshal(jsonBytes, &jsonObj); err != nil {
		return ""
	}
	prettyJSON, err := json.MarshalIndent(jsonObj, "", "  ")
	if err != nil {
		return ""
	}
	return string(prettyJSON)
}

// That's about 40 frames per second. It's slightly less
// because the action of taking a snapshot takes time too.
const snapshotInterval = 25 * time.Millisecond

type ServerConfig struct {
	AgentType          mf.AgentType
	Process            *termexec.Process
	Port               int
	ChatBasePath       string
	AllowedHosts       []string
	AllowedOrigins     []string
	UseXForwardedHost  bool
}

func parseAllowedHosts(hosts []string) ([]string, error) {
	if slices.Contains(hosts, "*") {
		return []string{}, nil
	}
	for _, host := range hosts {
		if strings.Contains(host, "*") {
			return nil, xerrors.Errorf("wildcard characters are not supported: %q", host)
		}
		if strings.Contains(host, "http://") || strings.Contains(host, "https://") {
			return nil, xerrors.Errorf("host must not contain http:// or https://: %q", host)
		}
	}
	// Normalize hosts to bare hostnames/IPs by stripping any port and brackets.
	// This ensures allowed entries match the Host header hostname only.
	normalized := make([]string, 0, len(hosts))
	for _, raw := range hosts {
		h := strings.TrimSpace(raw)
		// If it's an IPv6 literal (possibly bracketed) without an obvious port, keep the literal.
		unbracketed := strings.Trim(h, "[]")
		if ip := net.ParseIP(unbracketed); ip != nil {
			// It's an IP literal; use the bare form without brackets.
			normalized = append(normalized, unbracketed)
			continue
		}
		// If likely host:port (single colon) or bracketed host, use url.Parse to extract hostname.
		if strings.Count(h, ":") == 1 || (strings.HasPrefix(h, "[") && strings.Contains(h, "]")) {
			if u, err := url.Parse("http://" + h); err == nil {
				hn := u.Hostname()
				if hn != "" {
					normalized = append(normalized, hn)
					continue
				}
			}
		}
		// Fallback: use as-is (e.g., hostname without port)
		normalized = append(normalized, h)
	}
	return normalized, nil
}

func parseAllowedOrigins(origins []string) ([]string, error) {
	for _, origin := range origins {
		if !strings.Contains(origin, "*") && !(strings.Contains(origin, "http://") || strings.Contains(origin, "https://")) {
			return nil, xerrors.Errorf("origin must contain http:// or https://: %q", origin)
		}
	}
	return origins, nil
}

// NewServer creates a new server instance
func NewServer(ctx context.Context, config ServerConfig) (*Server, error) {
	router := chi.NewMux()

	logger := logctx.From(ctx)

	allowedHosts, err := parseAllowedHosts(config.AllowedHosts)
	if err != nil {
		return nil, xerrors.Errorf("failed to validate allowed hosts: %w", err)
	}
	if len(allowedHosts) > 0 {
		logger.Info(fmt.Sprintf("Allowed hosts: %s", strings.Join(allowedHosts, ", ")))
	} else {
		logger.Info("Allowed hosts: *")
	}

	allowedOrigins, err := parseAllowedOrigins(config.AllowedOrigins)
	if err != nil {
		return nil, xerrors.Errorf("failed to validate allowed origins: %w", err)
	}
	logger.Info(fmt.Sprintf("Allowed origins: %s", strings.Join(allowedOrigins, ", ")))

	// Enforce allowed hosts in a custom middleware that ignores the port during matching.
	badHostHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Invalid host header. Allowed hosts: "+strings.Join(allowedHosts, ", "), http.StatusBadRequest)
	})
	router.Use(hostAuthorizationMiddleware(allowedHosts, config.UseXForwardedHost, badHostHandler))

	corsMiddleware := cors.New(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300, // Maximum value not ignored by any of major browsers
	})
	router.Use(corsMiddleware.Handler)

	humaConfig := huma.DefaultConfig("AgentAPI", "0.3.3")
	humaConfig.Info.Description = "HTTP API for Claude Code, Goose, and Aider.\n\nhttps://github.com/coder/agentapi"
	api := humachi.New(router, humaConfig)
	formatMessage := func(message string, userInput string) string {
		return mf.FormatAgentMessage(config.AgentType, message, userInput)
	}
	conversation := st.NewConversation(ctx, st.ConversationConfig{
		AgentIO: config.Process,
		GetTime: func() time.Time {
			return time.Now()
		},
		SnapshotInterval:      snapshotInterval,
		ScreenStabilityLength: 2 * time.Second,
		FormatMessage:         formatMessage,
	})
	emitter := NewEventEmitter(1024)
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
	}

	// Register API routes
	s.registerRoutes()

	return s, nil
}

// Handler returns the underlying chi.Router for testing purposes.
func (s *Server) Handler() http.Handler {
	return s.router
}

// hostAuthorizationMiddleware enforces that the request Host header matches one of the allowed
// hosts, ignoring any port in the comparison. If allowedHosts is empty, all hosts are allowed.
// If useXForwardedHost is true and the X-Forwarded-Host header is present, that header is used
// as the source of host. Hostname is extracted via url.Parse to handle IPv6 and strip ports.
func hostAuthorizationMiddleware(allowedHosts []string, useXForwardedHost bool, badHostHandler http.Handler) func(next http.Handler) http.Handler {
	// Copy for safety; also build a map for O(1) lookups with case-insensitive keys.
	allowed := make(map[string]struct{}, len(allowedHosts))
	for _, h := range allowedHosts {
		allowed[strings.ToLower(h)] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if len(allowedHosts) == 0 { // wildcard semantics: allow all
				next.ServeHTTP(w, r)
				return
			}
			// Choose header source
			rawHost := r.Host
			if useXForwardedHost {
				if xfhs := r.Header.Values("X-Forwarded-Host"); len(xfhs) > 0 {
					// Use the first value and trim anything after a comma
					h := xfhs[0]
					if idx := strings.IndexByte(h, ','); idx >= 0 {
						h = h[:idx]
					}
					rawHost = strings.TrimSpace(h)
				}
			}
			if rawHost == "" {
				badHostHandler.ServeHTTP(w, r)
				return
			}
			// Extract hostname via url.Parse; ignore any port.
			if u, err := url.Parse("http://" + rawHost); err == nil {
				hostname := u.Hostname()
				if _, ok := allowed[strings.ToLower(hostname)]; ok {
					next.ServeHTTP(w, r)
					return
				}
			}
			badHostHandler.ServeHTTP(w, r)
		})
	}
}

func (s *Server) StartSnapshotLoop(ctx context.Context) {
	s.conversation.StartSnapshotLoop(ctx)
	go func() {
		for {
			s.emitter.UpdateStatusAndEmitChanges(s.conversation.Status())
			s.emitter.UpdateMessagesAndEmitChanges(s.conversation.Messages())
			s.emitter.UpdateScreenAndEmitChanges(s.conversation.Screen())
			time.Sleep(snapshotInterval)
		}
	}()
}

// registerRoutes sets up all API endpoints
func (s *Server) registerRoutes() {
	// GET /status endpoint
	huma.Get(s.api, "/status", s.getStatus, func(o *huma.Operation) {
		o.Description = "Returns the current status of the agent."
	})

	// GET /messages endpoint
	huma.Get(s.api, "/messages", s.getMessages, func(o *huma.Operation) {
		o.Description = "Returns a list of messages representing the conversation history with the agent."
	})

	// POST /message endpoint
	huma.Post(s.api, "/message", s.createMessage, func(o *huma.Operation) {
		o.Description = "Send a message to the agent. For messages of type 'user', the agent's status must be 'stable' for the operation to complete successfully. Otherwise, this endpoint will return an error."
	})

	// GET /events endpoint
	sse.Register(s.api, huma.Operation{
		OperationID: "subscribeEvents",
		Method:      http.MethodGet,
		Path:        "/events",
		Summary:     "Subscribe to events",
		Description: "The events are sent as Server-Sent Events (SSE). Initially, the endpoint returns a list of events needed to reconstruct the current state of the conversation and the agent's status. After that, it only returns events that have occurred since the last event was sent.\n\nNote: When an agent is running, the last message in the conversation history is updated frequently, and the endpoint sends a new message update event each time.",
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

	s.router.Handle("/", http.HandlerFunc(s.redirectToChat))

	// Serve static files for the chat interface under /chat
	s.registerStaticFileRoutes()
}

// getStatus handles GET /status
func (s *Server) getStatus(ctx context.Context, input *struct{}) (*StatusResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status := s.conversation.Status()
	agentStatus := convertStatus(status)

	resp := &StatusResponse{}
	resp.Body.Status = agentStatus

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
			Id:      msg.Id,
			Role:    msg.Role,
			Content: msg.Message,
			Time:    msg.Time,
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

// registerStaticFileRoutes sets up routes for serving static files
func (s *Server) registerStaticFileRoutes() {
	chatHandler := FileServerWithIndexFallback(s.chatBasePath)

	// Mount the file server at /chat
	s.router.Handle("/chat", http.StripPrefix("/chat", chatHandler))
	s.router.Handle("/chat/*", http.StripPrefix("/chat", chatHandler))
}

func (s *Server) redirectToChat(w http.ResponseWriter, r *http.Request) {
	rdir, err := url.JoinPath(s.chatBasePath, "embed")
	if err != nil {
		s.logger.Error("Failed to construct redirect URL", "error", err)
		http.Error(w, "Failed to redirect", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, rdir, http.StatusTemporaryRedirect)
}
