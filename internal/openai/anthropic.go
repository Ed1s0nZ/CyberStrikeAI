package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"go.uber.org/zap"
)

// Anthropic Messages API request/response types

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
	Type      string      `json:"type"`
	Text      string      `json:"text,omitempty"`
	ID        string      `json:"id,omitempty"`
	Name      string      `json:"name,omitempty"`
	Input     interface{} `json:"input,omitempty"`
	ToolUseID string      `json:"tool_use_id,omitempty"`
	Content   string      `json:"content,omitempty"` // for tool_result
}

type anthropicTool struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	InputSchema interface{} `json:"input_schema"`
}

type anthropicResponse struct {
	ID           string                  `json:"id"`
	Type         string                  `json:"type"`
	Role         string                  `json:"role"`
	Model        string                  `json:"model"`
	Content      []anthropicContentBlock `json:"content"`
	StopReason   string                  `json:"stop_reason"`
	StopSequence *string                 `json:"stop_sequence"`
	Usage        *anthropicUsage         `json:"usage,omitempty"`
	Error        *anthropicError         `json:"error,omitempty"`
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type anthropicError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// anthropicChatCompletion translates OpenAI format to Anthropic Messages API,
// calls the Anthropic-compatible endpoint, and translates the response back.
func (c *Client) anthropicChatCompletion(ctx context.Context, payload interface{}, out interface{}) error {
	// Parse the OpenAI-format payload
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	var openAIReq struct {
		Model    string            `json:"model"`
		Messages []json.RawMessage `json:"messages"`
		Tools    []json.RawMessage `json:"tools,omitempty"`
	}
	if err := json.Unmarshal(payloadBytes, &openAIReq); err != nil {
		return fmt.Errorf("parse openai request: %w", err)
	}

	// Convert messages
	var systemPrompt string
	var anthropicMsgs []anthropicMessage

	for _, rawMsg := range openAIReq.Messages {
		var msg struct {
			Role       string          `json:"role"`
			Content    string          `json:"content"`
			ToolCalls  json.RawMessage `json:"tool_calls,omitempty"`
			ToolCallID string          `json:"tool_call_id,omitempty"`
		}
		if err := json.Unmarshal(rawMsg, &msg); err != nil {
			return fmt.Errorf("parse message: %w", err)
		}

		switch msg.Role {
		case "system":
			if systemPrompt != "" {
				systemPrompt += "\n\n"
			}
			systemPrompt += msg.Content

		case "user":
			anthropicMsgs = append(anthropicMsgs, anthropicMessage{
				Role:    "user",
				Content: msg.Content,
			})

		case "assistant":
			if len(msg.ToolCalls) > 0 && string(msg.ToolCalls) != "null" {
				// Assistant message with tool calls → convert to content blocks
				var toolCalls []struct {
					ID       string `json:"id"`
					Type     string `json:"type"`
					Function struct {
						Name      string          `json:"name"`
						Arguments json.RawMessage `json:"arguments"`
					} `json:"function"`
				}
				if err := json.Unmarshal(msg.ToolCalls, &toolCalls); err != nil {
					return fmt.Errorf("parse tool_calls: %w", err)
				}

				var blocks []anthropicContentBlock
				if msg.Content != "" {
					blocks = append(blocks, anthropicContentBlock{
						Type: "text",
						Text: msg.Content,
					})
				}
				for _, tc := range toolCalls {
					var input interface{}
					// Arguments may be a JSON string or already an object
					argStr := string(tc.Function.Arguments)
					if len(argStr) > 0 && argStr[0] == '"' {
						// It's a JSON-encoded string, decode it first
						var s string
						if err := json.Unmarshal(tc.Function.Arguments, &s); err == nil {
							json.Unmarshal([]byte(s), &input)
						}
					}
					if input == nil {
						json.Unmarshal(tc.Function.Arguments, &input)
					}
					if input == nil {
						input = map[string]interface{}{}
					}

					blocks = append(blocks, anthropicContentBlock{
						Type:  "tool_use",
						ID:    tc.ID,
						Name:  tc.Function.Name,
						Input: input,
					})
				}
				anthropicMsgs = append(anthropicMsgs, anthropicMessage{
					Role:    "assistant",
					Content: blocks,
				})
			} else {
				anthropicMsgs = append(anthropicMsgs, anthropicMessage{
					Role:    "assistant",
					Content: msg.Content,
				})
			}

		case "tool":
			// Tool result → merge into the last user message or create a new one
			// Anthropic expects tool_result blocks in a "user" role message
			block := anthropicContentBlock{
				Type:      "tool_result",
				ToolUseID: msg.ToolCallID,
				Content:   msg.Content,
			}

			// Check if last message is already a user message with tool_result blocks
			if len(anthropicMsgs) > 0 {
				last := &anthropicMsgs[len(anthropicMsgs)-1]
				if last.Role == "user" {
					if blocks, ok := last.Content.([]anthropicContentBlock); ok {
						last.Content = append(blocks, block)
						continue
					}
				}
			}

			anthropicMsgs = append(anthropicMsgs, anthropicMessage{
				Role:    "user",
				Content: []anthropicContentBlock{block},
			})
		}
	}

	// Merge consecutive same-role messages (Anthropic requires alternating roles)
	anthropicMsgs = mergeConsecutiveMessages(anthropicMsgs)

	// Convert tools
	var anthropicTools []anthropicTool
	for _, rawTool := range openAIReq.Tools {
		var tool struct {
			Type     string `json:"type"`
			Function struct {
				Name        string      `json:"name"`
				Description string      `json:"description"`
				Parameters  interface{} `json:"parameters"`
			} `json:"function"`
		}
		if err := json.Unmarshal(rawTool, &tool); err != nil {
			continue
		}
		anthropicTools = append(anthropicTools, anthropicTool{
			Name:        tool.Function.Name,
			Description: tool.Function.Description,
			InputSchema: tool.Function.Parameters,
		})
	}

	// Build Anthropic request — streaming mode (original working config).
	// extractToolCallsFromText is a safety net for text-embedded tool calls.
	anthReq := anthropicRequest{
		Model:     openAIReq.Model,
		Messages:  anthropicMsgs,
		System:    systemPrompt,
		MaxTokens: 32768,
		Tools:     anthropicTools,
		Stream:    true,
	}

	body, err := json.Marshal(anthReq)
	if err != nil {
		return fmt.Errorf("marshal anthropic request: %w", err)
	}

	baseURL := strings.TrimSuffix(c.config.BaseURL, "/")
	if baseURL == "" {
		return fmt.Errorf("anthropic base_url is required")
	}

	c.logger.Debug("sending Anthropic streaming request",
		zap.String("url", baseURL+"/v1/messages"),
		zap.Int("payloadSizeKB", len(body)/1024),
		zap.Int("messagesCount", len(anthropicMsgs)),
		zap.Int("toolsCount", len(anthropicTools)),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build anthropic request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	// Support both API key auth and no-auth (local proxy)
	if strings.TrimSpace(c.config.APIKey) != "" {
		req.Header.Set("x-api-key", c.config.APIKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	}

	requestStart := time.Now()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("call anthropic api: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		c.logger.Warn("Anthropic messages returned non-200",
			zap.Int("status", resp.StatusCode),
			zap.String("body", string(respBody)),
		)
		return &APIError{
			StatusCode: resp.StatusCode,
			Body:       string(respBody),
		}
	}

	var anthResp *anthropicResponse

	if anthReq.Stream {
		// Parse SSE stream and assemble the final response
		anthResp, err = c.readAnthropicStream(ctx, resp.Body)
		if err != nil {
			return fmt.Errorf("read anthropic stream: %w", err)
		}
	} else {
		// Non-streaming: read JSON response directly
		respBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return fmt.Errorf("read anthropic response: %w", readErr)
		}
		anthResp = &anthropicResponse{}
		if err := json.Unmarshal(respBody, anthResp); err != nil {
			return fmt.Errorf("unmarshal anthropic response: %w", err)
		}
	}

	// Extract tool calls embedded as text (e.g. "[Tool Call: name({...})]")
	// The model sometimes writes tool calls as text instead of structured blocks.
	anthResp.Content = extractToolCallsFromText(anthResp.Content)

	// Log content block details for debugging
	var textBlocks, toolBlocks int
	for _, b := range anthResp.Content {
		switch b.Type {
		case "text":
			textBlocks++
		case "tool_use":
			toolBlocks++
		}
	}
	c.logger.Info("received Anthropic response",
		zap.Duration("duration", time.Since(requestStart)),
		zap.Bool("streamed", anthReq.Stream),
		zap.Int("contentBlocks", len(anthResp.Content)),
		zap.Int("textBlocks", textBlocks),
		zap.Int("toolBlocks", toolBlocks),
		zap.String("stopReason", anthResp.StopReason),
	)

	if anthResp.Error != nil {
		openAIResp := map[string]interface{}{
			"error": map[string]interface{}{
				"message": anthResp.Error.Message,
				"type":    anthResp.Error.Type,
			},
		}
		result, _ := json.Marshal(openAIResp)
		return json.Unmarshal(result, out)
	}

	// Convert Anthropic response → OpenAI response format
	openAIResp := c.convertAnthropicToOpenAI(anthResp)

	result, err := json.Marshal(openAIResp)
	if err != nil {
		return fmt.Errorf("marshal converted response: %w", err)
	}

	if out != nil {
		if err := json.Unmarshal(result, out); err != nil {
			return fmt.Errorf("unmarshal converted response: %w", err)
		}
	}

	return nil
}

// readAnthropicStream parses an Anthropic SSE stream and assembles the complete response.
// Stream events: message_start, content_block_start, content_block_delta, content_block_stop, message_delta, message_stop
func (c *Client) readAnthropicStream(ctx context.Context, body io.Reader) (*anthropicResponse, error) {
	result := &anthropicResponse{}
	var currentBlockIndex int
	// Track content blocks being built
	blocks := make(map[int]*anthropicContentBlock)
	// Track tool_use input JSON fragments
	toolInputBuffers := make(map[int]*strings.Builder)

	scanner := newSSEScanner(body)
	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("context cancelled: %w", ctx.Err())
		default:
		}

		eventType, data, err := scanner.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read SSE event: %w", err)
		}

		// Some proxies strip the "event:" field and only forward "data:" lines.
		// In that case, extract the event type from the "type" field in the JSON data.
		if eventType == "" && data != "" {
			var typeProbe struct {
				Type string `json:"type"`
			}
			if json.Unmarshal([]byte(data), &typeProbe) == nil && typeProbe.Type != "" {
				eventType = typeProbe.Type
			}
		}

		if eventType != "ping" && eventType != "" {
			c.logger.Debug("SSE event received",
				zap.String("eventType", eventType),
				zap.Int("dataLen", len(data)),
			)
		}

		switch eventType {
		case "message_start":
			// {"type":"message_start","message":{"id":"...","type":"message","role":"assistant",...}}
			var envelope struct {
				Message anthropicResponse `json:"message"`
			}
			if err := json.Unmarshal([]byte(data), &envelope); err == nil {
				result.ID = envelope.Message.ID
				result.Type = envelope.Message.Type
				result.Role = envelope.Message.Role
				result.Model = envelope.Message.Model
				result.Usage = envelope.Message.Usage
			}

		case "content_block_start":
			// {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}
			var event struct {
				Index        int                    `json:"index"`
				ContentBlock anthropicContentBlock  `json:"content_block"`
			}
			if err := json.Unmarshal([]byte(data), &event); err == nil {
				currentBlockIndex = event.Index
				block := event.ContentBlock
				blocks[currentBlockIndex] = &block
				if block.Type == "tool_use" {
					toolInputBuffers[currentBlockIndex] = &strings.Builder{}
				}
			}

		case "content_block_delta":
			// Text: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}
			// Tool: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"..."}}
			var event struct {
				Index int `json:"index"`
				Delta struct {
					Type        string `json:"type"`
					Text        string `json:"text,omitempty"`
					PartialJSON string `json:"partial_json,omitempty"`
				} `json:"delta"`
			}
			if err := json.Unmarshal([]byte(data), &event); err == nil {
				block := blocks[event.Index]
				if block == nil {
					continue
				}
				switch event.Delta.Type {
				case "text_delta":
					block.Text += event.Delta.Text
				case "input_json_delta":
					if buf, ok := toolInputBuffers[event.Index]; ok {
						buf.WriteString(event.Delta.PartialJSON)
					}
				}
			}

		case "content_block_stop":
			// Finalize the block
			var event struct {
				Index int `json:"index"`
			}
			if err := json.Unmarshal([]byte(data), &event); err == nil {
				block := blocks[event.Index]
				if block != nil && block.Type == "tool_use" {
					if buf, ok := toolInputBuffers[event.Index]; ok {
						var input interface{}
						if err := json.Unmarshal([]byte(buf.String()), &input); err == nil {
							block.Input = input
						} else {
							block.Input = map[string]interface{}{}
						}
					}
				}
			}

		case "message_delta":
			// {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":42}}
			var event struct {
				Delta struct {
					StopReason   string  `json:"stop_reason"`
					StopSequence *string `json:"stop_sequence"`
				} `json:"delta"`
				Usage *anthropicUsage `json:"usage,omitempty"`
			}
			if err := json.Unmarshal([]byte(data), &event); err == nil {
				result.StopReason = event.Delta.StopReason
				result.StopSequence = event.Delta.StopSequence
				if event.Usage != nil && result.Usage != nil {
					result.Usage.OutputTokens = event.Usage.OutputTokens
				}
			}

		case "message_stop":
			// Stream complete — assemble content blocks in order
			maxIdx := -1
			for idx := range blocks {
				if idx > maxIdx {
					maxIdx = idx
				}
			}
			for i := 0; i <= maxIdx; i++ {
				if b, ok := blocks[i]; ok {
					result.Content = append(result.Content, *b)
				}
			}
			return result, nil

		case "error":
			var event struct {
				Error anthropicError `json:"error"`
			}
			if err := json.Unmarshal([]byte(data), &event); err == nil {
				result.Error = &event.Error
				return result, nil
			}

		case "ping":
			// Keep-alive, ignore
		}
	}

	// If we got here without message_stop, assemble what we have
	if len(blocks) > 0 && len(result.Content) == 0 {
		maxIdx := -1
		for idx := range blocks {
			if idx > maxIdx {
				maxIdx = idx
			}
		}
		for i := 0; i <= maxIdx; i++ {
			if b, ok := blocks[i]; ok {
				result.Content = append(result.Content, *b)
			}
		}
	}

	return result, nil
}

