package multiagent

import "context"

type einoStreamRetryFunc func(error) (restarted bool, fatal error)
type einoPartialResultFunc func(error) (*RunResult, error)

type einoStreamErrorHandler struct {
	ctx            context.Context
	conversationID string
	progress       func(eventType, message string, data interface{})
	einoRoleTag    func(agent string) string
	retry          einoStreamRetryFunc
	takePartial    einoPartialResultFunc
}

type einoStreamErrorHandleResult struct {
	Handled   bool
	Restarted bool
	Result    *RunResult
	Err       error
}

func newEinoStreamErrorHandler(
	ctx context.Context,
	conversationID string,
	progress func(eventType, message string, data interface{}),
	einoRoleTag func(agent string) string,
	retry einoStreamRetryFunc,
	takePartial einoPartialResultFunc,
) *einoStreamErrorHandler {
	if einoRoleTag == nil {
		einoRoleTag = func(string) string { return "" }
	}
	return &einoStreamErrorHandler{
		ctx:            ctx,
		conversationID: conversationID,
		progress:       progress,
		einoRoleTag:    einoRoleTag,
		retry:          retry,
		takePartial:    takePartial,
	}
}

func (h *einoStreamErrorHandler) Handle(streamErr error, agentName string) einoStreamErrorHandleResult {
	if h == nil || streamErr == nil {
		return einoStreamErrorHandleResult{}
	}
	if isEinoTurnLoopPreemptErr(h.ctx, streamErr) {
		// Host context is still alive: TurnLoop preempt canceled the in-flight
		// model/tool stream. Keep the outer iterator open so the queued
		// interrupt_continue turn can start.
		return einoStreamErrorHandleResult{Handled: true}
	}
	if isInterruptContinue(h.ctx) {
		result, err := h.partial(streamErr)
		return einoStreamErrorHandleResult{Handled: true, Result: result, Err: err}
	}
	if h.progress != nil {
		h.progress("eino_stream_error", streamErr.Error(), map[string]interface{}{
			"conversationId": h.conversationID,
			"source":         "eino",
			"einoAgent":      agentName,
			"einoRole":       h.einoRoleTag(agentName),
		})
	}
	restarted, retErr := h.retryStream(streamErr)
	if retErr != nil {
		result, err := h.partial(retErr)
		return einoStreamErrorHandleResult{Handled: true, Result: result, Err: err}
	}
	return einoStreamErrorHandleResult{Handled: true, Restarted: restarted}
}

func (h *einoStreamErrorHandler) retryStream(err error) (bool, error) {
	if h == nil || h.retry == nil {
		return false, nil
	}
	return h.retry(err)
}

func (h *einoStreamErrorHandler) partial(err error) (*RunResult, error) {
	if h == nil || h.takePartial == nil {
		return nil, err
	}
	return h.takePartial(err)
}
