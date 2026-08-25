package multiagent

import (
	"context"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type guardClassicSummaryModel struct {
	out *schema.Message
}

func (m *guardClassicSummaryModel) Generate(context.Context, []*schema.Message, ...model.Option) (*schema.Message, error) {
	return m.out, nil
}

func (m *guardClassicSummaryModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return schema.StreamReaderFromArray([]*schema.Message{m.out}), nil
}

func TestNonEmptySummaryChatModelReportsEmptyContentDiagnostics(t *testing.T) {
	msg := schema.AssistantMessage("", nil)
	msg.ReasoningContent = "只返回了思考，没有最终摘要"
	msg.ResponseMeta = &schema.ResponseMeta{
		FinishReason: "stop",
		Usage: &schema.TokenUsage{
			PromptTokens:     10,
			CompletionTokens: 3,
			TotalTokens:      13,
			CompletionTokensDetails: schema.CompletionTokensDetails{
				ReasoningTokens: 3,
			},
		},
	}
	_, err := newNonEmptySummaryChatModel(&guardClassicSummaryModel{out: msg}).Generate(context.Background(), nil)
	if err == nil {
		t.Fatal("expected empty summary error")
	}
	text := err.Error()
	for _, want := range []string{
		"summary content is empty",
		"reasoning_runes=",
		`finish_reason="stop"`,
		"reasoning_tokens=3",
		"DeepSeek thinking",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("error missing %q:\n%s", want, text)
		}
	}
}

type guardAgenticSummaryModel struct {
	out *schema.AgenticMessage
}

func (m *guardAgenticSummaryModel) Generate(context.Context, []*schema.AgenticMessage, ...model.Option) (*schema.AgenticMessage, error) {
	return m.out, nil
}

func (m *guardAgenticSummaryModel) Stream(context.Context, []*schema.AgenticMessage, ...model.Option) (*schema.StreamReader[*schema.AgenticMessage], error) {
	return schema.StreamReaderFromArray([]*schema.AgenticMessage{m.out}), nil
}

func TestNonEmptyAgenticSummaryModelReportsEmptyContentDiagnostics(t *testing.T) {
	msg := &schema.AgenticMessage{
		Role: schema.AgenticRoleTypeAssistant,
		ContentBlocks: []*schema.ContentBlock{
			schema.NewContentBlock(&schema.Reasoning{Text: "只返回了思考，没有最终摘要"}),
		},
		ResponseMeta: &schema.AgenticResponseMeta{
			TokenUsage: &schema.TokenUsage{
				PromptTokens:     10,
				CompletionTokens: 3,
				TotalTokens:      13,
				CompletionTokensDetails: schema.CompletionTokensDetails{
					ReasoningTokens: 3,
				},
			},
		},
	}
	_, err := newNonEmptyAgenticSummaryModel(&guardAgenticSummaryModel{out: msg}).Generate(context.Background(), nil)
	if err == nil {
		t.Fatal("expected empty summary error")
	}
	text := err.Error()
	for _, want := range []string{
		"summary content is empty",
		"reasoning_runes=",
		"reasoning_blocks=1",
		"reasoning_tokens=3",
		"DeepSeek thinking",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("error missing %q:\n%s", want, text)
		}
	}
}
