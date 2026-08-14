package multiagent

import (
	"errors"
	"os"

	"go.uber.org/zap"
)

type einoRunCompletionHandler struct {
	conversationID string
	orchMode       string
	progress       func(eventType, message string, data interface{})
	logger         *zap.Logger

	pending      *einoPendingToolCalls
	cpStore      *fileCheckPointStore
	checkPointID string
}

type einoRunCompletionHandlerConfig struct {
	ConversationID string
	OrchMode       string
	Progress       func(eventType, message string, data interface{})
	Logger         *zap.Logger
	Pending        *einoPendingToolCalls
	Checkpoint     *fileCheckPointStore
	CheckpointID   string
}

func newEinoRunCompletionHandler(cfg einoRunCompletionHandlerConfig) *einoRunCompletionHandler {
	return &einoRunCompletionHandler{
		conversationID: cfg.ConversationID,
		orchMode:       cfg.OrchMode,
		progress:       cfg.Progress,
		logger:         cfg.Logger,
		pending:        cfg.Pending,
		cpStore:        cfg.Checkpoint,
		checkPointID:   cfg.CheckpointID,
	}
}

func (h *einoRunCompletionHandler) Complete() {
	if h == nil {
		return
	}
	h.flushOrphanedPending()
	h.cleanupCheckpoint()
}

func (h *einoRunCompletionHandler) flushOrphanedPending() {
	if h.pending == nil {
		return
	}
	orphanCount := h.pending.Count()
	if orphanCount <= 0 {
		return
	}
	h.pending.FlushAsFailed(errors.New("pending tool call missing result before run completion"))
	if h.progress != nil {
		h.progress("eino_pending_orphaned", "pending tool calls were force-closed at run end", map[string]interface{}{
			"conversationId": h.conversationID,
			"source":         "eino",
			"orchestration":  h.orchMode,
			"pendingCount":   orphanCount,
		})
	}
}

func (h *einoRunCompletionHandler) cleanupCheckpoint() {
	if h.cpStore == nil || h.checkPointID == "" {
		return
	}
	p, err := h.cpStore.path(h.checkPointID)
	if err != nil {
		return
	}
	if rmErr := os.Remove(p); rmErr != nil && !os.IsNotExist(rmErr) && h.logger != nil {
		h.logger.Warn("eino checkpoint cleanup failed", zap.String("path", p), zap.Error(rmErr))
	}
}
