package multiagent

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

func TestIsEinoContextOverflowError(t *testing.T) {
	t.Parallel()
	cases := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{errors.New("context length exceeded"), true},
		{errors.New("maximum context length"), true},
		{errors.New("input is too long for model"), true},
		{errors.New("HTTP 429 Too Many Requests"), false},
		{errors.New("invalid api key"), false},
	}
	for _, tc := range cases {
		if got := isEinoContextOverflowError(tc.err); got != tc.want {
			t.Fatalf("isEinoContextOverflowError(%v) = %v, want %v", tc.err, got, tc.want)
		}
	}
}

func TestTruncateRoundMessagesToTokenBudget(t *testing.T) {
	huge := strings.Repeat("x", 8000)
	round := messageRound{messages: []adk.Message{
		assistantToolCallsMsg("", "c1"),
		schema.ToolMessage(huge, "c1"),
	}}
	out, err := truncateRoundMessagesToTokenBudget(
		context.Background(), round, 256, einoSummarizationTokenCounter("gpt-4o"), 512, "",
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, msg := range out {
		if msg != nil && msg.Role == schema.Tool && len(msg.Content) >= len(huge) {
			t.Fatalf("expected truncated tool output, got len=%d", len(msg.Content))
		}
	}
}

func TestBuildBudgetedSummarizationModelInputTruncatesOversizedLatestRound(t *testing.T) {
	huge := strings.Repeat("x", 8000)
	msgs := []adk.Message{
		assistantToolCallsMsg("", "call-latest"),
		schema.ToolMessage(huge, "call-latest"),
	}
	counter := einoSummarizationTokenCounter("gpt-4o")
	input, dropped, err := buildBudgetedSummarizationModelInput(
		context.Background(),
		schema.SystemMessage("sys"),
		schema.UserMessage("instr"),
		msgs,
		counter,
		512,
		summarizationInputBudgetOpts{toolMaxBytes: 256},
	)
	if err != nil {
		t.Fatal(err)
	}
	if dropped != 0 {
		t.Fatalf("expected no dropped rounds, got %d", dropped)
	}
	toolContent := ""
	for _, msg := range input {
		if msg != nil && msg.Role == schema.Tool {
			toolContent = msg.Content
		}
	}
	if len(toolContent) >= len(huge) {
		t.Fatalf("expected oversized tool output to be compacted, got len=%d", len(toolContent))
	}
}

func TestModelInputSoftBudgetNeverErrors(t *testing.T) {
	mw := &modelInputSoftBudgetMiddleware{
		maxTokens:    4,
		toolMaxBytes: 16,
		counter:      fixedTokenCounter(4),
		phase:        "test",
	}
	state := &adk.ChatModelAgentState{Messages: []adk.Message{
		schema.UserMessage("u"),
		assistantToolCallsMsg("", "c1"),
		schema.ToolMessage(strings.Repeat("t", 200), "c1"),
	}}
	_, out, err := mw.BeforeModelRewriteState(context.Background(), state, nil)
	if err != nil {
		t.Fatalf("soft budget must not error: %v", err)
	}
	if out == nil || len(out.Messages) == 0 {
		t.Fatal("expected compacted messages")
	}
}
