package multiagent

import (
	"errors"
	"io"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

func TestAdaptAgenticEventToEinoEventsAssistantMessage(t *testing.T) {
	usage := &schema.TokenUsage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15}
	ev := &adk.TypedAgentEvent[*schema.AgenticMessage]{
		AgentName: "agentic",
		Output: &adk.TypedAgentOutput[*schema.AgenticMessage]{
			MessageOutput: &adk.TypedMessageVariant[*schema.AgenticMessage]{
				Message: &schema.AgenticMessage{
					Role:         schema.AgenticRoleTypeAssistant,
					ResponseMeta: &schema.AgenticResponseMeta{TokenUsage: usage},
					ContentBlocks: []*schema.ContentBlock{
						schema.NewContentBlock(&schema.Reasoning{Text: "think"}),
						schema.NewContentBlock(&schema.AssistantGenText{Text: "calling"}),
						schema.NewContentBlock(&schema.FunctionToolCall{CallID: "call-1", Name: "scan", Arguments: `{"host":"127.0.0.1"}`}),
					},
				},
			},
			CustomizedOutput: "custom",
		},
	}

	got := adaptAgenticEventToEinoEvents(ev)
	if len(got) != 1 {
		t.Fatalf("events = %d, want 1", len(got))
	}
	mv := got[0].Output.MessageOutput
	if got[0].AgentName != "agentic" || got[0].Output.CustomizedOutput != "custom" {
		t.Fatalf("event metadata = %#v", got[0])
	}
	if mv.Role != schema.Assistant || mv.Message.Role != schema.Assistant {
		t.Fatalf("role = %q/%q, want assistant", mv.Role, mv.Message.Role)
	}
	if mv.Message.Content != "calling" || mv.Message.ReasoningContent != "think" {
		t.Fatalf("message text = %#v", mv.Message)
	}
	if len(mv.Message.ToolCalls) != 1 || mv.Message.ToolCalls[0].ID != "call-1" || mv.Message.ToolCalls[0].Function.Name != "scan" {
		t.Fatalf("tool calls = %#v", mv.Message.ToolCalls)
	}
	if mv.Message.ResponseMeta == nil || mv.Message.ResponseMeta.Usage != usage {
		t.Fatalf("usage = %#v, want original usage", mv.Message.ResponseMeta)
	}
}

func TestAdaptAgenticEventToEinoEventsPureToolResult(t *testing.T) {
	ev := &adk.TypedAgentEvent[*schema.AgenticMessage]{
		Output: &adk.TypedAgentOutput[*schema.AgenticMessage]{
			MessageOutput: &adk.TypedMessageVariant[*schema.AgenticMessage]{
				Message: &schema.AgenticMessage{
					Role: schema.AgenticRoleTypeUser,
					ContentBlocks: []*schema.ContentBlock{
						schema.NewContentBlock(&schema.FunctionToolResult{
							CallID: "call-2",
							Name:   "execute",
							Content: []*schema.FunctionToolResultContentBlock{{
								Type: schema.FunctionToolResultContentBlockTypeText,
								Text: &schema.UserInputText{Text: "done"},
							}},
						}),
					},
				},
			},
		},
	}

	got := adaptAgenticEventToEinoEvents(ev)
	if len(got) != 1 {
		t.Fatalf("events = %d, want 1", len(got))
	}
	msg := got[0].Output.MessageOutput.Message
	if got[0].Output.MessageOutput.Role != schema.Tool || msg.Role != schema.Tool || msg.ToolName != "execute" || msg.ToolCallID != "call-2" || msg.Content != "done" {
		t.Fatalf("tool event = %#v message=%#v", got[0].Output.MessageOutput, msg)
	}
}

func TestAdaptAgenticEventToEinoEventsSplitsMixedToolResult(t *testing.T) {
	ev := &adk.TypedAgentEvent[*schema.AgenticMessage]{
		Output: &adk.TypedAgentOutput[*schema.AgenticMessage]{
			MessageOutput: &adk.TypedMessageVariant[*schema.AgenticMessage]{
				Message: &schema.AgenticMessage{
					Role: schema.AgenticRoleTypeAssistant,
					ContentBlocks: []*schema.ContentBlock{
						schema.NewContentBlock(&schema.AssistantGenText{Text: "text"}),
						schema.NewContentBlock(&schema.FunctionToolResult{
							CallID: "call-3",
							Name:   "grep",
							Content: []*schema.FunctionToolResultContentBlock{{
								Type: schema.FunctionToolResultContentBlockTypeText,
								Text: &schema.UserInputText{Text: "match"},
							}},
						}),
					},
				},
			},
		},
	}

	got := adaptAgenticEventToEinoEvents(ev)
	if len(got) != 2 {
		t.Fatalf("events = %d, want assistant + tool", len(got))
	}
	if got[0].Output.MessageOutput.Role != schema.Assistant || got[0].Output.MessageOutput.Message.Content != "text" {
		t.Fatalf("assistant event = %#v", got[0].Output.MessageOutput)
	}
	if got[1].Output.MessageOutput.Role != schema.Tool || got[1].Output.MessageOutput.Message.ToolName != "grep" {
		t.Fatalf("tool event = %#v", got[1].Output.MessageOutput)
	}
}

