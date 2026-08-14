package multiagent

import (
	"testing"

	"cyberstrike-ai/internal/openai"
)

func TestEinoReasoningStreamEmitterStreamingLifecycle(t *testing.T) {
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
	emitter := newEinoReasoningStreamEmitter("conv-1", "deep", "lead", "orchestrator", progress, func() string {
		return "reasoning-1"
	})

	if !emitter.EmitDelta("he") {
		t.Fatal("first reasoning delta should emit")
	}
	if !emitter.EmitDelta("hello") {
		t.Fatal("cumulative reasoning chunk should emit tail")
	}
	if got := emitter.Finish(); got != "hello" {
		t.Fatalf("finish body = %q, want hello", got)
	}

	if len(events) != 4 {
		t.Fatalf("events = %#v, want start + 2 deltas + end", events)
	}
	if events[0].eventType != "reasoning_chain_stream_start" || events[0].message != " " {
		t.Fatalf("event[0] = %#v", events[0])
	}
	if events[1].eventType != "reasoning_chain_stream_delta" || events[1].message != "he" {
		t.Fatalf("event[1] = %#v", events[1])
	}
	if events[2].eventType != "reasoning_chain_stream_delta" || events[2].message != "llo" {
		t.Fatalf("event[2] = %#v", events[2])
	}
	if got := events[2].data[openai.SSEAccumulatedKey]; got != "hello" {
		t.Fatalf("accumulated = %#v, want hello", got)
	}
	if events[3].eventType != "reasoning_chain_stream_end" || events[3].message != "hello" {
		t.Fatalf("event[3] = %#v", events[3])
	}
	if got := events[3].data["einoRole"]; got != "orchestrator" {
		t.Fatalf("einoRole = %#v", got)
	}
}

func TestEinoReasoningStreamEmitterComplete(t *testing.T) {
	var eventType, message string
	var data map[string]interface{}
	progress := func(et, msg string, raw interface{}) {
		eventType = et
		message = msg
		data, _ = raw.(map[string]interface{})
	}

	ok := newEinoReasoningStreamEmitter("conv-1", "supervisor", "worker", "sub", progress, nil).EmitComplete(" thought ")
	if !ok {
		t.Fatal("complete reasoning should emit")
	}
	if eventType != "reasoning_chain" || message != "thought" {
		t.Fatalf("event = %s %q", eventType, message)
	}
	if data["conversationId"] != "conv-1" || data["einoAgent"] != "worker" || data["einoRole"] != "sub" {
		t.Fatalf("bad event data: %#v", data)
	}
}

func TestEinoReasoningStreamEmitterNoProgressStillBuffers(t *testing.T) {
	emitter := newEinoReasoningStreamEmitter("conv", "deep", "lead", "orchestrator", nil, nil)
	if emitter.EmitDelta("hello") {
		t.Fatal("nil progress should not emit")
	}
	if got := emitter.Finish(); got != "hello" {
		t.Fatalf("finish body = %q, want hello", got)
	}
}
