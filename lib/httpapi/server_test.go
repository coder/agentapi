package httpapi_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"testing"

	"github.com/coder/agentapi/lib/httpapi"
	"github.com/coder/agentapi/lib/logctx"
	"github.com/coder/agentapi/lib/msgfmt"
	"github.com/stretchr/testify/require"
)

func normalizeSchema(t *testing.T, schema any) any {
	t.Helper()
	switch val := (schema).(type) {
	case *any:
		normalizeSchema(t, *val)
	case []any:
		for i := range val {
			normalizeSchema(t, &val[i])
		}
		sort.SliceStable(val, func(i, j int) bool {
			return fmt.Sprintf("%v", val[i]) < fmt.Sprintf("%v", val[j])
		})
	case map[string]any:
		for k := range val {
			valUnderKey := val[k]
			normalizeSchema(t, &valUnderKey)
			val[k] = valUnderKey
		}
	}
	return schema
}

// Ensure the OpenAPI schema on disk is up to date.
// To update the schema, run `go run main.go server --print-openapi dummy > openapi.json`.
func TestOpenAPISchema(t *testing.T) {
	t.Parallel()

	ctx := logctx.WithLogger(context.Background(), slog.New(slog.NewTextHandler(os.Stdout, nil)))
	srv := httpapi.NewServer(ctx, httpapi.ServerConfig{
		AgentType:    msgfmt.AgentTypeClaude,
		Process:      nil,
		Port:         0,
		ChatBasePath: "/chat",
	})
	currentSchemaStr := srv.GetOpenAPI()
	var currentSchema any
	if err := json.Unmarshal([]byte(currentSchemaStr), &currentSchema); err != nil {
		t.Fatalf("failed to unmarshal current schema: %s", err)
	}

	diskSchemaFile, err := os.OpenFile("../../openapi.json", os.O_RDONLY, 0)
	if err != nil {
		t.Fatalf("failed to open disk schema: %s", err)
	}
	defer func() {
		_ = diskSchemaFile.Close()
	}()

	diskSchemaBytes, err := io.ReadAll(diskSchemaFile)
	if err != nil {
		t.Fatalf("failed to read disk schema: %s", err)
	}
	var diskSchema any
	if err := json.Unmarshal(diskSchemaBytes, &diskSchema); err != nil {
		t.Fatalf("failed to unmarshal disk schema: %s", err)
	}

	normalizeSchema(t, &currentSchema)
	normalizeSchema(t, &diskSchema)

	require.Equal(t, currentSchema, diskSchema)
}

func TestServer_redirectToChat(t *testing.T) {
	cases := []struct {
		name                 string
		chatBasePath         string
		expectedResponseCode int
		expectedLocation     string
	}{
		{"default base path", "/chat", http.StatusTemporaryRedirect, "/chat/embed"},
		{"custom base path", "/custom", http.StatusTemporaryRedirect, "/custom/embed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tCtx := logctx.WithLogger(context.Background(), slog.New(slog.NewTextHandler(os.Stdout, nil)))
			s := httpapi.NewServer(tCtx, httpapi.ServerConfig{
				AgentType:    msgfmt.AgentTypeClaude,
				Process:      nil,
				Port:         0,
				ChatBasePath: tc.chatBasePath,
			})
			tsServer := httptest.NewServer(s.Handler())
			t.Cleanup(tsServer.Close)

			client := &http.Client{
				CheckRedirect: func(req *http.Request, via []*http.Request) error {
					return http.ErrUseLastResponse
				},
			}
			resp, err := client.Get(tsServer.URL + "/")
			require.NoError(t, err, "unexpected error making GET request")
			t.Cleanup(func() {
				_ = resp.Body.Close()
			})
			require.Equal(t, tc.expectedResponseCode, resp.StatusCode, "expected %d status code", tc.expectedResponseCode)
			loc := resp.Header.Get("Location")
			require.Equal(t, tc.expectedLocation, loc, "expected Location %q, got %q", tc.expectedLocation, loc)
		})
	}
}
