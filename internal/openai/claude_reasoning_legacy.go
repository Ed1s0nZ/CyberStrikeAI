package openai

import "strings"

// claudeReasoningRoundTripSep is retained only to render historical traces
// written by the removed OpenAI-to-Claude HTTP bridge.
const claudeReasoningRoundTripSep = "\n---CSAI_CLAUDE_THINKING_BLOCKS---\n"

// DisplayReasoningContent strips the obsolete bridge metadata suffix from
// historical records. Native AgenticMessage reasoning does not add this suffix.
func DisplayReasoningContent(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	i := strings.LastIndex(s, claudeReasoningRoundTripSep)
	if i < 0 {
		return s
	}
	return strings.TrimSpace(s[:i])
}
