package openai

import (
	"bytes"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// reasoningToolChoiceCompatRoundTripper strips thinking/reasoning fields from
// chat/completions requests that force tool_choice, which some gateways reject
// when thinking mode is enabled on the same request.
type reasoningToolChoiceCompatRoundTripper struct {
	base http.RoundTripper
}

func (rt *reasoningToolChoiceCompatRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if rt == nil || rt.base == nil || req == nil || req.Body == nil {
		if rt != nil && rt.base != nil {
			return rt.base.RoundTrip(req)
		}
		return http.DefaultTransport.RoundTrip(req)
	}
	if req.Method != http.MethodPost || !strings.HasSuffix(req.URL.Path, "/chat/completions") {
		return rt.base.RoundTrip(req)
	}

	body, err := io.ReadAll(req.Body)
	_ = req.Body.Close()
	if err != nil {
		return nil, err
	}

	patched, perr := StripReasoningIfForcedToolChoice(body)
	if perr != nil {
		patched = body
	}
	req.Body = io.NopCloser(bytes.NewReader(patched))
	req.ContentLength = int64(len(patched))
	req.Header.Set("Content-Length", strconv.Itoa(len(patched)))
	return rt.base.RoundTrip(req)
}
