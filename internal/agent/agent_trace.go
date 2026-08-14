package agent

import (
	"encoding/json"
	"strings"
)

const ModelFacingTraceVersionKey = "cyberstrike_model_facing_trace_version"

// IsModelFacingTraceJSON reports whether a persisted trace was produced from the final
// model-boundary state. Legacy traces have no version marker and require one-time migration.
func IsModelFacingTraceJSON(traceInputJSON string) bool {
	var raw []map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(traceInputJSON)), &raw); err != nil {
		return false
	}
	for _, msg := range raw {
		extra, _ := msg["extra"].(map[string]interface{})
		v, ok := extra[ModelFacingTraceVersionKey]
		if !ok {
			continue
		}
		switch n := v.(type) {
		case float64:
			return n >= 1
		case int:
			return n >= 1
		}
	}
	return false
}

// ParseTraceMessages 解析落库的 last_react_input（OpenAI 风格 messages JSON 数组）。
func ParseTraceMessages(traceInputJSON string) ([]ChatMessage, error) {
	traceInputJSON = strings.TrimSpace(traceInputJSON)
	if traceInputJSON == "" {
		return nil, nil
	}
	var raw []map[string]interface{}
	if err := json.Unmarshal([]byte(traceInputJSON), &raw); err != nil {
		return nil, err
	}
	modelFacing := IsModelFacingTraceJSON(traceInputJSON)
	out := make([]ChatMessage, 0, len(raw))
	for _, msgMap := range raw {
		msg := ChatMessage{}
		role, _ := msgMap["role"].(string)
		if role == "" {
			continue
		}
		msg.Role = role
		msg.ModelFacingTrace = modelFacing
		if content, ok := msgMap["content"].(string); ok {
			msg.Content = content
		}
		if rc, ok := msgMap["reasoning_content"].(string); ok && strings.TrimSpace(rc) != "" {
			msg.ReasoningContent = rc
		}
		if toolCallsRaw, ok := msgMap["tool_calls"]; ok && toolCallsRaw != nil {
			if toolCallsArray, ok := toolCallsRaw.([]interface{}); ok {
				for _, tcRaw := range toolCallsArray {
					tcMap, ok := tcRaw.(map[string]interface{})
					if !ok {
						continue
					}
					toolCall := ToolCall{}
					if id, ok := tcMap["id"].(string); ok {
						toolCall.ID = id
					}
					if toolType, ok := tcMap["type"].(string); ok {
						toolCall.Type = toolType
					}
					if funcMap, ok := tcMap["function"].(map[string]interface{}); ok {
						toolCall.Function = FunctionCall{}
						if name, ok := funcMap["name"].(string); ok {
							toolCall.Function.Name = name
						}
						if argsRaw, ok := funcMap["arguments"]; ok {
							if argsStr, ok := argsRaw.(string); ok {
								var argsMap map[string]interface{}
								if err := json.Unmarshal([]byte(argsStr), &argsMap); err == nil {
									toolCall.Function.Arguments = argsMap
								}
							} else if argsMap, ok := argsRaw.(map[string]interface{}); ok {
								toolCall.Function.Arguments = argsMap
							}
						}
					}
					if toolCall.ID != "" {
						msg.ToolCalls = append(msg.ToolCalls, toolCall)
					}
				}
			}
		}
		if toolCallID, ok := msgMap["tool_call_id"].(string); ok {
			msg.ToolCallID = toolCallID
		}
		if tn, ok := msgMap["tool_name"].(string); ok && strings.TrimSpace(tn) != "" {
			msg.ToolName = strings.TrimSpace(tn)
		} else if tn, ok := msgMap["name"].(string); ok && strings.TrimSpace(tn) != "" && strings.EqualFold(msg.Role, "tool") {
			msg.ToolName = strings.TrimSpace(tn)
		}
		out = append(out, msg)
	}
	return out, nil
}

// ExtractLastUserTurnMessages 仅保留最后一次 user 提问起的消息（不含更早的用户轮次；跳过 system）。
// 与「继续对话」续跑所用轨迹范围一致：当前任务轮次，而非整段多轮对话历史。
func ExtractLastUserTurnMessages(msgs []ChatMessage) []ChatMessage {
	if len(msgs) == 0 {
		return msgs
	}
	lastUser := -1
	for i, m := range msgs {
		if strings.EqualFold(m.Role, "user") {
			lastUser = i
		}
	}
	if lastUser < 0 {
		return msgs
	}
	trimmed := msgs[lastUser:]
	out := make([]ChatMessage, 0, len(trimmed))
	for _, m := range trimmed {
		if strings.EqualFold(m.Role, "system") {
			continue
		}
		out = append(out, m)
	}
	return out
}

// ExtractLastUserTurnTraceJSON 在 JSON 轨迹上裁剪为最后一次 user 起的片段（供落库格式直接处理）。
func ExtractLastUserTurnTraceJSON(traceInputJSON string) string {
	traceInputJSON = strings.TrimSpace(traceInputJSON)
	if traceInputJSON == "" {
		return traceInputJSON
	}
	var arr []map[string]interface{}
	if err := json.Unmarshal([]byte(traceInputJSON), &arr); err != nil {
		return traceInputJSON
	}
	lastUser := -1
	for i, m := range arr {
		if r, _ := m["role"].(string); strings.EqualFold(r, "user") {
			lastUser = i
		}
	}
	if lastUser <= 0 {
		return traceInputJSON
	}
	trimmed := arr[lastUser:]
	b, err := json.Marshal(trimmed)
	if err != nil {
		return traceInputJSON
	}
	return string(b)
}

// MergeAssistantTraceOutput 将 last_react_output 合并进轨迹最后一条 assistant（与 loadHistoryFromAgentTrace 一致）。
func MergeAssistantTraceOutput(msgs []ChatMessage, assistantOut string) []ChatMessage {
	assistantOut = strings.TrimSpace(assistantOut)
	if assistantOut == "" || len(msgs) == 0 {
		return msgs
	}
	out := append([]ChatMessage(nil), msgs...)
	last := &out[len(out)-1]
	if strings.EqualFold(last.Role, "assistant") && len(last.ToolCalls) == 0 {
		last.Content = assistantOut
		return out
	}
	out = append(out, ChatMessage{
		Role:    "assistant",
		Content: assistantOut,
	})
	return out
}

// MessagesToTraceJSON 将消息带序列化为 JSON（跳过 system）。
func MessagesToTraceJSON(msgs []ChatMessage) (string, error) {
	filtered := make([]ChatMessage, 0, len(msgs))
	for _, m := range msgs {
		if strings.EqualFold(m.Role, "system") {
			continue
		}
		filtered = append(filtered, m)
	}
	b, err := json.Marshal(filtered)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
