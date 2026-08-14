package multiagent

import "testing"

func TestEinoMainAssistantStreamHandlerEmitsAndRecords(t *testing.T) {
	var eventTypes []string
	var messages []string
	progress := func(eventType, message string, _ interface{}) {
		eventTypes = append(eventTypes, eventType)
		messages = append(messages, message)
	}
	out := newEinoAssistantOutputAccumulator("deep")
	runMsgs := newEinoRunMessageAccumulator(nil)
	emitter := newEinoMainResponseStreamEmitter("conv-1", "deep", "lead", "stream-1", 2, progress, nil)
	handler := newEinoMainAssistantStreamHandler(einoMainAssistantStreamHandlerConfig{
		AgentName:       "lead",
		Emitter:         emitter,
		AssistantOutput: out,
		RunMessages:     runMsgs,
	})

	if !handler.EmitDelta("he") {
		t.Fatal("first delta should emit")
	}
	if !handler.EmitDelta("hello") {
		t.Fatal("cumulative chunk should emit tail")
	}
	if got := handler.Finish(); got != "hello" {
		t.Fatalf("finish = %q, want hello", got)
	}

	if len(eventTypes) != 3 || eventTypes[0] != "response_start" || eventTypes[1] != "response_delta" || eventTypes[2] != "response_delta" {
		t.Fatalf("events = %#v", eventTypes)
	}
	if messages[1] != "he" || messages[2] != "llo" {
		t.Fatalf("delta messages = %#v", messages)
	}
	if out.LastAssistant() != "hello" {
		t.Fatalf("last assistant = %q", out.LastAssistant())
	}
	if msgs := runMsgs.Messages(); len(msgs) != 1 || msgs[0].Content != "hello" {
		t.Fatalf("run messages = %#v", msgs)
	}
}

func TestEinoMainAssistantStreamHandlerSuppressesDuplicateExecuteStdout(t *testing.T) {
	var eventTypes []string
	progress := func(eventType, _ string, _ interface{}) {
		eventTypes = append(eventTypes, eventType)
	}
	stdoutDup := newEinoExecuteStdoutSuppressor()
	stdoutDup.Record("execute", "hello", false)
	out := newEinoAssistantOutputAccumulator("deep")
	runMsgs := newEinoRunMessageAccumulator(nil)
	handler := newEinoMainAssistantStreamHandler(einoMainAssistantStreamHandlerConfig{
		AgentName:        "lead",
		Emitter:          newEinoMainResponseStreamEmitter("conv-1", "deep", "lead", "stream-1", 1, progress, nil),
		StdoutSuppressor: stdoutDup,
		AssistantOutput:  out,
		RunMessages:      runMsgs,
	})

	if handler.EmitDelta("hello") {
		t.Fatal("duplicate execute stdout should not emit delta")
	}
	if got := handler.Finish(); got != "hello" {
		t.Fatalf("finish = %q, want hello", got)
	}
	if len(eventTypes) != 0 {
		t.Fatalf("events = %#v, want none", eventTypes)
	}
	if stdoutDup.Peek() != "" {
		t.Fatal("duplicate target should be cleared on finish")
	}
	if out.LastAssistant() != "hello" {
		t.Fatalf("last assistant = %q", out.LastAssistant())
	}
	if msgs := runMsgs.Messages(); len(msgs) != 1 || msgs[0].Content != "hello" {
		t.Fatalf("run messages = %#v", msgs)
	}
}

func TestEinoMainAssistantStreamHandlerRecordsWithoutProgress(t *testing.T) {
	out := newEinoAssistantOutputAccumulator("plan_execute")
	runMsgs := newEinoRunMessageAccumulator(nil)
	handler := newEinoMainAssistantStreamHandler(einoMainAssistantStreamHandlerConfig{
		AgentName:       "executor",
		Emitter:         newEinoMainResponseStreamEmitter("conv-1", "plan_execute", "executor", "stream-1", 1, nil, nil),
		AssistantOutput: out,
		RunMessages:     runMsgs,
	})

	handler.EmitDelta(`{"response":"done"}`)
	if got := handler.Finish(); got != `{"response":"done"}` {
		t.Fatalf("finish = %q", got)
	}
	if out.LastPlanExecuteExecutor() != "done" {
		t.Fatalf("executor output = %q", out.LastPlanExecuteExecutor())
	}
	if msgs := runMsgs.Messages(); len(msgs) != 1 || msgs[0].Content != `{"response":"done"}` {
		t.Fatalf("run messages = %#v", msgs)
	}
}
