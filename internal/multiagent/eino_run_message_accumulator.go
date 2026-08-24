package multiagent

import (
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

type einoRunMessageAccumulator struct {
	baseCount int
	msgs      []adk.Message
}

func newEinoRunMessageAccumulator(base []adk.Message) *einoRunMessageAccumulator {
	msgs := append([]adk.Message(nil), base...)
	return &einoRunMessageAccumulator{
		baseCount: len(msgs),
		msgs:      msgs,
	}
}

func (a *einoRunMessageAccumulator) Append(msg adk.Message) bool {
	if a == nil || msg == nil {
		return false
	}
	a.msgs = append(a.msgs, msg)
	return true
}

func (a *einoRunMessageAccumulator) AppendToolMessage(content, toolCallID string, opts ...schema.ToolMessageOption) bool {
	if strings.TrimSpace(toolCallID) == "" {
		return false
	}
	return a.Append(schema.ToolMessage(content, toolCallID, opts...))
}

func (a *einoRunMessageAccumulator) AppendAssistantText(content string) bool {
	content = strings.TrimSpace(content)
	if content == "" {
		return false
	}
	return a.Append(schema.AssistantMessage(content, nil))
}

func (a *einoRunMessageAccumulator) AppendAssistantToolCalls(toolCalls []schema.ToolCall) bool {
	if len(toolCalls) == 0 {
		return false
	}
	return a.Append(schema.AssistantMessage("", toolCalls))
}

func (a *einoRunMessageAccumulator) Messages() []adk.Message {
	if a == nil {
		return nil
	}
	return a.msgs
}

func (a *einoRunMessageAccumulator) NewMessages() []adk.Message {
	if a == nil {
		return nil
	}
	if a.baseCount < 0 || a.baseCount >= len(a.msgs) {
		return nil
	}
	return a.msgs[a.baseCount:]
}

func (a *einoRunMessageAccumulator) BaseCount() int {
	if a == nil {
		return 0
	}
	return a.baseCount
}

func (a *einoRunMessageAccumulator) HasNewMessages() bool {
	return a != nil && len(a.msgs) > a.baseCount
}