func TestAdaptAgenticEventToEinoEventsStreamingAssistant(t *testing.T) {
	stream := schema.StreamReaderFromArray([]*schema.AgenticMessage{
		{
			Role: schema.AgenticRoleTypeAssistant,
			ContentBlocks: []*schema.ContentBlock{
				schema.NewContentBlock(&schema.AssistantGenText{Text: "hel"}),
			},
		},
		{
			Role: schema.AgenticRoleTypeAssistant,
			ContentBlocks: []*schema.ContentBlock{
				schema.NewContentBlock(&schema.AssistantGenText{Text: "lo"}),
			},
		},
	})
	ev := &adk.TypedAgentEvent[*schema.AgenticMessage]{
		Output: &adk.TypedAgentOutput[*schema.AgenticMessage]{
			MessageOutput: &adk.TypedMessageVariant[*schema.AgenticMessage]{
				IsStreaming:   true,
				MessageStream: stream,
				AgenticRole:   schema.AgenticRoleTypeAssistant,
			},
		},
	}

	got := adaptAgenticEventToEinoEvents(ev)
	if len(got) != 1 {
		t.Fatalf("events = %d, want 1", len(got))
	}
	mv := got[0].Output.MessageOutput
	if !mv.IsStreaming || mv.Role != schema.Assistant {
		t.Fatalf("stream variant = %#v", mv)
	}
	first, err := mv.MessageStream.Recv()
	if err != nil || first.Content != "hel" {
		t.Fatalf("first = %#v err=%v", first, err)
	}
	second, err := mv.MessageStream.Recv()
	if err != nil || second.Content != "lo" {
		t.Fatalf("second = %#v err=%v", second, err)
	}
	_, err = mv.MessageStream.Recv()
	if !errors.Is(err, io.EOF) {
		t.Fatalf("final err = %v, want EOF", err)
	}
}

func TestAdaptAgenticStreamingToolResultFeedsClassicToolResultHandler(t *testing.T) {
	stream := schema.StreamReaderFromArray([]*schema.AgenticMessage{
		{
			Role: schema.AgenticRoleTypeUser,
			ContentBlocks: []*schema.ContentBlock{
				schema.NewContentBlock(&schema.FunctionToolResult{
					CallID: "call-agentic-stream",
					Name:   "execute",
					Content: []*schema.FunctionToolResultContentBlock{{
						Type: schema.FunctionToolResultContentBlockTypeText,
						Text: &schema.UserInputText{Text: "partial "},
					}},
				}),
			},
		},
		{
			Role: schema.AgenticRoleTypeUser,
			ContentBlocks: []*schema.ContentBlock{
				schema.NewContentBlock(&schema.FunctionToolResult{
					CallID: "call-agentic-stream",
					Name:   "execute",
					Content: []*schema.FunctionToolResultContentBlock{{
						Type: schema.FunctionToolResultContentBlockTypeText,
						Text: &schema.UserInputText{Text: "done"},
					}},
				}),
			},
		},
	})
	ev := &adk.TypedAgentEvent[*schema.AgenticMessage]{
		AgentName: "agentic",
		Output: &adk.TypedAgentOutput[*schema.AgenticMessage]{
			MessageOutput: &adk.TypedMessageVariant[*schema.AgenticMessage]{
				IsStreaming:   true,
				MessageStream: stream,
				AgenticRole:   schema.AgenticRoleTypeUser,
			},
		},
	}

	got := adaptAgenticEventToEinoEvents(ev)
	if len(got) != 1 || got[0].Output == nil || got[0].Output.MessageOutput == nil {
		t.Fatalf("events = %#v", got)
	}
	mv := got[0].Output.MessageOutput
	if !mv.IsStreaming || mv.Role != schema.Tool {
		t.Fatalf("streaming variant = %#v, want tool stream", mv)
	}

	var event map[string]interface{}
	runMessages := newEinoRunMessageAccumulator(nil)
	emitter := newEinoToolResultProgressEmitter(einoToolResultProgressEmitterConfig{
		ConversationID: "conv-agentic",
		Progress: func(eventType, _ string, data interface{}) {
			if eventType == "tool_result" {
				event, _ = data.(map[string]interface{})
			}
		},
	})
	handler := newEinoToolResultEventHandler(einoToolResultEventHandlerConfig{
		RunMessages: runMessages,
		Emitter:     emitter,
	})
	if !handler.HandleStreaming(mv, "agentic") {
		t.Fatal("agentic streaming tool result was not handled")
	}
	if event["toolName"] != "execute" || event["toolCallId"] != "call-agentic-stream" || event["result"] != "partial done" {
		t.Fatalf("tool result event = %#v", event)
	}
	msgs := runMessages.Messages()
	if len(msgs) != 1 || msgs[0].ToolName != "execute" || msgs[0].ToolCallID != "call-agentic-stream" || msgs[0].Content != "partial done" {
		t.Fatalf("run messages = %#v", msgs)
	}
}

func TestAdaptAgenticEventToEinoEventsPreservesErrorOnlyEvent(t *testing.T) {
	wantErr := errors.New("boom")
	ev := &adk.TypedAgentEvent[*schema.AgenticMessage]{AgentName: "agentic", Err: wantErr}

	got := adaptAgenticEventToEinoEvents(ev)
	if len(got) != 1 || got[0].AgentName != "agentic" || !errors.Is(got[0].Err, wantErr) {
		t.Fatalf("events = %#v", got)
	}
}
