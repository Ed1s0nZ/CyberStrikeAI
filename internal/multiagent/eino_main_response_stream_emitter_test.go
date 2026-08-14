package multiagent

import (
	"testing"

	"cyberstrike-ai/internal/openai"
)

func TestEinoMainResponseStreamEmitterEmitsStartOnceAndTail(t *testing.T) {
	type progressEvent struct {
		eventType string
		message   string
		data      map[string]interface{}
	}
	var events []progressEvent
	progress := func(eventType, message string, data interface{}) {
		m, _ := data.(map[string]interface{})
		events = append(events, progressEvent{eventType: eventType, message: message, data: m})
	}

	emitter := newEinoMainResponseStreamEmitter(
		"conv-1", "supervisor", "lead", "stream-1", 3, progress, func() []string { return []string{"mcp-1"} },
	)
	if !emitter.EmitDelta("he", "he") {
		t.Fatal("first delta should be emitted")
	}
	if !emitter.EmitTailFromFull("hello") {
		t.Fatal("tail should be emitted")
	}
	if emitter.EmitTailFromFull("hello") {
		t.Fatal("duplicate tail should not be emitted")
	}

	if len(events) != 3 {
		t.Fatalf("events = %#v, want start + 2 deltas", events)
	}
	if events[0].eventType != "response_start" {
		t.Fatalf("event[0] = %s, want response_start", events[0].eventType)
	}
	if events[1].eventType != "response_delta" || events[1].message != "he" {
		t.Fatalf("event[1] = %#v, want first delta", events[1])
	}
	if events[2].eventType != "response_delta" || events[2].message != "llo" {
		t.Fatalf("event[2] = %#v, want tail delta", events[2])
	}
	if got := events[2].data[openai.SSEAccumulatedKey]; got != "hello" {
		t.Fatalf("accumulated = %#v, want hello", got)
	}
	if got := events[0].data["messageGeneratedBy"]; got != "eino:lead" {
		t.Fatalf("messageGeneratedBy = %#v", got)
	}
	if got := events[0].data["iteration"]; got != 3 {
		t.Fatalf("iteration = %#v", got)
	}
}

func TestEinoMainResponseStreamEmitterNoProgress(t *testing.T) {
	emitter := newEinoMainResponseStreamEmitter("conv", "deep", "agent", "stream", 1, nil, nil)
	if emitter.EmitDelta("hello", "hello") {
		t.Fatal("nil progress should not emit")
	}
	if emitter.EmitTailFromFull("hello") {
		t.Fatal("nil progress should not emit tail")
	}
}
