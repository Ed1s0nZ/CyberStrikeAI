package multiagent

import (
	"testing"

	"cyberstrike-ai/internal/einomcp"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

func TestEinoToolResultEventHandlerHandlesStreamingToolResult(t *testing.T) {
	var events []map[string]interface{}
	runMessages := newEinoRunMessageAccumulator(nil)
	recovered := false
	emitter := newEinoToolResultProgressEmitter(einoToolResultProgressEmitterConfig{
		ConversationID: "conv-1",
		Progress: func(eventType, _ string, data interface{}) {
			if eventType != "tool_result" {
				return
			}
			m, _ := data.(map[string]interface{})
			events = append(events, m)
		},
	})
	handler := newEinoToolResultEventHandler(einoToolResultEventHandlerConfig{
		RunMessages: runMessages,
		Emitter:     emitter,
		ConfirmRecovery: func() {
			recovered = true
		},
	})
	stream := schema.StreamReaderFromArray([]*schema.Message{
		{Role: schema.Tool, Content: "hello ", ToolCallID: "call-1"},
		{Role: schema.Tool, Content: "world", ToolCallID: "call-1"},
	})
	mv := &adk.MessageVariant{
		IsStreaming:   true,
		Role:          schema.Tool,
		ToolName:      "execute",
		MessageStream: stream,
	}

	if !handler.HandleStreaming(mv, "worker") {
		t.Fatal("streaming tool result was not handled")
	}
	if !recovered {
		t.Fatal("expected retry recovery confirmation")
	}
	msgs := runMessages.Messages()
	if len(msgs) != 1 || msgs[0].Role != schema.Tool || msgs[0].Content != "hello world" || msgs[0].ToolCallID != "call-1" {
		t.Fatalf("run messages = %#v", msgs)
	}
	if len(events) != 1 {
		t.Fatalf("events = %#v, want one tool_result", events)
	}
	if events[0]["toolName"] != "execute" || events[0]["toolCallId"] != "call-1" || events[0]["result"] != "hello world" {
		t.Fatalf("event data = %#v", events[0])
	}
}

func TestEinoToolResultEventHandlerHandlesMaterializedToolResult(t *testing.T) {
	var event map[string]interface{}
	emitter := newEinoToolResultProgressEmitter(einoToolResultProgressEmitterConfig{
		ConversationID: "conv-1",
		Progress: func(eventType, _ string, data interface{}) {
			if eventType == "tool_result" {
				event, _ = data.(map[string]interface{})
			}
		},
	})
	handler := newEinoToolResultEventHandler(einoToolResultEventHandlerConfig{Emitter: emitter})
	msg := schema.ToolMessage(einomcp.ToolErrorPrefix+"bad command", "call-2", schema.WithToolName("execute"))
	mv := &adk.MessageVariant{Role: schema.Tool}

	if !handler.HandleMaterialized(mv, msg, "worker") {
		t.Fatal("materialized tool result was not handled")
	}
	if event["toolName"] != "execute" || event["toolCallId"] != "call-2" {
		t.Fatalf("event identity = %#v", event)
	}
	if event["result"] != "bad command" || event["isError"] != true || event["success"] != false {
		t.Fatalf("event result flags = %#v", event)
	}
}

func TestEinoToolResultEventHandlerIgnoresNonToolOutput(t *testing.T) {
	handler := newEinoToolResultEventHandler(einoToolResultEventHandlerConfig{})
	if handler.HandleStreaming(&adk.MessageVariant{IsStreaming: true, Role: schema.Assistant}, "worker") {
		t.Fatal("assistant stream should not be handled as tool result")
	}
	if handler.HandleMaterialized(&adk.MessageVariant{Role: schema.Assistant}, schema.AssistantMessage("hi", nil), "worker") {
		t.Fatal("assistant message should not be handled as tool result")
	}
}
