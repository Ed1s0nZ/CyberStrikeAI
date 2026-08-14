package multiagent

import (
	"testing"

	"cyberstrike-ai/internal/einomcp"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

func TestEinoMaterializedMessageEventHandlerHandlesMainAssistant(t *testing.T) {
	var events []string
	runMessages := newEinoRunMessageAccumulator(nil)
	assistantOutput := newEinoAssistantOutputAccumulator("deep")
	usage := newEinoRunUsageAccumulator()
	runProgress := newEinoRunProgressTracker(
		"deep", "lead", "conv-1",
		func(eventType, _ string, _ interface{}) { events = append(events, eventType) },
		func(agent string) bool { return agent == "lead" },
		nil,
	)
	handler := newEinoMaterializedMessageEventHandler(einoMaterializedMessageEventHandlerConfig{
		ConversationID:       "conv-1",
		OrchMode:             "deep",
		Progress:             func(eventType, _ string, _ interface{}) { events = append(events, eventType) },
		RunMessages:          runMessages,
		Usage:                usage,
		AssistantOutput:      assistantOutput,
		RunProgress:          runProgress,
		StreamsMainAssistant: func(agent string) bool { return agent == "lead" },
		EinoRoleTag:          func(string) string { return "orchestrator" },
		NextMainStreamID:     func() string { return "main-complete-1" },
	})
	msg := schema.AssistantMessage(" done ", nil)
	msg.ReasoningContent = "thought"
	msg.ResponseMeta = &schema.ResponseMeta{Usage: &schema.TokenUsage{
		PromptTokens:     11,
		CompletionTokens: 7,
		TotalTokens:      18,
	}}
	mv := &adk.MessageVariant{Role: schema.Assistant}

	if !handler.Handle(mv, msg, "lead") {
		t.Fatal("main assistant message was not handled")
	}
	if assistantOutput.LastAssistant() != "done" {
		t.Fatalf("last assistant = %q", assistantOutput.LastAssistant())
	}
	if msgs := runMessages.Messages(); len(msgs) != 1 || msgs[0].Content != " done " {
		t.Fatalf("run messages = %#v", msgs)
	}
	if got := usage.Summary(); got.ModelCalls != 1 || got.TotalTokens != 18 {
		t.Fatalf("usage = %#v, want one assistant model call", got)
	}
	if !containsString(events, "reasoning_chain") || !containsString(events, "response_start") || !containsString(events, "response_delta") {
		t.Fatalf("events = %#v, want reasoning and response events", events)
	}
}

func TestEinoMaterializedMessageEventHandlerHandlesSubAssistant(t *testing.T) {
	var events []string
	runMessages := newEinoRunMessageAccumulator(nil)
	assistantOutput := newEinoAssistantOutputAccumulator("deep")
	handler := newEinoMaterializedMessageEventHandler(einoMaterializedMessageEventHandlerConfig{
		ConversationID:       "conv-1",
		OrchMode:             "deep",
		Progress:             func(eventType, _ string, _ interface{}) { events = append(events, eventType) },
		RunMessages:          runMessages,
		AssistantOutput:      assistantOutput,
		StreamsMainAssistant: func(agent string) bool { return agent == "lead" },
		EinoRoleTag:          func(string) string { return "sub" },
	})

	if !handler.Handle(&adk.MessageVariant{Role: schema.Assistant}, schema.AssistantMessage("sub done", nil), "worker") {
		t.Fatal("sub assistant message was not handled")
	}
	if assistantOutput.LastAssistant() != "" {
		t.Fatalf("sub assistant should not update main output, got %q", assistantOutput.LastAssistant())
	}
	if len(runMessages.Messages()) != 1 {
		t.Fatalf("run messages = %#v, want appended original message", runMessages.Messages())
	}
	if !containsString(events, "eino_agent_reply") {
		t.Fatalf("events = %#v, want sub reply event", events)
	}
}

func TestEinoMaterializedMessageEventHandlerHandlesToolCallsAndToolResult(t *testing.T) {
	var events []string
	var marked []toolCallPendingInfo
	runMessages := newEinoRunMessageAccumulator(nil)
	runProgress := newEinoRunProgressTracker(
		"deep", "lead", "conv-1",
		func(eventType, _ string, _ interface{}) { events = append(events, eventType) },
		func(agent string) bool { return agent == "lead" },
		nil,
	)
	toolResultEmitter := newEinoToolResultProgressEmitter(einoToolResultProgressEmitterConfig{
		ConversationID: "conv-1",
		Progress:       func(eventType, _ string, _ interface{}) { events = append(events, eventType) },
	})
	toolResultHandler := newEinoToolResultEventHandler(einoToolResultEventHandlerConfig{Emitter: toolResultEmitter})
	handler := newEinoMaterializedMessageEventHandler(einoMaterializedMessageEventHandlerConfig{
		ConversationID:    "conv-1",
		OrchMode:          "deep",
		Progress:          func(eventType, _ string, _ interface{}) { events = append(events, eventType) },
		RunMessages:       runMessages,
		RunProgress:       runProgress,
		ToolResultHandler: toolResultHandler,
		MarkPending: func(info toolCallPendingInfo) {
			marked = append(marked, info)
		},
	})

	toolCallMsg := &schema.Message{
		Role: schema.Assistant,
		ToolCalls: []schema.ToolCall{{
			ID:   "call-1",
			Type: "function",
			Function: schema.FunctionCall{
				Name:      "execute",
				Arguments: `{"command":`,
			},
		}},
	}
	if !handler.Handle(&adk.MessageVariant{Role: schema.Assistant}, toolCallMsg, "lead") {
		t.Fatal("tool call message was not handled")
	}
	toolMsg := schema.ToolMessage(einomcp.ToolErrorPrefix+"bad command", "call-1", schema.WithToolName("execute"))
	if !handler.Handle(&adk.MessageVariant{Role: schema.Tool}, toolMsg, "lead") {
		t.Fatal("tool message was not handled")
	}

	if !containsString(events, "tool_call") || !containsString(events, "tool_result") || containsString(events, "model_output_rejected") {
		t.Fatalf("events = %#v, want real tool_call and tool_result without model-output recovery", events)
	}
	if len(marked) != 1 || marked[0].ToolCallID != "call-1" || marked[0].ToolName != "execute" {
		t.Fatalf("marked pending = %#v", marked)
	}
	if len(runMessages.Messages()) != 2 {
		t.Fatalf("run messages = %#v, want assistant and tool messages", runMessages.Messages())
	}
}

func TestEinoMaterializedMessageEventHandlerIgnoresNil(t *testing.T) {
	handler := newEinoMaterializedMessageEventHandler(einoMaterializedMessageEventHandlerConfig{})
	if handler.Handle(nil, nil, "lead") {
		t.Fatal("nil message should be ignored")
	}
}
