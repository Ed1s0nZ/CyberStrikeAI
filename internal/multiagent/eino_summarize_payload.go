package multiagent

import (
	"strings"

	"cyberstrike-ai/internal/config"
	copenai "cyberstrike-ai/internal/openai"
)

// stripReasoningFromSummarizationPayload removes thinking / reasoning fields from a
// chat-completions JSON body. Applied only to summarization Generate calls via
// model.ModelOptions on the shared ChatModel — main-agent requests are unchanged.
func stripReasoningFromSummarizationPayload(rawBody []byte, oa *config.OpenAIConfig) ([]byte, error) {
	if shouldDisableDeepSeekThinkingForSummarization(oa) {
		return copenai.DisableThinkingForChatCompletionBody(rawBody)
	}
	return copenai.StripReasoningFromChatCompletionBody(rawBody)
}

func shouldDisableDeepSeekThinkingForSummarization(oa *config.OpenAIConfig) bool {
	if oa == nil {
		return false
	}
	if oa.IsDeepSeekEndpointOrModel() {
		return true
	}
	profile := strings.ToLower(strings.TrimSpace(oa.Reasoning.ProfileEffective()))
	switch profile {
	case "deepseek", "deepseek_compat":
		return true
	case "", "auto":
		return false
	default:
		return false
	}
}
