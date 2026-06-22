package multiagent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"testing"
	"time"

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
		{"rate limit", errors.New(`{"error":"rate limit exceeded"}`), true},
		{"connection reset", errors.New("read tcp: connection reset by peer"), true},
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
	if got := einoTransientRetryBackoff(0, max); got != 2*time.Second {
		t.Fatalf("attempt 0: got %v", got)
	}
	if got := einoTransientRetryBackoff(4, max); got != 30*time.Second {
		t.Fatalf("attempt 4 capped: got %v", got)
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
