package openai

import (
	"encoding/json"
	"strings"
)

// claudeReasoningRoundTripSep separates human-readable reasoning from a JSON payload of
// Anthropic thinking blocks (with signatures) for multi-turn extended thinking + tools.
// Not shown in UI (see DisplayReasoningContent).
const claudeReasoningRoundTripSep = "\n---CSAI_CLAUDE_THINKING_BLOCKS---\n"

// DisplayReasoningContent returns reasoning text suitable for the UI (strips internal
// Claude round-trip JSON suffix). Safe for DeepSeek/plain reasoning strings (no-op).
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

func appendClaudeReasoningRoundTrip(display string, blocks []claudeContentBlock) string {
	var payload []map[string]string
	for _, b := range blocks {
		if b.Type != "thinking" {
			continue
		}
		payload = append(payload, map[string]string{
			"type":      b.Type,
			"thinking":  b.Thinking,
			"signature": b.Signature,
		})
	}
	if len(payload) == 0 {
		return strings.TrimSpace(display)
	}
	js, err := json.Marshal(payload)
	if err != nil {
		return strings.TrimSpace(display)
	}
	d := strings.TrimSpace(display)
	if d == "" {
		return claudeReasoningRoundTripSep + string(js)
	}
	return d + claudeReasoningRoundTripSep + string(js)
}

// parseClaudeReasoningAssistantBlocks extracts Anthropic thinking blocks from an OpenAI-style
// reasoning_content string. When no suffix is present, blocks is nil (caller must not invent signatures).
func parseClaudeReasoningAssistantBlocks(reasoningContent string) (display string, blocks []claudeContentBlock) {
	reasoningContent = strings.TrimSpace(reasoningContent)
	if reasoningContent == "" {
		return "", nil
	}
	idx := strings.LastIndex(reasoningContent, claudeReasoningRoundTripSep)
	if idx < 0 {
		return reasoningContent, nil
	}
	display = strings.TrimSpace(reasoningContent[:idx])
	jsonPart := strings.TrimSpace(reasoningContent[idx+len(claudeReasoningRoundTripSep):])
	var arr []struct {
		Type      string `json:"type"`
		Thinking  string `json:"thinking"`
		Signature string `json:"signature"`
	}
	if err := json.Unmarshal([]byte(jsonPart), &arr); err != nil {
		return reasoningContent, nil
	}
	for _, x := range arr {
		if x.Type != "thinking" {
			continue
		}
		blocks = append(blocks, claudeContentBlock{Type: "thinking", Thinking: x.Thinking, Signature: x.Signature})
	}
	return display, blocks
}
