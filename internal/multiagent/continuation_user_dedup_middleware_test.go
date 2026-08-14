package multiagent

import (
	"context"
	"strings"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

func continuationUser(text string) adk.Message {
	return &schema.Message{
		Role: schema.User,
		UserInputMultiContent: []schema.MessageInputPart{
			{Type: schema.ChatMessagePartTypeText, Text: continuationSessionMarker + "\n" + text},
			{Type: schema.ChatMessagePartTypeText, Text: "Please continue the conversation from where we left it off."},
		},
	}
}

func TestDedupContinuationUserMessages_KeepsLatest(t *testing.T) {
	msgs := []adk.Message{
		continuationUser("summary old"),
		schema.UserMessage("real task"),
		continuationUser("summary new"),
	}
	out, dropped := dedupContinuationUserMessages(msgs)
	if dropped != 1 {
		t.Fatalf("dropped=%d want 1", dropped)
	}
	if len(out) != 2 {
		t.Fatalf("len=%d want 2", len(out))
	}
	if out[0].Role != schema.User || adkUserMessageText(out[0]) != "real task" {
		t.Fatalf("first should remain real task, got %q", adkUserMessageText(out[0]))
	}
	if !strings.Contains(adkUserMessageText(out[1]), "summary new") {
		t.Fatalf("latest continuation not kept: %q", adkUserMessageText(out[1]))
	}
}

func TestDedupContinuationUserMessages_NoOpSingle(t *testing.T) {
	msgs := []adk.Message{continuationUser("only"), schema.UserMessage("task")}
	out, dropped := dedupContinuationUserMessages(msgs)
	if dropped != 0 || len(out) != 2 {
		t.Fatalf("unexpected change dropped=%d len=%d", dropped, len(out))
	}
}

func TestContinuationUserDedupMiddleware(t *testing.T) {
	mw := newContinuationUserDedupMiddleware(nil, "test")
	state := &adk.ChatModelAgentState{Messages: []adk.Message{
		continuationUser("old"),
		continuationUser("new"),
		schema.UserMessage("task"),
	}}
	_, out, err := mw.(*continuationUserDedupMiddleware).BeforeModelRewriteState(context.Background(), state, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Messages) != 2 {
		t.Fatalf("want 2 messages after dedup, got %d", len(out.Messages))
	}
}
