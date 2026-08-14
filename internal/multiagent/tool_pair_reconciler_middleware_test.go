package multiagent

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

func runToolPairReconciler(t *testing.T, msgs []adk.Message) []adk.Message {
	t.Helper()
	mw := newToolPairReconcilerMiddleware(nil, "test").(*toolPairReconcilerMiddleware)
	_, out, err := mw.BeforeModelRewriteState(
		context.Background(),
		&adk.ChatModelAgentState{Messages: msgs},
		&adk.ModelContext{},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return out.Messages
}

func assertCompleteImmediateToolPairs(t *testing.T, msgs []adk.Message) {
	t.Helper()
	for i, msg := range msgs {
		if msg == nil || msg.Role != schema.Assistant || len(msg.ToolCalls) == 0 {
			continue
		}
		want := make(map[string]struct{}, len(msg.ToolCalls))
		for _, tc := range msg.ToolCalls {
			if tc.ID == "" {
				t.Fatal("empty tool call id remained")
			}
			if _, duplicate := want[tc.ID]; duplicate {
				t.Fatalf("duplicate tool call id remained: %s", tc.ID)
			}
			want[tc.ID] = struct{}{}
		}
		seen := make(map[string]struct{}, len(want))
		for j := i + 1; j < len(msgs) && msgs[j] != nil && msgs[j].Role == schema.Tool; j++ {
			id := msgs[j].ToolCallID
			if _, ok := want[id]; !ok {
				t.Fatalf("unexpected tool result %q after assistant %d", id, i)
			}
			if _, duplicate := seen[id]; duplicate {
				t.Fatalf("duplicate tool result %q", id)
			}
			seen[id] = struct{}{}
		}
		if len(seen) != len(want) {
			t.Fatalf("assistant %d: want %d results, got %d", i, len(want), len(seen))
		}
	}
}

func TestToolPairReconcilerPatchesPartialMultiToolBatch(t *testing.T) {
	msgs := []adk.Message{
		schema.UserMessage("start"),
		assistantToolCallsMsg("", "c1", "c2"),
		schema.ToolMessage("r1", "c1"),
		schema.UserMessage("continue"),
	}
	out := runToolPairReconciler(t, msgs)
	assertCompleteImmediateToolPairs(t, out)
	if len(out) != 5 || out[3].Role != schema.Tool || out[3].ToolCallID != "c2" {
		t.Fatalf("missing c2 result was not inserted in place: %+v", out)
	}
	if out[3].Content != patchedMissingToolResult {
		t.Fatalf("unexpected patched content: %q", out[3].Content)
	}
}

func TestToolPairReconcilerDropsMisplacedDuplicateAndOrphanResults(t *testing.T) {
	msgs := []adk.Message{
		schema.ToolMessage("old", "orphan"),
		assistantToolCallsMsg("", "c1"),
		schema.ToolMessage("first", "c1"),
		schema.ToolMessage("duplicate", "c1"),
		schema.ToolMessage("wrong", "other"),
		schema.UserMessage("next"),
		schema.ToolMessage("late", "c1"),
	}
	out := runToolPairReconciler(t, msgs)
	assertCompleteImmediateToolPairs(t, out)
	toolCount := 0
	for _, msg := range out {
		if msg.Role == schema.Tool {
			toolCount++
			if msg.Content != "first" || msg.ToolCallID != "c1" {
				t.Fatalf("unexpected retained tool result: %+v", msg)
			}
		}
	}
	if toolCount != 1 {
		t.Fatalf("want one retained tool result, got %d", toolCount)
	}
}

func TestToolPairReconcilerRepairsEmptyAndRepeatedCallIDs(t *testing.T) {
	msgs := []adk.Message{
		assistantToolCallsMsg("", "", "same"),
		schema.ToolMessage("same-1", "same"),
		assistantToolCallsMsg("", "same"),
		schema.ToolMessage("same-2", "same"),
	}
	out := runToolPairReconciler(t, msgs)
	assertCompleteImmediateToolPairs(t, out)
	all := make(map[string]struct{})
	for _, msg := range out {
		if msg.Role != schema.Assistant {
			continue
		}
		for _, tc := range msg.ToolCalls {
			if _, duplicate := all[tc.ID]; duplicate {
				t.Fatalf("global duplicate tool call id remained: %s", tc.ID)
			}
			all[tc.ID] = struct{}{}
		}
	}
}

func TestToolPairReconcilerNoOpForValidHistory(t *testing.T) {
	msgs := []adk.Message{
		schema.UserMessage("start"),
		assistantToolCallsMsg("", "c1", "c2"),
		schema.ToolMessage("r1", "c1"),
		schema.ToolMessage("r2", "c2"),
		schema.AssistantMessage("done", nil),
	}
	mw := newToolPairReconcilerMiddleware(nil, "test").(*toolPairReconcilerMiddleware)
	in := &adk.ChatModelAgentState{Messages: msgs}
	_, out, err := mw.BeforeModelRewriteState(context.Background(), in, &adk.ModelContext{})
	if err != nil {
		t.Fatal(err)
	}
	if out != in {
		t.Fatal("valid history should use the no-op fast path")
	}
}
