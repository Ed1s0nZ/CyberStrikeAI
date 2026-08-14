package multiagent

import (
	"encoding/json"
	"strings"
)

// UnwrapPlanExecuteUserText 若模型输出单层 JSON 且含常见「对用户回复」字段，则取出纯文本；否则原样返回。
// 用于 Plan-Execute 下 executor 套 `{"response":"..."}` 或误把 replanner/planner JSON 当作最终气泡时的缓解。
func UnwrapPlanExecuteUserText(s string) string {
	s = strings.TrimSpace(s)
	if len(s) < 2 || s[0] != '{' || s[len(s)-1] != '}' {
		return s
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return s
	}
	for _, key := range []string{
		"response", "answer", "message", "content", "output",
		"final_answer", "reply", "text", "result_text",
	} {
		v, ok := m[key]
		if !ok || v == nil {
			continue
		}
		str, ok := v.(string)
		if !ok {
			continue
		}
		if t := strings.TrimSpace(str); t != "" {
			return t
		}
	}
	return s
}
