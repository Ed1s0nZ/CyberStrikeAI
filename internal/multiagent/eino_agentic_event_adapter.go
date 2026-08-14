package multiagent

import (
	"io"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

// adaptAgenticEventToEinoEvents converts typed AgenticMessage ADK events into
// the classic schema.Message events consumed by the existing SSE/MCP drain.
func adaptAgenticEventToEinoEvents(ev *adk.TypedAgentEvent[*schema.AgenticMessage]) []*adk.AgentEvent {
	if ev == nil {
		return nil
	}
	base := func(output *adk.AgentOutput) *adk.AgentEvent {
		return &adk.AgentEvent{
			AgentName: ev.AgentName,
			RunPath:   append([]adk.RunStep(nil), ev.RunPath...),
			Output:    output,
			Action:    ev.Action,
			Err:       ev.Err,
		}
	}
	if ev.Output == nil {
		return []*adk.AgentEvent{base(nil)}
	}
	customized := ev.Output.CustomizedOutput
	mv := ev.Output.MessageOutput
	if mv == nil {
		return []*adk.AgentEvent{base(&adk.AgentOutput{CustomizedOutput: customized})}
	}
	if mv.IsStreaming {
		return []*adk.AgentEvent{base(&adk.AgentOutput{
			MessageOutput: &adk.MessageVariant{
				IsStreaming:   true,
				MessageStream: agenticStreamToEinoStream(mv.MessageStream),
				Role:          agenticVariantRole(mv),
			},
			CustomizedOutput: customized,
		})}
	}

	msgs := AgenticMessageToEino(mv.Message)
	if len(msgs) == 0 {
		return []*adk.AgentEvent{base(&adk.AgentOutput{CustomizedOutput: customized})}
	}
	out := make([]*adk.AgentEvent, 0, len(msgs))
	for i, msg := range msgs {
		eventCustomized := any(nil)
		if i == 0 {
			eventCustomized = customized
		}
		out = append(out, base(&adk.AgentOutput{
			MessageOutput: &adk.MessageVariant{
				Message:  msg,
				Role:     msg.Role,
				ToolName: msg.ToolName,
			},
			CustomizedOutput: eventCustomized,
		}))
	}
	return out
}

func agenticStreamToEinoStream(sr *schema.StreamReader[*schema.AgenticMessage]) *schema.StreamReader[*schema.Message] {
	out, writer := schema.Pipe[*schema.Message](8)
	go func() {
		defer writer.Close()
		if sr == nil {
			return
		}
		defer sr.Close()
		for {
			chunk, err := sr.Recv()
			if err != nil {
				if err != io.EOF {
					writer.Send(nil, err)
				}
				return
			}
			for _, msg := range AgenticMessageToEino(chunk) {
				if msg != nil && writer.Send(msg, nil) {
					return
				}
			}
		}
	}()
	return out
}

func agenticVariantRole(mv *adk.TypedMessageVariant[*schema.AgenticMessage]) schema.RoleType {
	if mv == nil {
		return schema.Assistant
	}
	switch mv.AgenticRole {
	case schema.AgenticRoleTypeSystem:
		return schema.System
	case schema.AgenticRoleTypeUser:
		// In Agentic ReAct output, user-role events from the graph are local
		// FunctionToolResult messages emitted by AgenticToolsNode.
		return schema.Tool
	default:
		return schema.Assistant
	}
}
