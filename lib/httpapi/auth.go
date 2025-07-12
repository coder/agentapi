package httpapi

import (
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
)

// AuthConfig holds authentication configuration
type AuthConfig struct {
	APIKey   string
	Required bool
}

// NewAuthConfig creates a new AuthConfig from environment variables
func NewAuthConfig() *AuthConfig {
	apiKey := os.Getenv("AGENTAPI_KEY")
	required := apiKey != ""

	return &AuthConfig{
		APIKey:   apiKey,
		Required: required,
	}
}

// AuthMiddleware returns a middleware function that validates API keys for API endpoints only
func (a *AuthConfig) AuthMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip authentication if not required
			if !a.Required {
				next.ServeHTTP(w, r)
				return
			}

			// Skip authentication for static files and certain paths
			if a.shouldSkipAuth(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			var token string

			// For SSE endpoints (/events), check query parameter first since EventSource doesn't support custom headers
			if strings.HasPrefix(r.URL.Path, "/events") {
				queryToken := r.URL.Query().Get("api_key")
				if queryToken != "" {
					token = queryToken
				}
			}

			// If no token from query parameter, check Authorization header
			if token == "" {
				authHeader := r.Header.Get("Authorization")
				if authHeader == "" {
					http.Error(w, "Missing Authorization header or api_key query parameter", http.StatusUnauthorized)
					return
				}

				// Check for Bearer token format
				const bearerPrefix = "Bearer "
				if !strings.HasPrefix(authHeader, bearerPrefix) {
					http.Error(w, "Authorization header must start with 'Bearer '", http.StatusUnauthorized)
					return
				}

				// Extract the token
				token = strings.TrimPrefix(authHeader, bearerPrefix)
			}

			if token == "" {
				http.Error(w, "Missing API key in Authorization header or api_key query parameter", http.StatusUnauthorized)
				return
			}

			// Validate the token
			if token != a.APIKey {
				http.Error(w, "Invalid API key", http.StatusUnauthorized)
				return
			}

			// Authentication successful, proceed with the request
			next.ServeHTTP(w, r)
		})
	}
}

// shouldSkipAuth determines if authentication should be skipped for a given path
func (a *AuthConfig) shouldSkipAuth(path string) bool {
	// Skip authentication for static files and web interface
	skipPaths := []string{
		"/chat",
		"/openapi.json",
		"/docs",
	}

	for _, skipPath := range skipPaths {
		if strings.HasPrefix(path, skipPath) {
			return true
		}
	}

	// Skip root redirect
	if path == "/" {
		return true
	}

	return false
}

// ProtectedRoutes returns a new router with authentication middleware applied
func (a *AuthConfig) ProtectedRoutes() chi.Router {
	router := chi.NewRouter()
	router.Use(a.AuthMiddleware())
	return router
}