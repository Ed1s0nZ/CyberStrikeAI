package reasoning

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"cyberstrike-ai/internal/config"

	einoopenai "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/schema"
)

var reasoningPayloadKeysForTest = []string{"thinking", "reasoning_effort", "output_config", "reasoning"}

func assertNoReasoningFields(t *testing.T, cfg *einoopenai.ChatModelConfig) {
	t.Helper()
	if cfg.ReasoningEffort != "" {
		t.Fatalf("expected ReasoningEffort omitted, got %q", cfg.ReasoningEffort)
	}
	for _, key := range reasoningPayloadKeysForTest {
		if _, ok := cfg.ExtraFields[key]; ok {
			t.Fatalf("expected %q omitted, got %#v", key, cfg.ExtraFields)
		}
	}
}

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

func TestAgenticOpenAIExtraFields_openAICompatReasoningEffort(t *testing.T) {
	oa := &config.OpenAIConfig{
		Reasoning: config.OpenAIReasoningConfig{
			Profile: "openai_compat",
			Mode:    "on",
			Effort:  "high",
			ExtraRequestFields: map[string]interface{}{
				"vendor_option": true,
			},
		},
	}
	got := AgenticOpenAIExtraFields(oa, nil)
	if got["reasoning_effort"] != "high" {
		t.Fatalf("reasoning_effort=%#v, want high in %#v", got["reasoning_effort"], got)
	}
	if got["vendor_option"] != true {
		t.Fatalf("vendor option not preserved: %#v", got)
	}
}

func TestAgenticOpenAIExtraFields_reasoningOffPreservesUnrelatedFields(t *testing.T) {
	oa := &config.OpenAIConfig{
		Model: "gpt-4o-mini",
		Reasoning: config.OpenAIReasoningConfig{
			Profile: "openai_compat",
			Mode:    "off",
			Effort:  "high",
			ExtraRequestFields: map[string]interface{}{
				"reasoning_effort": "high",
				"thinking":         map[string]any{"type": "enabled"},
				"vendor_option":    true,
			},
		},
	}
	got := AgenticOpenAIExtraFields(oa, nil)
	for _, key := range reasoningPayloadKeysForTest {
		if _, ok := got[key]; ok {
			t.Fatalf("agentic fields unexpectedly contain %q: %#v", key, got)
		}
	}
	if got["vendor_option"] != true {
		t.Fatalf("vendor option not preserved: %#v", got)
	}
}

func TestAgenticOpenAIPlannerExtraFields_deepseekDisablesThinking(t *testing.T) {
	oa := &config.OpenAIConfig{
		BaseURL: "https://api.deepseek.com",
		Model:   "deepseek-chat",
		Reasoning: config.OpenAIReasoningConfig{
			Profile: "auto",
			Mode:    "on",
			ExtraRequestFields: map[string]interface{}{
				"reasoning_effort": "high",
				"vendor_option":    true,
			},
		},
	}
	got := AgenticOpenAIPlannerExtraFields(oa)
	if got["reasoning_effort"] != nil {
		t.Fatalf("planner should strip reasoning_effort: %#v", got)
	}
	thinking, ok := got["thinking"].(map[string]any)
	if !ok || thinking["type"] != "disabled" {
		t.Fatalf("expected deepseek thinking disabled, got %#v", got)
	}
	if got["vendor_option"] != true {
		t.Fatalf("vendor option not preserved: %#v", got)
	}
}

func TestAgenticOpenAIPlannerExtraFields_openAIProfileWinsOverDeepseekEndpoint(t *testing.T) {
	oa := &config.OpenAIConfig{
		BaseURL: "https://api.deepseek.com/v1",
		Model:   "deepseek-v4-flash",
		Reasoning: config.OpenAIReasoningConfig{
			Profile: "openai_compat",
			Mode:    "on",
			Effort:  "high",
			ExtraRequestFields: map[string]interface{}{
				"reasoning_effort": "high",
				"vendor_option":    true,
			},
		},
	}
	got := AgenticOpenAIPlannerExtraFields(oa)
	for _, key := range reasoningPayloadKeysForTest {
		if _, ok := got[key]; ok {
			t.Fatalf("planner fields unexpectedly contain %q: %#v", key, got)
		}
	}
	if got["vendor_option"] != true {
		t.Fatalf("vendor option not preserved: %#v", got)
	}
}

func TestApplyPlanExecutePlannerModelConfig_stripsReasoningWhenGlobalOn(t *testing.T) {
	cfg := &einoopenai.ChatModelConfig{ExtraFields: map[string]any{
		"thinking":         map[string]any{"type": "enabled"},
		"reasoning_effort": "high",
		"vendor_option":    true,
	}}
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
	assertNoReasoningFields(t, cfg)
	if cfg.ExtraFields["vendor_option"] != true {
		t.Fatalf("expected unrelated extra field preserved, got %#v", cfg.ExtraFields)
	}
}

