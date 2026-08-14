package multiagent

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

func assistantToolCallsMsg(content string, callIDs ...string) *schema.Message {
	tcs := make([]schema.ToolCall, 0, len(callIDs))
	for _, id := range callIDs {
		tcs = append(tcs, schema.ToolCall{
			ID:   id,
			Type: "function",
			Function: schema.FunctionCall{
				Name:      "stub_tool",
				Arguments: `{}`,
			},
		})
	}
	return schema.AssistantMessage(content, tcs)
}

func TestOrphanToolPruner_NoOpWhenPaired(t *testing.T) {
	mw := newOrphanToolPrunerMiddleware(nil, "test").(*orphanToolPrunerMiddleware)

	msgs := []adk.Message{
		schema.SystemMessage("sys"),
		schema.UserMessage("hi"),
		assistantToolCallsMsg("", "c1", "c2"),
		schema.ToolMessage("r1", "c1"),
		schema.ToolMessage("r2", "c2"),
		schema.AssistantMessage("done", nil),
	}
	in := &adk.ChatModelAgentState{Messages: msgs}

	_, out, err := mw.BeforeModelRewriteState(context.Background(), in, &adk.ModelContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out == nil {
		t.Fatal("expected non-nil state")
	}
	if len(out.Messages) != len(msgs) {
		t.Fatalf("expected %d messages kept, got %d", len(msgs), len(out.Messages))
	}
	// 快路径：未发现孤儿时必须原地返回 state，不分配新切片。
	if &out.Messages[0] != &msgs[0] {
		t.Fatalf("expected state to be returned as-is (same backing slice) when no orphan present")
	}
}

func TestOrphanToolPruner_DropsOrphanToolMessages(t *testing.T) {
	mw := newOrphanToolPrunerMiddleware(nil, "test").(*orphanToolPrunerMiddleware)

	msgs := []adk.Message{
		schema.SystemMessage("sys"),
		// 摘要前的 assistant(tc: c_old) 已被裁剪，但对应的 tool 结果漏保留了。
		schema.ToolMessage("orphan result", "c_old"),
		schema.UserMessage("continue"),
		assistantToolCallsMsg("", "c_new"),
		schema.ToolMessage("r_new", "c_new"),
	}
	in := &adk.ChatModelAgentState{Messages: msgs}

	_, out, err := mw.BeforeModelRewriteState(context.Background(), in, &adk.ModelContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out == nil {
		t.Fatal("expected non-nil state")
	}
	if len(out.Messages) != len(msgs)-1 {
		t.Fatalf("expected %d messages after pruning, got %d", len(msgs)-1, len(out.Messages))
	}
	for _, m := range out.Messages {
		if m != nil && m.Role == schema.Tool && m.ToolCallID == "c_old" {
			t.Fatalf("orphan tool message with ToolCallID=c_old should have been dropped")
		}
	}
	// 合法的 tool(c_new) 必须保留。
	foundNew := false
	for _, m := range out.Messages {
		if m != nil && m.Role == schema.Tool && m.ToolCallID == "c_new" {
			foundNew = true
			break
		}
	}
	if !foundNew {
		t.Fatal("paired tool message (c_new) must be retained")
	}
}

func TestOrphanToolPruner_EmptyToolCallIDIsIgnored(t *testing.T) {
	// 空 ToolCallID 的 tool 消息在真实场景中极罕见，但不应当被误判为孤儿。
	// 语义上把它当作"无法校验，保留"，避免误删。
	mw := newOrphanToolPrunerMiddleware(nil, "test").(*orphanToolPrunerMiddleware)

	odd := schema.ToolMessage("no_id", "")
	msgs := []adk.Message{
		schema.UserMessage("hi"),
		odd,
		schema.AssistantMessage("ok", nil),
	}
	in := &adk.ChatModelAgentState{Messages: msgs}

	_, out, err := mw.BeforeModelRewriteState(context.Background(), in, &adk.ModelContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Messages) != len(msgs) {
		t.Fatalf("empty ToolCallID tool message should be kept, got %d messages", len(out.Messages))
	}
}

func TestOrphanToolPruner_NilAndEmpty(t *testing.T) {
	mw := newOrphanToolPrunerMiddleware(nil, "test").(*orphanToolPrunerMiddleware)

	ctx := context.Background()
	// nil state
	if _, out, err := mw.BeforeModelRewriteState(ctx, nil, &adk.ModelContext{}); err != nil || out != nil {
		t.Fatalf("nil state: expected (nil,nil), got (%v,%v)", out, err)
	}
	// empty messages
	empty := &adk.ChatModelAgentState{}
	if _, out, err := mw.BeforeModelRewriteState(ctx, empty, &adk.ModelContext{}); err != nil || out != empty {
		t.Fatalf("empty messages: expected same state, got (%v,%v)", out, err)
	}
}
