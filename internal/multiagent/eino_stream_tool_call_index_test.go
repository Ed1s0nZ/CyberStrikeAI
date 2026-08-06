package multiagent

import (
	"context"
	"io"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type streamToolCallIndexFakeModel struct {
	chunks []*schema.Message
}

func (m *streamToolCallIndexFakeModel) Generate(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	return nil, nil
}

func (m *streamToolCallIndexFakeModel) Stream(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return schema.StreamReaderFromArray(m.chunks), nil
}

func (m *streamToolCallIndexFakeModel) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

func TestStreamToolCallIndexRepairSeparatesConflictingIDs(t *testing.T) {
	index := 0
	wrapped := newStreamToolCallIndexRepairModel(&streamToolCallIndexFakeModel{chunks: []*schema.Message{
		schema.AssistantMessage("", []schema.ToolCall{{
			Index: &index, ID: "fc_call_0", Type: "function",
			Function: schema.FunctionCall{Name: "search", Arguments: `{"query":"one"}`},
		}}),
		schema.AssistantMessage("", []schema.ToolCall{{
			Index: &index, ID: "fc_call_1", Type: "function",
			Function: schema.FunctionCall{Name: "task", Arguments: `{"query":"two"}`},
		}}),
	}})

	got := readStreamToolCallChunks(t, wrapped)
	merged, err := schema.ConcatMessages(got)
	if err != nil {
		t.Fatalf("ConcatMessages() error = %v", err)
	}
	if len(merged.ToolCalls) != 2 {
		t.Fatalf("tool call count = %d, want 2", len(merged.ToolCalls))
	}
	if merged.ToolCalls[0].ID != "fc_call_0" || merged.ToolCalls[1].ID != "fc_call_1" {
		t.Fatalf("tool call IDs = %#v", merged.ToolCalls)
	}
	if merged.ToolCalls[0].Index == nil || *merged.ToolCalls[0].Index != 0 || merged.ToolCalls[1].Index == nil || *merged.ToolCalls[1].Index != 1 {
		t.Fatalf("tool call indexes = %#v", merged.ToolCalls)
	}
}

func TestStreamToolCallIndexRepairPreservesFragmentsForOneID(t *testing.T) {
	index := 0
	wrapped := newStreamToolCallIndexRepairModel(&streamToolCallIndexFakeModel{chunks: []*schema.Message{
		schema.AssistantMessage("", []schema.ToolCall{{
			Index: &index, ID: "call_0", Type: "function",
			Function: schema.FunctionCall{Name: "search", Arguments: `{"query":"`},
		}}),
		schema.AssistantMessage("", []schema.ToolCall{{
			Index: &index, ID: "call_0", Type: "function",
			Function: schema.FunctionCall{Arguments: `one"}`},
		}}),
	}})

	got := readStreamToolCallChunks(t, wrapped)
	merged, err := schema.ConcatMessages(got)
	if err != nil {
		t.Fatalf("ConcatMessages() error = %v", err)
	}
	if len(merged.ToolCalls) != 1 || merged.ToolCalls[0].Function.Arguments != `{"query":"one"}` {
		t.Fatalf("tool calls = %#v", merged.ToolCalls)
	}
}

func TestStreamToolCallIndexRepairLeavesValidParallelIndexesUntouched(t *testing.T) {
	first, second := 0, 1
	wrapped := newStreamToolCallIndexRepairModel(&streamToolCallIndexFakeModel{chunks: []*schema.Message{
		schema.AssistantMessage("", []schema.ToolCall{
			{Index: &first, ID: "call_0", Type: "function", Function: schema.FunctionCall{Name: "search", Arguments: `{}`}},
			{Index: &second, ID: "call_1", Type: "function", Function: schema.FunctionCall{Name: "task", Arguments: `{}`}},
		}),
	}})

	got := readStreamToolCallChunks(t, wrapped)
	if len(got) != 1 || len(got[0].ToolCalls) != 2 {
		t.Fatalf("chunks = %#v", got)
	}
	if *got[0].ToolCalls[0].Index != 0 || *got[0].ToolCalls[1].Index != 1 {
		t.Fatalf("tool call indexes changed: %#v", got[0].ToolCalls)
	}
}

func readStreamToolCallChunks(t *testing.T, chatModel model.ToolCallingChatModel) []*schema.Message {
	t.Helper()
	stream, err := chatModel.Stream(context.Background(), nil)
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}
	defer stream.Close()

	var chunks []*schema.Message
	for {
		chunk, recvErr := stream.Recv()
		if recvErr == io.EOF {
			return chunks
		}
		if recvErr != nil {
			t.Fatalf("Recv() error = %v", recvErr)
		}
		chunks = append(chunks, chunk)
	}
}
