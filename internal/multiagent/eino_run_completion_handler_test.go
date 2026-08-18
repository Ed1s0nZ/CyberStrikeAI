package multiagent

import (
	"context"
	"os"
	"testing"
)

func TestEinoRunCompletionHandlerFlushesOrphansAndCleansCheckpoint(t *testing.T) {
	var events []struct {
		eventType string
		data      map[string]interface{}
	}
	progress := func(eventType, _ string, data interface{}) {
		m, _ := data.(map[string]interface{})
		events = append(events, struct {
			eventType string
			data      map[string]interface{}
		}{eventType: eventType, data: m})
	}
	pending := newEinoPendingToolCalls("conv-1", progress)
	pending.Mark(toolCallPendingInfo{ToolCallID: "call-1", ToolName: "execute", EinoAgent: "lead", EinoRole: "orchestrator"})
	store, err := newFileCheckPointStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Set(context.Background(), "cp-1", []byte("checkpoint")); err != nil {
		t.Fatal(err)
	}
	cpPath, err := store.path("cp-1")
	if err != nil {
		t.Fatal(err)
	}

	newEinoRunCompletionHandler(einoRunCompletionHandlerConfig{
		ConversationID: "conv-1",
		OrchMode:       "deep",
		Progress:       progress,
		Pending:        pending,
		Checkpoint:     store,
		CheckpointID:   "cp-1",
	}).Complete()

	if pending.Count() != 0 {
		t.Fatalf("pending count = %d, want 0", pending.Count())
	}
	if _, err := os.Stat(cpPath); !os.IsNotExist(err) {
		t.Fatalf("checkpoint should be removed, stat err=%v", err)
	}
	var orphanEvent map[string]interface{}
	var failedToolResult map[string]interface{}
	for _, ev := range events {
		switch ev.eventType {
		case "eino_pending_orphaned":
			orphanEvent = ev.data
		case "tool_result":
			failedToolResult = ev.data
		}
	}
	if orphanEvent == nil || orphanEvent["conversationId"] != "conv-1" || orphanEvent["orchestration"] != "deep" || orphanEvent["pendingCount"] != 1 {
		t.Fatalf("orphan event = %#v", orphanEvent)
	}
	if failedToolResult == nil || failedToolResult["toolCallId"] != "call-1" || failedToolResult["isError"] != true {
		t.Fatalf("failed tool result = %#v", failedToolResult)
	}
}

func TestEinoRunCompletionHandlerNoopWithoutState(t *testing.T) {
	newEinoRunCompletionHandler(einoRunCompletionHandlerConfig{}).Complete()
	var h *einoRunCompletionHandler
	h.Complete()
}
