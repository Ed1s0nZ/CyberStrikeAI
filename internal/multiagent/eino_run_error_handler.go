package multiagent

import (
	"context"
	"errors"
	"fmt"
	"strings"

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
	userErr := einoUserFacingRunError(err)
	data := map[string]interface{}{
		"conversationId": h.conversationID,
		"source":         "eino",
		"error":          err.Error(),
	}
	if kind != "" {
		data["errorKind"] = kind
	} else if userErr.kind != "" {
		data["errorKind"] = userErr.kind
	}
	if userErr.summary != "" {
		data["errorSummary"] = userErr.summary
	}
	if userErr.retryExhausted {
		data["retryExhausted"] = true
		if userErr.totalRetries > 0 {
			data["totalRetries"] = userErr.totalRetries
		}
	}
	if userErr.rawLastError != "" {
		data["lastError"] = userErr.rawLastError
	}
	if userErr.technicalError != "" {
		data["technicalError"] = userErr.technicalError
	}
	if userErr.hasModelOriginalError {
		data["modelOriginalError"] = userErr.rawLastError
	} else if userErr.retryExhausted {
		data["hasModelOriginalError"] = false
	}
	message := err.Error()
	if userErr.message != "" {
		message = userErr.message
	}
	h.progress("error", message, data)
}

type einoRunUserError struct {
	message               string
	kind                  string
	summary               string
	rawLastError          string
	technicalError        string
	retryExhausted        bool
	totalRetries          int
	hasModelOriginalError bool
}

func einoUserFacingRunError(err error) einoRunUserError {
	var out einoRunUserError
	if err == nil {
		return out
	}
	var retryErr *adk.RetryExhaustedError
	if !errors.As(err, &retryErr) {
		return out
	}
	out.retryExhausted = true
	out.totalRetries = retryErr.TotalRetries
	lastErr := retryErr.LastErr
	if lastErr == nil {
		out.kind = "model_retry_exhausted"
		out.summary = "模型调用多次重试后仍未成功。"
		out.message = out.summary
		return out
	}
	out.rawLastError = strings.TrimSpace(lastErr.Error())
	if isEinoShouldRetryOutputRejected(lastErr) {
		out.kind = "model_output_rejected"
		out.summary = "模型未返回原始错误；输出被重试策略拒绝。"
		out.technicalError = out.rawLastError
		out.message = formatEinoRetryExhaustedMessage(out.summary, retryErr.TotalRetries)
		return out
	}
	kind, summary := einoTransientRunErrorUserDetail(lastErr)
	if strings.TrimSpace(summary) == "" {
		summary = einoTrimRetryErrorSummary(lastErr.Error())
	}
	if kind == "" {
		kind = "model_retry_exhausted"
	}
	out.kind = kind
	out.summary = summary
	out.hasModelOriginalError = out.rawLastError != ""
	out.message = formatEinoRetryExhaustedMessage(summary, retryErr.TotalRetries)
	return out
}

func isEinoShouldRetryOutputRejected(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "model output rejected by shouldretry")
}

func formatEinoRetryExhaustedMessage(summary string, totalRetries int) string {
	summary = strings.TrimSpace(summary)
	if summary == "" {
		summary = "模型调用多次重试后仍未成功。"
	}
	if totalRetries > 0 {
		return fmt.Sprintf("模型调用重试已耗尽（已重试 %d 次）：%s", totalRetries, summary)
	}
	return "模型调用重试已耗尽：" + summary
}
