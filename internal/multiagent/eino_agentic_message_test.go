package multiagent

import (
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestEinoMessageToAgenticPreservesAssistantToolCalls(t *testing.T) {
	msg := &schema.Message{
		Role:             schema.Assistant,
		Content:          "I will scan it.",
		ReasoningContent: "Need enumerate first.",
		ToolCalls: []schema.ToolCall{{
			ID:   "call-1",
			Type: "function",
			Function: schema.FunctionCall{
				Name:      "nmap",
				Arguments: `{"target":"127.0.0.1"}`,
			},
		}},
		Extra: map[string]any{"trace": "kept"},
	}

	got := EinoMessageToAgentic(msg)
	if got.Role != schema.AgenticRoleTypeAssistant {
		t.Fatalf("role = %q, want assistant", got.Role)
	}
	if len(got.ContentBlocks) != 3 {
		t.Fatalf("blocks = %d, want 3", len(got.ContentBlocks))
	}
	if got.ContentBlocks[0].Reasoning == nil || got.ContentBlocks[0].Reasoning.Text != msg.ReasoningContent {
		t.Fatalf("reasoning block = %#v", got.ContentBlocks[0])
	}
	if got.ContentBlocks[1].AssistantGenText == nil || got.ContentBlocks[1].AssistantGenText.Text != msg.Content {
		t.Fatalf("assistant text block = %#v", got.ContentBlocks[1])
	}
	call := got.ContentBlocks[2].FunctionToolCall
	if call == nil || call.CallID != "call-1" || call.Name != "nmap" || call.Arguments != `{"target":"127.0.0.1"}` {
		t.Fatalf("tool call block = %#v", got.ContentBlocks[2])
	}
	if got.Extra["trace"] != "kept" {
		t.Fatalf("extra = %#v", got.Extra)
	}
}

func TestEinoMessageToAgenticMapsToolResultAsUserFunctionResult(t *testing.T) {
	msg := &schema.Message{
		Role:       schema.Tool,
		Content:    "22/tcp open ssh",
		ToolCallID: "call-ssh",
		ToolName:   "nmap",
	}

	got := EinoMessageToAgentic(msg)
	if got.Role != schema.AgenticRoleTypeUser {
		t.Fatalf("role = %q, want user", got.Role)
	}
	if len(got.ContentBlocks) != 1 || got.ContentBlocks[0].FunctionToolResult == nil {
		t.Fatalf("blocks = %#v", got.ContentBlocks)
	}
	result := got.ContentBlocks[0].FunctionToolResult
	if result.CallID != "call-ssh" || result.Name != "nmap" {
		t.Fatalf("tool result metadata = %#v", result)
	}
	if len(result.Content) != 1 || result.Content[0].Text == nil || result.Content[0].Text.Text != "22/tcp open ssh" {
		t.Fatalf("tool result content = %#v", result.Content)
	}
}

func TestAgenticMessageToEinoPreservesAssistantBlocks(t *testing.T) {
	msg := &schema.AgenticMessage{
		Role: schema.AgenticRoleTypeAssistant,
		ContentBlocks: []*schema.ContentBlock{
			schema.NewContentBlock(&schema.Reasoning{Text: "Think first."}),
			schema.NewContentBlock(&schema.AssistantGenText{Text: "Calling scanner."}),
			schema.NewContentBlock(&schema.FunctionToolCall{
				CallID:    "call-2",
				Name:      "scan",
				Arguments: `{"host":"example.com"}`,
			}),
		},
	}

	got := AgenticMessageToEino(msg)
	if len(got) != 1 {
		t.Fatalf("messages = %d, want 1", len(got))
	}
	if got[0].Role != schema.Assistant || got[0].Content != "Calling scanner." || got[0].ReasoningContent != "Think first." {
		t.Fatalf("assistant message = %#v", got[0])
	}
	if len(got[0].ToolCalls) != 1 || got[0].ToolCalls[0].ID != "call-2" || got[0].ToolCalls[0].Function.Name != "scan" {
		t.Fatalf("tool calls = %#v", got[0].ToolCalls)
	}
}

func TestAgenticMessageToEinoSplitsPureToolResult(t *testing.T) {
	msg := &schema.AgenticMessage{
		Role: schema.AgenticRoleTypeUser,
		ContentBlocks: []*schema.ContentBlock{
			schema.NewContentBlock(&schema.FunctionToolResult{
				CallID: "call-3",
				Name:   "execute",
				Content: []*schema.FunctionToolResultContentBlock{{
					Type: schema.FunctionToolResultContentBlockTypeText,
					Text: &schema.UserInputText{Text: "done"},
				}},
			}),
		},
	}

	got := AgenticMessageToEino(msg)
	if len(got) != 1 {
		t.Fatalf("messages = %d, want 1", len(got))
	}
	if got[0].Role != schema.Tool || got[0].ToolCallID != "call-3" || got[0].ToolName != "execute" || got[0].Content != "done" {
		t.Fatalf("tool message = %#v", got[0])
	}
}

func TestEinoAgenticRoundTripForSupportedFields(t *testing.T) {
	msgs := []*schema.Message{
		schema.SystemMessage("system"),
		schema.UserMessage("user"),
		{
			Role:    schema.Assistant,
			Content: "assistant",
			ToolCalls: []schema.ToolCall{{
				ID:       "call-4",
				Type:     "function",
				Function: schema.FunctionCall{Name: "grep", Arguments: `{"q":"token"}`},
			}},
		},
		{
			Role:       schema.Tool,
			Content:    "match",
			ToolCallID: "call-4",
			ToolName:   "grep",
		},
	}

	got := AgenticMessagesToEino(EinoMessagesToAgentic(msgs))
	if len(got) != len(msgs) {
		t.Fatalf("round trip messages = %d, want %d: %#v", len(got), len(msgs), got)
	}
	for i := range msgs {
		if got[i].Role != msgs[i].Role || got[i].Content != msgs[i].Content || got[i].ToolCallID != msgs[i].ToolCallID || got[i].ToolName != msgs[i].ToolName {
			t.Fatalf("message[%d] = %#v, want %#v", i, got[i], msgs[i])
		}
		if len(got[i].ToolCalls) != len(msgs[i].ToolCalls) {
			t.Fatalf("message[%d] tool calls = %#v, want %#v", i, got[i].ToolCalls, msgs[i].ToolCalls)
		}
	}
}
