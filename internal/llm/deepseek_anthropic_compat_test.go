package llm

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"
)

type captureRoundTripper struct {
	body string
}

func (rt *captureRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	rt.body = string(body)
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(nil)),
		Request:    req,
	}, nil
}

func TestDeepSeekAnthropicCompatStripsOnlyCustomToolType(t *testing.T) {
	t.Parallel()
	capture := &captureRoundTripper{}
	client := newDeepSeekAnthropicCompatibleClient(&http.Client{Transport: capture})
	req, err := http.NewRequest(
		http.MethodPost,
		"https://api.deepseek.com/anthropic/v1/messages",
		strings.NewReader(`{"tools":[{"type":"custom","name":"mcp_tool","input_schema":{"type":"object"}},{"type":"web_search_20260209","name":"web_search"}]}`),
	)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	_ = resp.Body.Close()
	if strings.Contains(capture.body, `"type":"custom"`) {
		t.Fatalf("custom discriminator was not removed: %s", capture.body)
	}
	if !strings.Contains(capture.body, `"type":"web_search_20260209"`) {
		t.Fatalf("server tool discriminator was removed: %s", capture.body)
	}
	if !strings.Contains(capture.body, `"name":"mcp_tool"`) {
		t.Fatalf("custom tool definition was removed: %s", capture.body)
	}
}
