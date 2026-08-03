package multiagent

import (
	"context"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// streamToolCallIndexRepairModel isolates an OpenAI-compatible streaming
// protocol defect before Eino concatenates response chunks. Some providers
// reuse a tool-call index for different non-empty tool-call IDs in one stream.
// Eino correctly rejects that shape because one index represents one call.
//
// The wrapper keeps valid streams untouched. When it sees the conflicting
// shape, it assigns each distinct ID a stable, stream-local index so Eino can
// retain all calls instead of aborting the agent run.
type streamToolCallIndexRepairModel struct {
	base model.ToolCallingChatModel
}

func newStreamToolCallIndexRepairModel(base model.ToolCallingChatModel) model.ToolCallingChatModel {
	if base == nil {
		return nil
	}
	return &streamToolCallIndexRepairModel{base: base}
}

func (m *streamToolCallIndexRepairModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	return m.base.Generate(ctx, input, opts...)
}

func (m *streamToolCallIndexRepairModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	stream, err := m.base.Stream(ctx, input, opts...)
	if err != nil {
		return nil, err
	}
	state := newStreamToolCallIndexRepairState()
	return schema.StreamReaderWithConvert(stream, state.repairMessage), nil
}

func (m *streamToolCallIndexRepairModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	withTools, err := m.base.WithTools(tools)
	if err != nil {
		return nil, err
	}
	return newStreamToolCallIndexRepairModel(withTools), nil
}

type streamToolCallIndexRepairState struct {
	indexByID     map[string]int
	idByIndex     map[int]string
	nextFreeIndex int
}

func newStreamToolCallIndexRepairState() *streamToolCallIndexRepairState {
	return &streamToolCallIndexRepairState{
		indexByID: make(map[string]int),
		idByIndex: make(map[int]string),
	}
}

func (s *streamToolCallIndexRepairState) repairMessage(msg *schema.Message) (*schema.Message, error) {
	if msg == nil || len(msg.ToolCalls) == 0 {
		return msg, nil
	}

	var calls []schema.ToolCall
	changed := false
	for i := range msg.ToolCalls {
		call := msg.ToolCalls[i]
		if call.Index == nil || call.ID == "" {
			continue
		}

		sourceIndex := *call.Index
		if sourceIndex >= s.nextFreeIndex {
			s.nextFreeIndex = sourceIndex + 1
		}

		assigned, known := s.indexByID[call.ID]
		if !known {
			assigned = sourceIndex
			if owner, occupied := s.idByIndex[assigned]; occupied && owner != call.ID {
				assigned = s.takeFreeIndex()
			}
			s.indexByID[call.ID] = assigned
			s.idByIndex[assigned] = call.ID
		}
		if assigned == sourceIndex {
			continue
		}
		if calls == nil {
			calls = append([]schema.ToolCall(nil), msg.ToolCalls...)
		}
		index := assigned
		calls[i].Index = &index
		changed = true
	}

	if !changed {
		return msg, nil
	}
	out := *msg
	out.ToolCalls = calls
	return &out, nil
}

func (s *streamToolCallIndexRepairState) takeFreeIndex() int {
	for {
		candidate := s.nextFreeIndex
		s.nextFreeIndex++
		if _, occupied := s.idByIndex[candidate]; !occupied {
			return candidate
		}
	}
}
