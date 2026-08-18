package workflow

type NodeOutputEnvelope struct {
	Kind     string `json:"kind"`
	NodeID   string `json:"node_id"`
	NodeType string `json:"node_type"`
	Status   string `json:"status"`
	Output   any    `json:"output"`
}

type StartOutput struct {
	NodeOutputEnvelope
	Message        any `json:"message"`
	ConversationID any `json:"conversationId"`
	ProjectID      any `json:"projectId"`
}

type ConditionOutput struct {
	NodeOutputEnvelope
	Condition string `json:"condition"`
	Matched   bool   `json:"matched"`
}

type ToolOutput struct {
	NodeOutputEnvelope
	ToolName    string         `json:"tool_name"`
	Arguments   map[string]any `json:"arguments"`
	ExecutionID string         `json:"execution_id"`
	IsError     bool           `json:"is_error"`
}

type AgentOutput struct {
	NodeOutputEnvelope
	Mode            string   `json:"mode"`
	MCPExecutionIDs []string `json:"mcp_execution_ids"`
}

type HITLOutput struct {
	NodeOutputEnvelope
	Prompt   string `json:"prompt"`
	Reviewer string `json:"reviewer"`
	Approved bool   `json:"approved"`
	Mode     string `json:"mode"`
}

type OutputNodeOutput struct {
	NodeOutputEnvelope
	OutputKey string         `json:"output_key"`
	Outputs   map[string]any `json:"outputs"`
}

func envelope(kind, nodeID, nodeType, status string, output any) NodeOutputEnvelope {
	return NodeOutputEnvelope{Kind: kind, NodeID: nodeID, NodeType: nodeType, Status: status, Output: output}
}

func outputMap(env NodeOutputEnvelope, extra map[string]any) map[string]any {
	out := map[string]any{
		"kind":      env.Kind,
		"node_id":   env.NodeID,
		"node_type": env.NodeType,
		"status":    env.Status,
		"output":    env.Output,
		"typed":     env,
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

func startOutputMap(node graphNode, message, conversationID, projectID any) map[string]any {
	typed := StartOutput{
		NodeOutputEnvelope: envelope("start", node.ID, node.Type, "completed", message),
		Message:            message,
		ConversationID:     conversationID,
		ProjectID:          projectID,
	}
	return outputMap(typed.NodeOutputEnvelope, map[string]any{
		"message":        typed.Message,
		"conversationId": typed.ConversationID,
		"projectId":      typed.ProjectID,
		"typed":          typed,
	})
}

func conditionOutputMap(node graphNode, expr string, matched bool) map[string]any {
	typed := ConditionOutput{
		NodeOutputEnvelope: envelope("condition", node.ID, node.Type, "completed", matched),
		Condition:          expr,
		Matched:            matched,
	}
	return outputMap(typed.NodeOutputEnvelope, map[string]any{"condition": expr, "matched": matched, "typed": typed})
}

func outputNodeOutputMap(node graphNode, key string, value any) map[string]any {
	typed := OutputNodeOutput{
		NodeOutputEnvelope: envelope("output", node.ID, node.Type, "completed", value),
		OutputKey:          key,
		Outputs:            map[string]any{key: value},
	}
	return outputMap(typed.NodeOutputEnvelope, map[string]any{"output_key": key, "outputs": typed.Outputs, "typed": typed})
}

func endOutputMap(node graphNode, value any) map[string]any {
	typed := envelope("end", node.ID, node.Type, "completed", value)
	return outputMap(typed, nil)
}

func toolOutputMap(node graphNode, output string, toolName string, args map[string]any, executionID string, isError bool) map[string]any {
	typed := ToolOutput{
		NodeOutputEnvelope: envelope("tool", node.ID, node.Type, "completed", output),
		ToolName:           toolName,
		Arguments:          args,
		ExecutionID:        executionID,
		IsError:            isError,
	}
	return outputMap(typed.NodeOutputEnvelope, map[string]any{
		"tool_name":    toolName,
		"arguments":    args,
		"execution_id": executionID,
		"is_error":     isError,
		"typed":        typed,
	})
}

func agentOutputMap(node graphNode, response, mode string, mcpIDs []string) map[string]any {
	typed := AgentOutput{
		NodeOutputEnvelope: envelope("agent", node.ID, node.Type, "completed", response),
		Mode:               mode,
		MCPExecutionIDs:    mcpIDs,
	}
	return outputMap(typed.NodeOutputEnvelope, map[string]any{"mode": mode, "mcp_execution_ids": mcpIDs, "typed": typed})
}

func hitlOutputMap(node graphNode, status string, output string, prompt string, reviewer string, approved bool) map[string]any {
	typed := HITLOutput{
		NodeOutputEnvelope: envelope("hitl", node.ID, node.Type, status, output),
		Prompt:             prompt,
		Reviewer:           reviewer,
		Approved:           approved,
		Mode:               "interactive",
	}
	return outputMap(typed.NodeOutputEnvelope, map[string]any{
		"prompt":   prompt,
		"reviewer": reviewer,
		"approved": approved,
		"mode":     "interactive",
		"typed":    typed,
	})
}
