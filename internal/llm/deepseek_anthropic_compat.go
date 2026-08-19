package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// newDeepSeekAnthropicCompatibleClient compensates for DeepSeek's Anthropic
// endpoint lagging behind the current Anthropic SDK. The SDK emits
// {"type":"custom"} for function tools, while DeepSeek expects the older
// name/input_schema/description shape without that discriminator.
//
// This is a field-level compatibility fix; requests still originate from
// Eino's native agenticclaude model and remain Anthropic Messages API requests.
func newDeepSeekAnthropicCompatibleClient(base *http.Client) *http.Client {
	if base == nil {
		base = http.DefaultClient
	}
	cloned := *base
	transport := base.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	cloned.Transport = &deepSeekAnthropicCompatRoundTripper{base: transport}
	return &cloned
}

type deepSeekAnthropicCompatRoundTripper struct {
	base http.RoundTripper
}

func (rt *deepSeekAnthropicCompatRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req == nil || req.Body == nil || req.Method != http.MethodPost {
		return rt.base.RoundTrip(req)
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, fmt.Errorf("read DeepSeek Anthropic request: %w", err)
	}
	_ = req.Body.Close()

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		req.Body = io.NopCloser(bytes.NewReader(body))
		return rt.base.RoundTrip(req)
	}
	tools, ok := payload["tools"].([]any)
	if !ok {
		req.Body = io.NopCloser(bytes.NewReader(body))
		return rt.base.RoundTrip(req)
	}
	changed := false
	for _, rawTool := range tools {
		tool, ok := rawTool.(map[string]any)
		if !ok || tool["type"] != "custom" {
			continue
		}
		delete(tool, "type")
		changed = true
	}
	if changed {
		body, err = json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal DeepSeek Anthropic request: %w", err)
		}
	}
	req.Body = io.NopCloser(bytes.NewReader(body))
	req.ContentLength = int64(len(body))
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(body)), nil
	}
	return rt.base.RoundTrip(req)
}
