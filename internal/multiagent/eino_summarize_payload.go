package multiagent

import (
	copenai "cyberstrike-ai/internal/openai"
)

// stripReasoningFromSummarizationPayload removes thinking / reasoning fields from a
// chat-completions JSON body. Applied only to summarization Generate calls via
// model.ModelOptions on the shared ChatModel — main-agent requests are unchanged.
func stripReasoningFromSummarizationPayload(rawBody []byte) ([]byte, error) {
	return copenai.StripReasoningFromChatCompletionBody(rawBody)
}
