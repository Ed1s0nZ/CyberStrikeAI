package multiagent

import (
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

func TestEinoAssistantStreamEventHandlerHandlesMainAssistantStream(t *testing.T) {
	var events []string
	runMessages := newEinoRunMessageAccumulator(nil)
	assistantOutput := newEinoAssistantOutputAccumulator("deep")
	usage := newEinoRunUsageAccumulator()
	handler := newEinoAssistantStreamEventHandler(einoAssistantStreamEventHandlerConfig{
		ConversationID:       "conv-1",
		OrchMode:             "deep",
		RunMessages:          runMessages,
		Usage:                usage,
		AssistantOutput:      assistantOutput,
		StreamsMainAssistant: func(agent string) bool { return agent == "lead" },
		EinoRoleTag:          func(string) string { return "orchestrator" },
		NextMainStreamID:     func() string { return "main-stream-1" },
		Progress: func(eventType, _ string, _ interface{}) {
			events = append(events, eventType)
		},
	})
	mv := &adk.MessageVariant{
		IsStreaming: true,
		Role:        schema.Assistant,
		MessageStream: schema.StreamReaderFromArray([]*schema.Message{
			{Role: schema.Assistant, Content: "he", ResponseMeta: &schema.ResponseMeta{Usage: &schema.TokenUsage{PromptTokens: 10, CompletionTokens: 1, TotalTokens: 11}}},
			{Role: schema.Assistant, Content: "hello", ResponseMeta: &schema.ResponseMeta{Usage: &schema.TokenUsage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15}}},
		}),
	}

	handled, err := handler.Handle(mv, "lead")
	if !handled || err != nil {
		t.Fatalf("handled=%v err=%v", handled, err)
	}
	if assistantOutput.LastAssistant() != "hello" {
		t.Fatalf("last assistant = %q", assistantOutput.LastAssistant())
	}
	if msgs := runMessages.Messages(); len(msgs) != 1 || msgs[0].Content != "hello" {
		t.Fatalf("run messages = %#v", msgs)
	}
	if got := usage.Summary(); got.ModelCalls != 1 || got.PromptTokens != 10 || got.CompletionTokens != 5 || got.TotalTokens != 15 {
		t.Fatalf("usage = %#v, want one stream model call", got)
	}
	if !containsString(events, "response_start") || !containsString(events, "response_delta") {
		t.Fatalf("events = %#v, want response stream events", events)
	}
}

func TestEinoAssistantStreamEventHandlerHandlesSubAgentStream(t *testing.T) {
	var events []string
	runMessages := newEinoRunMessageAccumulator(nil)
	assistantOutput := newEinoAssistantOutputAccumulator("deep")
	handler := newEinoAssistantStreamEventHandler(einoAssistantStreamEventHandlerConfig{
		ConversationID:       "conv-1",
		OrchMode:             "deep",
		RunMessages:          runMessages,
		AssistantOutput:      assistantOutput,
		StreamsMainAssistant: func(agent string) bool { return agent == "lead" },
		EinoRoleTag:          func(string) string { return "sub" },
		NextSubAgentReplyStreamID: func() string {
			return "sub-stream-1"
		},
		Progress: func(eventType, _ string, _ interface{}) {
			events = append(events, eventType)
		},
	})
	mv := &adk.MessageVariant{
		IsStreaming:   true,
		Role:          schema.Assistant,
		MessageStream: schema.StreamReaderFromArray([]*schema.Message{{Role: schema.Assistant, Content: "sub reply"}}),
	}

	handled, err := handler.Handle(mv, "worker")
	if !handled || err != nil {
		t.Fatalf("handled=%v err=%v", handled, err)
	}
	if len(runMessages.Messages()) != 0 {
		t.Fatalf("sub stream should not append main run text, got %#v", runMessages.Messages())
	}
	if assistantOutput.LastAssistant() != "" {
		t.Fatalf("sub stream should not record main assistant, got %q", assistantOutput.LastAssistant())
	}
	if !containsString(events, "eino_agent_reply_stream_start") ||
		!containsString(events, "eino_agent_reply_stream_delta") ||
		!containsString(events, "eino_agent_reply_stream_end") {
		t.Fatalf("events = %#v, want sub reply stream events", events)
	}
}

func TestEinoAssistantStreamEventHandlerCompletesToolFragments(t *testing.T) {
	idx := 0
	var events []string
	runMessages := newEinoRunMessageAccumulator(nil)
	runProgress := newEinoRunProgressTracker(
		"deep", "lead", "conv-1",
		func(eventType, _ string, _ interface{}) { events = append(events, eventType) },
		func(agent string) bool { return agent == "lead" },
		nil,
	)
	completion := newEinoStreamToolCallCompletionHandler(einoStreamToolCallCompletionHandlerConfig{
		ConversationID: "conv-1",
		OrchMode:       "deep",
		Progress:       func(eventType, _ string, _ interface{}) { events = append(events, eventType) },
		RunProgress:    runProgress,
		RunMessages:    runMessages,
	})
	handler := newEinoAssistantStreamEventHandler(einoAssistantStreamEventHandlerConfig{
		ConversationID:       "conv-1",
		OrchMode:             "deep",
		RunMessages:          runMessages,
		StreamsMainAssistant: func(string) bool { return true },
		ToolCallCompletion:   completion,
	})
	mv := &adk.MessageVariant{
		IsStreaming: true,
		Role:        schema.Assistant,
		MessageStream: schema.StreamReaderFromArray([]*schema.Message{
			{Role: schema.Assistant, ToolCalls: []schema.ToolCall{{ID: "call-1", Index: &idx, Type: "function", Function: schema.FunctionCall{Name: "execute", Arguments: `{"command":`}}}},
			{Role: schema.Assistant, ToolCalls: []schema.ToolCall{{Index: &idx, Function: schema.FunctionCall{Arguments: `"pwd"}`}}}},
		}),
	}

	handled, err := handler.Handle(mv, "lead")
	if !handled || err != nil {
		t.Fatalf("handled=%v err=%v", handled, err)
	}
	msgs := runMessages.Messages()
	if len(msgs) != 1 || len(msgs[0].ToolCalls) != 1 || msgs[0].ToolCalls[0].Function.Arguments != `{"command":"pwd"}` {
		t.Fatalf("run messages = %#v, want merged tool call", msgs)
	}
	if !containsString(events, "tool_call") {
		t.Fatalf("events = %#v, want tool_call", events)
	}
}

func TestEinoAssistantStreamEventHandlerIgnoresToolStream(t *testing.T) {
	handler := newEinoAssistantStreamEventHandler(einoAssistantStreamEventHandlerConfig{})
	handled, err := handler.Handle(&adk.MessageVariant{IsStreaming: true, Role: schema.Tool, MessageStream: schema.StreamReaderFromArray([]*schema.Message{})}, "lead")
	if handled || err != nil {
		t.Fatalf("handled=%v err=%v, want ignored", handled, err)
	}
}
