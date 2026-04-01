package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
)

// ---------- Anthropic API types ----------

type anthropicRequest struct {
	Model     string             `json:"model"`
	Messages  []anthropicMessage `json:"messages"`
	System    string             `json:"system,omitempty"`
	MaxTokens int                `json:"max_tokens"`
	Tools     []anthropicTool    `json:"tools,omitempty"`
	Stream    bool               `json:"stream,omitempty"`
}

type anthropicMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // string or []anthropicContentBlock
}

type anthropicContentBlock struct {
	Type      string          `json:"type"`                 // "text", "tool_use", "tool_result"
	Text      string          `json:"text,omitempty"`       // text
	ID        string          `json:"id,omitempty"`         // tool_use
	Name      string          `json:"name,omitempty"`       // tool_use
	Input     json.RawMessage `json:"input,omitempty"`      // tool_use
	ToolUseID string          `json:"tool_use_id,omitempty"` // tool_result
	Content   interface{}     `json:"content,omitempty"`     // tool_result (string or blocks) — will be set explicitly
}

type anthropicTool struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	InputSchema interface{} `json:"input_schema"`
}

type anthropicResponse struct {
	ID         string             `json:"id"`
	Type       string             `json:"type"`
	Content    []anthropicContent `json:"content"`
	StopReason string             `json:"stop_reason"`
	Usage      anthropicUsage     `json:"usage"`
}

