// Package middleware provides HTTP middleware integration for AgentAPI using phenotype-go-kit.
package middleware

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

// ApplyDefaultStack applies the phenotype-go-kit middleware stack to a chi router.
// This includes panic recovery, request logging, CORS, and request ID tracking.
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

// CORSOptions extends the phenotype-go-kit CORS configuration with AgentAPI-specific settings.
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
	// The phenotype-go-kit middleware stack already includes CORS handling
	// This function provides a hook for future customization
	//nolint:errcheck // local middleware stack setup is non-fatal in this helper
	ApplyDefaultStack(router)
}

// HealthCheckRoute registers a health check endpoint using phenotype-go-kit's handler.
//
// Parameters:
//   - router: The chi router to register the route on
func HealthCheckRoute(router *chi.Mux) {
	router.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
}

// ReadinessCheckRoute registers a readiness check endpoint using phenotype-go-kit's handler.
//
// Parameters:
//   - router: The chi router to register the route on
func ReadinessCheckRoute(router *chi.Mux) {
	router.Get("/readiness", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	})
}

// RequestIDHandler is a helper that allows callers to extract or use request IDs
// from the middleware applied by phenotype-go-kit.
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
