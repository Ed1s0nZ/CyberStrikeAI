package multiagent

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type capturingAgenticChatModel struct {
	mu     sync.Mutex
	inputs [][]*schema.AgenticMessage
	output *schema.AgenticMessage
}

func (m *capturingAgenticChatModel) Generate(_ context.Context, input []*schema.AgenticMessage, _ ...model.Option) (*schema.AgenticMessage, error) {
	m.mu.Lock()
	m.inputs = append(m.inputs, input)
	m.mu.Unlock()
	if m.output != nil {
		return m.output, nil
	}
	return &schema.AgenticMessage{
		Role:          schema.AgenticRoleTypeAssistant,
		ContentBlocks: []*schema.ContentBlock{schema.NewContentBlock(&schema.AssistantGenText{Text: "agentic answer"})},
	}, nil
}

func (m *capturingAgenticChatModel) Stream(_ context.Context, input []*schema.AgenticMessage, _ ...model.Option) (*schema.StreamReader[*schema.AgenticMessage], error) {
	msg, err := m.Generate(context.Background(), input)
	if err != nil {
		return nil, err
	}
	return schema.StreamReaderFromArray([]*schema.AgenticMessage{msg}), nil
}

func (m *capturingAgenticChatModel) snapshotInputs() [][]*schema.AgenticMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([][]*schema.AgenticMessage, len(m.inputs))
	copy(out, m.inputs)
	return out
}

func TestNewEinoAgenticChatModelAgentAdapterRunsThroughClassicAgentBoundary(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	trace := newModelFacingTraceHolder()
	fakeModel := &capturingAgenticChatModel{}
	agent, err := newEinoAgenticChatModelAgentAdapter(ctx, einoAgenticChatModelAgentConfig{
		Name:        "agentic",
		Description: "agentic adapter test",
		Instruction: "system instruction",
		Model:       fakeModel,
		Handlers: appendEinoAgenticChatModelTailMiddlewares(nil, einoChatModelTailConfig{
			phase: "agentic",
			trace: trace,
		}),
	})
	if err != nil {
		t.Fatalf("newEinoAgenticChatModelAgentAdapter: %v", err)
	}

	iter := agent.Run(ctx, &adk.AgentInput{
		Messages: []*schema.Message{schema.UserMessage("classic input")},
	})
	var last *adk.AgentEvent
	for {
		ev, ok := iter.Next()
		if !ok {
			break
		}
		if ev.Err != nil {
			t.Fatalf("agent event error: %v", ev.Err)
		}
		last = ev
	}
	if last == nil || last.Output == nil || last.Output.MessageOutput == nil {
		t.Fatalf("last event = %#v, want message output", last)
	}
	if got := last.Output.MessageOutput.Message.Content; got != "agentic answer" {
		t.Fatalf("classic output content = %q, want agentic answer", got)
	}

	inputs := fakeModel.snapshotInputs()
	if len(inputs) != 1 {
		t.Fatalf("model calls = %d, want 1", len(inputs))
	}
	if len(inputs[0]) != 2 {
		t.Fatalf("model input messages = %d, want instruction + user", len(inputs[0]))
	}
	if inputs[0][0].Role != schema.AgenticRoleTypeSystem || agenticMessageText(inputs[0][0]) != "system instruction" {
		t.Fatalf("first agentic input = %#v", inputs[0][0])
	}
	if inputs[0][1].Role != schema.AgenticRoleTypeUser || agenticMessageText(inputs[0][1]) != "classic input" {
		t.Fatalf("second agentic input = %#v", inputs[0][1])
	}

	snapshot := trace.Snapshot()
	if len(snapshot) != 2 || snapshot[0].Role != schema.System || snapshot[1].Role != schema.User {
		t.Fatalf("trace snapshot = %#v, want classic system + user trace", snapshot)
	}
}

func TestNewEinoAgenticChatModelAgentAdapterPreservesTypedToolCallsForToolLayerRecovery(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fakeModel := &capturingAgenticChatModel{
		output: &schema.AgenticMessage{
			Role: schema.AgenticRoleTypeAssistant,
			ContentBlocks: []*schema.ContentBlock{schema.NewContentBlock(&schema.FunctionToolCall{
				CallID:    "call-1",
				Name:      "exec",
				Arguments: `{"command":"` + strings.Repeat("x", 20000) + `"}`,
			})},
		},
	}
	agent, err := newEinoAgenticChatModelAgentAdapter(ctx, einoAgenticChatModelAgentConfig{
		Name:        "agentic",
		Description: "agentic adapter test",
		Model:       fakeModel,
		Handlers: appendEinoAgenticChatModelTailMiddlewares(nil, einoChatModelTailConfig{
			phase: "agentic",
		}),
	})
	if err != nil {
		t.Fatalf("newEinoAgenticChatModelAgentAdapter: %v", err)
	}
	iter := agent.Run(ctx, &adk.AgentInput{Messages: []*schema.Message{schema.UserMessage("run")}})
	var last *adk.AgentEvent
	for {
		ev, ok := iter.Next()
		if !ok {
			break
		}
		if ev.Err != nil {
			t.Fatalf("agent event error: %v", ev.Err)
		}
		last = ev
	}
	if last == nil || last.Output == nil || last.Output.MessageOutput == nil {
		t.Fatalf("last event = %#v, want message output", last)
	}
	msg := last.Output.MessageOutput.Message
	if len(msg.ToolCalls) != 1 {
		t.Fatalf("tool calls = %#v, want one tool call", msg.ToolCalls)
	}
	args := msg.ToolCalls[0].Function.Arguments
	if !strings.Contains(args, strings.Repeat("x", 32)) || strings.Contains(args, modelOutputRecoveryKey) {
		t.Fatalf("agentic tool args were unexpectedly rewritten: %q", args)
	}
}

func TestNewEinoAgenticChatModelAgentAdapterRequiresModel(t *testing.T) {
	t.Parallel()
	if _, err := newEinoAgenticChatModelAgentAdapter(context.Background(), einoAgenticChatModelAgentConfig{}); err == nil {
		t.Fatal("expected missing model error")
	}
}
