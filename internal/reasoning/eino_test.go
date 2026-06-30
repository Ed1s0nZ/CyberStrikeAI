package reasoning

import (
	"testing"

	"cyberstrike-ai/internal/config"

	einoopenai "github.com/cloudwego/eino-ext/components/model/openai"
)

func TestEffortStringForAPI_passthrough(t *testing.T) {
	cases := map[string]string{
		"max":    "max",
		"xhigh":  "xhigh",
		"HIGH":   "high",
		"Medium": "medium",
	}
	for in, want := range cases {
		if got := effortStringForAPI(in); got != want {
			t.Fatalf("%q -> %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeEffort_maxAndXhigh(t *testing.T) {
	if normalizeEffort("xhigh") != "xhigh" {
		t.Fatal("xhigh not accepted")
	}
	if normalizeEffort("max") != "max" {
		t.Fatal("max not accepted")
	}
}

func TestApplyOpenAICompat_xhighExtraField(t *testing.T) {
	cfg := &einoopenai.ChatModelConfig{}
	oa := &config.OpenAIConfig{
		Reasoning: config.OpenAIReasoningConfig{
			Profile: "openai_compat",
			Mode:    "on",
			Effort:  "xhigh",
		},
	}
	ApplyToEinoChatModelConfig(cfg, oa, nil)
	if cfg.ExtraFields == nil {
		t.Fatal("expected ExtraFields")
	}
	if got, _ := cfg.ExtraFields["reasoning_effort"].(string); got != "xhigh" {
		t.Fatalf("reasoning_effort=%q", got)
	}
}

func TestApplyPlanExecutePlannerModelConfig_stripsReasoningWhenGlobalOn(t *testing.T) {
	cfg := &einoopenai.ChatModelConfig{}
	oa := &config.OpenAIConfig{
		BaseURL: "https://antchat.example.com/v1",
		Model:   "minimax-m3",
		Reasoning: config.OpenAIReasoningConfig{
			Profile: "openai_compat",
			Mode:    "on",
			Effort:  "high",
		},
	}
	ApplyPlanExecutePlannerModelConfig(cfg, oa)
	if cfg.ReasoningEffort != "" {
		t.Fatalf("expected ReasoningEffort cleared, got %q", cfg.ReasoningEffort)
	}
	th, ok := cfg.ExtraFields["thinking"].(map[string]any)
	if !ok || th["type"] != "disabled" {
		t.Fatalf("expected thinking disabled, got %#v", cfg.ExtraFields)
	}
	if _, ok := cfg.ExtraFields["reasoning_effort"]; ok {
		t.Fatalf("expected reasoning_effort stripped, got %#v", cfg.ExtraFields)
	}
}

func TestApplyReasoningOff_disablesThinking(t *testing.T) {
	cfg := &einoopenai.ChatModelConfig{}
	oa := &config.OpenAIConfig{
		BaseURL: "https://api.openai.com/v1",
		Model:   "gpt-4o",
		Reasoning: config.OpenAIReasoningConfig{
			Mode: "off",
		},
	}
	ApplyToEinoChatModelConfig(cfg, oa, nil)
	th, ok := cfg.ExtraFields["thinking"].(map[string]any)
	if !ok || th["type"] != "disabled" {
		t.Fatalf("expected thinking disabled, got %#v", cfg.ExtraFields)
	}
}

func TestApplyOpenAICompat_maxPassthrough(t *testing.T) {
	cfg := &einoopenai.ChatModelConfig{}
	oa := &config.OpenAIConfig{
		Reasoning: config.OpenAIReasoningConfig{
			Profile: "openai_compat",
			Mode:    "on",
			Effort:  "max",
		},
	}
	ApplyToEinoChatModelConfig(cfg, oa, nil)
	got, _ := cfg.ExtraFields["reasoning_effort"].(string)
	if got != "max" {
		t.Fatalf("max effort wire=%q, want max", got)
	}
}

func TestApplyClaude_adaptiveOutputConfigEffort(t *testing.T) {
	cfg := &einoopenai.ChatModelConfig{}
	oa := &config.OpenAIConfig{
		Provider: "claude",
		Model:    "claude-opus-4-8",
		Reasoning: config.OpenAIReasoningConfig{
			Mode:   "on",
			Effort: "xhigh",
		},
	}
	ApplyToEinoChatModelConfig(cfg, oa, nil)
	th, ok := cfg.ExtraFields["thinking"].(map[string]any)
	if !ok || th["type"] != "adaptive" {
		t.Fatalf("thinking=%#v", cfg.ExtraFields["thinking"])
	}
	oc, ok := cfg.ExtraFields["output_config"].(map[string]any)
	if !ok {
		t.Fatal("expected output_config")
	}
	if oc["effort"] != "xhigh" {
		t.Fatalf("effort=%v", oc["effort"])
	}
}

func TestApplyClaude_sonnet37OfficialBudget(t *testing.T) {
	cfg := &einoopenai.ChatModelConfig{}
	oa := &config.OpenAIConfig{
		Provider: "claude",
		Model:    "claude-3-7-sonnet-latest",
		Reasoning: config.OpenAIReasoningConfig{
			Mode:   "on",
			Effort: "low", // 3.7 has no output_config.effort; effort is not mapped to budget_tokens
		},
	}
	ApplyToEinoChatModelConfig(cfg, oa, nil)
	th, ok := cfg.ExtraFields["thinking"].(map[string]any)
	if !ok || th["type"] != "enabled" {
		t.Fatalf("thinking=%#v", cfg.ExtraFields["thinking"])
	}
	if th["budget_tokens"] != claudeSonnet37DefaultBudgetTokens {
		t.Fatalf("budget_tokens=%v, want official example %d", th["budget_tokens"], claudeSonnet37DefaultBudgetTokens)
	}
	if _, hasOC := cfg.ExtraFields["output_config"]; hasOC {
		t.Fatal("sonnet 3.7 should not set output_config")
	}
}

func TestApplyClaude_onWithoutEffortOmitsOutputConfig(t *testing.T) {
	cfg := &einoopenai.ChatModelConfig{}
	oa := &config.OpenAIConfig{
		Provider: "claude",
		Model:    "claude-sonnet-4-6",
		Reasoning: config.OpenAIReasoningConfig{
			Mode: "on",
		},
	}
	ApplyToEinoChatModelConfig(cfg, oa, nil)
	if _, hasOC := cfg.ExtraFields["output_config"]; hasOC {
		t.Fatal("on without explicit effort should omit output_config (API default high)")
	}
}

func TestApplyClaude_autoWithoutEffortSkipsOutputConfig(t *testing.T) {
	cfg := &einoopenai.ChatModelConfig{}
	oa := &config.OpenAIConfig{
		Provider: "claude",
		Model:    "claude-sonnet-4-6",
		Reasoning: config.OpenAIReasoningConfig{
			Mode: "auto",
		},
	}
	ApplyToEinoChatModelConfig(cfg, oa, nil)
	if _, hasOC := cfg.ExtraFields["output_config"]; hasOC {
		t.Fatal("auto without effort should omit output_config")
	}
}
