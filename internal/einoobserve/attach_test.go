package einoobserve

import (
	"context"
	"testing"

	"cyberstrike-ai/internal/config"
)

func TestAttachAgentRunCallbacks_Disabled(t *testing.T) {
	ctx := context.Background()
	cfg := &config.MultiAgentEinoCallbacksConfig{Enabled: false}
	out := AttachAgentRunCallbacks(ctx, cfg, Params{})
	if out != ctx {
		t.Fatalf("expected same ctx when disabled")
	}
}

func TestAttachAgentRunCallbacksUsesProvidedRunID(t *testing.T) {
	emit := true
	var gotRunID string
	ctx := context.Background()
	cfg := &config.MultiAgentEinoCallbacksConfig{Enabled: true, Mode: "sse", SseTraceToClient: &emit}

	AttachAgentRunCallbacks(ctx, cfg, Params{
		RunID: "run-shared",
		Progress: func(eventType, _ string, data interface{}) {
			if eventType != "eino_trace_run" {
				return
			}
			if m, ok := data.(map[string]interface{}); ok {
				gotRunID, _ = m["runId"].(string)
			}
		},
	})

	if gotRunID != "run-shared" {
		t.Fatalf("runId = %q, want run-shared", gotRunID)
	}
}

func TestTruncateRunes(t *testing.T) {
	if got := truncateRunes("abc", 10); got != "abc" {
		t.Fatalf("got %q", got)
	}
	if got := truncateRunes("abcdefghij", 4); got != "abcd…" {
		t.Fatalf("got %q", got)
	}
}
