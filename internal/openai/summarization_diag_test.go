package openai

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"go.uber.org/zap"
)

type staticRoundTripper struct {
	status int
	body   string
}

func (s *staticRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: s.status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(s.body)),
	}, nil
}

func TestSummarizationResponseEmptyChoices(t *testing.T) {
	if !summarizationResponseEmptyChoices([]byte(`{"choices":[]}`)) {
		t.Fatal("expected empty choices")
	}
	if summarizationResponseEmptyChoices([]byte(`{"choices":[{"index":0}]}`)) {
		t.Fatal("expected non-empty choices")
	}
}

func TestSummarizationDiagRoundTripper_SkipsWithoutHeader(t *testing.T) {
	client := &http.Client{
		Transport: &summarizationDiagRoundTripper{
			base:   &staticRoundTripper{status: 200, body: `{"choices":[]}`},
			logger: zap.NewNop(),
		},
	}
	req, _ := http.NewRequest(http.MethodPost, "https://example.com/v1/chat/completions", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
}
