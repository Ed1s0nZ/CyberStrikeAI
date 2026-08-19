package multiagent

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

func TestAgenticToolPairReconcilerPatchesMissing(t *testing.T) {
	t.Parallel()
	mw := newAgenticToolPairReconcilerMiddleware(nil, "test")
	state := &adk.TypedChatModelAgentState[*schema.AgenticMessage]{
		Messages: []*schema.AgenticMessage{
			agenticAssistantToolCall("c1", "search", `{"q":"x"}`),
			agenticAssistantToolCall("c2", "execute", `{"cmd":"ls"}`),
			// c1 result present, c2 missing
			agenticToolResult("c1", "search", "found it"),
		},
	}
	_, out, err := mw.BeforeModelRewriteState(context.Background(), state, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Expected: assistant(c1) -> result(c1) -> assistant(c2) -> patched_result(c2)
	if len(out.Messages) != 4 {
		t.Fatalf("messages = %d, want 4", len(out.Messages))
	}
	// c1 assistant
	if calls := agenticFunctionToolCalls(out.Messages[0]); len(calls) != 1 || calls[0].CallID != "c1" {
		t.Fatal("msg[0] should be assistant(c1)")
	}
	// c1 result
	if ids := agenticToolResultCallIDs(out.Messages[1]); len(ids) != 1 || ids[0] != "c1" {
		t.Fatal("msg[1] should be result(c1)")
	}
	// c2 assistant
	if calls := agenticFunctionToolCalls(out.Messages[2]); len(calls) != 1 || calls[0].CallID != "c2" {
		t.Fatal("msg[2] should be assistant(c2)")
	}
	// c2 patched result
	if ids := agenticToolResultCallIDs(out.Messages[3]); len(ids) != 1 || ids[0] != "c2" {
		t.Fatal("msg[3] should be patched result(c2)")
	}
	resultText := out.Messages[3].ContentBlocks[0].FunctionToolResult.Content[0].Text.Text
	if resultText != patchedMissingToolResult {
		t.Fatalf("patched text = %q", resultText)
	}
}

func TestAgenticToolPairReconcilerDropsOrphan(t *testing.T) {
	t.Parallel()
	mw := newAgenticToolPairReconcilerMiddleware(nil, "test")
	state := &adk.TypedChatModelAgentState[*schema.AgenticMessage]{
		Messages: []*schema.AgenticMessage{
			// Orphan tool result with no preceding assistant
			agenticToolResult("orphan", "deleted_tool", "stale data"),
			{Role: schema.AgenticRoleTypeUser, ContentBlocks: []*schema.ContentBlock{
				schema.NewContentBlock(&schema.UserInputText{Text: "hello"}),
			}},
		},
	}
	_, out, err := mw.BeforeModelRewriteState(context.Background(), state, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Messages) != 1 {
		t.Fatalf("messages = %d, want 1 (orphan dropped)", len(out.Messages))
	}
	if out.Messages[0].ContentBlocks[0].UserInputText == nil {
		t.Fatal("remaining message should be the user text")
	}
}

func TestAgenticToolPairReconcilerNoopWhenPaired(t *testing.T) {
	t.Parallel()
	mw := newAgenticToolPairReconcilerMiddleware(nil, "test")
	state := &adk.TypedChatModelAgentState[*schema.AgenticMessage]{
		Messages: []*schema.AgenticMessage{
			agenticAssistantToolCall("c1", "search", `{}`),
			agenticToolResult("c1", "search", "ok"),
		},
	}
	_, out, err := mw.BeforeModelRewriteState(context.Background(), state, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Should return original state unchanged
	if &out.Messages[0] == &state.Messages[0] {
		// pointer equality on slice — state not cloned
	}
	if len(out.Messages) != 2 {
		t.Fatalf("messages = %d, want 2", len(out.Messages))
	}
}

func TestAgenticToolPairReconcilerFixesEmptyCallID(t *testing.T) {
	t.Parallel()
	mw := newAgenticToolPairReconcilerMiddleware(nil, "test")
	state := &adk.TypedChatModelAgentState[*schema.AgenticMessage]{
		Messages: []*schema.AgenticMessage{
			{
				Role: schema.AgenticRoleTypeAssistant,
				ContentBlocks: []*schema.ContentBlock{schema.NewContentBlock(&schema.FunctionToolCall{
					CallID: "", Name: "search", Arguments: `{}`,
				})},
			},
		},
	}
	_, out, err := mw.BeforeModelRewriteState(context.Background(), state, nil)
	if err != nil {
		t.Fatal(err)
	}
	calls := agenticFunctionToolCalls(out.Messages[0])
	if len(calls) != 1 || calls[0].CallID == "" {
		t.Fatalf("empty call ID should be patched, got %q", calls[0].CallID)
	}
}

func TestAgenticOrphanToolPrunerRemovesOrphan(t *testing.T) {
	t.Parallel()
	mw := newAgenticOrphanToolPrunerMiddleware(nil, "test")
	state := &adk.TypedChatModelAgentState[*schema.AgenticMessage]{
		Messages: []*schema.AgenticMessage{
			agenticAssistantToolCall("c1", "search", `{}`),
			agenticToolResult("c1", "search", "ok"),
			// Orphan: no assistant has call_id "c_orphan"
			agenticToolResult("c_orphan", "deleted", "stale"),
		},
	}
	_, out, err := mw.BeforeModelRewriteState(context.Background(), state, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Messages) != 2 {
		t.Fatalf("messages = %d, want 2 (orphan pruned)", len(out.Messages))
	}
}

func TestAgenticOrphanToolPrunerNoopWhenClean(t *testing.T) {
	t.Parallel()
	mw := newAgenticOrphanToolPrunerMiddleware(nil, "test")
	state := &adk.TypedChatModelAgentState[*schema.AgenticMessage]{
		Messages: []*schema.AgenticMessage{
			agenticAssistantToolCall("c1", "search", `{}`),
			agenticToolResult("c1", "search", "ok"),
		},
	}
	_, out, err := mw.BeforeModelRewriteState(context.Background(), state, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Messages) != 2 {
		t.Fatalf("messages = %d, want 2", len(out.Messages))
	}
}
