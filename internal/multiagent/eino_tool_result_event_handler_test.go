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

func TestEinoToolResultEventHandlerSplitsParallelStreamingResults(t *testing.T) {
	var events []map[string]interface{}
	runMessages := newEinoRunMessageAccumulator(nil)
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
	})
	stream := schema.StreamReaderFromArray([]*schema.Message{
		{Role: schema.Tool, Content: "nmap 1/2 start ", ToolCallID: "call-1", ToolName: "nmap"},
		{Role: schema.Tool, Content: "nmap 2/2 start ", ToolCallID: "call-2", ToolName: "nmap"},
		{Role: schema.Tool, Content: "22/tcp open", ToolCallID: "call-1", ToolName: "nmap"},
		{Role: schema.Tool, Content: "80/tcp open", ToolCallID: "call-2", ToolName: "nmap"},
	})
	mv := &adk.MessageVariant{
		IsStreaming:   true,
		Role:          schema.Tool,
		ToolName:      "nmap",
		MessageStream: stream,
	}

	if !handler.HandleStreaming(mv, "worker") {
		t.Fatal("streaming tool result was not handled")
	}
	if len(events) != 2 {
		t.Fatalf("events = %#v, want two tool_result", events)
	}
	if events[0]["toolCallId"] != "call-1" || events[0]["result"] != "nmap 1/2 start 22/tcp open" {
		t.Fatalf("first event = %#v", events[0])
	}
	if events[1]["toolCallId"] != "call-2" || events[1]["result"] != "nmap 2/2 start 80/tcp open" {
		t.Fatalf("second event = %#v", events[1])
	}
	msgs := runMessages.Messages()
	if len(msgs) != 2 || msgs[0].ToolCallID != "call-1" || msgs[1].ToolCallID != "call-2" {
		t.Fatalf("run messages = %#v", msgs)
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

func TestEinoToolResultEventHandlerMarksCanceledStreamAsInterrupted(t *testing.T) {
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
	reader, writer := schema.Pipe[*schema.Message](1)
	writer.Send(nil, adk.ErrStreamCanceled)
	writer.Close()
	mv := &adk.MessageVariant{
		IsStreaming:   true,
		Role:          schema.Tool,
		ToolName:      "http-framework-test",
		MessageStream: reader,
	}

	if !handler.HandleStreaming(mv, "penetration") {
		t.Fatal("canceled tool stream was not handled")
	}
	if event["toolName"] != "http-framework-test" {
		t.Fatalf("event = %#v", event)
	}
	if event["isError"] != true || event["success"] != false {
		t.Fatalf("event flags = %#v, want interrupted tool result", event)
	}
	result, _ := event["result"].(string)
	if result == "stream canceled" || result == "" {
		t.Fatalf("result = %q, want interrupt-continue notice", result)
	}
}