func TestApplyPlanExecutePlannerModelConfig_openAIProfileWinsOverDeepseekEndpoint(t *testing.T) {
	cfg := &einoopenai.ChatModelConfig{ExtraFields: map[string]any{
		"thinking":         map[string]any{"type": "enabled"},
		"reasoning_effort": "high",
		"vendor_option":    true,
	}}
	oa := &config.OpenAIConfig{
		BaseURL: "https://api.deepseek.com/v1",
		Model:   "deepseek-v4-flash",
		Reasoning: config.OpenAIReasoningConfig{
			Profile: "openai_compat",
			Mode:    "on",
			Effort:  "high",
		},
	}
	ApplyPlanExecutePlannerModelConfig(cfg, oa)
	assertNoReasoningFields(t, cfg)
	if cfg.ExtraFields["vendor_option"] != true {
		t.Fatalf("expected unrelated extra field preserved, got %#v", cfg.ExtraFields)
	}
}

func TestApplyReasoningOff_omitsAllReasoningFields(t *testing.T) {
	cfg := &einoopenai.ChatModelConfig{ExtraFields: map[string]any{
		"thinking":      map[string]any{"type": "enabled"},
		"output_config": map[string]any{"effort": "high"},
	}}
	oa := &config.OpenAIConfig{
		BaseURL: "https://api.openai.com/v1",
		Model:   "gpt-4o-mini",
		Reasoning: config.OpenAIReasoningConfig{
			Mode:    "off",
			Effort:  "high",
			Profile: "openai_compat",
			ExtraRequestFields: map[string]interface{}{
				"thinking":      map[string]any{"type": "disabled"},
				"reasoning":     map[string]any{"effort": "high"},
				"vendor_option": true,
			},
		},
	}
	ApplyToEinoChatModelConfig(cfg, oa, nil)
	assertNoReasoningFields(t, cfg)
	if cfg.ExtraFields["vendor_option"] != true {
		t.Fatalf("expected unrelated extra field preserved, got %#v", cfg.ExtraFields)
	}
}

func TestApplyReasoningOff_openAICompatDeepseekModelOmitsAllReasoningFields(t *testing.T) {
	allowClient := false
	cfg := &einoopenai.ChatModelConfig{ExtraFields: map[string]any{
		"thinking":         map[string]any{"type": "enabled"},
		"reasoning_effort": "high",
	}}
	oa := &config.OpenAIConfig{
		Provider: "openai_compatible",
		BaseURL:  "http://your-gateway:port/v1",
		Model:    "deepseek-v4-flash-0731",
		Reasoning: config.OpenAIReasoningConfig{
			Mode:                 "off",
			Effort:               "high",
			Profile:              "openai_compat",
			AllowClientReasoning: &allowClient,
			ExtraRequestFields: map[string]interface{}{
				"thinking":      map[string]any{"type": "disabled"},
				"output_config": map[string]any{"effort": "high"},
				"vendor_option": true,
			},
		},
	}
	ApplyToEinoChatModelConfig(cfg, oa, nil)
	assertNoReasoningFields(t, cfg)
	if cfg.ExtraFields["vendor_option"] != true {
		t.Fatalf("expected unrelated extra field preserved, got %#v", cfg.ExtraFields)
	}
}

func TestAgenticOpenAIExtraFields_openAICompatDeepseekModelOmitsAllReasoningFields(t *testing.T) {
	oa := &config.OpenAIConfig{
		Provider: "openai_compatible",
		BaseURL:  "http://your-gateway:port/v1",
		Model:    "deepseek-v4-flash-0731",
		Reasoning: config.OpenAIReasoningConfig{
			Mode:    "off",
			Effort:  "high",
			Profile: "openai_compat",
			ExtraRequestFields: map[string]interface{}{
				"thinking":         map[string]any{"type": "disabled"},
				"reasoning_effort": "high",
				"vendor_option":    true,
			},
		},
	}
	got := AgenticOpenAIExtraFields(oa, nil)
	for _, key := range reasoningPayloadKeysForTest {
		if _, ok := got[key]; ok {
			t.Fatalf("agentic fields unexpectedly contain %q: %#v", key, got)
		}
	}
	if got["vendor_option"] != true {
		t.Fatalf("vendor option not preserved: %#v", got)
	}
}

