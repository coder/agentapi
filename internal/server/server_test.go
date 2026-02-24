package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/coder/agentapi/internal/routing"
)

func TestNewServer(t *testing.T) {
	bifrost, _ := routing.NewAgentBifrost("http://localhost:8080")
	server := New(8080, bifrost)

	if server == nil {
		t.Fatal("Expected non-nil server instance")
	}

	if server.port != 8080 {
		t.Errorf("Expected port 8080, got %d", server.port)
	}

	if server.router == nil {
		t.Fatal("Expected non-nil router")
	}
}

func TestNewServer_DifferentPort(t *testing.T) {
	bifrost, _ := routing.NewAgentBifrost("http://localhost:8080")
	server := New(9999, bifrost)

	if server.port != 9999 {
		t.Errorf("Expected port 9999, got %d", server.port)
	}
}

func TestShutdown_NilServer(t *testing.T) {
	bifrost, _ := routing.NewAgentBifrost("http://localhost:8080")
	server := New(9999, bifrost)

	// Should not panic when http.Server is nil
	server.Shutdown()
}

func TestHealthHandler(t *testing.T) {
	bifrost, _ := routing.NewAgentBifrost("http://localhost:8080")
	server := New(8080, bifrost)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	server.health(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if status, ok := response["status"]; !ok || status != "ok" {
		t.Errorf("Expected status field to be 'ok', got %v", status)
	}
}

func TestChatCompletionsHandler_InvalidJSON(t *testing.T) {
	bifrost, _ := routing.NewAgentBifrost("http://localhost:8080")
	server := New(8080, bifrost)

	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.chatCompletions(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for invalid JSON, got %d", w.Code)
	}
}

func TestChatCompletionsHandler_DefaultAgent(t *testing.T) {
	bifrost, _ := routing.NewAgentBifrost("http://localhost:8080")
	server := New(8080, bifrost)

	payload := map[string]string{
		"prompt": "Test without agent",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.chatCompletions(w, req)

	// Response indicates handler processed the request (may fail with connection error)
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected 200 or 500, got %d", w.Code)
	}
}

func TestListRulesHandler(t *testing.T) {
	bifrost, _ := routing.NewAgentBifrost("http://localhost:8080")
	server := New(8080, bifrost)

	req := httptest.NewRequest("GET", "/admin/rules", nil)
	w := httptest.NewRecorder()

	server.listRules(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	if _, ok := response["rules"]; !ok {
		t.Fatal("Expected 'rules' field in response")
	}
}

func TestSetRuleHandler(t *testing.T) {
	bifrost, _ := routing.NewAgentBifrost("http://localhost:8080")
	server := New(8080, bifrost)

	rule := routing.RoutingRule{
		Agent:          "test-agent",
		PreferredModel: "gpt-4o",
		FallbackModels: []string{"claude-3-5-sonnet-20241022"},
		MaxRetries:     5,
		Timeout:        60,
	}
	body, _ := json.Marshal(rule)

	req := httptest.NewRequest("POST", "/admin/rules", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.setRule(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	if status, ok := response["status"]; !ok || status != "ok" {
		t.Errorf("Expected status 'ok', got %v", status)
	}
}

func TestSetRuleHandler_InvalidJSON(t *testing.T) {
	bifrost, _ := routing.NewAgentBifrost("http://localhost:8080")
	server := New(8080, bifrost)

	req := httptest.NewRequest("POST", "/admin/rules", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.setRule(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestListSessionsHandler(t *testing.T) {
	bifrost, _ := routing.NewAgentBifrost("http://localhost:8080")
	server := New(8080, bifrost)

	req := httptest.NewRequest("GET", "/admin/sessions", nil)
	w := httptest.NewRecorder()

	server.listSessions(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	if _, ok := response["sessions"]; !ok {
		t.Fatal("Expected 'sessions' field in response")
	}
}

func TestProxyHandler(t *testing.T) {
	bifrost, _ := routing.NewAgentBifrost("http://localhost:8080")
	server := New(8080, bifrost)

	// Create chi router to test path params
	r := chi.NewRouter()
	r.HandleFunc("/proxy/*", server.proxy)

	req := httptest.NewRequest("GET", "/proxy/test/path", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	if method, ok := response["method"]; !ok || method != "GET" {
		t.Errorf("Expected method GET, got %v", method)
	}
}

func TestProxyHandler_POSTMethod(t *testing.T) {
	bifrost, _ := routing.NewAgentBifrost("http://localhost:8080")
	server := New(8080, bifrost)

	r := chi.NewRouter()
	r.HandleFunc("/proxy/*", server.proxy)

	req := httptest.NewRequest("POST", "/proxy/some/resource", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	if method, ok := response["method"]; !ok || method != "POST" {
		t.Errorf("Expected method POST, got %v", method)
	}
}

func TestAgentHandler_StartAgent(t *testing.T) {
	bifrost, _ := routing.NewAgentBifrost("http://localhost:8080")
	handler := NewAgentHandler(bifrost)

	payload := map[string]string{
		"agent": "claude",
		"model": "claude-sonnet-4",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/agent/start", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandleStartAgent(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response StartAgentResponse
	json.Unmarshal(w.Body.Bytes(), &response)

	if response.Status != "running" {
		t.Errorf("Expected status 'running', got %v", response.Status)
	}

	if response.SessionID == "" {
		t.Error("Expected non-empty session_id")
	}
}

func TestAgentHandler_Models(t *testing.T) {
	bifrost, _ := routing.NewAgentBifrost("http://localhost:8080")
	handler := NewAgentHandler(bifrost)

	req := httptest.NewRequest("GET", "/models", nil)
	w := httptest.NewRecorder()

	handler.HandleModels(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	models, ok := response["models"].([]interface{})
	if !ok {
		t.Fatal("Expected 'models' array in response")
	}

	if len(models) == 0 {
		t.Error("Expected non-empty models list")
	}
}
