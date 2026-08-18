package llm

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// AgenticChatModelAdapter exposes a text-oriented AgenticModel as a classic
// BaseChatModel for Eino components that have not adopted AgenticMessage yet.
// It adapts only Eino's in-memory message shape; no HTTP protocol is translated.
type AgenticChatModelAdapter struct {
	model model.AgenticModel
	tools []*schema.ToolInfo
}

func NewAgenticChatModelAdapter(agenticModel model.AgenticModel) model.ChatModel {
	return &AgenticChatModelAdapter{model: agenticModel}
}

func (a *AgenticChatModelAdapter) BindTools(tools []*schema.ToolInfo) error {
	if a == nil || a.model == nil {
		return fmt.Errorf("agentic chat adapter: model is nil")
	}
	a.tools = append([]*schema.ToolInfo(nil), tools...)
	return nil
}

func (a *AgenticChatModelAdapter) Generate(
	ctx context.Context,
	input []*schema.Message,
	opts ...model.Option,
) (*schema.Message, error) {
	if a == nil || a.model == nil {
		return nil, fmt.Errorf("agentic chat adapter: model is nil")
	}
	out, err := a.model.Generate(ctx, classicMessagesToAgentic(input), commonAgenticOptions(a.tools, opts...)...)
	if err != nil {
		return nil, err
	}
	return agenticMessageToClassic(out), nil
}

func (a *AgenticChatModelAdapter) Stream(
	ctx context.Context,
	input []*schema.Message,
	opts ...model.Option,
) (*schema.StreamReader[*schema.Message], error) {
	if a == nil || a.model == nil {
		return nil, fmt.Errorf("agentic chat adapter: model is nil")
	}
	stream, err := a.model.Stream(ctx, classicMessagesToAgentic(input), commonAgenticOptions(a.tools, opts...)...)
	if err != nil {
		return nil, err
	}
	return schema.StreamReaderWithConvert(stream, func(msg *schema.AgenticMessage) (*schema.Message, error) {
		return agenticMessageToClassic(msg), nil
	}), nil
}

func classicMessagesToAgentic(input []*schema.Message) []*schema.AgenticMessage {
	out := make([]*schema.AgenticMessage, 0, len(input))
	for _, msg := range input {
		if msg == nil {
			continue
		}
		role := schema.AgenticRoleTypeUser
		switch msg.Role {
		case schema.System:
			role = schema.AgenticRoleTypeSystem
		case schema.Assistant:
			role = schema.AgenticRoleTypeAssistant
		}
		agentic := &schema.AgenticMessage{Role: role}
		if msg.Role == schema.Assistant {
			if msg.Content != "" {
				agentic.ContentBlocks = append(agentic.ContentBlocks, schema.NewContentBlock(&schema.AssistantGenText{Text: msg.Content}))
			}
		} else if msg.Content != "" {
			agentic.ContentBlocks = append(agentic.ContentBlocks, schema.NewContentBlock(&schema.UserInputText{Text: msg.Content}))
		}
		out = append(out, agentic)
	}
	return out
}

func agenticMessageToClassic(msg *schema.AgenticMessage) *schema.Message {
	if msg == nil {
		return nil
	}
	content, reasoning := AgenticText(msg)
	return &schema.Message{
		Role:             schema.Assistant,
		Content:          content,
		ReasoningContent: reasoning,
	}
}

func commonAgenticOptions(boundTools []*schema.ToolInfo, opts ...model.Option) []model.Option {
	common := model.GetCommonOptions(&model.Options{
		Tools: append([]*schema.ToolInfo(nil), boundTools...),
	}, opts...)
	out := make([]model.Option, 0, 6)
	if common.Temperature != nil {
		out = append(out, model.WithTemperature(*common.Temperature))
	}
	if common.Model != nil {
		out = append(out, model.WithModel(*common.Model))
	}
	if common.TopP != nil {
		out = append(out, model.WithTopP(*common.TopP))
	}
	if common.MaxTokens != nil {
		out = append(out, model.WithMaxTokens(*common.MaxTokens))
	}
	if len(common.Stop) > 0 {
		out = append(out, model.WithStop(common.Stop))
	}
	if common.Tools != nil {
		out = append(out, model.WithTools(common.Tools))
	}
	return out
}
