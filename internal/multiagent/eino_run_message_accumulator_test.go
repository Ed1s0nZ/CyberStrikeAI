package multiagent

import (
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

func TestEinoRunMessageAccumulatorTracksBaseAndAppends(t *testing.T) {
	acc := newEinoRunMessageAccumulator([]adk.Message{schema.UserMessage("hi")})

	if acc.BaseCount() != 1 {
		t.Fatalf("base count = %d, want 1", acc.BaseCount())
	}
	if acc.HasNewMessages() {
		t.Fatal("fresh accumulator should not have new messages")
	}

	if !acc.AppendAssistantText("  hello  ") {
		t.Fatal("assistant text should append")
	}
	if !acc.HasNewMessages() {
		t.Fatal("expected new messages after append")
	}
	msgs := acc.Messages()
	if len(msgs) != 2 || msgs[1].Role != schema.Assistant || msgs[1].Content != "hello" {
		t.Fatalf("messages = %#v", msgs)
	}
	newMsgs := acc.NewMessages()
	if len(newMsgs) != 1 || newMsgs[0].Content != "hello" {
		t.Fatalf("new messages = %#v", newMsgs)
	}
}

func TestEinoRunMessageAccumulatorToolMessage(t *testing.T) {
	acc := newEinoRunMessageAccumulator(nil)
	if acc.AppendToolMessage("ignored", "") {
		t.Fatal("blank tool call id should not append")
	}
	if !acc.AppendToolMessage("result", "call-1", schema.WithToolName("execute")) {
		t.Fatal("tool message should append")
	}
	msgs := acc.Messages()
	if len(msgs) != 1 || msgs[0].Role != schema.Tool || msgs[0].Content != "result" || msgs[0].ToolCallID != "call-1" || msgs[0].ToolName != "execute" {
		t.Fatalf("tool message = %#v", msgs)
	}
}

func TestEinoRunMessageAccumulatorAssistantToolCalls(t *testing.T) {
	acc := newEinoRunMessageAccumulator(nil)
	if acc.AppendAssistantToolCalls(nil) {
		t.Fatal("empty tool calls should not append")
	}
	if !acc.AppendAssistantToolCalls([]schema.ToolCall{{
		ID: "call-1",
		Function: schema.FunctionCall{
			Name:      "execute",
			Arguments: `{}`,
		},
	}}) {
		t.Fatal("assistant tool calls should append")
	}
	msgs := acc.Messages()
	if len(msgs) != 1 || msgs[0].Role != schema.Assistant || len(msgs[0].ToolCalls) != 1 {
		t.Fatalf("assistant tool call message = %#v", msgs)
	}
}
