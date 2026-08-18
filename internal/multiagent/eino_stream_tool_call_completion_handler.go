package multiagent

import "github.com/cloudwego/eino/schema"

type einoStreamToolCallCompletionHandlerConfig struct {
	ConversationID string
	OrchMode       string
	Progress       func(eventType, message string, data interface{})
	RunProgress    *einoRunProgressTracker
	RunMessages    *einoRunMessageAccumulator
	MarkPending    func(toolCallPendingInfo)
}

type einoStreamToolCallCompletionHandler struct {
	conversationID string
	orchMode       string
	progress       func(eventType, message string, data interface{})
	runProgress    *einoRunProgressTracker
	runMessages    *einoRunMessageAccumulator
	markPending    func(toolCallPendingInfo)
}

func newEinoStreamToolCallCompletionHandler(cfg einoStreamToolCallCompletionHandlerConfig) *einoStreamToolCallCompletionHandler {
	return &einoStreamToolCallCompletionHandler{
		conversationID: cfg.ConversationID,
		orchMode:       cfg.OrchMode,
		progress:       cfg.Progress,
		runProgress:    cfg.RunProgress,
		runMessages:    cfg.RunMessages,
		markPending:    cfg.MarkPending,
	}
}

func (h *einoStreamToolCallCompletionHandler) Complete(fragments []schema.ToolCall, agentName string) *schema.Message {
	if h == nil {
		return nil
	}
	var lastToolChunk *schema.Message
	if merged := mergeStreamingToolCallFragments(fragments); len(merged) > 0 {
		lastToolChunk = mergeMessageToolCalls(&schema.Message{ToolCalls: merged})
	}
	if h.runProgress != nil {
		h.runProgress.EmitToolCalls(lastToolChunk, agentName, h.markPending)
	}
	if lastToolChunk != nil && len(lastToolChunk.ToolCalls) > 0 && h.runMessages != nil {
		h.runMessages.AppendAssistantToolCalls(lastToolChunk.ToolCalls)
	}
	return lastToolChunk
}
