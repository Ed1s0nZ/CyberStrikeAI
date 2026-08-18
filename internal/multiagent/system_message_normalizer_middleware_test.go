package multiagent

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

func TestStripADKSystemMessages(t *testing.T) {
	in := []adk.Message{
		schema.SystemMessage("a"),
		schema.UserMessage("u"),
		schema.SystemMessage("b"),
		schema.AssistantMessage("x", nil),
	}
	out := stripADKSystemMessages(in)
	if len(out) != 2 {
		t.Fatalf("got %d messages, want 2", len(out))
	}
	if out[0].Role != schema.User || out[1].Role != schema.Assistant {
		t.Fatalf("unexpected roles: %s, %s", out[0].Role, out[1].Role)
	}
}

func TestEinoMessagesForRunRestart_StripsSystemFromTrace(t *testing.T) {
	holder := newModelFacingTraceHolder()
	holder.storeFromState(&adk.ChatModelAgentState{Messages: []adk.Message{
		schema.SystemMessage("sys-1"),
		schema.SystemMessage("sys-2"),
		schema.UserMessage("task"),
	}})
	msgs, src := einoMessagesForRunRestart(&einoADKRunLoopArgs{ModelFacingTrace: holder}, nil, nil, 0)
	if src != einoRestartContextModelTrace {
		t.Fatalf("source: got %q want model_trace", src)
	}
	if len(msgs) != 1 || msgs[0].Role != schema.User {
		t.Fatalf("expected user-only restart msgs, got %+v", msgs)
	}
}

func TestSystemMessageNormalizerMiddleware_MergesDuplicates(t *testing.T) {
	mw := newSystemMessageNormalizerMiddleware(nil, "test")
	state := &adk.ChatModelAgentState{Messages: []adk.Message{
		schema.SystemMessage("a"),
		schema.SystemMessage("b"),
		schema.UserMessage("u"),
	}}
	_, out, err := mw.(*systemMessageNormalizerMiddleware).BeforeModelRewriteState(context.Background(), state, nil)
	if err != nil {
		t.Fatal(err)
	}
	if countADKSystemMessages(out.Messages) != 1 {
		t.Fatalf("want 1 system, got %d", countADKSystemMessages(out.Messages))
	}
	if out.Messages[0].Content != "a\n\nb" {
		t.Fatalf("merged content: %q", out.Messages[0].Content)
	}
}

func TestSystemMessageNormalizerMiddleware_NoOpSingleSystem(t *testing.T) {
	mw := newSystemMessageNormalizerMiddleware(nil, "test")
	state := &adk.ChatModelAgentState{Messages: []adk.Message{
		schema.SystemMessage("only"),
		schema.UserMessage("u"),
	}}
	_, out, err := mw.(*systemMessageNormalizerMiddleware).BeforeModelRewriteState(context.Background(), state, nil)
	if err != nil {
		t.Fatal(err)
	}
	if out != state {
		t.Fatalf("expected same state pointer for no-op")
	}
}
