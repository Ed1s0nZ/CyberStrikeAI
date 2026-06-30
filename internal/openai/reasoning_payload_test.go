package openai

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestStripReasoningFromChatCompletionBody(t *testing.T) {
	in := []byte(`{"model":"deepseek-chat","messages":[],"thinking":{"type":"enabled"},"reasoning_effort":"high"}`)
	out, err := StripReasoningFromChatCompletionBody(in)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if strings.Contains(s, "thinking") || strings.Contains(s, "reasoning_effort") {
		t.Fatalf("expected reasoning fields stripped, got %s", s)
	}
	if !strings.Contains(s, `"model":"deepseek-chat"`) {
		t.Fatalf("expected model preserved, got %s", s)
	}

	plain := []byte(`{"model":"gpt-4o","messages":[]}`)
	out2, err := StripReasoningFromChatCompletionBody(plain)
	if err != nil {
		t.Fatal(err)
	}
	if string(out2) != string(plain) {
		t.Fatalf("expected unchanged payload, got %s", out2)
	}
}

func TestStripReasoningIfForcedToolChoice(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		strip   bool
		contain string
	}{
		{
			name:  "required strips thinking",
			in:    `{"model":"minimax","messages":[],"thinking":{"type":"enabled"},"tool_choice":"required","tools":[]}`,
			strip: true,
		},
		{
			name:  "object tool_choice strips thinking",
			in:    `{"model":"qwen","messages":[],"thinking":{"type":"enabled"},"tool_choice":{"type":"function","function":{"name":"respond"}}}`,
			strip: true,
		},
		{
			name:    "auto keeps thinking",
			in:      `{"model":"qwen","messages":[],"thinking":{"type":"enabled"},"tool_choice":"auto"}`,
			strip:   false,
			contain: "thinking",
		},
		{
			name:    "no tool_choice keeps thinking",
			in:      `{"model":"qwen","messages":[],"thinking":{"type":"enabled"}}`,
			strip:   false,
			contain: "thinking",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := StripReasoningIfForcedToolChoice([]byte(tc.in))
			if err != nil {
				t.Fatal(err)
			}
			s := string(out)
			hasThinking := strings.Contains(s, "thinking")
			if tc.strip && hasThinking {
				t.Fatalf("expected thinking stripped, got %s", s)
			}
			if !tc.strip && tc.contain != "" && !strings.Contains(s, tc.contain) {
				t.Fatalf("expected %q in %s", tc.contain, s)
			}
			if !tc.strip && string(out) != tc.in {
				t.Fatalf("expected unchanged payload, got %s", s)
			}
		})
	}
}

func TestReasoningToolChoiceCompatRoundTripper(t *testing.T) {
	var gotBody string
	rt := &reasoningToolChoiceCompatRoundTripper{
		base: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			b, _ := io.ReadAll(req.Body)
			gotBody = string(b)
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"ok"}}]}`)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		}),
	}
	req, err := http.NewRequest(http.MethodPost, "https://example.com/v1/chat/completions", strings.NewReader(
		`{"model":"m","thinking":{"type":"enabled"},"tool_choice":"required","messages":[]}`,
	))
	if err != nil {
		t.Fatal(err)
	}
	_, err = rt.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(gotBody, "thinking") {
		t.Fatalf("expected thinking stripped in transit, got %s", gotBody)
	}
	if !strings.Contains(gotBody, `"tool_choice":"required"`) {
		t.Fatalf("expected tool_choice preserved, got %s", gotBody)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
