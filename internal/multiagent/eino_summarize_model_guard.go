package multiagent

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type einoSummarizationModelError struct {
	err error
}

func newEinoSummarizationModelError(err error) error {
	if err == nil {
		return nil
	}
	var existing *einoSummarizationModelError
	if errors.As(err, &existing) {
		return err
	}
	return &einoSummarizationModelError{err: err}
}

func (e *einoSummarizationModelError) Error() string {
	if e == nil || e.err == nil {
		return "summarization model error"
	}
	return "summarization model error: " + e.err.Error()
}

func (e *einoSummarizationModelError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func einoSummarizationModelRawErrorText(err error) (string, bool) {
	var summaryErr *einoSummarizationModelError
	if !errors.As(err, &summaryErr) || summaryErr == nil || summaryErr.err == nil {
		return "", false
	}
	return strings.TrimSpace(summaryErr.err.Error()), true
}

type nonEmptySummaryChatModel struct {
	base model.BaseChatModel
}

func newNonEmptySummaryChatModel(base model.BaseChatModel) model.BaseChatModel {
	return &nonEmptySummaryChatModel{base: base}
}

func (m *nonEmptySummaryChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	out, err := m.base.Generate(ctx, input, opts...)
	if err != nil {
		return out, newEinoSummarizationModelError(err)
	}
	if strings.TrimSpace(classicAssistantTextContent(out)) == "" {
		return out, newEinoSummarizationModelError(fmt.Errorf("summary content is empty: %s", classicSummaryEmptyDiagnostics(out)))
	}
	return out, nil
}

func (m *nonEmptySummaryChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return m.base.Stream(ctx, input, opts...)
}

type nonEmptyAgenticSummaryModel struct {
	base model.BaseModel[*schema.AgenticMessage]
}

func newNonEmptyAgenticSummaryModel(base model.BaseModel[*schema.AgenticMessage]) model.BaseModel[*schema.AgenticMessage] {
	return &nonEmptyAgenticSummaryModel{base: base}
}

func (m *nonEmptyAgenticSummaryModel) Generate(ctx context.Context, input []*schema.AgenticMessage, opts ...model.Option) (*schema.AgenticMessage, error) {
	out, err := m.base.Generate(ctx, input, opts...)
	if err != nil {
		return out, newEinoSummarizationModelError(err)
	}
	if strings.TrimSpace(agenticAssistantTextContent(out)) == "" {
		return out, newEinoSummarizationModelError(fmt.Errorf("summary content is empty: %s", agenticSummaryEmptyDiagnostics(out)))
	}
	return out, nil
}

func (m *nonEmptyAgenticSummaryModel) Stream(ctx context.Context, input []*schema.AgenticMessage, opts ...model.Option) (*schema.StreamReader[*schema.AgenticMessage], error) {
	return m.base.Stream(ctx, input, opts...)
}

func classicAssistantTextContent(msg *schema.Message) string {
	if msg == nil || msg.Role != schema.Assistant {
		return ""
	}
	parts := make([]string, 0, len(msg.AssistantGenMultiContent))
	for _, part := range msg.AssistantGenMultiContent {
		if part.Type == schema.ChatMessagePartTypeText && part.Text != "" {
			parts = append(parts, part.Text)
		}
	}
	if len(parts) > 0 {
		return strings.Join(parts, "\n")
	}
	return msg.Content
}

func agenticAssistantTextContent(msg *schema.AgenticMessage) string {
	if msg == nil || msg.Role != schema.AgenticRoleTypeAssistant {
		return ""
	}
	parts := make([]string, 0, len(msg.ContentBlocks))
	for _, block := range msg.ContentBlocks {
		if block != nil && block.AssistantGenText != nil {
			parts = append(parts, block.AssistantGenText.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func classicSummaryEmptyDiagnostics(msg *schema.Message) string {
	if msg == nil {
		return "model returned nil message"
	}
	var textParts, reasoningParts, otherParts int
	var multiTextRunes, multiReasoningRunes int
	for _, part := range msg.AssistantGenMultiContent {
		switch part.Type {
		case schema.ChatMessagePartTypeText:
			textParts++
			multiTextRunes += len([]rune(strings.TrimSpace(part.Text)))
		case schema.ChatMessagePartTypeReasoning:
			reasoningParts++
			if part.Reasoning != nil {
				multiReasoningRunes += len([]rune(strings.TrimSpace(part.Reasoning.Text)))
			}
		default:
			otherParts++
		}
	}
	reasoningRunes := len([]rune(strings.TrimSpace(msg.ReasoningContent))) + multiReasoningRunes
	fields := []string{
		fmt.Sprintf("role=%s", msg.Role),
		fmt.Sprintf("content_runes=%d", len([]rune(strings.TrimSpace(msg.Content)))+multiTextRunes),
		fmt.Sprintf("reasoning_runes=%d", reasoningRunes),
		fmt.Sprintf("text_parts=%d", textParts),
		fmt.Sprintf("reasoning_parts=%d", reasoningParts),
		fmt.Sprintf("other_parts=%d", otherParts),
		fmt.Sprintf("tool_calls=%d", len(msg.ToolCalls)),
	}
	if msg.ResponseMeta != nil {
		fields = append(fields, fmt.Sprintf("finish_reason=%q", msg.ResponseMeta.FinishReason))
		if usage := msg.ResponseMeta.Usage; usage != nil {
			fields = append(fields,
				fmt.Sprintf("prompt_tokens=%d", usage.PromptTokens),
				fmt.Sprintf("completion_tokens=%d", usage.CompletionTokens),
				fmt.Sprintf("total_tokens=%d", usage.TotalTokens),
				fmt.Sprintf("reasoning_tokens=%d", usage.CompletionTokensDetails.ReasoningTokens),
			)
		}
	}
	if reasoningRunes > 0 {
		fields = append(fields, "hint=模型返回了 reasoning_content 但没有返回可作为摘要正文的 content；请检查 DeepSeek thinking 是否已在摘要请求中关闭")
	}
	return strings.Join(fields, " ")
}

func agenticSummaryEmptyDiagnostics(msg *schema.AgenticMessage) string {
	if msg == nil {
		return "model returned nil agentic message"
	}
	var textBlocks, reasoningBlocks, otherBlocks int
	var textRunes, reasoningRunes int
	for _, block := range msg.ContentBlocks {
		if block == nil {
			continue
		}
		switch {
		case block.AssistantGenText != nil:
			textBlocks++
			textRunes += len([]rune(strings.TrimSpace(block.AssistantGenText.Text)))
		case block.Reasoning != nil:
			reasoningBlocks++
			reasoningRunes += len([]rune(strings.TrimSpace(block.Reasoning.Text)))
		default:
			otherBlocks++
		}
	}
	fields := []string{
		fmt.Sprintf("role=%s", msg.Role),
		fmt.Sprintf("content_runes=%d", textRunes),
		fmt.Sprintf("reasoning_runes=%d", reasoningRunes),
		fmt.Sprintf("text_blocks=%d", textBlocks),
		fmt.Sprintf("reasoning_blocks=%d", reasoningBlocks),
		fmt.Sprintf("other_blocks=%d", otherBlocks),
	}
	if msg.ResponseMeta != nil && msg.ResponseMeta.TokenUsage != nil {
		usage := msg.ResponseMeta.TokenUsage
		fields = append(fields,
			fmt.Sprintf("prompt_tokens=%d", usage.PromptTokens),
			fmt.Sprintf("completion_tokens=%d", usage.CompletionTokens),
			fmt.Sprintf("total_tokens=%d", usage.TotalTokens),
			fmt.Sprintf("reasoning_tokens=%d", usage.CompletionTokensDetails.ReasoningTokens),
		)
	}
	if reasoningRunes > 0 {
		fields = append(fields, "hint=模型返回了 reasoning block 但没有返回可作为摘要正文的 text block；请检查 DeepSeek thinking 是否已在摘要请求中关闭")
	}
	return strings.Join(fields, " ")
}
