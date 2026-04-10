package multiagent

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/cloudwego/eino/compose"
)

func TestIsSoftRecoverableToolError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "unexpected end of JSON input",
			err:      errors.New("unexpected end of JSON input"),
			expected: true,
		},
		{
			name:     "failed to unmarshal task tool input json",
			err:      errors.New("failed to unmarshal task tool input json: unexpected end of JSON input"),
			expected: true,
		},
		{
			name:     "invalid tool arguments JSON",
			err:      errors.New("invalid tool arguments JSON: unexpected end of JSON input"),
			expected: true,
		},
		{
			name:     "json invalid character",
			err:      errors.New(`invalid character '}' looking for beginning of value in JSON`),
			expected: true,
		},
		{
			name:     "subagent type not found",
			err:      errors.New("subagent type recon_agent not found"),
			expected: true,
		},
		{
			name:     "tool not found",
			err:      errors.New("tool nmap_scan not found in toolsNode indexes"),
			expected: true,
		},
		{
			name:     "unrelated network error",
			err:      errors.New("connection refused"),
			expected: false,
		},
		{
			name:     "context cancelled",
			err:      context.Canceled,
			expected: false,
		},
		{
			name: "real json unmarshal error",
			err: func() error {
				var v map[string]interface{}
				return json.Unmarshal([]byte(`{"key": `), &v)
			}(),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isSoftRecoverableToolError(tt.err)
			if got != tt.expected {
				t.Errorf("isSoftRecoverableToolError(%v) = %v, want %v", tt.err, got, tt.expected)
			}
		})
	}
}

func TestSoftRecoveryToolCallMiddleware_PassesThrough(t *testing.T) {
	mw := softRecoveryToolCallMiddleware()
	called := false
	next := func(ctx context.Context, input *compose.ToolInput) (*compose.ToolOutput, error) {
		called = true
		return &compose.ToolOutput{Result: "success"}, nil
	}
	wrapped := mw(next)
	out, err := wrapped(context.Background(), &compose.ToolInput{
		Name:      "test_tool",
		Arguments: `{"key": "value"}`,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("next endpoint was not called")
	}
	if out.Result != "success" {
		t.Fatalf("expected 'success', got %q", out.Result)
	}
}

func TestSoftRecoveryToolCallMiddleware_ConvertsJSONError(t *testing.T) {
	mw := softRecoveryToolCallMiddleware()
	next := func(ctx context.Context, input *compose.ToolInput) (*compose.ToolOutput, error) {
		return nil, errors.New("failed to unmarshal task tool input json: unexpected end of JSON input")
	}
	wrapped := mw(next)
	out, err := wrapped(context.Background(), &compose.ToolInput{
		Name:      "task",
		Arguments: `{"subagent_type": "recon`,
	})
	if err != nil {
		t.Fatalf("expected nil error (soft recovery), got: %v", err)
	}
	if out == nil || out.Result == "" {
		t.Fatal("expected non-empty recovery message")
	}
	if !containsAll(out.Result, "[Tool Error]", "task", "JSON") {
		t.Fatalf("recovery message missing expected content: %s", out.Result)
	}
}

func TestSoftRecoveryToolCallMiddleware_PropagatesNonRecoverable(t *testing.T) {
	mw := softRecoveryToolCallMiddleware()
	origErr := errors.New("connection timeout to remote server")
	next := func(ctx context.Context, input *compose.ToolInput) (*compose.ToolOutput, error) {
		return nil, origErr
	}
	wrapped := mw(next)
	_, err := wrapped(context.Background(), &compose.ToolInput{
		Name:      "test_tool",
		Arguments: `{}`,
	})
	if err == nil {
		t.Fatal("expected error to propagate for non-recoverable errors")
	}
	if err != origErr {
		t.Fatalf("expected original error, got: %v", err)
	}
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !contains(s, sub) {
			return false
		}
	}
	return true
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && searchString(s, sub)
}

func searchString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
