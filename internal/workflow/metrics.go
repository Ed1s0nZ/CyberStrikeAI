package workflow

import (
	"fmt"
	"strconv"
)

func accumulateWorkflowMetric(state *WorkflowLocalState, key string, delta any) {
	if state == nil {
		return
	}
	if state.Metrics == nil {
		state.Metrics = make(map[string]any)
	}
	current := numericMetric(state.Metrics[key])
	state.Metrics[key] = current + numericMetric(delta)
}

func numericMetric(v any) float64 {
	switch n := v.(type) {
	case int:
		return float64(n)
	case int32:
		return float64(n)
	case int64:
		return float64(n)
	case float32:
		return float64(n)
	case float64:
		return n
	case string:
		f, _ := strconv.ParseFloat(n, 64)
		return f
	default:
		f, _ := strconv.ParseFloat(fmt.Sprint(v), 64)
		return f
	}
}

func collectAgentMetrics(state *WorkflowLocalState, data interface{}) {
	m, ok := data.(map[string]interface{})
	if !ok || state == nil {
		return
	}
	for _, key := range []string{"prompt_tokens", "completion_tokens", "total_tokens", "cost", "input_tokens", "output_tokens"} {
		if v, ok := m[key]; ok {
			accumulateWorkflowMetric(state, key, v)
		}
	}
	if usage, ok := m["usage"].(map[string]interface{}); ok {
		for _, key := range []string{"prompt_tokens", "completion_tokens", "total_tokens", "input_tokens", "output_tokens"} {
			if v, ok := usage[key]; ok {
				accumulateWorkflowMetric(state, key, v)
			}
		}
	}
}
