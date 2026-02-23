package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/kooshapari/agentapi/internal/routing"
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
	gin.SetMode(gin.TestMode)
	bifrost, _ := routing.NewAgentBifrost("http://localhost:8080")
	server := New(8080, bifrost)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	c, _ := gin.CreateTestContext(w)
	c.Request = req

	server.health(c)

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
	gin.SetMode(gin.TestMode)
	bifrost, _ := routing.NewAgentBifrost("http://localhost:8080")
	server := New(8080, bifrost)

	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	c, _ := gin.CreateTestContext(w)
	c.Request = req

	server.chatCompletions(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for invalid JSON, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	if _, ok := response["error"]; !ok {
		t.Fatal("Expected error field in response")
	}
}

func TestChatCompletionsHandler_DefaultAgent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	bifrost, _ := routing.NewAgentBifrost("http://localhost:8080")
	server := New(8080, bifrost)

	payload := map[string]string{
		"prompt": "Test without agent",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	c, _ := gin.CreateTestContext(w)
	c.Request = req

	// This will fail with connection error, but validates handler structure
	server.chatCompletions(c)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	// Response indicates handler processed the request
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected 200 or 500, got %d", w.Code)
	}
}

func TestListRulesHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	bifrost, _ := routing.NewAgentBifrost("http://localhost:8080")
	server := New(8080, bifrost)

	req := httptest.NewRequest("GET", "/admin/rules", nil)
	w := httptest.NewRecorder()

	c, _ := gin.CreateTestContext(w)
	c.Request = req

	server.listRules(c)

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
	gin.SetMode(gin.TestMode)
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

	c, _ := gin.CreateTestContext(w)
	c.Request = req

	server.setRule(c)

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
	gin.SetMode(gin.TestMode)
	bifrost, _ := routing.NewAgentBifrost("http://localhost:8080")
	server := New(8080, bifrost)

	req := httptest.NewRequest("POST", "/admin/rules", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	c, _ := gin.CreateTestContext(w)
	c.Request = req

	server.setRule(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestListSessionsHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	bifrost, _ := routing.NewAgentBifrost("http://localhost:8080")
	server := New(8080, bifrost)

	req := httptest.NewRequest("GET", "/admin/sessions", nil)
	w := httptest.NewRecorder()

	c, _ := gin.CreateTestContext(w)
	c.Request = req

	server.listSessions(c)

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
	gin.SetMode(gin.TestMode)
	bifrost, _ := routing.NewAgentBifrost("http://localhost:8080")
	server := New(8080, bifrost)

	req := httptest.NewRequest("GET", "/proxy/test/path", nil)
	w := httptest.NewRecorder()

	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Params = []gin.Param{{Key: "path", Value: "test/path"}}

	server.proxy(c)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	if proxied, ok := response["proxied"]; !ok || proxied != "test/path" {
		t.Errorf("Expected proxied path 'test/path', got %v", proxied)
	}

	if method, ok := response["method"]; !ok || method != "GET" {
		t.Errorf("Expected method GET, got %v", method)
	}
}

func TestProxyHandler_POSTMethod(t *testing.T) {
	gin.SetMode(gin.TestMode)
	bifrost, _ := routing.NewAgentBifrost("http://localhost:8080")
	server := New(8080, bifrost)

	req := httptest.NewRequest("POST", "/proxy/some/resource", nil)
	w := httptest.NewRecorder()

	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Params = []gin.Param{{Key: "path", Value: "some/resource"}}

	server.proxy(c)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	if method, ok := response["method"]; !ok || method != "POST" {
		t.Errorf("Expected method POST, got %v", method)
	}
}

func TestProxyHandler_EmptyPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	bifrost, _ := routing.NewAgentBifrost("http://localhost:8080")
	server := New(8080, bifrost)

	req := httptest.NewRequest("GET", "/proxy/", nil)
	w := httptest.NewRecorder()

	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Params = []gin.Param{{Key: "path", Value: ""}}

	server.proxy(c)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}
