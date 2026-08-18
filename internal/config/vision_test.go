package config

import "testing"

func TestVisionConfig_OpenAICfgEffective_fallbackToMain(t *testing.T) {
	main := OpenAIConfig{
		APIKey:   "main-key",
		BaseURL:  "https://main.example/v1",
		Model:    "main-model",
		Provider: "openai",
	}
	v := VisionConfig{Model: "qwen-vl-max"}
	out := v.OpenAICfgEffective(main)
	if out.APIKey != main.APIKey || out.BaseURL != main.BaseURL || out.Provider != main.Provider {
		t.Fatalf("expected openai fallback, got key=%q url=%q provider=%q", out.APIKey, out.BaseURL, out.Provider)
	}
	if out.Model != "qwen-vl-max" {
		t.Fatalf("model: %s", out.Model)
	}
}

func TestVisionConfig_OpenAICfgEffective(t *testing.T) {
	main := OpenAIConfig{
		APIKey:  "main-key",
		BaseURL: "https://main.example/v1",
		Model:   "main-model",
		Provider: "openai",
		Reasoning: OpenAIReasoningConfig{Mode: "on"},
	}
	v := VisionConfig{
		Model:    "vl-model",
		APIKey:   "vl-key",
		BaseURL:  "https://vl.example/v1",
		Provider: "claude",
	}
	out := v.OpenAICfgEffective(main)
	if out.APIKey != "vl-key" || out.BaseURL != "https://vl.example/v1" || out.Model != "vl-model" {
		t.Fatalf("unexpected merge: %+v", out)
	}
	if out.Provider != "claude" {
		t.Fatalf("provider: %s", out.Provider)
	}
	if out.Reasoning.Mode != "off" {
		t.Fatalf("reasoning should be off for vision, got %s", out.Reasoning.Mode)
	}
}

func TestVisionConfig_Ready(t *testing.T) {
	if (VisionConfig{Enabled: true, Model: "x"}).Ready() != true {
		t.Fatal("expected ready")
	}
	if (VisionConfig{Enabled: true}).Ready() != false {
		t.Fatal("expected not ready without model")
	}
}
