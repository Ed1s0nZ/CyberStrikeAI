package multiagent

import (
	"strings"

	"github.com/cloudwego/eino/schema"
)

// EinoMessagesToAgentic converts the project's current ADK message history to
// Eino's native AgenticMessage shape. It intentionally covers the text,
// reasoning, function tool-call, and function tool-result channels used by the
// agent runtime today; unsupported multimodal/provider-specific fields stay in
// schema.Message until a real AgenticModel backend is wired.
func EinoMessagesToAgentic(msgs []*schema.Message) []*schema.AgenticMessage {
	if len(msgs) == 0 {
		return nil
	}
	out := make([]*schema.AgenticMessage, 0, len(msgs))
	for _, msg := range msgs {
		if msg == nil {
			continue
		}
		out = append(out, EinoMessageToAgentic(msg))
	}
	return out
}

func EinoMessageToAgentic(msg *schema.Message) *schema.AgenticMessage {
	if msg == nil {
		return nil
	}
	out := &schema.AgenticMessage{
		Role:  messageRoleToAgentic(msg.Role),
		Extra: cloneAnyMap(msg.Extra),
	}
	if msg.ResponseMeta != nil {
		out.ResponseMeta = &schema.AgenticResponseMeta{TokenUsage: msg.ResponseMeta.Usage}
	}
	if text := strings.TrimSpace(msg.ReasoningContent); text != "" {
		out.ContentBlocks = append(out.ContentBlocks, schema.NewContentBlock(&schema.Reasoning{Text: msg.ReasoningContent}))
	}
	switch msg.Role {
	case schema.Assistant:
		if msg.Content != "" {
			out.ContentBlocks = append(out.ContentBlocks, schema.NewContentBlock(&schema.AssistantGenText{Text: msg.Content}))
		}
		for _, tc := range msg.ToolCalls {
			out.ContentBlocks = append(out.ContentBlocks, schema.NewContentBlock(&schema.FunctionToolCall{
				CallID:    tc.ID,
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			}))
		}
	case schema.Tool:
		out.Role = schema.AgenticRoleTypeUser
		out.ContentBlocks = append(out.ContentBlocks, schema.NewContentBlock(&schema.FunctionToolResult{
			CallID: msg.ToolCallID,
			Name:   msg.ToolName,
			Content: []*schema.FunctionToolResultContentBlock{{
				Type: schema.FunctionToolResultContentBlockTypeText,
				Text: &schema.UserInputText{Text: msg.Content},
			}},
		}))
	default:
		if msg.Content != "" {
			out.ContentBlocks = append(out.ContentBlocks, schema.NewContentBlock(&schema.UserInputText{Text: msg.Content}))
		}
	}
	return out
}

// AgenticMessagesToEino converts AgenticMessage values back into the classic
// schema.Message form used by the existing ADK event drain and persistence code.
func AgenticMessagesToEino(msgs []*schema.AgenticMessage) []*schema.Message {
	if len(msgs) == 0 {
		return nil
	}
	out := make([]*schema.Message, 0, len(msgs))
	for _, msg := range msgs {
		if msg == nil {
			continue
		}
		out = append(out, AgenticMessageToEino(msg)...)
	}
	return out
}

func AgenticMessageToEino(msg *schema.AgenticMessage) []*schema.Message {
	if msg == nil {
		return nil
	}
	base := &schema.Message{
		Role:  agenticRoleToMessage(msg.Role),
		Extra: cloneAnyMap(msg.Extra),
	}
	if msg.ResponseMeta != nil {
		base.ResponseMeta = &schema.ResponseMeta{Usage: msg.ResponseMeta.TokenUsage}
	}
	var toolResults []*schema.Message
	for _, block := range msg.ContentBlocks {
		if block == nil {
			continue
		}
		switch {
		case block.Reasoning != nil:
			base.ReasoningContent += block.Reasoning.Text
		case block.UserInputText != nil:
			base.Content += block.UserInputText.Text
		case block.AssistantGenText != nil:
			base.Role = schema.Assistant
			base.Content += block.AssistantGenText.Text
		case block.FunctionToolCall != nil:
			var index *int
			if block.StreamingMeta != nil {
				i := block.StreamingMeta.Index
				index = &i
			}
			base.Role = schema.Assistant
			base.ToolCalls = append(base.ToolCalls, schema.ToolCall{
				Index: index,
				ID:    block.FunctionToolCall.CallID,
				Type:  "function",
				Function: schema.FunctionCall{
					Name:      block.FunctionToolCall.Name,
					Arguments: block.FunctionToolCall.Arguments,
				},
			})
		case block.FunctionToolResult != nil:
			toolResults = append(toolResults, functionToolResultToMessage(block.FunctionToolResult))
		}
	}
	if len(toolResults) > 0 && base.Content == "" && base.ReasoningContent == "" && len(base.ToolCalls) == 0 {
		return toolResults
	}
	out := []*schema.Message{base}
	out = append(out, toolResults...)
	return out
}

func messageRoleToAgentic(role schema.RoleType) schema.AgenticRoleType {
	switch role {
	case schema.System:
		return schema.AgenticRoleTypeSystem
	case schema.Assistant:
		return schema.AgenticRoleTypeAssistant
	default:
		return schema.AgenticRoleTypeUser
	}
}

func agenticRoleToMessage(role schema.AgenticRoleType) schema.RoleType {
	switch role {
	case schema.AgenticRoleTypeSystem:
		return schema.System
	case schema.AgenticRoleTypeAssistant:
		return schema.Assistant
	default:
		return schema.User
	}
}

func functionToolResultToMessage(result *schema.FunctionToolResult) *schema.Message {
	if result == nil {
		return nil
	}
	parts := make([]string, 0, len(result.Content))
	for _, block := range result.Content {
		if block == nil || block.Text == nil {
			continue
		}
		parts = append(parts, block.Text.Text)
	}
	return &schema.Message{
		Role:       schema.Tool,
		Content:    strings.Join(parts, ""),
		ToolCallID: result.CallID,
		ToolName:   result.Name,
	}
}

func cloneAnyMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
