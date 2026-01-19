package acpio

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestACPAgentIO(t *testing.T) {
	tests := []struct {
		fixture  string
		input    string
		expected string
	}{
		{
			fixture:  "basic_prompt.json",
			input:    "Hello",
			expected: "Hi there!",
		},
		{
			fixture: "streaming_chunks.json",
			input:   "Tell me a story",
			expected: `Once upon a time,
there was a programmer
who wrote great tests.`,
		},
	}
	for _, tt := range tests {
		t.Run(strings.TrimSuffix(tt.fixture, ".json"), func(t *testing.T) {
			agentIO := setupMockAgent(t, "testdata/"+tt.fixture)

			_, err := agentIO.Write([]byte(tt.input))
			require.NoError(t, err)
			assert.Equal(t, tt.expected, agentIO.ReadScreen())
		})
	}
}

// Exchange represents a single request-response exchange in the mock agent protocol
type Exchange struct {
	Expect  json.RawMessage   `json:"expect"`
	Respond []json.RawMessage `json:"respond"`
}

// loadExchanges loads exchanges from a golden file
func loadExchanges(t *testing.T, path string) []Exchange {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var exchanges []Exchange
	require.NoError(t, json.Unmarshal(data, &exchanges))
	return exchanges
}

// setupMockAgent loads exchanges, starts mock agent goroutine, registers cleanup
func setupMockAgent(t *testing.T, fixturePath string) *ACPAgentIO {
	t.Helper()
	exchanges := loadExchanges(t, fixturePath)

	// Two pipes: client→agent (c2a), agent→client (a2c)
	c2aR, c2aW := io.Pipe()
	a2cR, a2cW := io.Pipe()

	done := make(chan struct{})
	go func() {
		defer close(done)
		runMockAgent(t, exchanges, c2aR, a2cW)
	}()

	t.Cleanup(func() {
		c2aW.Close()
		a2cR.Close()
		<-done
	})

	ctx := context.Background()
	agentIO, err := NewWithPipes(ctx, c2aW, a2cR)
	require.NoError(t, err)
	return agentIO
}

// runMockAgent simulates an ACP agent by reading JSON-RPC requests and responding per exchanges
func runMockAgent(t *testing.T, exchanges []Exchange, input io.Reader, output io.Writer) {
	t.Helper()
	scanner := bufio.NewScanner(input)
	exchIdx := 0

	for scanner.Scan() {
		if exchIdx >= len(exchanges) {
			t.Errorf("unexpected request beyond exchanges: %s", scanner.Text())
			return
		}

		line := scanner.Bytes()

		// Parse incoming request to get the id
		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		if err := json.Unmarshal(line, &req); err != nil {
			t.Errorf("failed to parse request: %v", err)
			return
		}

		// Verify method matches expected
		var expected struct {
			Method string `json:"method"`
		}
		if err := json.Unmarshal(exchanges[exchIdx].Expect, &expected); err != nil {
			t.Errorf("failed to parse expected: %v", err)
			return
		}
		if req.Method != expected.Method {
			t.Errorf("expected method %q, got %q", expected.Method, req.Method)
			return
		}

		// Send all responses for this exchange
		for _, resp := range exchanges[exchIdx].Respond {
			// Check if this is a notification (has method) or a response (has result)
			var respObj map[string]json.RawMessage
			if err := json.Unmarshal(resp, &respObj); err != nil {
				t.Errorf("failed to parse response: %v", err)
				return
			}

			if _, hasMethod := respObj["method"]; hasMethod {
				// It's a notification, send as-is with jsonrpc field
				notification := map[string]json.RawMessage{
					"jsonrpc": json.RawMessage(`"2.0"`),
				}
				for k, v := range respObj {
					notification[k] = v
				}
				data, _ := json.Marshal(notification)
				output.Write(data)
				output.Write([]byte("\n"))
			} else if _, hasResult := respObj["result"]; hasResult {
				// It's a response, add the request id
				response := map[string]json.RawMessage{
					"jsonrpc": json.RawMessage(`"2.0"`),
					"id":      req.ID,
					"result":  respObj["result"],
				}
				data, _ := json.Marshal(response)
				output.Write(data)
				output.Write([]byte("\n"))
			}
		}

		exchIdx++
	}
}
