package multiagent

import (
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestEinoExtractFallbackAssistantFromMsgs_exitToolMessage(t *testing.T) {
	u := schema.UserMessage("hi")
	tm := schema.ToolMessage("answer for user", "call-exit-1")
	tm.ToolName = "exit"
	if got := einoExtractFallbackAssistantFromMsgs([]*schema.Message{u, tm}); got != "answer for user" {
		t.Fatalf("got %q", got)
	}
}

func TestEinoExtractFallbackAssistantFromMsgs_lastExitWins(t *testing.T) {
	msgs := []*schema.Message{
		schema.UserMessage("hi"),
		toolExitMsg("first", "c1"),
		toolExitMsg("second", "c2"),
	}
	if got := einoExtractFallbackAssistantFromMsgs(msgs); got != "second" {
		t.Fatalf("got %q", got)
	}
}

func TestEinoExtractFallbackAssistantFromMsgs_fromAssistantToolCalls(t *testing.T) {
	m := schema.AssistantMessage("", []schema.ToolCall{{
		ID:   "x",
		Type: "function",
		Function: schema.FunctionCall{
			Name:      "exit",
			Arguments: `{"final_result":"from args"}`,
		},
	}})
	if got := einoExtractFallbackAssistantFromMsgs([]*schema.Message{m}); got != "from args" {
		t.Fatalf("got %q", got)
	}
}

func TestEinoExtractFallbackAssistantFromMsgs_prefersToolOverEarlierAssistant(t *testing.T) {
	asst := schema.AssistantMessage("", []schema.ToolCall{{
		ID:   "x",
		Type: "function",
		Function: schema.FunctionCall{
			Name:      "exit",
			Arguments: `{"final_result":"from args"}`,
		},
	}})
	tool := toolExitMsg("from tool", "c1")
	if got := einoExtractFallbackAssistantFromMsgs([]*schema.Message{asst, tool}); got != "from tool" {
		t.Fatalf("got %q", got)
	}
}

func toolExitMsg(content, callID string) *schema.Message {
	m := schema.ToolMessage(content, callID)
	m.ToolName = "exit"
	return m
}