// sseScanner reads SSE events from a reader
type sseScanner struct {
	reader  *bufio.Reader
}

func newSSEScanner(r io.Reader) *sseScanner {
	return &sseScanner{reader: bufio.NewReader(r)}
}

// Next returns the next SSE event (eventType, data, error).
// Returns io.EOF when the stream ends.
func (s *sseScanner) Next() (string, string, error) {
	eventType := ""
	var dataLines []string

	for {
		line, err := s.reader.ReadString('\n')
		line = strings.TrimRight(line, "\r\n")

		if err != nil {
			if err == io.EOF {
				// If we have accumulated data, return it
				if len(dataLines) > 0 {
					return eventType, strings.Join(dataLines, "\n"), nil
				}
				return "", "", io.EOF
			}
			return "", "", err
		}

		if line == "" {
			// Empty line = end of event
			if len(dataLines) > 0 {
				return eventType, strings.Join(dataLines, "\n"), nil
			}
			continue
		}

		if strings.HasPrefix(line, "event: ") {
			eventType = strings.TrimPrefix(line, "event: ")
		} else if strings.HasPrefix(line, "data: ") {
			dataLines = append(dataLines, strings.TrimPrefix(line, "data: "))
		} else if line == "data:" {
			dataLines = append(dataLines, "")
		}
	}
}

