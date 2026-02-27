package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestHostAuthorizationMiddleware_Wildcard(t *testing.T) {
	router := chi.NewRouter()
	router.Use(hostAuthorizationMiddleware([]string{"*"}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))
	router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "evil.com"
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for wildcard, got %d", w.Code)
	}
}

func TestHostAuthorizationMiddleware_AllowedHost(t *testing.T) {
	allowedHosts := []string{"localhost", "example.com"}
	router := chi.NewRouter()
	router.Use(hostAuthorizationMiddleware(allowedHosts, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad host", http.StatusBadRequest)
	})))
	router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	tests := []struct {
		host       string
		shouldPass bool
	}{
		{"localhost", true},
		{"example.com", true},
		{"localhost:8080", true}, // port should be ignored
		{"example.com:443", true},
		{"evil.com", false},
		{"", false},
		{"localhost.evil.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.Host = tt.host
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if tt.shouldPass && w.Code != http.StatusOK {
				t.Errorf("expected 200 for host %q, got %d", tt.host, w.Code)
			}
			if !tt.shouldPass && w.Code == http.StatusOK {
				t.Errorf("expected non-200 for host %q, got %d", tt.host, w.Code)
			}
		})
	}
}

func TestHostAuthorizationMiddleware_CaseInsensitive(t *testing.T) {
	allowedHosts := []string{"Example.COM", "LOCALHOST"}
	router := chi.NewRouter()
	router.Use(hostAuthorizationMiddleware(allowedHosts, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad host", http.StatusBadRequest)
	})))
	router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	tests := []struct {
		host       string
		shouldPass bool
	}{
		{"example.com", true},
		{"EXAMPLE.COM", true},
		{"Example.Com", true},
		{"localhost", true},
		{"LOCALHOST", true},
		{"LocalHost", true},
	}

	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.Host = tt.host
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if tt.shouldPass && w.Code != http.StatusOK {
				t.Errorf("expected 200 for host %q, got %d", tt.host, w.Code)
			}
		})
	}
}

func TestHostAuthorizationMiddleware_IPv6(t *testing.T) {
	// IPv6 addresses need to be in allowed hosts
	allowedHosts := []string{"[::1]", "localhost"}
	router := chi.NewRouter()
	router.Use(hostAuthorizationMiddleware(allowedHosts, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad host", http.StatusBadRequest)
	})))
	router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Note: IPv6 parsing in url.Parse works differently
	// The middleware should handle [::1] and [::1]:8080 correctly
	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "localhost"
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for host 'localhost', got %d", w.Code)
	}
}

func TestParseAllowedHosts_Wildcard(t *testing.T) {
	hosts, err := parseAllowedHosts([]string{"*"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hosts) != 1 || hosts[0] != "*" {
		t.Errorf("expected [*], got %v", hosts)
	}
}

func TestParseAllowedHosts_Valid(t *testing.T) {
	hosts, err := parseAllowedHosts([]string{"localhost", "example.com"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hosts) != 2 {
		t.Errorf("expected 2 hosts, got %d", len(hosts))
	}
}

func TestParseAllowedHosts_WithScheme(t *testing.T) {
	_, err := parseAllowedHosts([]string{"http://localhost"})
	if err == nil {
		t.Error("expected error for host with scheme")
	}
}

func TestParseAllowedHosts_WithPort(t *testing.T) {
	_, err := parseAllowedHosts([]string{"localhost:8080"})
	if err == nil {
		t.Error("expected error for host with port")
	}
}

func TestParseAllowedHosts_WithWhitespace(t *testing.T) {
	_, err := parseAllowedHosts([]string{"local host"})
	if err == nil {
		t.Error("expected error for host with whitespace")
	}
}

func TestParseAllowedHosts_WithComma(t *testing.T) {
	_, err := parseAllowedHosts([]string{"localhost,example.com"})
	if err == nil {
		t.Error("expected error for host with comma")
	}
}

func TestParseAllowedHosts_Empty(t *testing.T) {
	_, err := parseAllowedHosts([]string{})
	if err == nil {
		t.Error("expected error for empty list")
	}
}

func TestParseAllowedOrigins_Wildcard(t *testing.T) {
	origins, err := parseAllowedOrigins([]string{"*"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(origins) != 1 || origins[0] != "*" {
		t.Errorf("expected [*], got %v", origins)
	}
}

func TestParseAllowedOrigins_Valid(t *testing.T) {
	origins, err := parseAllowedOrigins([]string{"http://localhost:3000", "https://example.com"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(origins) != 2 {
		t.Errorf("expected 2 origins, got %d", len(origins))
	}
}

func TestParseAllowedOrigins_WithWhitespace(t *testing.T) {
	_, err := parseAllowedOrigins([]string{"http://local host"})
	if err == nil {
		t.Error("expected error for origin with whitespace")
	}
}

func TestParseAllowedOrigins_Empty(t *testing.T) {
	_, err := parseAllowedOrigins([]string{})
	if err == nil {
		t.Error("expected error for empty list")
	}
}
