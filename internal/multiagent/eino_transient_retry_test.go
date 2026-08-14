package multiagent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	einoopenai "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

func TestIsEinoTransientRunError(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"io eof", io.EOF, false},
		{"plain eof text", errors.New("EOF"), false},
		{"post chat completions eof", errors.New(`Post "https://token-plan-cn.xiaomimimo.com/v1/chat/completions": EOF`), true},
		{"post eof wraps io.EOF", fmt.Errorf(`Post %q: %w`, "https://token-plan-cn.xiaomimimo.com/v1/chat/completions", io.EOF), true},
		{"429", errors.New("HTTP 429 Too Many Requests"), true},
		{"typed 429", &einoopenai.APIError{HTTPStatusCode: 429}, true},
		{"typed 400", &einoopenai.APIError{HTTPStatusCode: 400, Message: "Invalid request body"}, false},
		{"400 with unrelated number", errors.New("status code: 400, request id contains 500"), false},
		{"409", errors.New("HTTP 409 Conflict"), true},
		{"rate limit", errors.New(`{"error":"rate limit exceeded"}`), true},
		{"connection reset", errors.New("read tcp: connection reset by peer"), true},
		{"http2 goaway", errors.New("failed to receive stream chunk: error, http2: server sent GOAWAY and closed the connection; LastStreamID=791, ErrCode=NO_ERROR"), true},
		{"unexpected internal stream chunk", errors.New("failed to receive stream chunk: error, The service encountered an unexpected internal error. Request id: 0217851391106464f01ec66621d0980a42fd45436ed75957a6a0a"), true},
		{"unexpected eof", errors.New("unexpected EOF"), true},
		{"503", errors.New("upstream returned 503"), true},
		{"iteration limit", errors.New("max iteration reached"), false},
		{"canceled", context.Canceled, false},
		{"deadline", context.DeadlineExceeded, false},
		{"auth", errors.New("invalid api key"), false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := isEinoTransientRunError(tc.err); got != tc.want {
				t.Fatalf("isEinoTransientRunError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestEinoTransientRetryBackoff(t *testing.T) {
	t.Parallel()
	max := 30 * time.Second
	if got := einoTransientRetryBackoff(0, max); got < time.Second || got > 2*time.Second {
		t.Fatalf("attempt 0 outside equal-jitter range [1s,2s]: %v", got)
	}
	if got := einoTransientRetryBackoff(4, max); got < 15*time.Second || got > 30*time.Second {
		t.Fatalf("attempt 4 outside capped equal-jitter range [15s,30s]: %v", got)
	}
}

func TestEinoTransientRunErrorUserDetail(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		err      error
		wantKind string
	}{
		{"rate limit", errors.New("HTTP 429 Too Many Requests"), "rate_limit"},
		{"upstream", errors.New("upstream returned 503"), "upstream_server"},
		{"network", errors.New("read tcp: connection reset by peer"), "network"},
		{"stream", errors.New("unexpected end of JSON"), "stream"},
		{"stream chunk", errors.New("failed to receive stream chunk: error, The service encountered an unexpected internal error. Request id: abc"), "upstream_busy"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			kind, summary := einoTransientRunErrorUserDetail(tc.err)
			if kind != tc.wantKind {
				t.Fatalf("kind=%q, want %q", kind, tc.wantKind)
			}
			if summary == "" {
				t.Fatal("summary should not be empty")
			}
		})
	}
}

func TestEinoTrimRetryErrorSummary(t *testing.T) {
	t.Parallel()
	raw := strings.Repeat("报错 ", 260)
	got := einoTrimRetryErrorSummary(raw)
	if len([]rune(got)) > 503 {
		t.Fatalf("summary too long: %d runes", len([]rune(got)))
	}
	if !strings.HasSuffix(got, "...") {
		t.Fatal("trimmed summary should end with ellipsis")
	}
}

