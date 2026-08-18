package multiagent

import (
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

func TestEinoToolResultProgressEmitterInfersPendingAndDedupes(t *testing.T) {
	var events []map[string]interface{}
	progress := func(eventType, _ string, data interface{}) {
		if eventType != "tool_result" {
			return
		}
		m, _ := data.(map[string]interface{})
		events = append(events, m)
	}
	pending := newEinoPendingToolCalls("conv-1", nil)
	pending.Mark(toolCallPendingInfo{
		ToolCallID: "call-1",
		ToolName:   "execute",
		EinoAgent:  "worker",
		EinoRole:   "sub",
	})
	stdoutDup := newEinoExecuteStdoutSuppressor()
	emitter := newEinoToolResultProgressEmitter(einoToolResultProgressEmitterConfig{
		ConversationID:   "conv-1",
		OrchestratorName: "lead",
		Progress:         progress,
		EinoRoleTag: func(agent string) string {
			if agent == "worker" {
				return "sub"
			}
			return "orchestrator"
		},
		Pending:          pending,
		ExecuteStdoutDup: stdoutDup,
	})

	if !emitter.Emit(nil, "execute", "hello", "", false, "worker") {
		t.Fatal("first tool result should emit")
	}
	if !emitter.Emit(nil, "execute", "duplicate without id", "", false, "worker") {
		t.Fatal("id-less result should still emit after pending queue is empty")
	}
	if emitter.Emit(nil, "execute", "duplicate", "call-1", false, "worker") {
		t.Fatal("duplicate toolCallId should not emit")
	}

	if len(events) != 2 {
		t.Fatalf("events = %#v, want two emitted results", events)
	}
	if events[0]["toolCallId"] != "call-1" || events[0]["einoRole"] != "sub" {
		t.Fatalf("first event data = %#v", events[0])
	}
	if _, ok := events[1]["toolCallId"]; ok {
		t.Fatalf("second event should not invent toolCallId: %#v", events[1])
	}
	if got := stdoutDup.Peek(); got != "duplicate without id" {
		t.Fatalf("execute stdout suppressor = %q, want last emitted execute stdout", got)
	}
	if pending.Count() != 0 {
		t.Fatalf("pending count = %d, want 0", pending.Count())
	}
}

func TestEinoToolResultProgressEmitterBackgroundWaitDisplaysRunning(t *testing.T) {
	var data map[string]interface{}
	progress := func(eventType, _ string, raw interface{}) {
		if eventType == "tool_result" {
			data, _ = raw.(map[string]interface{})
		}
	}
	body := `工具已提交到后台执行，但本次等待已到达上限。

execution_id: 3eaaa391-050b-4be1-a870-48a855923cb7
tool: exec
status: running
wait_timeout: 10s`
	emitter := newEinoToolResultProgressEmitter(einoToolResultProgressEmitterConfig{
		ConversationID: "conv-1",
		Progress:       progress,
	})

	if !emitter.Emit(nil, "exec", body, "call-1", true, "lead") {
		t.Fatal("background wait result should emit")
	}
	if data["success"] != true || data["isError"] != false || data["status"] != "background_running" {
		t.Fatalf("background display flags = %#v", data)
	}
	if data["modelFacingIsError"] != true {
		t.Fatalf("modelFacingIsError = %#v", data["modelFacingIsError"])
	}
	if data["executionId"] != "3eaaa391-050b-4be1-a870-48a855923cb7" {
		t.Fatalf("executionId = %#v", data["executionId"])
	}
}

func TestEinoToolResultProgressEmitterHidesModelOutputRejectedResult(t *testing.T) {
	called := false
	emitter := newEinoToolResultProgressEmitter(einoToolResultProgressEmitterConfig{
		ConversationID: "conv-1",
		Progress: func(eventType, _ string, _ interface{}) {
			if eventType == "tool_result" {
				called = true
			}
		},
	})

	if emitter.Emit(nil, "task", modelOutputRejectedResultPrefix+" Tool call was not executed.", "call-1", true, "lead") {
		t.Fatal("model output rejected result should not emit")
	}
	if called {
		t.Fatal("progress should not receive model output rejected tool_result")
	}
}

func TestEinoToolResultProgressEmitterTruncatesPreview(t *testing.T) {
	var data map[string]interface{}
	progress := func(eventType, _ string, raw interface{}) {
		if eventType == "tool_result" {
			data, _ = raw.(map[string]interface{})
		}
	}
	long := ""
	for i := 0; i < 205; i++ {
		long += "x"
	}
	emitter := newEinoToolResultProgressEmitter(einoToolResultProgressEmitterConfig{
		ConversationID: "conv-1",
		Progress:       progress,
	})
	emitter.Emit(nil, "", long, "", false, "")

	if data["toolName"] != "unknown" {
		t.Fatalf("tool name = %#v", data["toolName"])
	}
	if got, _ := data["resultPreview"].(string); len(got) != 203 || got[200:] != "..." {
		t.Fatalf("preview = %q len=%d", got, len(got))
	}
}

func TestEinoToolResultProgressEmitterBackfillsArgumentsFromRunMessages(t *testing.T) {
	var data map[string]interface{}
	progress := func(eventType, _ string, raw interface{}) {
		if eventType == "tool_result" {
			data, _ = raw.(map[string]interface{})
		}
	}
	runMessages := newEinoRunMessageAccumulator([]adk.Message{
		&schema.Message{
			Role: schema.Assistant,
			ToolCalls: []schema.ToolCall{{
				ID:   "call-read",
				Type: "function",
				Function: schema.FunctionCall{
					Name:      "read_file",
					Arguments: `{"path":"/tmp/requirements.txt"}`,
				},
			}},
		},
	})
	emitter := newEinoToolResultProgressEmitter(einoToolResultProgressEmitterConfig{
		ConversationID: "conv-1",
		Progress:       progress,
		RunMessages:    runMessages,
	})

	if !emitter.Emit(nil, "read_file", "ok", "call-read", false, "lead") {
		t.Fatal("expected tool result emit")
	}
	args, ok := data["argumentsObj"].(map[string]interface{})
	if !ok {
		t.Fatalf("argumentsObj = %#v", data["argumentsObj"])
	}
	if args["path"] != "/tmp/requirements.txt" {
		t.Fatalf("path = %#v, want /tmp/requirements.txt", args["path"])
	}
	if data["arguments"] != `{"path":"/tmp/requirements.txt"}` {
		t.Fatalf("arguments = %#v", data["arguments"])
	}
}
