package multiagent

import (
	"context"
	"errors"

	"github.com/cloudwego/eino/adk"
)

type einoRunErrorHandler struct {
	conversationID       string
	orchMode             string
	progress             func(eventType, message string, data interface{})
	pending              *einoPendingToolCalls
	nativeCancelFallback func() error
}

type einoRunErrorHandlerConfig struct {
	ConversationID       string
	OrchMode             string
	Progress             func(eventType, message string, data interface{})
	Pending              *einoPendingToolCalls
	NativeCancelFallback func() error
}

func newEinoRunErrorHandler(cfg einoRunErrorHandlerConfig) *einoRunErrorHandler {
	return &einoRunErrorHandler{
		conversationID:       cfg.ConversationID,
		orchMode:             cfg.OrchMode,
		progress:             cfg.Progress,
		pending:              cfg.Pending,
		nativeCancelFallback: cfg.NativeCancelFallback,
	}
}

func (h *einoRunErrorHandler) Handle(runErr error) error {
	if h == nil || runErr == nil {
		return runErr
	}
	var cancelErr *adk.CancelError
	if errors.As(runErr, &cancelErr) {
		h.flushPending(runErr)
		if h.nativeCancelFallback != nil {
			return h.nativeCancelFallback()
		}
		return context.Canceled
	}
	if errors.Is(runErr, context.DeadlineExceeded) {
		h.flushPending(runErr)
		h.emitError(runErr, "timeout")
		return runErr
	}
	if errors.Is(runErr, context.Canceled) {
		h.flushPending(runErr)
		h.emitError(runErr, "")
		return runErr
	}
	if isEinoIterationLimitError(runErr) {
		h.flushPending(runErr)
		if h.progress != nil {
			h.progress("iteration_limit_reached", runErr.Error(), map[string]interface{}{
				"conversationId": h.conversationID,
				"source":         "eino",
				"orchestration":  h.orchMode,
			})
		}
		h.emitError(runErr, "iteration_limit")
		return runErr
	}
	h.flushPending(runErr)
	h.emitError(runErr, "")
	return runErr
}

func (h *einoRunErrorHandler) flushPending(err error) {
	if h != nil && h.pending != nil {
		h.pending.FlushAsFailed(err)
	}
}

func (h *einoRunErrorHandler) emitError(err error, kind string) {
	if h == nil || h.progress == nil || err == nil {
		return
	}
	data := map[string]interface{}{
		"conversationId": h.conversationID,
		"source":         "eino",
	}
	if kind != "" {
		data["errorKind"] = kind
	}
	h.progress("error", err.Error(), data)
}
