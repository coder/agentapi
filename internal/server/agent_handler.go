package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/coder/agentapi/internal/routing"
)

// AgentHandler handles agent lifecycle endpoints
type AgentHandler struct {
	router   *routing.AgentBifrost
	sessions map[string]*AgentSessionInfo
	mut      sync.RWMutex
}

// AgentSessionInfo tracks running agent sessions
type AgentSessionInfo struct {
	ID       string    `json:"id"`
	Agent    string    `json:"agent"`
	Model    string    `json:"model"`
	Status   string    `json:"status"` // running, completed, failed, stopped
	Started  time.Time `json:"started"`
	Ended    time.Time `json:"ended,omitempty"`
	ExitCode int       `json:"exit_code,omitempty"`
	Output   string    `json:"output,omitempty"`
	Error    string    `json:"error,omitempty"`
	WorkDir  string    `json:"work_dir"`
	Prompt   string    `json:"prompt,omitempty"`
}

// StartAgentRequest is the request body for starting an agent
type StartAgentRequest struct {
	Agent   string `json:"agent"`             // claude, codex, gemini
	Model   string `json:"model"`             // model to use
	Prompt  string `json:"prompt,omitempty"` // initial prompt
	WorkDir string `json:"cwd,omitempty"`    // working directory
}

// StartAgentResponse is returned when an agent is started
type StartAgentResponse struct {
	SessionID string `json:"session_id"`
	Status    string `json:"status"`
	Message   string `json:"message,omitempty"`
}

// AgentStatusResponse returns session status
type AgentStatusResponse struct {
	SessionID string    `json:"session_id"`
	Status    string    `json:"status"`
	Agent     string    `json:"agent"`
	Model     string    `json:"model"`
	Started   time.Time `json:"started"`
	Duration  string    `json:"duration,omitempty"`
	Output    string    `json:"output,omitempty"`
	Error     string    `json:"error,omitempty"`
}

// ModelRunRequest for one-shot model commands
type ModelRunRequest struct {
	Model   string `json:"model"`
	Prompt  string `json:"prompt"`
	WorkDir string `json:"cwd,omitempty"`
}

// ModelRunResponse for one-shot model commands
type ModelRunResponse struct {
	Output   string `json:"output"`
	ExitCode int    `json:"exit_code"`
	Error    string `json:"error,omitempty"`
}

// ModelInfo describes an available model
type ModelInfo struct {
	ID           string   `json:"id"`
	Provider     string   `json:"provider"`
	Capabilities []string `json:"capabilities"`
}

// NewAgentHandler creates a new agent handler
func NewAgentHandler(router *routing.AgentBifrost) *AgentHandler {
	return &AgentHandler{
		router:   router,
		sessions: make(map[string]*AgentSessionInfo),
	}
}

