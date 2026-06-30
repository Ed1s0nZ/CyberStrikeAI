package openai

import (
	"github.com/bytedance/sonic"
)

// reasoningPayloadKeys are OpenAI-compatible root fields that enable "thinking" /
// extended-reasoning modes on gateways such as DashScope/Qwen and MiniMax.
var reasoningPayloadKeys = []string{
	"thinking",
	"reasoning_effort",
	"output_config",
	"reasoning",
}

// StripReasoningFromChatCompletionBody removes thinking / reasoning fields from a
// chat-completions JSON body.
func StripReasoningFromChatCompletionBody(rawBody []byte) ([]byte, error) {
	var payload map[string]any
	if err := sonic.Unmarshal(rawBody, &payload); err != nil {
		return rawBody, nil
	}
	if !stripReasoningFields(payload) {
		return rawBody, nil
	}
	out, err := sonic.Marshal(payload)
	if err != nil {
		return rawBody, err
	}
	return out, nil
}

// StripReasoningIfForcedToolChoice removes thinking / reasoning fields when the
// request sets tool_choice to "required" or an object. Several providers reject
// that combination (e.g. DashScope: "tool_choice does not support being set to
// required or object in thinking mode").
func StripReasoningIfForcedToolChoice(rawBody []byte) ([]byte, error) {
	var payload map[string]any
	if err := sonic.Unmarshal(rawBody, &payload); err != nil {
		return rawBody, nil
	}
	if !forcedToolChoiceIncompatibleWithThinking(payload) {
		return rawBody, nil
	}
	if !stripReasoningFields(payload) {
		return rawBody, nil
	}
	out, err := sonic.Marshal(payload)
	if err != nil {
		return rawBody, err
	}
	return out, nil
}

func stripReasoningFields(payload map[string]any) bool {
	changed := false
	for _, key := range reasoningPayloadKeys {
		if _, ok := payload[key]; ok {
			delete(payload, key)
			changed = true
		}
	}
	return changed
}

func forcedToolChoiceIncompatibleWithThinking(payload map[string]any) bool {
	tc, ok := payload["tool_choice"]
	if !ok || tc == nil {
		return false
	}
	switch v := tc.(type) {
	case string:
		return v == "required"
	case map[string]any:
		return true
	default:
		return false
	}
}
