package handler

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestConversationHandlerDeleteConversationCancelsRunningTask(t *testing.T) {
	tm := NewAgentTaskManager()
	ctx, cancel := context.WithCancelCause(context.Background())
	_, err := tm.StartTask("conv-1", "hello", cancel)
	if err != nil {
		t.Fatalf("StartTask: %v", err)
	}

	h := &AgentHandler{tasks: tm, logger: zap.NewNop()}
	h.CancelRunningTaskForConversation("conv-1")

	select {
	case <-ctx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("task context was not cancelled")
	}
	if cause := context.Cause(ctx); cause != ErrTaskCancelled {
		t.Fatalf("expected ErrTaskCancelled, got %v", cause)
	}
}
