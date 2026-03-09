package claude

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"go.uber.org/zap"
)

func skipIfNoClaude(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("claude"); err != nil {
		t.Skip("claude binary not found in PATH, skipping integration test")
	}
}

func TestCLIClient_SendPrompt_Hello(t *testing.T) {
	skipIfNoClaude(t)

	logger := zap.NewNop()
	client := NewCLIClient(logger)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	result, err := client.SendPrompt(ctx, "Say hello in one short sentence.", nil)
	if err != nil {
		t.Fatalf("SendPrompt failed: %v", err)
	}

	t.Logf("Type:          %s", result.Type)
	t.Logf("Subtype:       %s", result.Subtype)
	t.Logf("Result:        %s", result.Result)
	t.Logf("SessionID:     %s", result.SessionID)
	t.Logf("CostUSD:       %f", result.CostUSD)
	t.Logf("TotalCostUSD:  %f", result.TotalCostUSD)
	t.Logf("DurationMs:    %f", result.DurationMs)
	t.Logf("DurationAPIMs: %f", result.DurationAPIMs)
	t.Logf("NumTurns:      %d", result.NumTurns)
	t.Logf("IsError:       %v", result.IsError)
	t.Logf("InputTokens:   %d", result.Usage.InputTokens)
	t.Logf("OutputTokens:  %d", result.Usage.OutputTokens)
	t.Logf("CacheRead:     %d", result.Usage.CacheReadInputTokens)
	t.Logf("CacheCreation: %d", result.Usage.CacheCreationInputTokens)

	if result.Type == "" {
		t.Errorf("expected non-empty Type")
	}
	if result.Result == "" {
		t.Errorf("expected non-empty Result")
	}
	if result.SessionID == "" {
		t.Errorf("expected non-empty SessionID")
	}
	if result.IsError {
		t.Errorf("expected IsError to be false")
	}
	if result.Usage.InputTokens <= 0 {
		t.Errorf("expected InputTokens > 0, got %d", result.Usage.InputTokens)
	}
	if result.Usage.OutputTokens <= 0 {
		t.Errorf("expected OutputTokens > 0, got %d", result.Usage.OutputTokens)
	}
	if result.CostUSD <= 0 && result.TotalCostUSD <= 0 {
		t.Errorf("expected CostUSD or TotalCostUSD > 0, got cost=%f total=%f", result.CostUSD, result.TotalCostUSD)
	}
	if result.DurationMs <= 0 {
		t.Errorf("expected DurationMs > 0, got %f", result.DurationMs)
	}
}

func TestCLIClient_SendPrompt_WithOptions(t *testing.T) {
	skipIfNoClaude(t)

	logger := zap.NewNop()
	client := NewCLIClient(logger)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	opts := &PromptOptions{
		SystemPrompt: "You are a helpful assistant. Always respond in exactly one sentence.",
		MaxTurns:     1,
	}

	result, err := client.SendPrompt(ctx, "What is 2+2?", opts)
	if err != nil {
		t.Fatalf("SendPrompt with options failed: %v", err)
	}

	t.Logf("Result: %s", result.Result)
	t.Logf("SessionID: %s", result.SessionID)
	t.Logf("NumTurns: %d", result.NumTurns)

	if result.Result == "" {
		t.Errorf("expected non-empty Result")
	}
	if result.SessionID == "" {
		t.Errorf("expected non-empty SessionID")
	}
}

func TestCLIClient_BuildArgs(t *testing.T) {
	logger := zap.NewNop()
	client := NewCLIClient(logger)

	tests := []struct {
		name     string
		prompt   string
		opts     *PromptOptions
		expected []string
	}{
		{
			name:     "no options",
			prompt:   "hello",
			opts:     nil,
			expected: []string{"-p", "hello", "--output-format", "json"},
		},
		{
			name:   "all options",
			prompt: "test prompt",
			opts: &PromptOptions{
				SystemPrompt: "be helpful",
				SessionID:    "abc-123",
				MaxTurns:     5,
				AllowedTools: []string{"Read", "Write"},
			},
			expected: []string{
				"-p", "test prompt", "--output-format", "json",
				"--system-prompt", "be helpful",
				"--resume", "abc-123",
				"--max-turns", "5",
				"--allowedTools", "Read,Write",
			},
		},
		{
			name:   "partial options",
			prompt: "partial",
			opts: &PromptOptions{
				MaxTurns: 3,
			},
			expected: []string{"-p", "partial", "--output-format", "json", "--max-turns", "3"},
		},
		{
			name:   "empty options struct",
			prompt: "empty",
			opts:   &PromptOptions{},
			expected: []string{"-p", "empty", "--output-format", "json"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := client.buildArgs(tt.prompt, tt.opts)
			if len(got) != len(tt.expected) {
				t.Fatalf("expected %d args, got %d: %v", len(tt.expected), len(got), got)
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("arg[%d]: expected %q, got %q", i, tt.expected[i], got[i])
				}
			}
		})
	}
}
