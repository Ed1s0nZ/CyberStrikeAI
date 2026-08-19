package openai

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"cyberstrike-ai/internal/config"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestChatCompletionUsesNativeClaudeMessagesAPI(t *testing.T) {
	t.Parallel()
	httpClient := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/messages" {
			t.Fatalf("request path = %q, want /v1/messages", req.URL.Path)
		}
		if req.Header.Get("x-api-key") != "test-key" {
			t.Fatalf("x-api-key header = %q", req.Header.Get("x-api-key"))
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body: io.NopCloser(strings.NewReader(`{
				"id":"msg_1",
				"type":"message",
				"role":"assistant",
				"model":"claude-test",
				"content":[{"type":"text","text":"native ok"}],
				"stop_reason":"end_turn",
				"usage":{"input_tokens":1,"output_tokens":2}
			}`)),
			Request: req,
		}, nil
	})}
	client := NewClient(&config.OpenAIConfig{
		Provider: "claude",
		BaseURL:  "https://example.test",
		APIKey:   "test-key",
		Model:    "claude-test",
	}, httpClient, nil)

	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	err := client.ChatCompletion(context.Background(), map[string]any{
		"model": "claude-test",
		"messages": []map[string]string{
			{"role": "user", "content": "hello"},
		},
		"max_completion_tokens": 16,
	}, &out)
	if err != nil {
		t.Fatalf("ChatCompletion: %v", err)
	}
	if len(out.Choices) != 1 || out.Choices[0].Message.Content != "native ok" {
		t.Fatalf("response = %#v", out)
	}
}