func TestEinoMessagesForRunRestart(t *testing.T) {
	t.Parallel()
	base := []adk.Message{schema.UserMessage("hi")}
	acc := append([]adk.Message(nil), base...)
	acc = append(acc, schema.AssistantMessage("step1", nil))

	got, src := einoMessagesForRunRestart(nil, base, acc, len(base))
	if src != einoRestartContextAccumulated || len(got) != 2 {
		t.Fatalf("accumulated: src=%s len=%d", src, len(got))
	}

	holder := newModelFacingTraceHolder()
	holder.storeFromState(&adk.ChatModelAgentState{
		Messages: []adk.Message{schema.UserMessage("u"), schema.AssistantMessage("model-view", nil)},
	})
	got2, src2 := einoMessagesForRunRestart(&einoADKRunLoopArgs{ModelFacingTrace: holder}, base, acc, len(base))
	if src2 != einoRestartContextModelTrace || len(got2) != 2 {
		t.Fatalf("model trace: src=%s len=%d", src2, len(got2))
	}
}

func TestEinoRunRetryMaxAttemptsFromArgs(t *testing.T) {
	t.Parallel()
	if einoRunRetryMaxAttempts(nil) != defaultEinoRunRetryMaxAttempts {
		t.Fatal("nil args should use default")
	}
	if einoRunRetryMaxAttempts(&einoADKRunLoopArgs{RunRetryMaxAttempts: 3}) != 3 {
		t.Fatal("custom max attempts")
	}
	if RunRetryMaxAttemptsFromConfig(nil) != defaultEinoRunRetryMaxAttempts {
		t.Fatal("config nil should use default")
	}
}

func TestEinoTransientRunRetrierReset(t *testing.T) {
	t.Parallel()
	r := newEinoTransientRunRetrier(einoTransientRunRetryPolicy{maxAttempts: 10, maxBackoff: 30 * time.Second})
	r.attempts = 3
	r.reset()
	if r.attempt() != 0 {
		t.Fatalf("after reset: attempt=%d, want 0", r.attempt())
	}
	// 重置后下一次退避应从 1s~2s equal-jitter 窗口起算（attempt index 0）。
	if got := einoTransientRetryBackoff(r.attempt(), r.policy.maxBackoff); got < time.Second || got > 2*time.Second {
		t.Fatalf("backoff after reset outside [1s,2s]: %v", got)
	}
}

func TestEinoTransientRunRetrierConsecutiveFailures(t *testing.T) {
	t.Parallel()
	r := newEinoTransientRunRetrier(einoTransientRunRetryPolicy{maxAttempts: 10, maxBackoff: 30 * time.Second})
	ctx := context.Background()
	runErr := errors.New("internal server error")
	args := &einoADKRunLoopArgs{}
	base := []adk.Message{schema.UserMessage("hi")}

	for want := 1; want <= 3; want++ {
		restarted, _, _, _, err := r.tryRetry(ctx, runErr, args, base, nil, len(base))
		if err != nil {
			t.Fatalf("tryRetry attempt %d: %v", want, err)
		}
		if !restarted {
			t.Fatalf("tryRetry attempt %d: want restarted", want)
		}
		if got := r.attempt(); got != want {
			t.Fatalf("after failure %d: attempt=%d, want %d", want, got, want)
		}
	}
	r.reset()
	if r.attempt() != 0 {
		t.Fatalf("after successful recovery reset: attempt=%d, want 0", r.attempt())
	}
}

func TestAppendUserMessageIfNeeded(t *testing.T) {
	t.Parallel()
	msgs := []adk.Message{schema.UserMessage("old task")}
	out := appendUserMessageIfNeeded(msgs, "你好，你是谁")
	if len(out) != 2 || out[1].Content != "你好，你是谁" {
		t.Fatalf("should append user: len=%d", len(out))
	}
	dup := appendUserMessageIfNeeded(out, "你好，你是谁")
	if len(dup) != 2 {
		t.Fatalf("should not duplicate user message: len=%d", len(dup))
	}
}

func TestAppendUserMessageIfNeeded_repeatPromptAfterAssistant(t *testing.T) {
	t.Parallel()
	msgs := []adk.Message{
		schema.UserMessage("扫描 example.com"),
		schema.AssistantMessage("开始扫描...", nil),
	}
	out := appendUserMessageIfNeeded(msgs, "扫描 example.com")
	if len(out) != 3 {
		t.Fatalf("should append new user turn after assistant reply: len=%d", len(out))
	}
	if out[2].Role != schema.User || out[2].Content != "扫描 example.com" {
		t.Fatalf("tail should be repeated user prompt, got role=%s content=%q", out[2].Role, out[2].Content)
	}
}
