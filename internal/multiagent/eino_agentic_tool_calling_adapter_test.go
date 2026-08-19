package multiagent

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type capturingAgenticToolCallingModel struct {
	input   []*schema.AgenticMessage
	options *model.Options
}

func (m *capturingAgenticToolCallingModel) Generate(
	_ context.Context,
	input []*schema.AgenticMessage,
	opts ...model.Option,
) (*schema.AgenticMessage, error) {
	m.input = input
	m.options = model.GetCommonOptions(nil, opts...)
	return &schema.AgenticMessage{
		Role: schema.AgenticRoleTypeAssistant,
		ContentBlocks: []*schema.ContentBlock{
			schema.NewContentBlock(&schema.FunctionToolCall{
				CallID:    "call-1",
				Name:      "emit_plan",
				Arguments: `{"steps":["inspect"]}`,
			}),
		},
	}, nil
}

func (m *capturingAgenticToolCallingModel) Stream(
	context.Context,
	[]*schema.AgenticMessage,
	...model.Option,
) (*schema.StreamReader[*schema.AgenticMessage], error) {
	return schema.StreamReaderFromArray([]*schema.AgenticMessage{
		{
			Role: schema.AgenticRoleTypeAssistant,
			ContentBlocks: []*schema.ContentBlock{
				schema.NewContentBlockChunk(&schema.FunctionToolCall{
					CallID:    "call-1",
					Name:      "emit_plan",
					Arguments: `{"steps":[`,
				}, &schema.StreamingMeta{Index: 0}),
			},
		},
		{
			Role: schema.AgenticRoleTypeAssistant,
			ContentBlocks: []*schema.ContentBlock{
				schema.NewContentBlockChunk(&schema.FunctionToolCall{
					Arguments: `"inspect"]}`,
				}, &schema.StreamingMeta{Index: 0}),
			},
		},
	}), nil
}

func TestAgenticToolCallingAdapterConvertsForcedToolChoice(t *testing.T) {
	t.Parallel()
	native := &capturingAgenticToolCallingModel{}
	adapter, err := newAgenticToolCallingChatModelAdapter(native).WithTools([]*schema.ToolInfo{{
		Name: "emit_plan",
		Desc: "emit a structured plan",
	}})
	if err != nil {
		t.Fatalf("WithTools: %v", err)
	}

	out, err := adapter.Generate(
		context.Background(),
		[]*schema.Message{schema.UserMessage("plan this task")},
		model.WithToolChoice(schema.ToolChoiceForced),
	)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(native.input) != 1 || native.input[0].Role != schema.AgenticRoleTypeUser {
		t.Fatalf("native input = %#v", native.input)
	}
	if native.options == nil || len(native.options.Tools) != 1 || native.options.Tools[0].Name != "emit_plan" {
		t.Fatalf("native tools = %#v", native.options)
	}
	if native.options.AgenticToolChoice == nil || native.options.AgenticToolChoice.Type != schema.ToolChoiceForced {
		t.Fatalf("agentic tool choice = %#v", native.options.AgenticToolChoice)
	}
	if len(out.ToolCalls) != 1 || out.ToolCalls[0].Function.Name != "emit_plan" {
		t.Fatalf("classic output = %#v", out)
	}
}

func TestAgenticToolCallingAdapterPreservesStreamingToolCallIndex(t *testing.T) {
	t.Parallel()
	adapter := newAgenticToolCallingChatModelAdapter(&capturingAgenticToolCallingModel{})
	stream, err := adapter.Stream(context.Background(), []*schema.Message{
		schema.UserMessage("plan this task"),
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	out, err := schema.ConcatMessageStream(stream)
	if err != nil {
		t.Fatalf("ConcatMessageStream: %v", err)
	}
	if len(out.ToolCalls) != 1 {
		t.Fatalf("tool calls = %#v, want one merged call", out.ToolCalls)
	}
	call := out.ToolCalls[0]
	if call.Index == nil || *call.Index != 0 {
		t.Fatalf("tool call index = %#v", call.Index)
	}
	if call.Function.Name != "emit_plan" || call.Function.Arguments != `{"steps":["inspect"]}` {
		t.Fatalf("merged tool call = %#v", call)
	}
}
