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
	h.progress("error", EinoClientRunErrorMessage(err), data)
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
	summarizationModelErr bool
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
	if raw, ok := einoSummarizationModelRawErrorText(lastErr); ok {
		out.rawLastError = raw
		out.summarizationModelErr = true
	}
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
	if out.summarizationModelErr {
		summary = einoTrimRetryErrorSummary(out.rawLastError)
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

// EinoClientRunErrorMessage returns the error text that should be shown directly
// to clients. When native retry hides the final provider failure behind a retry
// wrapper, prefer the original last model error so summarization/model issues are
// diagnosable from the frontend without opening server logs.
func EinoClientRunErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	userErr := einoUserFacingRunError(err)
	if userErr.retryExhausted {
		if userErr.hasModelOriginalError && userErr.rawLastError != "" {
			if userErr.summarizationModelErr {
				return formatEinoSummarizationRetryExhaustedRawModelMessage(userErr.rawLastError, userErr.totalRetries)
			}
			return formatEinoRetryExhaustedRawModelMessage(userErr.rawLastError, userErr.totalRetries)
		}
		if userErr.message != "" {
			return userErr.message
		}
	}
	if raw, ok := einoSummarizationModelRawErrorText(err); ok {
		return formatEinoSummarizationRawModelMessage(raw)
	}
	return err.Error()
}

func formatEinoRetryExhaustedRawModelMessage(raw string, totalRetries int) string {
	raw = strings.TrimSpace(raw)
	prefix := "模型调用重试已耗尽"
	if totalRetries > 0 {
		prefix = fmt.Sprintf("模型调用重试已耗尽（已重试 %d 次）", totalRetries)
	}
	if raw == "" {
		return prefix
	}
	return prefix + "，最后一次模型原始错误：\n" + raw
}

func formatEinoSummarizationRetryExhaustedRawModelMessage(raw string, totalRetries int) string {
	raw = strings.TrimSpace(raw)
	prefix := "摘要阶段大模型调用失败，模型调用重试已耗尽"
	if totalRetries > 0 {
		prefix = fmt.Sprintf("摘要阶段大模型调用失败，模型调用重试已耗尽（已重试 %d 次）", totalRetries)
	}
	if raw == "" {
		return prefix
	}
	return prefix + "，最后一次大模型报错原文：\n" + raw
}

func formatEinoSummarizationRawModelMessage(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "摘要阶段大模型调用失败。"
	}
	return "摘要阶段大模型调用失败，大模型报错原文：\n" + raw
}

// EinoClientRunErrorFields returns structured diagnostic fields that handlers can
// attach to their final error event in addition to the visible message.
func EinoClientRunErrorFields(err error) map[string]interface{} {
	fields := make(map[string]interface{})
	if err == nil {
		return fields
	}
	userErr := einoUserFacingRunError(err)
	if userErr.kind != "" {
		fields["errorKind"] = userErr.kind
	}
	if userErr.summary != "" {
		fields["errorSummary"] = userErr.summary
	}
	if userErr.retryExhausted {
		fields["retryExhausted"] = true
		if userErr.totalRetries > 0 {
			fields["totalRetries"] = userErr.totalRetries
		}
	}
	if userErr.rawLastError != "" {
		fields["lastError"] = userErr.rawLastError
	}
	if userErr.technicalError != "" {
		fields["technicalError"] = userErr.technicalError
	}
	if userErr.hasModelOriginalError {
		fields["modelOriginalError"] = userErr.rawLastError
	} else if userErr.retryExhausted {
		fields["hasModelOriginalError"] = false
	}
	if userErr.summarizationModelErr {
		fields["errorPhase"] = "summarization"
		fields["summarizationModelError"] = true
		fields["modelOriginalError"] = userErr.rawLastError
	} else if raw, ok := einoSummarizationModelRawErrorText(err); ok {
		fields["errorPhase"] = "summarization"
		fields["summarizationModelError"] = true
		fields["modelOriginalError"] = raw
		fields["lastError"] = raw
	}
	return fields
}
