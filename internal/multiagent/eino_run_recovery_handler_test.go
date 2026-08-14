package multiagent

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

func TestEinoRunRecoveryHandlerRoutesContextOverflowBeforeTransient(t *testing.T) {
	baseMsgs := []adk.Message{schema.UserMessage("base")}
	overflow := newEinoContextOverflowRetryHandler(einoContextOverflowRetryConfig{
		Context:  context.Background(),
		Args:     &einoADKRunLoopArgs{},
		BaseMsgs: baseMsgs,
	})
	transient := newEinoTransientRunRetryHandler(einoTransientRunRetryHandlerConfig{
		Args:     &einoADKRunLoopArgs{},
		BaseMsgs: baseMsgs,
		Policy:   einoTransientRunRetryPolicy{maxAttempts: 1, maxBackoff: time.Nanosecond},
	})
	handler := newEinoRunRecoveryHandler(einoRunRecoveryHandlerConfig{
		ContextOverflow: overflow,
		Transient:       transient,
		BaseMsgs:        baseMsgs,
	})

	result := handler.Handle(errors.New("context length exceeded: upstream returned 503"), nil, 0)
	if !result.Handled || !result.Restarted || result.Fatal != nil {
		t.Fatalf("result = %+v, want context overflow restart", result)
	}
	second := handler.Handle(errors.New("upstream returned 503"), nil, 0)
	if !second.Handled || !second.Restarted || second.Fatal != nil {
		t.Fatalf("second result = %+v, want transient restart", second)
	}
}

func TestEinoRunRecoveryHandlerRoutesFatalFallback(t *testing.T) {
	handler := newEinoRunRecoveryHandler(einoRunRecoveryHandlerConfig{
		RunError: newEinoRunErrorHandler(einoRunErrorHandlerConfig{}),
	})
	result := handler.Handle(errors.New("invalid api key"), nil, 0)
	if !result.Handled || result.Restarted || result.Fatal == nil {
		t.Fatalf("result = %+v, want fatal fallback", result)
	}
}