func TestAgenticOpenAIPlannerExtraFields_openAICompatDeepseekModelOmitsAllReasoningFields(t *testing.T) {
	oa := &config.OpenAIConfig{
		Provider: "openai_compatible",
		BaseURL:  "http://your-gateway:port/v1",
		Model:    "deepseek-v4-flash-0731",
		Reasoning: config.OpenAIReasoningConfig{
			Mode:    "on",
			Effort:  "high",
			Profile: "openai_compat",
			ExtraRequestFields: map[string]interface{}{
				"thinking":         map[string]any{"type": "enabled"},
				"reasoning_effort": "high",
				"vendor_option":    true,
			},
		},
	}
	got := AgenticOpenAIPlannerExtraFields(oa)
	for _, key := range reasoningPayloadKeysForTest {
		if _, ok := got[key]; ok {
			t.Fatalf("planner fields unexpectedly contain %q: %#v", key, got)
		}
	}
	if got["vendor_option"] != true {
		t.Fatalf("vendor option not preserved: %#v", got)
	}
}

func TestApplyReasoningOff_clientOverrideOmit(t *testing.T) {
	cfg := &einoopenai.ChatModelConfig{}
	oa := &config.OpenAIConfig{Reasoning: config.OpenAIReasoningConfig{
		Mode: "on", Effort: "high", Profile: "openai_compat",
	}}
	ApplyToEinoChatModelConfig(cfg, oa, &ClientIntent{Mode: "off", Effort: "high"})
	assertNoReasoningFields(t, cfg)
}

func TestApplyReasoningOff_deepseekExplicitlyDisablesDefaultThinking(t *testing.T) {
	for _, profile := range []string{"deepseek_compat", "auto"} {
		t.Run(profile, func(t *testing.T) {
			cfg := &einoopenai.ChatModelConfig{ExtraFields: map[string]any{
				"reasoning_effort": "high",
				"vendor_option":    true,
			}}
			oa := &config.OpenAIConfig{
				BaseURL: "https://api.deepseek.com",
				Model:   "deepseek-v4-pro",
				Reasoning: config.OpenAIReasoningConfig{
					Mode: "off", Effort: "high", Profile: profile,
				},
			}
			ApplyToEinoChatModelConfig(cfg, oa, nil)
			if cfg.ReasoningEffort != "" {
				t.Fatalf("expected ReasoningEffort omitted, got %q", cfg.ReasoningEffort)
			}
			if _, ok := cfg.ExtraFields["reasoning_effort"]; ok {
				t.Fatalf("expected reasoning_effort omitted, got %#v", cfg.ExtraFields)
			}
			thinking, ok := cfg.ExtraFields["thinking"].(map[string]any)
			if !ok || thinking["type"] != "disabled" {
				t.Fatalf("expected DeepSeek thinking disabled, got %#v", cfg.ExtraFields)
			}
			if cfg.ExtraFields["vendor_option"] != true {
				t.Fatalf("expected unrelated extra field preserved, got %#v", cfg.ExtraFields)
			}
		})
	}
}

func TestApplyReasoningOff_openAIProfileWinsOverDeepseekEndpoint(t *testing.T) {
	cfg := &einoopenai.ChatModelConfig{ExtraFields: map[string]any{
		"reasoning_effort": "high",
		"vendor_option":    true,
	}}
	oa := &config.OpenAIConfig{
		BaseURL: "https://api.deepseek.com",
		Model:   "deepseek-v4-pro",
		Reasoning: config.OpenAIReasoningConfig{
			Mode: "off", Effort: "high", Profile: "openai_compat",
		},
	}
	ApplyToEinoChatModelConfig(cfg, oa, nil)
	assertNoReasoningFields(t, cfg)
	if cfg.ExtraFields["vendor_option"] != true {
		t.Fatalf("expected unrelated extra field preserved, got %#v", cfg.ExtraFields)
	}
}

func TestApplyReasoningOff_wirePayloadOmitsThinking(t *testing.T) {
	var requestBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}
		if err := json.Unmarshal(body, &requestBody); err != nil {
			t.Errorf("decode request body: %v; body=%s", err, body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"chatcmpl-test","object":"chat.completion","created":1,"model":"gpt-4o-mini","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
	}))
	defer srv.Close()

	cfg := &einoopenai.ChatModelConfig{
		APIKey:  "test-key",
		BaseURL: srv.URL,
		Model:   "gpt-4o-mini",
	}
	oa := &config.OpenAIConfig{
		BaseURL: "https://api.openai.com/v1",
		Model:   "gpt-4o-mini",
		Reasoning: config.OpenAIReasoningConfig{
			Mode: "off", Effort: "high", Profile: "openai_compat",
		},
	}
	ApplyToEinoChatModelConfig(cfg, oa, nil)
	model, err := einoopenai.NewChatModel(context.Background(), cfg)
	if err != nil {
		t.Fatalf("new chat model: %v", err)
	}
	if _, err := model.Generate(context.Background(), []*schema.Message{schema.UserMessage("hello")}); err != nil {
		t.Fatalf("generate: %v", err)
	}
	for _, key := range reasoningPayloadKeysForTest {
		if _, ok := requestBody[key]; ok {
			t.Fatalf("wire payload unexpectedly contains %q: %#v", key, requestBody)
		}
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
