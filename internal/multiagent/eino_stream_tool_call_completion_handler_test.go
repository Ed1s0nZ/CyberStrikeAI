package multiagent

import (
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestEinoStreamToolCallCompletionHandlerMergesEmitsAndPersistsToolCalls(t *testing.T) {
	idx := 0
	var eventTypes []string
	var marked []toolCallPendingInfo
	progress := func(eventType, _ string, _ interface{}) {
		eventTypes = append(eventTypes, eventType)
	}
	runMessages := newEinoRunMessageAccumulator(nil)
	runProgress := newEinoRunProgressTracker(
		"deep", "lead", "conv-1", progress,
		func(agent string) bool { return agent == "lead" },
		nil,
	)
	handler := newEinoStreamToolCallCompletionHandler(einoStreamToolCallCompletionHandlerConfig{
		ConversationID: "conv-1",
		OrchMode:       "deep",
		Progress:       progress,
		RunProgress:    runProgress,
		RunMessages:    runMessages,
		MarkPending: func(info toolCallPendingInfo) {
			marked = append(marked, info)
		},
	})

	chunk := handler.Complete([]schema.ToolCall{
		{
			ID:    "call-1",
			Type:  "function",
			Index: &idx,
			Function: schema.FunctionCall{
				Name:      "execute",
				Arguments: `{"command":`,
			},
		},
		{
			Index: &idx,
			Function: schema.FunctionCall{
				Arguments: `"pwd"}`,
			},
		},
	}, "lead")

	if chunk == nil || len(chunk.ToolCalls) != 1 {
		t.Fatalf("merged chunk = %#v, want one tool call", chunk)
	}
	if got := chunk.ToolCalls[0].Function.Arguments; got != `{"command":"pwd"}` {
		t.Fatalf("arguments = %q", got)
	}
	msgs := runMessages.Messages()
	if len(msgs) != 1 || len(msgs[0].ToolCalls) != 1 {
		t.Fatalf("run messages = %#v, want persisted assistant tool call", msgs)
	}
	if len(marked) != 1 || marked[0].ToolCallID != "call-1" || marked[0].ToolName != "execute" {
		t.Fatalf("marked pending = %#v", marked)
	}
	if !containsString(eventTypes, "tool_call") {
		t.Fatalf("event types = %#v, want tool_call", eventTypes)
	}
}

func TestEinoStreamToolCallCompletionHandlerPreservesStreamingToolArgumentsForToolLayerRecovery(t *testing.T) {
	idx := 0
	var eventTypes []string
	var marked []toolCallPendingInfo
	progress := func(eventType, _ string, _ interface{}) {
		eventTypes = append(eventTypes, eventType)
	}
	runMessages := newEinoRunMessageAccumulator(nil)
	runProgress := newEinoRunProgressTracker(
		"deep", "lead", "conv-1", progress,
		func(agent string) bool { return agent == "lead" },
		nil,
	)
	handler := newEinoStreamToolCallCompletionHandler(einoStreamToolCallCompletionHandlerConfig{
		ConversationID: "conv-1",
		OrchMode:       "deep",
		Progress:       progress,
		RunProgress:    runProgress,
		RunMessages:    runMessages,
		MarkPending: func(info toolCallPendingInfo) {
			marked = append(marked, info)
		},
	})

	chunk := handler.Complete([]schema.ToolCall{
		{
			ID:    "call-stream-unsafe",
			Type:  "function",
			Index: &idx,
			Function: schema.FunctionCall{
				Name:      "execute",
				Arguments: `{"command":"`,
			},
		},
		{
			Index: &idx,
			Function: schema.FunctionCall{
				Arguments: strings.Repeat("x", 256) + `"}`,
			},
		},
	}, "lead")

	if chunk == nil || len(chunk.ToolCalls) != 1 {
		t.Fatalf("chunk = %#v, want one tool call", chunk)
	}
	args := chunk.ToolCalls[0].Function.Arguments
	if !strings.Contains(args, strings.Repeat("x", 32)) {
		t.Fatalf("streaming arguments were unexpectedly rewritten: %q", args)
	}
	msgs := runMessages.Messages()
	if len(msgs) != 1 || len(msgs[0].ToolCalls) != 1 {
		t.Fatalf("run messages = %#v, want assistant tool call", msgs)
	}
	if got := msgs[0].ToolCalls[0].Function.Arguments; got != args {
		t.Fatalf("persisted tool call arguments = %q, want %q", got, args)
	}
	if len(marked) != 1 || marked[0].ToolCallID != "call-stream-unsafe" || marked[0].ToolName != "execute" {
		t.Fatalf("marked pending = %#v", marked)
	}
	if containsString(eventTypes, "model_output_rejected") || !containsString(eventTypes, "tool_call") {
		t.Fatalf("event types = %#v, want real tool_call without model-output recovery", eventTypes)
	}
}

func TestEinoStreamToolCallCompletionHandlerIgnoresEmptyFragments(t *testing.T) {
	runMessages := newEinoRunMessageAccumulator(nil)
	called := false
	handler := newEinoStreamToolCallCompletionHandler(einoStreamToolCallCompletionHandlerConfig{
		RunMessages: runMessages,
		Progress: func(string, string, interface{}) {
			called = true
		},
	})
	if chunk := handler.Complete(nil, "lead"); chunk != nil {
		t.Fatalf("chunk = %#v, want nil", chunk)
	}
	if len(runMessages.Messages()) != 0 {
		t.Fatalf("run messages = %#v, want empty", runMessages.Messages())
	}
	if called {
		t.Fatal("progress should not be called for empty fragments")
	}
}
