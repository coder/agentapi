package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kooshapari/agentapi/internal/routing"
)

// Server represents the agentapi HTTP server
type Server struct {
	port   int
	router *routing.AgentBifrost
	server *http.Server
}

// New creates a new agentapi server
func New(port int, router *routing.AgentBifrost) *Server {
	return &Server{
		port:   port,
		router: router,
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	
	// Health check
	r.GET("/health", s.health)
	
	// Agent routing endpoints
	r.POST("/v1/chat/completions", s.chatCompletions)
	
	// Management endpoints
	r.GET("/admin/rules", s.listRules)
	r.POST("/admin/rules", s.setRule)
	r.GET("/admin/sessions", s.listSessions)
	
	// Connect to cliproxy+bifrost
	r.Any("/proxy/*path", s.proxy)
	
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

func (s *Server) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}

func (s *Server) chatCompletions(c *gin.Context) {
	var req struct {
		Agent  string `json:"agent"`
		Model  string `json:"model"`
		Prompt string `json:"prompt"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	// Use "default" if no agent specified
	agent := req.Agent
	if agent == "" {
		agent = "default"
	}
	
	resp, err := s.router.RouteRequest(c.Request.Context(), agent, req.Prompt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, resp)
}

func (s *Server) listRules(c *gin.Context) {
	// Return configured rules
	c.JSON(http.StatusOK, gin.H{"rules": "configured"})
}

func (s *Server) setRule(c *gin.Context) {
	var rule routing.RoutingRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	s.router.SetRule(rule)
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (s *Server) listSessions(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"sessions": "active"})
}

func (s *Server) proxy(c *gin.Context) {
	// Proxy requests to cliproxy+bifrost
	path := c.Param("path")
	
	log.Printf("Proxying request to: %s", path)
	
	// Simple proxy - just forward the request
	c.JSON(http.StatusOK, gin.H{
		"proxied": path,
		"method":  c.Request.Method,
	})
}
