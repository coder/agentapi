package httpapi

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestAuthConfig_NewAuthConfig(t *testing.T) {
	tests := []struct {
		name       string
		apiKey     string
		wantAPIKey string
		wantReq    bool
	}{
		{
			name:       "no api key set",
			apiKey:     "",
			wantAPIKey: "",
			wantReq:    false,
		},
		{
			name:       "api key set",
			apiKey:     "test-key-123",
			wantAPIKey: "test-key-123",
			wantReq:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variable
			oldVal := os.Getenv("AGENTAPI_KEY")
			defer os.Setenv("AGENTAPI_KEY", oldVal)
			
			os.Setenv("AGENTAPI_KEY", tt.apiKey)

			config := NewAuthConfig()
			if config.APIKey != tt.wantAPIKey {
				t.Errorf("APIKey = %v, want %v", config.APIKey, tt.wantAPIKey)
			}
			if config.Required != tt.wantReq {
				t.Errorf("Required = %v, want %v", config.Required, tt.wantReq)
			}
		})
	}
}

func TestAuthConfig_AuthMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		apiKey         string
		authRequired   bool
		requestPath    string
		authHeader     string
		wantStatusCode int
	}{
		{
			name:           "no auth required",
			apiKey:         "",
			authRequired:   false,
			requestPath:    "/status",
			authHeader:     "",
			wantStatusCode: http.StatusOK,
		},
		{
			name:           "auth required, valid key",
			apiKey:         "test-key-123",
			authRequired:   true,
			requestPath:    "/status",
			authHeader:     "Bearer test-key-123",
			wantStatusCode: http.StatusOK,
		},
		{
			name:           "auth required, invalid key",
			apiKey:         "test-key-123",
			authRequired:   true,
			requestPath:    "/status",
			authHeader:     "Bearer wrong-key",
			wantStatusCode: http.StatusUnauthorized,
		},
		{
			name:           "auth required, missing header",
			apiKey:         "test-key-123",
			authRequired:   true,
			requestPath:    "/status",
			authHeader:     "",
			wantStatusCode: http.StatusUnauthorized,
		},
		{
			name:           "auth required, malformed header",
			apiKey:         "test-key-123",
			authRequired:   true,
			requestPath:    "/status",
			authHeader:     "Basic dGVzdA==",
			wantStatusCode: http.StatusUnauthorized,
		},
		{
			name:           "auth required, skip static files",
			apiKey:         "test-key-123",
			authRequired:   true,
			requestPath:    "/chat/index.html",
			authHeader:     "",
			wantStatusCode: http.StatusOK,
		},
		{
			name:           "auth required, skip root",
			apiKey:         "test-key-123",
			authRequired:   true,
			requestPath:    "/",
			authHeader:     "",
			wantStatusCode: http.StatusOK,
		},
		{
			name:           "auth required, skip openapi",
			apiKey:         "test-key-123",
			authRequired:   true,
			requestPath:    "/openapi.json",
			authHeader:     "",
			wantStatusCode: http.StatusOK,
		},
		{
			name:           "auth required, events with query param",
			apiKey:         "test-key-123",
			authRequired:   true,
			requestPath:    "/events?api_key=test-key-123",
			authHeader:     "",
			wantStatusCode: http.StatusOK,
		},
		{
			name:           "auth required, events with wrong query param",
			apiKey:         "test-key-123",
			authRequired:   true,
			requestPath:    "/events?api_key=wrong-key",
			authHeader:     "",
			wantStatusCode: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &AuthConfig{
				APIKey:   tt.apiKey,
				Required: tt.authRequired,
			}

			// Create a simple handler that returns 200 OK
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			// Wrap with auth middleware
			middleware := config.AuthMiddleware()
			wrappedHandler := middleware(handler)

			// Create request
			req := httptest.NewRequest("GET", tt.requestPath, nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			// Create response recorder
			rec := httptest.NewRecorder()

			// Call handler
			wrappedHandler.ServeHTTP(rec, req)

			// Check status code
			if rec.Code != tt.wantStatusCode {
				t.Errorf("status code = %v, want %v", rec.Code, tt.wantStatusCode)
			}
		})
	}
}

func TestAuthConfig_shouldSkipAuth(t *testing.T) {
	config := &AuthConfig{}

	tests := []struct {
		name string
		path string
		want bool
	}{
		{
			name: "api endpoint",
			path: "/status",
			want: false,
		},
		{
			name: "api endpoint messages",
			path: "/messages",
			want: false,
		},
		{
			name: "chat static file",
			path: "/chat/index.html",
			want: true,
		},
		{
			name: "chat base path",
			path: "/chat",
			want: true,
		},
		{
			name: "openapi spec",
			path: "/openapi.json",
			want: true,
		},
		{
			name: "docs",
			path: "/docs",
			want: true,
		},
		{
			name: "root path",
			path: "/",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := config.shouldSkipAuth(tt.path); got != tt.want {
				t.Errorf("shouldSkipAuth() = %v, want %v", got, tt.want)
			}
		})
	}
}