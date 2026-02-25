// Package benchmarks provides tokenledger integration for agentapi.
//
// This enables agentapi to use dynamic benchmark data for routing decisions.
// Integrates with tokenledger for:
// - Model quality scores
// - Cost per token
// - Latency metrics
package benchmarks

import (
	"sync"
	"time"
)

// BenchmarkData represents benchmark data for a model
type BenchmarkData struct {
	ModelID           string   `json:"model_id"`
	Provider          string   `json:"provider,omitempty"`
	IntelligenceIndex *float64 `json:"intelligence_index,omitempty"`
	CodingIndex       *float64 `json:"coding_index,omitempty"`
	SpeedTPS          *float64 `json:"speed_tps,omitempty"`
	LatencyMs         *float64 `json:"latency_ms,omitempty"`
	PricePer1MInput   *float64 `json:"price_per_1m_input,omitempty"`
	PricePer1MOutput  *float64 `json:"price_per_1m_output,omitempty"`
	ContextWindow     *int64   `json:"context_window,omitempty"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// Client fetches benchmarks from tokenledger
type Client struct {
	tokenledgerURL string
	cacheTTL      time.Duration
	cache         map[string]BenchmarkData
	mu            sync.RWMutex
}

// NewClient creates a new tokenledger benchmark client
func NewClient(tokenledgerURL string, cacheTTL time.Duration) *Client {
	return &Client{
		tokenledgerURL: tokenledgerURL,
		cacheTTL:      cacheTTL,
		cache:         make(map[string]BenchmarkData),
	}
}

// GetBenchmark returns benchmark data for a model
func (c *Client) GetBenchmark(modelID string) (*BenchmarkData, error) {
	c.mu.RLock()
	if data, ok := c.cache[modelID]; ok {
		c.mu.RUnlock()
		return &data, nil
	}
	c.mu.RUnlock()

	// In production, this would call tokenledger HTTP API
	// For now, return nil to use fallback
	return nil, nil
}