// HandleStartAgent POST /agent/start
func (h *AgentHandler) HandleStartAgent(w http.ResponseWriter, r *http.Request) {
	var req StartAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Agent == "" {
		req.Agent = "claude"
	}
	if req.Model == "" {
		req.Model = "default"
	}

	// Generate session ID
	sessionID := fmt.Sprintf("sess_%d_%s", time.Now().UnixNano(), req.Agent)

	// Create session
	session := &AgentSessionInfo{
		ID:      sessionID,
		Agent:   req.Agent,
		Model:   req.Model,
		Status:  "running",
		Started: time.Now(),
		WorkDir: req.WorkDir,
		Prompt:  req.Prompt,
	}

	h.mut.Lock()
	h.sessions[sessionID] = session
	h.mut.Unlock()

	log.Printf("Started agent session: %s (agent=%s, model=%s)", sessionID, req.Agent, req.Model)

	resp := StartAgentResponse{
		SessionID: sessionID,
		Status:    "running",
		Message:   fmt.Sprintf("Agent %s started with model %s", req.Agent, req.Model),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// HandleAgentStatus GET /agent/{id}/status
func (h *AgentHandler) HandleAgentStatus(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "id")
	if sessionID == "" {
		http.Error(w, "missing session id", http.StatusBadRequest)
		return
	}

	h.mut.RLock()
	session, ok := h.sessions[sessionID]
	h.mut.RUnlock()

	if !ok {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	duration := ""
	if session.Status != "running" {
		duration = time.Since(session.Started).Round(time.Millisecond).String()
	}

	resp := AgentStatusResponse{
		SessionID: session.ID,
		Status:    session.Status,
		Agent:     session.Agent,
		Model:     session.Model,
		Started:   session.Started,
		Duration:  duration,
		Output:    session.Output,
		Error:     session.Error,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// HandleAgentStop POST /agent/{id}/stop
func (h *AgentHandler) HandleAgentStop(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "id")
	if sessionID == "" {
		http.Error(w, "missing session id", http.StatusBadRequest)
		return
	}

	h.mut.Lock()
	session, ok := h.sessions[sessionID]
	if ok && session.Status == "running" {
		session.Status = "stopped"
		session.Ended = time.Now()
	}
	h.mut.Unlock()

	if !ok {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "stopped",
		"message": fmt.Sprintf("Session %s stopped", sessionID),
	})
}

// HandleAgentLogs GET /agent/{id}/logs (SSE endpoint)
func (h *AgentHandler) HandleAgentLogs(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "id")
	if sessionID == "" {
		http.Error(w, "missing session id", http.StatusBadRequest)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	// Send initial event
	fmt.Fprintf(w, "data: {\"type\":\"connected\",\"session_id\":\"%s\"}\n\n", sessionID)
	flusher.Flush()

	// In a real implementation, this would stream logs from the agent process
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			h.mut.RLock()
			session, ok := h.sessions[sessionID]
			h.mut.RUnlock()

			if !ok || session.Status != "running" {
				fmt.Fprintf(w, "data: {\"type\":\"end\",\"status\":\"%s\"}\n\n", session.Status)
				flusher.Flush()
				return
			}

			fmt.Fprintf(w, "data: {\"type\":\"heartbeat\",\"time\":\"%s\"}\n\n", time.Now().Format(time.RFC3339))
			flusher.Flush()
		}
	}
}

// HandleModelRun POST /model/run - one-shot model command
func (h *AgentHandler) HandleModelRun(w http.ResponseWriter, r *http.Request) {
	var req ModelRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// In a real implementation, this would execute the model and return output
	resp := ModelRunResponse{
		Output:   fmt.Sprintf("Model %s executed prompt: %s", req.Model, req.Prompt),
		ExitCode: 0,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// HandleModels GET /models - list available models
func (h *AgentHandler) HandleModels(w http.ResponseWriter, r *http.Request) {
	models := []ModelInfo{
		{ID: "claude-sonnet-4", Provider: "anthropic", Capabilities: []string{"text", "vision", "reasoning"}},
		{ID: "claude-opus-4", Provider: "anthropic", Capabilities: []string{"text", "vision", "reasoning"}},
		{ID: "claude-haiku-4", Provider: "anthropic", Capabilities: []string{"text", "vision"}},
		{ID: "gpt-4o", Provider: "openai", Capabilities: []string{"text", "vision"}},
		{ID: "gpt-4o-mini", Provider: "openai", Capabilities: []string{"text"}},
		{ID: "gemini-2.0-flash", Provider: "google", Capabilities: []string{"text", "vision"}},
		{ID: "codex", Provider: "openai", Capabilities: []string{"code"}},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"models": models,
	})
}

// RegisterRoutes registers agent routes on the chi router
func (h *AgentHandler) RegisterRoutes(r chi.Router) {
	r.Post("/agent/start", h.HandleStartAgent)
	r.Get("/agent/{id}/status", h.HandleAgentStatus)
	r.Post("/agent/{id}/stop", h.HandleAgentStop)
	r.Get("/agent/{id}/logs", h.HandleAgentLogs)
	r.Post("/model/run", h.HandleModelRun)
	r.Get("/models", h.HandleModels)
}
