package multiagent

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cloudwego/eino/adk"
)

func TestEinoRunErrorHandlerCancelUsesNativeFallback(t *testing.T) {
	pending := newEinoPendingToolCalls("conv-1", nil)
	pending.Mark(toolCallPendingInfo{ToolCallID: "call-1", ToolName: "execute"})
	want := errors.New("native cancel")

	got := newEinoRunErrorHandler(einoRunErrorHandlerConfig{
		ConversationID: "conv-1",
		Pending:        pending,
		NativeCancelFallback: func() error {
			return want
		},
	}).Handle(&adk.CancelError{Info: &adk.AgentCancelInfo{}})

	if !errors.Is(got, want) {
		t.Fatalf("err = %v, want native fallback", got)
	}
	if pending.Count() != 0 {
		t.Fatalf("pending count = %d, want 0", pending.Count())
	}
}

func TestEinoRunErrorHandlerTimeoutAndGeneralErrorProgress(t *testing.T) {
	for _, tc := range []struct {
		name      string
		err       error
		errorKind interface{}
	}{
		{name: "timeout", err: context.DeadlineExceeded, errorKind: "timeout"},
		{name: "general", err: errors.New("boom"), errorKind: nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var data map[string]interface{}
			got := newEinoRunErrorHandler(einoRunErrorHandlerConfig{
				ConversationID: "conv-1",
				Progress: func(eventType, _ string, raw interface{}) {
					if eventType == "error" {
						data, _ = raw.(map[string]interface{})
					}
				},
			}).Handle(tc.err)
			if !errors.Is(got, tc.err) {
				t.Fatalf("err = %v", got)
			}
			if data["conversationId"] != "conv-1" || data["source"] != "eino" {
				t.Fatalf("data = %#v", data)
			}
			if gotKind := data["errorKind"]; gotKind != tc.errorKind {
				t.Fatalf("errorKind = %#v, want %#v", gotKind, tc.errorKind)
			}
		})
	}
}

func TestEinoRunErrorHandlerRetryExhaustedEmptyOutputProgress(t *testing.T) {
	err := &adk.RetryExhaustedError{
		LastErr:      errors.New("model output rejected by ShouldRetry at attempt 5"),
		TotalRetries: 4,
	}
	var message string
	var data map[string]interface{}

	got := newEinoRunErrorHandler(einoRunErrorHandlerConfig{
		ConversationID: "conv-1",
		Progress: func(eventType, msg string, raw interface{}) {
			if eventType == "error" {
				message = msg
				data, _ = raw.(map[string]interface{})
			}
		},
	}).Handle(err)

	if !errors.Is(got, err) {
		t.Fatalf("err = %v", got)
	}
	if !strings.Contains(message, "模型调用重试已耗尽") ||
		!strings.Contains(message, "model output rejected by ShouldRetry at attempt 5") {
		t.Fatalf("message = %q", message)
	}
	if data["errorKind"] != "empty_model_output" {
		t.Fatalf("errorKind = %#v", data["errorKind"])
	}
	if data["errorSummary"] != "模型输出被重试策略拒绝；常见原因是空内容或缺少有效输出。" {
		t.Fatalf("errorSummary = %#v", data["errorSummary"])
	}
	if data["retryExhausted"] != true || data["totalRetries"] != 4 {
		t.Fatalf("retry metadata = %#v", data)
	}
	if data["lastError"] != "model output rejected by ShouldRetry at attempt 5" {
		t.Fatalf("lastError = %#v", data["lastError"])
	}
	if data["error"] != err.Error() {
		t.Fatalf("raw error = %#v, want %#v", data["error"], err.Error())
	}
}

func TestEinoRunErrorHandlerRetryExhaustedOriginalErrorProgress(t *testing.T) {
	err := &adk.RetryExhaustedError{
		LastErr:      errors.New("HTTP 429 Too Many Requests"),
		TotalRetries: 3,
	}
	var message string
	var data map[string]interface{}

	got := newEinoRunErrorHandler(einoRunErrorHandlerConfig{
		ConversationID: "conv-1",
		Progress: func(eventType, msg string, raw interface{}) {
			if eventType == "error" {
				message = msg
				data, _ = raw.(map[string]interface{})
			}
		},
	}).Handle(err)

	if !errors.Is(got, err) {
		t.Fatalf("err = %v", got)
	}
	if !strings.Contains(message, "HTTP 429 Too Many Requests") {
		t.Fatalf("message = %q", message)
	}
	if data["errorKind"] != "rate_limit" {
		t.Fatalf("errorKind = %#v", data["errorKind"])
	}
	if data["errorSummary"] != "HTTP 429 Too Many Requests" {
		t.Fatalf("errorSummary = %#v", data["errorSummary"])
	}
	if data["lastError"] != "HTTP 429 Too Many Requests" {
		t.Fatalf("lastError = %#v", data["lastError"])
	}
}

func TestEinoRunErrorHandlerIterationLimitProgress(t *testing.T) {
	var events []string
	var errorKind interface{}
	err := errors.New("maximum iteration reached")

	got := newEinoRunErrorHandler(einoRunErrorHandlerConfig{
		ConversationID: "conv-1",
		OrchMode:       "deep",
		Progress: func(eventType, _ string, raw interface{}) {
			events = append(events, eventType)
			if eventType == "error" {
				data, _ := raw.(map[string]interface{})
				errorKind = data["errorKind"]
			}
		},
	}).Handle(err)

	if !errors.Is(got, err) {
		t.Fatalf("err = %v", got)
	}
	if len(events) != 2 || events[0] != "iteration_limit_reached" || events[1] != "error" {
		t.Fatalf("events = %#v", events)
	}
	if errorKind != "iteration_limit" {
		t.Fatalf("errorKind = %#v", errorKind)
	}
}

func TestEinoRunErrorHandlerNilSafe(t *testing.T) {
	var h *einoRunErrorHandler
	if h.Handle(nil) != nil {
		t.Fatal("nil handler nil err should return nil")
	}
	err := errors.New("boom")
	if got := h.Handle(err); !errors.Is(got, err) {
		t.Fatalf("nil handler err = %v", got)
	}
}
