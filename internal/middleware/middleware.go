// Package middleware provides HTTP middleware integration for AgentAPI using chi.
package middleware

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

// ApplyDefaultStack applies the default middleware stack to a chi router.
// This includes panic recovery, request logging, and request ID tracking.
//
// Parameters:
//   - router: The chi router to apply middleware to
//
// Returns:
//   - error: An error if middleware setup fails
func ApplyDefaultStack(router *chi.Mux) error {
	router.Use(chimiddleware.RequestID)
	router.Use(chimiddleware.RealIP)
	router.Use(chimiddleware.Recoverer)
	router.Use(chimiddleware.Logger)
	return nil
}

// CORSOptions defines custom CORS configuration for AgentAPI.
type CORSOptions struct {
	AllowedOrigins []string
	AllowedHosts   []string
}

// ApplyCustomCORS applies custom CORS middleware with AgentAPI-specific configuration.
//
// Parameters:
//   - router: The chi router to apply middleware to
//   - options: CORS configuration options
func ApplyCustomCORS(router *chi.Mux, options CORSOptions) {
	corsOpts := cors.Options{
		AllowedOrigins: options.AllowedOrigins,
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders: []string{"Link"},
		MaxAge:         300,
	}
	router.Use(cors.Handler(corsOpts))
}

// HealthCheckRoute registers a health check endpoint.
//
// Parameters:
//   - router: The chi router to register the route on
func HealthCheckRoute(router *chi.Mux) {
	registerProbe(router, "/health", "ok")
}

// ReadinessCheckRoute registers a readiness check endpoint.
//
// Parameters:
//   - router: The chi router to register the route on
func ReadinessCheckRoute(router *chi.Mux) {
	registerProbe(router, "/readiness", "ready")
}

// RequestIDHandler is a helper that allows callers to extract or use request IDs
// from the middleware applied by the default stack.
type RequestIDHandler struct {
	timeout time.Duration
}

// NewRequestIDHandler creates a new RequestIDHandler.
//
// Parameters:
//   - timeout: The timeout for handling requests
//
// Returns:
//   - *RequestIDHandler: A new RequestIDHandler instance
func NewRequestIDHandler(timeout time.Duration) *RequestIDHandler {
	return &RequestIDHandler{
		timeout: timeout,
	}
}

// WrapHandler wraps a handler with timeout and other AgentAPI-specific middleware.
//
// Parameters:
//   - h: The handler to wrap
//
// Returns:
//   - http.Handler: The wrapped handler
func (h *RequestIDHandler) WrapHandler(next http.Handler) http.Handler {
	return http.TimeoutHandler(next, h.timeout, "Request timeout")
}

func registerProbe(router *chi.Mux, path string, body string) {
	router.Get(path, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": body})
	})
}
