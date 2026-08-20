package mcpserver

import (
	"bytes"
	"context"
	"errors"
	"log"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestLogUnknownToolCallsLogsOnError is the regression test for the
// accounting-mcp growth-point plan in docs/roadmap.md: calls to a tool name
// that doesn't exist must be logged (with the attempted tool name) so real
// usage data can drive which operations to add next.
func TestLogUnknownToolCallsLogsOnError(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(nil)

	next := func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		return nil, errors.New(`unknown tool "acts.frobnicate"`)
	}
	req := &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Name: "acts.frobnicate"}}

	_, err := logUnknownToolCalls(next)(context.Background(), "tools/call", req)
	if err == nil {
		t.Fatal("expected the underlying error to be returned unchanged")
	}

	logged := buf.String()
	if !strings.Contains(logged, "acts.frobnicate") {
		t.Errorf("expected log output to name the unknown tool, got: %q", logged)
	}
}

// TestLogUnknownToolCallsSkipsSuccessAndOtherMethods confirms the middleware
// stays quiet for the two cases that aren't "an unknown/invalid tool was
// called": a successful tools/call, and any non-tools/call method.
func TestLogUnknownToolCallsSkipsSuccessAndOtherMethods(t *testing.T) {
	tests := []struct {
		name   string
		method string
		err    error
	}{
		{"successful tools/call", "tools/call", nil},
		{"non-tools/call method with an error", "resources/read", errors.New("boom")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			log.SetOutput(&buf)
			defer log.SetOutput(nil)

			next := func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
				return nil, tt.err
			}
			req := &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Name: "whatever"}}

			if _, err := logUnknownToolCalls(next)(context.Background(), tt.method, req); err != tt.err {
				t.Errorf("expected the error to pass through unchanged, got %v want %v", err, tt.err)
			}
			if buf.Len() != 0 {
				t.Errorf("expected no log output, got: %q", buf.String())
			}
		})
	}
}
