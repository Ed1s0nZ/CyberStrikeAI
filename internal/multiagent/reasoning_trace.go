package multiagent

import (
	"encoding/json"
	"fmt"
	"strings"
)

// AggregatedReasoningFromTraceJSON concatenates non-empty assistant `reasoning_content`
// fields from last_react-style JSON (slice of message objects) in document order.
// Used to persist on the single assistant bubble row for audit and for GetMessages fallback
// when the full trace JSON is unavailable. For strict per-message replay, prefer last_react_input.
func AggregatedReasoningFromTraceJSON(traceJSON string) string {
	traceJSON = strings.TrimSpace(traceJSON)
	if traceJSON == "" {
		return ""
	}
	var arr []map[string]interface{}
	if err := json.Unmarshal([]byte(traceJSON), &arr); err != nil {
		return ""
	}
	var b strings.Builder
	for _, m := range arr {
		role, _ := m["role"].(string)
		if !strings.EqualFold(strings.TrimSpace(role), "assistant") {
			continue
		}
		rc := reasoningContentFromMessageMap(m)
		if rc == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(rc)
	}
	return b.String()
}

func reasoningContentFromMessageMap(m map[string]interface{}) string {
	if m == nil {
		return ""
	}
	switch v := m["reasoning_content"].(type) {
	case string:
		return strings.TrimSpace(v)
	case nil:
		return ""
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}
