package multiagent

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// agenticToolCallingChatModelAdapter lets Eino's classic plan_execute
// Planner/Replanner consume a native AgenticModel without translating the HTTP
// protocol. Only the in-memory Eino message and option shapes are adapted.
type agenticToolCallingChatModelAdapter struct {
	model model.AgenticModel
	tools []*schema.ToolInfo
}

func newAgenticToolCallingChatModelAdapter(agenticModel model.AgenticModel) model.ToolCallingChatModel {
	return &agenticToolCallingChatModelAdapter{model: agenticModel}
}

func (m *agenticToolCallingChatModelAdapter) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	if m == nil || m.model == nil {
		return nil, fmt.Errorf("agentic tool-calling adapter: model is nil")
	}
	clonedTools := append([]*schema.ToolInfo(nil), tools...)
	return &agenticToolCallingChatModelAdapter{model: m.model, tools: clonedTools}, nil
}

func (m *agenticToolCallingChatModelAdapter) Generate(
	ctx context.Context,
	input []*schema.Message,
	opts ...model.Option,
) (*schema.Message, error) {
	if m == nil || m.model == nil {
		return nil, fmt.Errorf("agentic tool-calling adapter: model is nil")
	}
	out, err := m.model.Generate(ctx, EinoMessagesToAgentic(input), m.agenticOptions(opts...)...)
	if err != nil {
		return nil, err
	}
	converted := AgenticMessageToEino(out)
	if len(converted) == 0 {
		return nil, fmt.Errorf("agentic tool-calling adapter: model returned no message")
	}
	return converted[0], nil
}

func (m *agenticToolCallingChatModelAdapter) Stream(
	ctx context.Context,
	input []*schema.Message,
	opts ...model.Option,
) (*schema.StreamReader[*schema.Message], error) {
	if m == nil || m.model == nil {
		return nil, fmt.Errorf("agentic tool-calling adapter: model is nil")
	}
	stream, err := m.model.Stream(ctx, EinoMessagesToAgentic(input), m.agenticOptions(opts...)...)
	if err != nil {
		return nil, err
	}
	return agenticStreamToEinoStream(stream), nil
}

func (m *agenticToolCallingChatModelAdapter) agenticOptions(opts ...model.Option) []model.Option {
	common := model.GetCommonOptions(&model.Options{
		Tools: append([]*schema.ToolInfo(nil), m.tools...),
	}, opts...)
	out := make([]model.Option, 0, 8)
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
	if common.AgenticToolChoice != nil {
		out = append(out, model.WithAgenticToolChoice(common.AgenticToolChoice))
	} else if common.ToolChoice != nil {
		out = append(out, model.WithAgenticToolChoice(classicToolChoiceToAgentic(
			*common.ToolChoice,
			common.AllowedToolNames,
		)))
	}
	return out
}

func classicToolChoiceToAgentic(choice schema.ToolChoice, allowedNames []string) *schema.AgenticToolChoice {
	allowed := make([]*schema.AllowedTool, 0, len(allowedNames))
	for _, name := range allowedNames {
		if name != "" {
			allowed = append(allowed, &schema.AllowedTool{FunctionName: name})
		}
	}
	out := &schema.AgenticToolChoice{Type: choice}
	switch choice {
	case schema.ToolChoiceAllowed:
		if len(allowed) > 0 {
			out.Allowed = &schema.AgenticAllowedToolChoice{Tools: allowed}
		}
	case schema.ToolChoiceForced:
		if len(allowed) > 0 {
			out.Forced = &schema.AgenticForcedToolChoice{Tools: allowed}
		}
	}
	return out
}
