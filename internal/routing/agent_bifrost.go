package routing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/coder/agentapi/internal/benchmarks"
)
// AgentBifrost is the Bifrost extension for agent-specific routing
// It sits between thegent and cliproxy+bifrost, providing:
// - Custom routing rules per agent
// - Session-aware load balancing
// - Agent-specific governance
// - Dynamic benchmark data from tokenledger
type AgentBifrost struct {
	cliproxyURL string
	client     *http.Client

	// Session management
	sessions    map[string]*AgentSession
	sessionsMut sync.RWMutex

	// Agent-specific routing rules
	rules      map[string]RoutingRule
	rulesMut   sync.RWMutex

	// Benchmark data for routing decisions
	benchmarks *benchmarks.Store
}

// AgentSession represents a session with routing metadata
type AgentSession struct {
	ID        string                 `json:"id"`
	Agent    string                 `json:"agent"`
	Started  time.Time             `json:"started"`
	Models   []string              `json:"models"`
	Metadata map[string]interface{} `json:"metadata"`
}

// RoutingRule defines routing behavior for an agent
type RoutingRule struct {
	Agent         string   `json:"agent"`
	PreferredModel string   `json:"preferred_model"`
	FallbackModels []string `json:"fallback_models"`
	MaxRetries   int      `json:"max_retries"`
	Timeout     int      `json:"timeout_seconds"`
}

// NewAgentBifrost creates a new agent routing layer
func NewAgentBifrost(cliproxyURL string) (*AgentBifrost, error) {
	return &AgentBifrost{
		cliproxyURL: cliproxyURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		sessions:   make(map[string]*AgentSession),
		rules:      make(map[string]RoutingRule),
		benchmarks: benchmarks.NewStore(),
	}, nil
}

// RouteRequest routes a request through the agent layer to cliproxy+bifrost
func (a *AgentBifrost) RouteRequest(ctx context.Context, agent string, prompt string) (*RoutingResponse, error) {
	// Get agent-specific routing rules
	rule := a.getRule(agent)
	
	// Build the request to cliproxy+bifrost
	reqBody := map[string]interface{}{
		"model":   rule.PreferredModel,
		"messages": []map[string]string{{"role": "user", "content": prompt}},
		"agent":   agent,
		"session": a.getOrCreateSession(agent),
	}
	
	// Forward to cliproxy+bifrost
	resp, err := a.forwardToCliproxy(ctx, reqBody)
	if err != nil {
		// Try fallback models
		for _, fallback := range rule.FallbackModels {
			reqBody["model"] = fallback
			resp, err = a.forwardToCliproxy(ctx, reqBody)
			if err == nil {
				break
			}
		}
	}
	
	return resp, err
}

// forwardToCliproxy sends request to cliproxy+bifrost
func (a *AgentBifrost) forwardToCliproxy(ctx context.Context, body map[string]interface{}) (*RoutingResponse, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	
	req, err := http.NewRequestWithContext(ctx, "POST", a.cliproxyURL+"/v1/chat/completions", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	req.Body = io.NopCloser(bytes.NewReader(jsonBody))
	
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request to cliproxy failed: %w", err)
	}
	defer resp.Body.Close()
	
	var routingResp RoutingResponse
	if err := json.NewDecoder(resp.Body).Decode(&routingResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return &routingResp, nil
}

// getRule returns routing rules for an agent
func (a *AgentBifrost) getRule(agent string) RoutingRule {
	a.rulesMut.RLock()
	defer a.rulesMut.RUnlock()
	
	if rule, ok := a.rules[agent]; ok {
		return rule
	}
	
	// Default rule
	return RoutingRule{
		Agent:         agent,
		PreferredModel: "claude-3-5-sonnet-20241022",
		FallbackModels: []string{"gpt-4o", "gemini-1.5-pro"},
		MaxRetries:   3,
		Timeout:      30,
	}
}

// SetRule sets a routing rule for an agent
func (a *AgentBifrost) SetRule(rule RoutingRule) {
	a.rulesMut.Lock()
	defer a.rulesMut.Unlock()
	a.rules[rule.Agent] = rule
	log.Printf("Set routing rule for agent %s: model=%s", rule.Agent, rule.PreferredModel)
}

// getOrCreateSession gets or creates a session for an agent
func (a *AgentBifrost) getOrCreateSession(agent string) string {
	a.sessionsMut.Lock()
	defer a.sessionsMut.Unlock()
	
	for id, sess := range a.sessions {
		if sess.Agent == agent && time.Since(sess.Started) < time.Hour {
			return id
		}
	}
	
	// Create new session
	id := fmt.Sprintf("sess_%d", time.Now().UnixNano())
	a.sessions[id] = &AgentSession{
		ID:        id,
		Agent:    agent,
		Started:  time.Now(),
		Metadata: make(map[string]interface{}),
	}
	return id
}

// RoutingResponse represents the response from routing
type RoutingResponse struct {
	ID      string          `json:"id"`
	Model  string          `json:"model"`
	Choices []Choice       `json:"choices"`
	Usage  Usage          `json:"usage"`
	Error  string         `json:"error,omitempty"`
}

type Choice struct {
	Message Message `json:"message"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens     int `json:"total_tokens"`
}

// ModelMetrics represents benchmark metrics for a model
type ModelMetrics struct {
	ModelID      string  `json:"model_id"`
	QualityScore float64 `json:"quality_score"`
	CostPer1K    float64 `json:"cost_per_1k"`
	LatencyMs    int     `json:"latency_ms"`
}

// GetModelMetrics returns benchmark metrics for a model
func (a *AgentBifrost) GetModelMetrics(modelID string) *ModelMetrics {
	if a.benchmarks == nil {
		return nil
	}

	return &ModelMetrics{
		ModelID:      modelID,
		QualityScore: a.benchmarks.GetQuality(modelID),
		CostPer1K:    a.benchmarks.GetCost(modelID),
		LatencyMs:    a.benchmarks.GetLatency(modelID),
	}
}