type anthropicContent struct {
	Type  string          `json:"type"`            // "text" or "tool_use"
	Text  string          `json:"text,omitempty"`  // text
	ID    string          `json:"id,omitempty"`    // tool_use
	Name  string          `json:"name,omitempty"`  // tool_use
	Input json.RawMessage `json:"input,omitempty"` // tool_use
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// ---------- Anthropic SSE event types ----------

type anthropicSSEMessageStart struct {
	Type    string `json:"type"`
	Message struct {
		ID    string `json:"id"`
		Model string `json:"model"`
		Usage struct {
			InputTokens int `json:"input_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

type anthropicSSEContentBlockStart struct {
	Type         string           `json:"type"`
	Index        int              `json:"index"`
	ContentBlock anthropicContent `json:"content_block"`
}

type anthropicSSEContentBlockDelta struct {
	Type  string `json:"type"`
	Index int    `json:"index"`
	Delta struct {
		Type        string          `json:"type"` // "text_delta" or "input_json_delta"
		Text        string          `json:"text,omitempty"`
		PartialJSON string          `json:"partial_json,omitempty"`
	} `json:"delta"`
}

type anthropicSSEMessageDelta struct {
	Type  string `json:"type"`
	Delta struct {
		StopReason string `json:"stop_reason"`
	} `json:"delta"`
	Usage struct {
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// ---------- Detection ----------

// isAnthropic checks if the config points to Anthropic API.
// Detects by: Provider field set to "anthropic", API key prefix "sk-ant-", or base URL containing "anthropic.com".
func (c *Client) isAnthropic() bool {
	if c.config == nil {
		return false
	}
	if strings.EqualFold(c.config.Provider, "anthropic") {
		return true
	}
	if strings.HasPrefix(c.config.APIKey, "sk-ant-") {
		return true
	}
	if strings.Contains(c.config.BaseURL, "anthropic.com") {
		return true
	}
	return false
}

// ---------- Request conversion ----------

// openaiToAnthropicRequest converts an OpenAI-format payload into an Anthropic Messages API request.
func openaiToAnthropicRequest(payload interface{}) (*anthropicRequest, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	var oai struct {
		Model       string            `json:"model"`
		Messages    []json.RawMessage `json:"messages"`
		Tools       []json.RawMessage `json:"tools,omitempty"`
		Stream      bool              `json:"stream,omitempty"`
		MaxTokens   int               `json:"max_tokens,omitempty"`
		Temperature *float64          `json:"temperature,omitempty"`
	}
	if err := json.Unmarshal(raw, &oai); err != nil {
		return nil, fmt.Errorf("unmarshal openai payload: %w", err)
	}

	req := &anthropicRequest{
		Model:     oai.Model,
		MaxTokens: oai.MaxTokens,
		Stream:    oai.Stream,
	}
	if req.MaxTokens <= 0 {
		req.MaxTokens = 8192
	}

	// Convert messages — extract system messages to top-level field
	var systemParts []string
	for _, rawMsg := range oai.Messages {
		var msg struct {
			Role       string          `json:"role"`
			Content    json.RawMessage `json:"content"`
			ToolCalls  []json.RawMessage `json:"tool_calls,omitempty"`
			ToolCallID string          `json:"tool_call_id,omitempty"`
			Name       string          `json:"name,omitempty"`
		}
		if err := json.Unmarshal(rawMsg, &msg); err != nil {
			continue
		}

		switch msg.Role {
		case "system":
			// Extract system prompt
			var text string
			if err := json.Unmarshal(msg.Content, &text); err != nil {
				// Could be array of content blocks
				text = string(msg.Content)
			}
			if strings.TrimSpace(text) != "" {
				systemParts = append(systemParts, text)
			}

		case "assistant":
			// May contain tool_calls
			var blocks []anthropicContentBlock
			// Try to get text content
			var textContent string
			if json.Unmarshal(msg.Content, &textContent) == nil && textContent != "" {
				blocks = append(blocks, anthropicContentBlock{
					Type: "text",
					Text: textContent,
				})
			}
			// Convert tool_calls
			for _, rawTC := range msg.ToolCalls {
				var tc struct {
					ID       string `json:"id"`
					Type     string `json:"type"`
					Function struct {
						Name      string          `json:"name"`
						Arguments json.RawMessage `json:"arguments"`
					} `json:"function"`
				}
				if err := json.Unmarshal(rawTC, &tc); err != nil {
					continue
				}
				// Arguments may be a string containing JSON
				var argObj json.RawMessage
				var argStr string
				if json.Unmarshal(tc.Function.Arguments, &argStr) == nil {
					// It's a JSON string — parse it
					argObj = json.RawMessage(argStr)
				} else {
					argObj = tc.Function.Arguments
				}
				blocks = append(blocks, anthropicContentBlock{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Function.Name,
					Input: argObj,
				})
			}
			if len(blocks) > 0 {
				req.Messages = append(req.Messages, anthropicMessage{
					Role:    "assistant",
					Content: blocks,
				})
			} else {
				// Plain text assistant message
				req.Messages = append(req.Messages, anthropicMessage{
					Role:    "assistant",
					Content: textContent,
				})
			}

		case "tool":
			// OpenAI tool result → Anthropic user message with tool_result content
			var resultText string
			if json.Unmarshal(msg.Content, &resultText) != nil {
				resultText = string(msg.Content)
			}
			toolResultBlock := map[string]interface{}{
				"type":        "tool_result",
				"tool_use_id": msg.ToolCallID,
				"content":     resultText,
			}
			// Check if the previous message is a user message with tool_result blocks — merge
			if len(req.Messages) > 0 {
				last := &req.Messages[len(req.Messages)-1]
				if last.Role == "user" {
					if existing, ok := last.Content.([]interface{}); ok {
						last.Content = append(existing, toolResultBlock)
						continue
					}
				}
			}
			req.Messages = append(req.Messages, anthropicMessage{
				Role:    "user",
				Content: []interface{}{toolResultBlock},
			})

		case "user":
			var textContent string
			if json.Unmarshal(msg.Content, &textContent) == nil {
				req.Messages = append(req.Messages, anthropicMessage{
					Role:    "user",
					Content: textContent,
				})
			} else {
				// Array content blocks — pass through
				var blocks []interface{}
				if json.Unmarshal(msg.Content, &blocks) == nil {
					req.Messages = append(req.Messages, anthropicMessage{
						Role:    "user",
						Content: blocks,
					})
				} else {
					req.Messages = append(req.Messages, anthropicMessage{
						Role:    "user",
						Content: string(msg.Content),
					})
				}
			}
		}
	}

	if len(systemParts) > 0 {
		req.System = strings.Join(systemParts, "\n\n")
	}

	// Convert tools
	for _, rawTool := range oai.Tools {
		var oaiTool struct {
			Type     string `json:"type"`
			Function struct {
				Name        string      `json:"name"`
				Description string      `json:"description,omitempty"`
				Parameters  interface{} `json:"parameters,omitempty"`
			} `json:"function"`
		}
		if err := json.Unmarshal(rawTool, &oaiTool); err != nil {
			continue
		}
		inputSchema := oaiTool.Function.Parameters
		if inputSchema == nil {
			inputSchema = map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
		}
		req.Tools = append(req.Tools, anthropicTool{
			Name:        oaiTool.Function.Name,
			Description: oaiTool.Function.Description,
			InputSchema: inputSchema,
		})
	}

	return req, nil
}

// ---------- Response conversion ----------

// anthropicResponseToOpenAI converts an Anthropic response to OpenAI ChatCompletion format.
func anthropicResponseToOpenAI(resp *anthropicResponse) (json.RawMessage, error) {
	// Build OpenAI response structure
	var contentParts []string
	var toolCalls []interface{}

	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			contentParts = append(contentParts, block.Text)
		case "tool_use":
			tc := map[string]interface{}{
				"id":   block.ID,
				"type": "function",
				"function": map[string]interface{}{
					"name":      block.Name,
					"arguments": string(block.Input),
				},
			}
			toolCalls = append(toolCalls, tc)
		}
	}

	content := strings.Join(contentParts, "")

	finishReason := "stop"
	if resp.StopReason == "tool_use" {
		finishReason = "tool_calls"
	} else if resp.StopReason == "max_tokens" {
		finishReason = "length"
	}

	message := map[string]interface{}{
		"role":    "assistant",
		"content": content,
	}
	if len(toolCalls) > 0 {
		message["tool_calls"] = toolCalls
	}

	oaiResp := map[string]interface{}{
		"id":      resp.ID,
		"object":  "chat.completion",
		"model":   "",
		"choices": []interface{}{
			map[string]interface{}{
				"index":         0,
				"message":       message,
				"finish_reason": finishReason,
			},
		},
		"usage": map[string]interface{}{
			"prompt_tokens":     resp.Usage.InputTokens,
			"completion_tokens": resp.Usage.OutputTokens,
			"total_tokens":      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		},
	}

	return json.Marshal(oaiResp)
}

// ---------- Non-streaming ----------

// anthropicChatCompletion handles non-streaming calls to Anthropic Messages API.
func (c *Client) anthropicChatCompletion(ctx context.Context, payload interface{}, out interface{}) error {
	anthReq, err := openaiToAnthropicRequest(payload)
	if err != nil {
		return fmt.Errorf("convert to anthropic request: %w", err)
	}
	anthReq.Stream = false

	body, err := json.Marshal(anthReq)
	if err != nil {
		return fmt.Errorf("marshal anthropic request: %w", err)
	}

	baseURL := strings.TrimSuffix(c.config.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://api.anthropic.com/v1"
	}
	// Normalize: if baseURL ends with /v1, use baseURL + /messages; otherwise append /v1/messages
	endpoint := baseURL + "/messages"
	if !strings.Contains(baseURL, "/v1") {
		endpoint = baseURL + "/v1/messages"
	}

	c.logger.Debug("sending Anthropic chat completion request",
		zap.Int("payloadSizeKB", len(body)/1024),
		zap.String("endpoint", endpoint))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build anthropic request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.config.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	// Rate limit
	c.limiter.wait()

	requestStart := time.Now()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("call anthropic api: %w", err)
	}
	defer resp.Body.Close()

	// Auto-retry on 429
	if resp.StatusCode == 429 {
		resp.Body.Close()
		retryAfter := 30 * time.Second
		c.limiter.backoff(retryAfter)
		c.logger.Warn("Anthropic rate limited (429), backing off", zap.Duration("backoff", retryAfter))
		c.limiter.wait()
		req2, _ := http.NewRequestWithContext(ctx, http.MethodPost, req.URL.String(), bytes.NewReader(body))
		req2.Header.Set("Content-Type", "application/json")
		req2.Header.Set("x-api-key", c.config.APIKey)
		req2.Header.Set("anthropic-version", "2023-06-01")
		resp, err = c.httpClient.Do(req2)
		if err != nil {
			return fmt.Errorf("call anthropic api (retry): %w", err)
		}
		defer resp.Body.Close()
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read anthropic response: %w", err)
	}

	c.logger.Debug("received Anthropic response",
		zap.Int("status", resp.StatusCode),
		zap.Duration("duration", time.Since(requestStart)),
		zap.Int("responseSizeKB", len(respBody)/1024),
	)

	if resp.StatusCode != http.StatusOK {
		c.logger.Warn("Anthropic chat completion returned non-200",
			zap.Int("status", resp.StatusCode),
			zap.String("body", string(respBody)),
		)
		return &APIError{
			StatusCode: resp.StatusCode,
			Body:       string(respBody),
		}
	}

	// Parse Anthropic response
	var anthResp anthropicResponse
	if err := json.Unmarshal(respBody, &anthResp); err != nil {
		c.logger.Error("failed to unmarshal Anthropic response",
			zap.Error(err),
			zap.String("body", string(respBody)),
		)
		return fmt.Errorf("unmarshal anthropic response: %w", err)
	}

	// Convert to OpenAI format
	oaiJSON, err := anthropicResponseToOpenAI(&anthResp)
	if err != nil {
		return fmt.Errorf("convert anthropic response to openai format: %w", err)
	}

	if out != nil {
		if err := json.Unmarshal(oaiJSON, out); err != nil {
			return fmt.Errorf("unmarshal converted response: %w", err)
		}
	}

	return nil
}

// ---------- Streaming ----------

// anthropicChatCompletionStream handles streaming calls to Anthropic (content only, no tool calls).
func (c *Client) anthropicChatCompletionStream(ctx context.Context, payload interface{}, onDelta func(string) error) (string, error) {
	content, _, _, err := c.anthropicChatCompletionStreamWithToolCalls(ctx, payload, onDelta)
	return content, err
}

// anthropicChatCompletionStreamWithToolCalls handles streaming with tool call accumulation.
func (c *Client) anthropicChatCompletionStreamWithToolCalls(
	ctx context.Context,
	payload interface{},
	onContentDelta func(string) error,
) (string, []StreamToolCall, string, error) {
	anthReq, err := openaiToAnthropicRequest(payload)
	if err != nil {
		return "", nil, "", fmt.Errorf("convert to anthropic request: %w", err)
	}
	anthReq.Stream = true

	body, err := json.Marshal(anthReq)
	if err != nil {
		return "", nil, "", fmt.Errorf("marshal anthropic request: %w", err)
	}

	baseURL := strings.TrimSuffix(c.config.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://api.anthropic.com/v1"
	}
	endpoint := baseURL + "/messages"
	if !strings.Contains(baseURL, "/v1") {
		endpoint = baseURL + "/v1/messages"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", nil, "", fmt.Errorf("build anthropic request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.config.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	// Rate limit
	c.limiter.wait()

	requestStart := time.Now()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", nil, "", fmt.Errorf("call anthropic api: %w", err)
	}
	defer resp.Body.Close()

	// Auto-retry on 429
	if resp.StatusCode == 429 {
		resp.Body.Close()
		retryAfter := 30 * time.Second
		c.limiter.backoff(retryAfter)
		c.logger.Warn("Anthropic stream rate limited (429), backing off", zap.Duration("backoff", retryAfter))
		c.limiter.wait()
		req2, _ := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		req2.Header.Set("Content-Type", "application/json")
		req2.Header.Set("x-api-key", c.config.APIKey)
		req2.Header.Set("anthropic-version", "2023-06-01")
		resp, err = c.httpClient.Do(req2)
		if err != nil {
			return "", nil, "", fmt.Errorf("call anthropic api (retry): %w", err)
		}
		defer resp.Body.Close()
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", nil, "", &APIError{
			StatusCode: resp.StatusCode,
			Body:       string(respBody),
		}
	}

	// Track content blocks by index
	type toolCallAccum struct {
		id   string
		name string
		args strings.Builder
	}
	blockToolCalls := make(map[int]*toolCallAccum)
	blockTypes := make(map[int]string) // index -> "text" or "tool_use"

	reader := bufio.NewReader(resp.Body)
	var full strings.Builder
	finishReason := ""

	// Anthropic SSE format:
	// event: <event_type>\n
	// data: <json>\n
	// \n
	var currentEvent string

	for {
		line, readErr := reader.ReadString('\n')
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return full.String(), nil, finishReason, fmt.Errorf("read anthropic stream: %w", readErr)
		}

		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Parse event type
		if strings.HasPrefix(trimmed, "event:") {
			currentEvent = strings.TrimSpace(strings.TrimPrefix(trimmed, "event:"))
			continue
		}

		// Parse data
		if !strings.HasPrefix(trimmed, "data:") {
			continue
		}
		dataStr := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
		if dataStr == "[DONE]" {
			break
		}

		switch currentEvent {
		case "message_start":
			// Contains initial message info — skip for now
			continue

		case "content_block_start":
			var blockStart anthropicSSEContentBlockStart
			if err := json.Unmarshal([]byte(dataStr), &blockStart); err != nil {
				continue
			}
			blockTypes[blockStart.Index] = blockStart.ContentBlock.Type
			if blockStart.ContentBlock.Type == "tool_use" {
				blockToolCalls[blockStart.Index] = &toolCallAccum{
					id:   blockStart.ContentBlock.ID,
					name: blockStart.ContentBlock.Name,
				}
			}

		case "content_block_delta":
			var blockDelta anthropicSSEContentBlockDelta
			if err := json.Unmarshal([]byte(dataStr), &blockDelta); err != nil {
				continue
			}
			switch blockDelta.Delta.Type {
			case "text_delta":
				text := blockDelta.Delta.Text
				if text != "" {
					full.WriteString(text)
					if onContentDelta != nil {
						if err := onContentDelta(text); err != nil {
							return full.String(), nil, finishReason, err
						}
					}
				}
			case "input_json_delta":
				if acc, ok := blockToolCalls[blockDelta.Index]; ok {
					acc.args.WriteString(blockDelta.Delta.PartialJSON)
				}
			}

		case "content_block_stop":
			// Block finished — nothing special to do
			continue

		case "message_delta":
			var msgDelta anthropicSSEMessageDelta
			if err := json.Unmarshal([]byte(dataStr), &msgDelta); err != nil {
				continue
			}
			if msgDelta.Delta.StopReason != "" {
				switch msgDelta.Delta.StopReason {
				case "tool_use":
					finishReason = "tool_calls"
				case "max_tokens":
					finishReason = "length"
				default:
					finishReason = "stop"
				}
			}

		case "message_stop":
			// Stream complete
			continue

		case "ping":
			continue

		case "error":
			return full.String(), nil, finishReason, fmt.Errorf("anthropic stream error: %s", dataStr)
		}

		currentEvent = ""
	}

	// Build tool calls in index order
	indices := make([]int, 0, len(blockToolCalls))
	for idx := range blockToolCalls {
		indices = append(indices, idx)
	}
	for i := 0; i < len(indices); i++ {
		for j := i + 1; j < len(indices); j++ {
			if indices[j] < indices[i] {
				indices[i], indices[j] = indices[j], indices[i]
			}
		}
	}

	toolCalls := make([]StreamToolCall, 0, len(indices))
	for _, idx := range indices {
		acc := blockToolCalls[idx]
		toolCalls = append(toolCalls, StreamToolCall{
			Index:           idx,
			ID:              acc.id,
			Type:            "function",
			FunctionName:    acc.name,
			FunctionArgsStr: acc.args.String(),
		})
	}

	c.logger.Debug("received Anthropic stream completion",
		zap.Duration("duration", time.Since(requestStart)),
		zap.Int("contentLen", full.Len()),
		zap.Int("toolCalls", len(toolCalls)),
		zap.String("finishReason", finishReason),
	)

	if strings.TrimSpace(finishReason) == "" {
		finishReason = "stop"
	}

	return full.String(), toolCalls, finishReason, nil
}
