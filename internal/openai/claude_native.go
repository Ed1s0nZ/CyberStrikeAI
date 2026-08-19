package openai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"cyberstrike-ai/internal/llm"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type claudeNativePayload struct {
	Model               string                `json:"model"`
	Messages            []claudeNativeMessage `json:"messages"`
	Temperature         *float32              `json:"temperature,omitempty"`
	TopP                *float32              `json:"top_p,omitempty"`
	MaxCompletionTokens int                   `json:"max_completion_tokens,omitempty"`
	MaxTokens           int                   `json:"max_tokens,omitempty"`
	Thinking            any                   `json:"thinking,omitempty"`
	OutputConfig        any                   `json:"output_config,omitempty"`
}

type claudeNativeMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

func (c *Client) isClaude() bool {
	return c != nil && c.config != nil && llm.IsClaudeProvider(c.config.Provider)
}

func (c *Client) claudeNativeChatCompletion(ctx context.Context, payload, out any) error {
	req, err := c.parseClaudeNativePayload(payload)
	if err != nil {
		return err
	}
	nativeModel, err := llm.NewClaudeAgenticModel(
		ctx,
		*c.config,
		c.httpClient,
		req.maxTokens(c.config.MaxCompletionTokensEffective()),
		req.extraFields(),
	)
	if err != nil {
		return fmt.Errorf("create native Claude model: %w", err)
	}
	resp, err := nativeModel.Generate(ctx, req.agenticMessages(), req.options()...)
	if err != nil {
		return fmt.Errorf("native Claude generate: %w", err)
	}
	return marshalClaudeNativeResponse(resp, req.model(c.config.Model), out)
}

func (c *Client) claudeNativeChatCompletionStream(
	ctx context.Context,
	payload any,
	onDelta func(delta string) error,
) (string, error) {
	req, err := c.parseClaudeNativePayload(payload)
	if err != nil {
		return "", err
	}
	nativeModel, err := llm.NewClaudeAgenticModel(
		ctx,
		*c.config,
		c.httpClient,
		req.maxTokens(c.config.MaxCompletionTokensEffective()),
		req.extraFields(),
	)
	if err != nil {
		return "", fmt.Errorf("create native Claude model: %w", err)
	}
	stream, err := nativeModel.Stream(ctx, req.agenticMessages(), req.options()...)
	if err != nil {
		return "", fmt.Errorf("native Claude stream: %w", err)
	}
	defer stream.Close()

	var full strings.Builder
	for {
		chunk, recvErr := stream.Recv()
		if errors.Is(recvErr, io.EOF) {
			return full.String(), nil
		}
		if recvErr != nil {
			return full.String(), fmt.Errorf("native Claude stream receive: %w", recvErr)
		}
		content, _ := llm.AgenticText(chunk)
		if content == "" {
			continue
		}
		full.WriteString(content)
		if onDelta != nil {
			if err := onDelta(content); err != nil {
				return full.String(), err
			}
		}
	}
}

func (c *Client) parseClaudeNativePayload(payload any) (*claudeNativePayload, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal Claude payload: %w", err)
	}
	var req claudeNativePayload
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, fmt.Errorf("unmarshal Claude payload: %w", err)
	}
	if strings.TrimSpace(req.model(c.config.Model)) == "" {
		return nil, fmt.Errorf("native Claude model is empty")
	}
	if len(req.Messages) == 0 {
		return nil, fmt.Errorf("native Claude messages are empty")
	}
	return &req, nil
}

func (p *claudeNativePayload) model(fallback string) string {
	if modelName := strings.TrimSpace(p.Model); modelName != "" {
		return modelName
	}
	return strings.TrimSpace(fallback)
}

func (p *claudeNativePayload) maxTokens(fallback int) int {
	if p.MaxCompletionTokens > 0 {
		return p.MaxCompletionTokens
	}
	if p.MaxTokens > 0 {
		return p.MaxTokens
	}
	return fallback
}

func (p *claudeNativePayload) extraFields() map[string]any {
	fields := make(map[string]any, 2)
	if p.Thinking != nil {
		fields["thinking"] = p.Thinking
	}
	if p.OutputConfig != nil {
		fields["output_config"] = p.OutputConfig
	}
	if len(fields) == 0 {
		return nil
	}
	return fields
}

func (p *claudeNativePayload) options() []model.Option {
	opts := make([]model.Option, 0, 3)
	if p.Temperature != nil {
		opts = append(opts, model.WithTemperature(*p.Temperature))
	}
	if p.TopP != nil {
		opts = append(opts, model.WithTopP(*p.TopP))
	}
	if p.MaxCompletionTokens > 0 || p.MaxTokens > 0 {
		opts = append(opts, model.WithMaxTokens(p.maxTokens(0)))
	}
	return opts
}

func (p *claudeNativePayload) agenticMessages() []*schema.AgenticMessage {
	out := make([]*schema.AgenticMessage, 0, len(p.Messages))
	for _, msg := range p.Messages {
		text := claudeNativeTextContent(msg.Content)
		role := schema.AgenticRoleTypeUser
		var block *schema.ContentBlock
		switch strings.ToLower(strings.TrimSpace(msg.Role)) {
		case "system":
			role = schema.AgenticRoleTypeSystem
			block = schema.NewContentBlock(&schema.UserInputText{Text: text})
		case "assistant":
			role = schema.AgenticRoleTypeAssistant
			block = schema.NewContentBlock(&schema.AssistantGenText{Text: text})
		default:
			block = schema.NewContentBlock(&schema.UserInputText{Text: text})
		}
		out = append(out, &schema.AgenticMessage{
			Role:          role,
			ContentBlocks: []*schema.ContentBlock{block},
		})
	}
	return out
}

func claudeNativeTextContent(raw json.RawMessage) string {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &parts); err == nil {
		var out strings.Builder
		for _, part := range parts {
			if part.Type == "" || part.Type == "text" {
				out.WriteString(part.Text)
			}
		}
		return out.String()
	}
	return strings.TrimSpace(string(raw))
}

func marshalClaudeNativeResponse(resp *schema.AgenticMessage, modelName string, out any) error {
	if out == nil {
		return nil
	}
	content, reasoning := llm.AgenticText(resp)
	id := ""
	if resp != nil && resp.ResponseMeta != nil && resp.ResponseMeta.ClaudeExtension != nil {
		id = resp.ResponseMeta.ClaudeExtension.ID
	}
	wire := map[string]any{
		"id":     id,
		"object": "chat.completion",
		"model":  modelName,
		"choices": []any{map[string]any{
			"index": 0,
			"message": map[string]any{
				"role":              "assistant",
				"content":           content,
				"reasoning_content": reasoning,
			},
			"finish_reason": "stop",
		}},
	}
	raw, err := json.Marshal(wire)
	if err != nil {
		return fmt.Errorf("marshal native Claude response: %w", err)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("unmarshal native Claude response: %w", err)
	}
	return nil
}
