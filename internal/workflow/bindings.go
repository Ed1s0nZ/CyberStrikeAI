package workflow

import (
	"encoding/json"
	"fmt"
	"strings"
)

// FieldBinding selects a value from workflow state (replaces {{...}} templates).
type FieldBinding struct {
	From  string `json:"from"`  // inputs | previous | <nodeId>
	Field string `json:"field"` // e.g. output, message
}

func parseFieldBinding(cfg map[string]any, keys ...string) (FieldBinding, bool) {
	for _, key := range keys {
		if cfg == nil {
			continue
		}
		raw, ok := cfg[key]
		if !ok || raw == nil {
			continue
		}
		switch v := raw.(type) {
		case map[string]any:
			return FieldBinding{
				From:  strings.TrimSpace(fmt.Sprint(v["from"])),
				Field: strings.TrimSpace(fmt.Sprint(v["field"])),
			}, true
		case string:
			s := strings.TrimSpace(v)
			if s == "" {
				continue
			}
			var b FieldBinding
			if err := json.Unmarshal([]byte(s), &b); err == nil && (b.From != "" || b.Field != "") {
				return b, true
			}
		}
	}
	return FieldBinding{}, false
}

func defaultBinding(from, field string) FieldBinding {
	return FieldBinding{From: from, Field: field}
}

func resolveBinding(b FieldBinding, state *WorkflowLocalState) any {
	from := strings.TrimSpace(b.From)
	field := strings.TrimSpace(b.Field)
	if field == "" {
		field = "output"
	}
	if from == "" || from == "previous" || from == "prev" {
		if strings.HasPrefix(field, "$") || strings.HasPrefix(field, ".") {
			return evalJSONPathValue(state.LastOutput, field)
		}
		if field == "output" && state.LastOutput != nil {
			return state.LastOutput["output"]
		}
		return valueFromPath("previous."+field, state)
	}
	if from == "inputs" || from == "input" {
		if strings.HasPrefix(field, "$") || strings.HasPrefix(field, ".") {
			return evalJSONPathValue(state.Inputs, field)
		}
		if field == "" {
			return state.Inputs
		}
		return valueFromPath("inputs."+field, state)
	}
	if from == "outputs" {
		if strings.HasPrefix(field, "$") || strings.HasPrefix(field, ".") {
			return evalJSONPathValue(state.Outputs, field)
		}
		return valueFromPath("outputs."+field, state)
	}
	if strings.HasPrefix(field, "$") || strings.HasPrefix(field, ".") {
		return evalJSONPathValue(valueFromPath(from, state), field)
	}
	return valueFromPath(from+"."+field, state)
}

func resolveBindingString(b FieldBinding, state *WorkflowLocalState) string {
	return strings.TrimSpace(fmt.Sprint(resolveBinding(b, state)))
}

func resolveNodeInputBinding(cfg map[string]any, state *WorkflowLocalState) string {
	if b, ok := parseFieldBinding(cfg, "input_binding"); ok {
		return resolveBindingString(b, state)
	}
	// legacy template field removed — default previous.output
	return resolveBindingString(defaultBinding("previous", "output"), state)
}

func resolveOutputSourceBinding(cfg map[string]any, state *WorkflowLocalState) any {
	if b, ok := parseFieldBinding(cfg, "source_binding"); ok {
		return resolveBinding(b, state)
	}
	return resolveBinding(defaultBinding("previous", "output"), state)
}

func resolveHITLPromptBinding(cfg map[string]any, state *WorkflowLocalState) string {
	if b, ok := parseFieldBinding(cfg, "prompt_binding"); ok {
		return resolveBindingString(b, state)
	}
	if s := cfgString(cfg, "prompt"); s != "" {
		return s
	}
	return resolveBindingString(defaultBinding("previous", "output"), state)
}

func toolArgumentBindings(cfg map[string]any) map[string]FieldBinding {
	raw, ok := cfg["argument_bindings"].(map[string]any)
	if !ok || len(raw) == 0 {
		return nil
	}
	out := make(map[string]FieldBinding, len(raw))
	for argName, v := range raw {
		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		out[argName] = FieldBinding{
			From:  strings.TrimSpace(fmt.Sprint(m["from"])),
			Field: strings.TrimSpace(fmt.Sprint(m["field"])),
		}
	}
	return out
}

func resolveToolArguments(cfg map[string]any, state *WorkflowLocalState) (map[string]interface{}, error) {
	bindings := toolArgumentBindings(cfg)
	if len(bindings) > 0 {
		args := make(map[string]interface{}, len(bindings))
		for k, b := range bindings {
			args[k] = resolveBinding(b, state)
		}
		return args, nil
	}
	raw := cfgString(cfg, "arguments")
	if raw == "" {
		return map[string]interface{}{}, nil
	}
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &args); err != nil {
		return nil, err
	}
	if args == nil {
		args = map[string]interface{}{}
	}
	return args, nil
}
