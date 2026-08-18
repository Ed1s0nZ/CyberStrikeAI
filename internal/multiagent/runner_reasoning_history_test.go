package multiagent

import (
	"strings"
	"testing"
	"unicode/utf8"

	"cyberstrike-ai/internal/agent"
	"cyberstrike-ai/internal/config"
)

func TestHistoryToMessagesPreservesReasoningContent(t *testing.T) {
	h := []agent.ChatMessage{
		{Role: "user", Content: "u"},
		{Role: "assistant", Content: "c", ReasoningContent: "r1", ToolCalls: []agent.ToolCall{{ID: "t1", Type: "function", Function: agent.FunctionCall{Name: "f", Arguments: map[string]interface{}{}}}}},
	}
	msgs := historyToMessages(h, nil, nil)
	if len(msgs) != 2 {
		t.Fatalf("len=%d", len(msgs))
	}
	am := msgs[1]
	if am.ReasoningContent != "r1" || am.Content != "c" {
		t.Fatalf("got reasoning=%q content=%q", am.ReasoningContent, am.Content)
	}
}

func TestHistoryToMessagesNormalizesLegacyRawToolOutput(t *testing.T) {
	mw := &config.MultiAgentEinoMiddlewareConfig{ReductionMaxLengthForTrunc: 128}
	h := []agent.ChatMessage{
		{Role: "assistant", ToolCalls: []agent.ToolCall{{ID: "t1", Type: "function", Function: agent.FunctionCall{Name: "http-framework-test"}}}},
		{Role: "tool", ToolCallID: "t1", ToolName: "http-framework-test", Content: strings.Repeat("响应正文", 1000)},
	}
	msgs := historyToMessages(h, nil, mw)
	if len(msgs) != 2 {
		t.Fatalf("len=%d", len(msgs))
	}
	if len(msgs[1].Content) > 128 {
		t.Fatalf("normalized tool bytes=%d, want <=128", len(msgs[1].Content))
	}
	if !strings.Contains(msgs[1].Content, "legacy tool output discarded") {
		t.Fatalf("missing migration marker: %q", msgs[1].Content)
	}
}

func TestHistoryToMessagesRestoresModelFacingTraceByteForByte(t *testing.T) {
	mw := &config.MultiAgentEinoMiddlewareConfig{
		ReductionMaxLengthForTrunc: 4096,
		LatestUserMessageMaxRunes:  64,
	}
	userContent := strings.Repeat("model-facing-user-", 100)
	toolContent := strings.Repeat("model-facing-tool-", 100)
	h := []agent.ChatMessage{
		{Role: "user", Content: userContent, ModelFacingTrace: true},
		{Role: "assistant", ModelFacingTrace: true, ToolCalls: []agent.ToolCall{{ID: "t1", Type: "function", Function: agent.FunctionCall{Name: "http-framework-test"}}}},
		{Role: "tool", ToolCallID: "t1", ToolName: "http-framework-test", Content: toolContent, ModelFacingTrace: true},
	}
	msgs := historyToMessages(h, nil, mw)
	if len(msgs) != 3 {
		t.Fatalf("len=%d", len(msgs))
	}
	if msgs[0].Content != userContent {
		t.Fatal("model-facing user content changed during restore")
	}
	if msgs[2].Content != toolContent {
		t.Fatal("model-facing tool content changed during restore")
	}
}

func TestHistoryToMessagesCapsOversizedModelFacingToolTrace(t *testing.T) {
	mw := &config.MultiAgentEinoMiddlewareConfig{ReductionMaxLengthForTrunc: 128}
	h := []agent.ChatMessage{
		{Role: "assistant", ModelFacingTrace: true, ToolCalls: []agent.ToolCall{{ID: "t1", Type: "function", Function: agent.FunctionCall{Name: "exec"}}}},
		{Role: "tool", ToolCallID: "t1", ToolName: "exec", Content: strings.Repeat("model-facing-tool-", 1000), ModelFacingTrace: true},
	}
	msgs := historyToMessages(h, nil, mw)
	if len(msgs) != 2 {
		t.Fatalf("len=%d", len(msgs))
	}
	if len(msgs[1].Content) > 128 {
		t.Fatalf("restored model-facing tool bytes=%d, want <=128", len(msgs[1].Content))
	}
	if !strings.Contains(msgs[1].Content, "legacy tool output discarded") {
		t.Fatalf("missing cap marker: %q", msgs[1].Content)
	}
}

func TestHistoryToMessagesNeverReinjectsRawOversizedUserFallback(t *testing.T) {
	appCfg := &config.Config{OpenAI: config.OpenAIConfig{MaxTotalTokens: 10000}}
	mw := &config.MultiAgentEinoMiddlewareConfig{LatestUserMessageMaxRunes: 8000}
	h := []agent.ChatMessage{{Role: "user", Content: strings.Repeat("原始用户输入", 2000)}}
	msgs := historyToMessages(h, appCfg, mw)
	if len(msgs) != 1 {
		t.Fatalf("len=%d", len(msgs))
	}
	if got := utf8.RuneCountInString(msgs[0].Content); got > 2000 {
		t.Fatalf("restored user runes=%d, want <=2000", got)
	}
	if !strings.Contains(msgs[0].Content, "historical user input normalized") {
		t.Fatalf("missing normalization marker: %q", msgs[0].Content)
	}
}
