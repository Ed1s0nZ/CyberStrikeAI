package multiagent

import (
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestEinoRunUsageAccumulatorSumsModelCalls(t *testing.T) {
	acc := newEinoRunUsageAccumulator()
	acc.AddUsage(&schema.TokenUsage{
		PromptTokens:     10,
		CompletionTokens: 4,
		TotalTokens:      14,
		PromptTokenDetails: schema.PromptTokenDetails{
			CachedTokens: 3,
		},
		CompletionTokensDetails: schema.CompletionTokensDetails{
			ReasoningTokens: 2,
		},
	})
	msg := schema.AssistantMessage("ok", nil)
	msg.ResponseMeta = &schema.ResponseMeta{Usage: &schema.TokenUsage{
		PromptTokens:     7,
		CompletionTokens: 5,
		TotalTokens:      12,
		CompletionTokensDetails: schema.CompletionTokensDetails{
			ReasoningTokens: 1,
		},
	}}
	acc.AddMessage(msg)

	got := acc.Summary()
	if got.ModelCalls != 2 || got.PromptTokens != 17 || got.CompletionTokens != 9 || got.TotalTokens != 26 || got.CachedTokens != 3 || got.ReasoningTokens != 3 {
		t.Fatalf("summary = %#v", got)
	}
}

func TestEinoRunUsageAccumulatorEmitOnce(t *testing.T) {
	acc := newEinoRunUsageAccumulator()
	acc.AddUsage(&schema.TokenUsage{PromptTokens: 1, CompletionTokens: 2, TotalTokens: 3})
	var events []map[string]interface{}
	progress := func(eventType, _ string, data interface{}) {
		if eventType != "eino_usage_summary" {
			return
		}
		if m, ok := data.(map[string]interface{}); ok {
			events = append(events, m)
		}
	}

	if !acc.EmitOnce("conv-1", "deep", "final", "gpt-test", progress, nil) {
		t.Fatal("first emit should return true")
	}
	if acc.EmitOnce("conv-1", "deep", "partial", "gpt-test", progress, nil) {
		t.Fatal("second emit should return false")
	}
	if len(events) != 1 {
		t.Fatalf("events = %#v, want one usage summary", events)
	}
	if events[0]["conversationId"] != "conv-1" || events[0]["orchestration"] != "deep" || events[0]["reason"] != "final" || events[0]["model"] != "gpt-test" || events[0]["totalTokens"] != 3 {
		t.Fatalf("event = %#v", events[0])
	}
}

func TestMaxEinoTokenUsageUsesLargestStreamChunkValues(t *testing.T) {
	var got *schema.TokenUsage
	got = maxEinoTokenUsage(got, &schema.TokenUsage{PromptTokens: 10, CompletionTokens: 2, TotalTokens: 12})
	got = maxEinoTokenUsage(got, &schema.TokenUsage{
		PromptTokens:     9,
		CompletionTokens: 5,
		TotalTokens:      14,
		CompletionTokensDetails: schema.CompletionTokensDetails{
			ReasoningTokens: 3,
		},
	})

	if got.PromptTokens != 10 || got.CompletionTokens != 5 || got.TotalTokens != 14 || got.CompletionTokensDetails.ReasoningTokens != 3 {
		t.Fatalf("usage = %#v", got)
	}
}