// toolCallMarker matches the start of "[Tool Call: toolName(" pattern.
var toolCallMarker = regexp.MustCompile(`\[Tool Call:\s*(\w[\w-]*)\(`)

// extractToolCallsFromText scans text content blocks for tool calls embedded as text
// (e.g. "[Tool Call: record_vulnerability({...})]") and converts them to proper tool_use blocks.
// Uses brace-balanced extraction for the JSON args to handle nested objects.
func extractToolCallsFromText(blocks []anthropicContentBlock) []anthropicContentBlock {
	var result []anthropicContentBlock
	for _, b := range blocks {
		if b.Type != "text" || b.Text == "" {
			result = append(result, b)
			continue
		}

		matches := toolCallMarker.FindAllStringSubmatchIndex(b.Text, -1)
		if len(matches) == 0 {
			result = append(result, b)
			continue
		}

		lastIdx := 0
		extracted := false
		for _, m := range matches {
			// m[0]:m[1] = full prefix match "[Tool Call: name("
			// m[2]:m[3] = tool name capture group
			toolName := b.Text[m[2]:m[3]]

			// Find the opening brace after "("
			braceStart := -1
			for i := m[1]; i < len(b.Text); i++ {
				if b.Text[i] == '{' {
					braceStart = i
					break
				}
				if b.Text[i] != ' ' && b.Text[i] != '\n' && b.Text[i] != '\t' {
					break // non-whitespace before brace — not a JSON arg
				}
			}
			if braceStart == -1 {
				continue
			}

			// Brace-balanced extraction
			depth := 0
			inStr := false
			esc := false
			braceEnd := -1
			for i := braceStart; i < len(b.Text); i++ {
				ch := b.Text[i]
				if esc {
					esc = false
					continue
				}
				if ch == '\\' && inStr {
					esc = true
					continue
				}
				if ch == '"' {
					inStr = !inStr
					continue
				}
				if inStr {
					continue
				}
				if ch == '{' {
					depth++
				} else if ch == '}' {
					depth--
					if depth == 0 {
						braceEnd = i + 1
						break
					}
				}
			}
			if braceEnd == -1 {
				continue // unbalanced
			}

			argsJSON := b.Text[braceStart:braceEnd]
			var args interface{}
			if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
				continue // invalid JSON
			}

			// Find the closing ")]" after the JSON
			endIdx := braceEnd
			for i := braceEnd; i < len(b.Text); i++ {
				if b.Text[i] == ']' {
					endIdx = i + 1
					break
				}
				if b.Text[i] != ')' && b.Text[i] != ' ' && b.Text[i] != '\n' {
					endIdx = braceEnd
					break
				}
			}

			// Emit text before this tool call
			if m[0] > lastIdx {
				before := strings.TrimSpace(b.Text[lastIdx:m[0]])
				if before != "" {
					result = append(result, anthropicContentBlock{Type: "text", Text: before})
				}
			}

			result = append(result, anthropicContentBlock{
				Type:  "tool_use",
				ID:    fmt.Sprintf("extracted_%s_%d", toolName, len(result)),
				Name:  toolName,
				Input: args,
			})
			lastIdx = endIdx
			extracted = true
		}

		if !extracted {
			result = append(result, b)
			continue
		}

		if lastIdx < len(b.Text) {
			remaining := strings.TrimSpace(b.Text[lastIdx:])
			if remaining != "" {
				result = append(result, anthropicContentBlock{Type: "text", Text: remaining})
			}
		}
	}
	return result
}

