package multiagent

import (
	"testing"

	"cyberstrike-ai/internal/openai"
)

func TestEinoSubAgentReplyEmitterStreamingLifecycle(t *testing.T) {
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
	emitter := newEinoSubAgentReplyEmitter("conv-1", "worker", progress, func() string { return "stream-1" })

	if !emitter.EmitDelta("he") {
		t.Fatal("first delta should emit")
	}
	if !emitter.EmitDelta("hello") {
		t.Fatal("cumulative chunk should emit tail")
	}
	if got := emitter.Finish(); got != "hello" {
		t.Fatalf("finish body = %q, want hello", got)
	}

	if len(events) != 4 {
		t.Fatalf("events = %#v, want start + 2 deltas + end", events)
	}
	if events[0].eventType != "eino_agent_reply_stream_start" {
		t.Fatalf("event[0] = %s", events[0].eventType)
	}
	if events[1].eventType != "eino_agent_reply_stream_delta" || events[1].message != "he" {
		t.Fatalf("event[1] = %#v", events[1])
	}
	if events[2].eventType != "eino_agent_reply_stream_delta" || events[2].message != "llo" {
		t.Fatalf("event[2] = %#v", events[2])
	}
	if got := events[2].data[openai.SSEAccumulatedKey]; got != "hello" {
		t.Fatalf("accumulated = %#v, want hello", got)
	}
	if events[3].eventType != "eino_agent_reply_stream_end" || events[3].message != "hello" {
		t.Fatalf("event[3] = %#v", events[3])
	}
	if got := events[0].data["einoAgent"]; got != "worker" {
		t.Fatalf("einoAgent = %#v", got)
	}
}

func TestEinoSubAgentReplyEmitterComplete(t *testing.T) {
	var eventType, message string
	var data map[string]interface{}
	progress := func(et, msg string, raw interface{}) {
		eventType = et
		message = msg
		data, _ = raw.(map[string]interface{})
	}

	ok := newEinoSubAgentReplyEmitter("conv-1", "worker", progress, nil).EmitComplete(" done ")
	if !ok {
		t.Fatal("complete reply should emit")
	}
	if eventType != "eino_agent_reply" || message != "done" {
		t.Fatalf("event = %s %q", eventType, message)
	}
	if data["conversationId"] != "conv-1" || data["einoAgent"] != "worker" || data["einoRole"] != "sub" {
		t.Fatalf("bad event data: %#v", data)
	}
}

func TestEinoSubAgentReplyEmitterNoProgressStillBuffers(t *testing.T) {
	emitter := newEinoSubAgentReplyEmitter("conv", "worker", nil, nil)
	if emitter.EmitDelta("hello") {
		t.Fatal("nil progress should not emit")
	}
	if got := emitter.Finish(); got != "hello" {
		t.Fatalf("finish body = %q, want hello", got)
	}
}
