package attackchain

import (
	"strings"
	"testing"

	"cyberstrike-ai/internal/agent"
	"cyberstrike-ai/internal/config"

	"go.uber.org/zap"
)

func testBuilder(maxTotal int) *Builder {
	return &Builder{
		logger:       zap.NewNop(),
		openAIConfig: &config.OpenAIConfig{Model: "gpt-4"},
		tokenCounter: agent.NewTikTokenCounter(),
		maxTokens:    maxTotal,
	}
}

func TestCompactFormattedToolBodies(t *testing.T) {
	long := strings.Repeat("x", 20000)
	in := "[user]: hi\n\n[tool] (tool_call_id: abc):\n" + long + "\n\n[assistant]: done\n"
	out := compactFormattedToolBodies(in, 500)
	if strings.Contains(out, strings.Repeat("x", 10000)) {
		t.Fatal("expected tool body to be truncated")
	}
	if !strings.Contains(out, "[user]: hi") {
		t.Fatal("expected user header preserved")
	}
	if !strings.Contains(out, "[assistant]: done") {
		t.Fatal("expected assistant header preserved")
	}
}

func TestFitAttackChainPayloadWithinBudget(t *testing.T) {
	b := testBuilder(32000)
	react := strings.Repeat("scan ", 50000)
	model := strings.Repeat("result ", 10000)
	r, m, truncated := b.fitAttackChainPayload(react, model)
	if !truncated {
		t.Fatal("expected truncation for large payload")
	}
	prompt := b.buildSimplePrompt(r, m)
	total := b.countTokens(prompt) + attackChainMaxCompletionTokens(b.maxTokens) + attackChainSystemReserve
	if total > b.maxTokens+attackChainSafetyReserve {
		t.Fatalf("prompt still too large: estimated %d > max %d", total, b.maxTokens)
	}
	_ = m
}

func TestAttackChainMaxCompletionTokens(t *testing.T) {
	if got := attackChainMaxCompletionTokens(120000); got != 15000 && got != 16384 {
		// 120000/8 = 15000
		if got < 4096 || got > 16384 {
			t.Fatalf("unexpected completion cap: %d", got)
		}
	}
	if got := attackChainMaxCompletionTokens(0); got != 8192 {
		t.Fatalf("expected default 8192, got %d", got)
	}
}