// convertAnthropicToOpenAI converts an Anthropic Messages response to OpenAI ChatCompletion format.
func (c *Client) convertAnthropicToOpenAI(resp *anthropicResponse) map[string]interface{} {
	var textContent string
	var toolCalls []map[string]interface{}

	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			if textContent != "" {
				textContent += "\n"
			}
			textContent += block.Text
		case "tool_use":
			inputJSON, _ := json.Marshal(block.Input)
			toolCalls = append(toolCalls, map[string]interface{}{
				"id":   block.ID,
				"type": "function",
				"function": map[string]interface{}{
					"name":      block.Name,
					"arguments": string(inputJSON),
				},
			})
		}
	}

	// Map stop_reason to finish_reason
	finishReason := "stop"
	switch resp.StopReason {
	case "end_turn":
		finishReason = "stop"
	case "max_tokens":
		finishReason = "length"
	case "tool_use":
		finishReason = "tool_calls"
	case "stop_sequence":
		finishReason = "stop"
	}

	message := map[string]interface{}{
		"role":    "assistant",
		"content": textContent,
	}
	if len(toolCalls) > 0 {
		message["tool_calls"] = toolCalls
	}

	result := map[string]interface{}{
		"id": resp.ID,
		"choices": []map[string]interface{}{
			{
				"message":       message,
				"finish_reason": finishReason,
			},
		},
	}

	return result
}

// mergeConsecutiveMessages merges consecutive messages with the same role
// since Anthropic requires strictly alternating user/assistant messages.
func mergeConsecutiveMessages(msgs []anthropicMessage) []anthropicMessage {
	if len(msgs) <= 1 {
		return msgs
	}

	var merged []anthropicMessage
	for _, msg := range msgs {
		if len(merged) > 0 && merged[len(merged)-1].Role == msg.Role {
			last := &merged[len(merged)-1]
			lastBlocks := toContentBlocks(last.Content)
			newBlocks := toContentBlocks(msg.Content)
			last.Content = append(lastBlocks, newBlocks...)
		} else {
			merged = append(merged, msg)
		}
	}
	return merged
}

// toContentBlocks converts message content (string or []anthropicContentBlock) to blocks.
func toContentBlocks(content interface{}) []anthropicContentBlock {
	switch v := content.(type) {
	case []anthropicContentBlock:
		return v
	case string:
		if v == "" {
			return nil
		}
		return []anthropicContentBlock{{Type: "text", Text: v}}
	default:
		return nil
	}
}
