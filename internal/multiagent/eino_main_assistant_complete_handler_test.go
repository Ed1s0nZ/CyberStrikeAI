package multiagent

import "testing"

func TestEinoMainAssistantCompleteHandlerEmitsAndRecords(t *testing.T) {
	var eventTypes []string
	var messages []string
	progress := func(eventType, message string, _ interface{}) {
		eventTypes = append(eventTypes, eventType)
		messages = append(messages, message)
	}
	out := newEinoAssistantOutputAccumulator("deep")
	handler := newEinoMainAssistantCompleteHandler(einoMainAssistantCompleteHandlerConfig{
		AgentName:       "lead",
		Emitter:         newEinoMainResponseStreamEmitter("conv-1", "deep", "lead", "stream-1", 2, progress, nil),
		AssistantOutput: out,
	})

	if !handler.EmitComplete(" hello ") {
		t.Fatal("complete assistant should emit")
	}
	if len(eventTypes) != 2 || eventTypes[0] != "response_start" || eventTypes[1] != "response_delta" {
		t.Fatalf("events = %#v", eventTypes)
	}
	if messages[1] != "hello" {
		t.Fatalf("delta message = %q", messages[1])
	}
	if out.LastAssistant() != "hello" {
		t.Fatalf("last assistant = %q", out.LastAssistant())
	}
}

func TestEinoMainAssistantCompleteHandlerSuppressesDuplicateExecuteStdout(t *testing.T) {
	var eventTypes []string
	progress := func(eventType, _ string, _ interface{}) {
		eventTypes = append(eventTypes, eventType)
	}
	stdoutDup := newEinoExecuteStdoutSuppressor()
	stdoutDup.Record("execute", "hello", false)
	out := newEinoAssistantOutputAccumulator("deep")
	handler := newEinoMainAssistantCompleteHandler(einoMainAssistantCompleteHandlerConfig{
		AgentName:        "lead",
		Emitter:          newEinoMainResponseStreamEmitter("conv-1", "deep", "lead", "stream-1", 1, progress, nil),
		StdoutSuppressor: stdoutDup,
		AssistantOutput:  out,
	})

	if handler.EmitComplete("hello") {
		t.Fatal("duplicate execute stdout should not emit")
	}
	if len(eventTypes) != 0 {
		t.Fatalf("events = %#v, want none", eventTypes)
	}
	if out.LastAssistant() != "hello" {
		t.Fatalf("last assistant = %q", out.LastAssistant())
	}
	if stdoutDup.Peek() != "" {
		t.Fatal("duplicate target should be consumed")
	}
}

func TestEinoMainAssistantCompleteHandlerRecordsWithoutProgress(t *testing.T) {
	out := newEinoAssistantOutputAccumulator("plan_execute")
	handler := newEinoMainAssistantCompleteHandler(einoMainAssistantCompleteHandlerConfig{
		AgentName:       "executor",
		Emitter:         newEinoMainResponseStreamEmitter("conv-1", "plan_execute", "executor", "stream-1", 1, nil, nil),
		AssistantOutput: out,
	})

	if handler.EmitComplete(`{"response":"done"}`) {
		t.Fatal("nil progress should not emit")
	}
	if out.LastPlanExecuteExecutor() != "done" {
		t.Fatalf("executor output = %q", out.LastPlanExecuteExecutor())
	}
}
