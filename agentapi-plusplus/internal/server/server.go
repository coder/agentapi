package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/coder/agentapi/internal/routing"
)

// Server represents the agentapi HTTP server
type Server struct {
	port         int
	router       *routing.AgentBifrost
	agentHandler *AgentHandler
	server       *http.Server
}

// New creates a new agentapi server
func New(port int, router *routing.AgentBifrost) *Server {
	s := &Server{
		port:   port,
		router: router,
	}
	s.agentHandler = NewAgentHandler(router)
	return s
}

// Start starts the HTTP server
func (s *Server) Start() error {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.Logger)

	// Health check
	r.Get("/health", s.health)

	// Agent lifecycle endpoints
	s.agentHandler.RegisterRoutes(r)

	// Agent routing endpoints
	r.Post("/v1/chat/completions", s.chatCompletions)

	// Management endpoints
	r.Route("/admin", func(r chi.Router) {
		r.Get("/rules", s.listRules)
		r.Post("/rules", s.setRule)
		r.Get("/sessions", s.listSessions)
	})

	// Connect to cliproxy+bifrost
	r.HandleFunc("/proxy/*", s.proxy)

	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: r,
	}

	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown() {
	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.server.Shutdown(ctx)
	}
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) chatCompletions(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Agent  string `json:"agent"`
		Model  string `json:"model"`
		Prompt string `json:"prompt"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Use "default" if no agent specified
	agent := req.Agent
	if agent == "" {
		agent = "default"
	}

	resp, err := s.router.RouteRequest(r.Context(), agent, req.Prompt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) listRules(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"rules": "configured"})
}

func (s *Server) setRule(w http.ResponseWriter, r *http.Request) {
	var rule routing.RoutingRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.router.SetRule(rule)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) listSessions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"sessions": "active"})
}

func (s *Server) proxy(w http.ResponseWriter, r *http.Request) {
	// Proxy requests to cliproxy+bifrost
	path := chi.URLParam(r, "*")
	log.Printf("Proxying request to: %s", path)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"proxied": path,
		"method":  r.Method,
	})
}
