package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"cyberstrike-ai/internal/multiagent"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// MultiAgentLoopStream runs a conversation through the native multi-agent
// orchestrator (internal/multiagent) and emits SSE progress + final response.
// The caller must have config.multi_agent.enabled=true; the handler fails
// fast with an SSE error/done pair when the feature is off rather than
// returning an HTTP error (the stream is already open by the time we check).
func (h *AgentHandler) MultiAgentLoopStream(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	if h.config == nil || !h.config.MultiAgent.Enabled {
		ev := StreamEvent{Type: "error", Message: "multi-agent mode is not enabled. Enable it in Settings, or set multi_agent.enabled: true in config.yaml and restart."}
		b, _ := json.Marshal(ev)
		fmt.Fprintf(c.Writer, "data: %s\n\n", b)
		done := StreamEvent{Type: "done", Message: ""}
		db, _ := json.Marshal(done)
		fmt.Fprintf(c.Writer, "data: %s\n\n", db)
		if flusher, ok := c.Writer.(http.Flusher); ok {
			flusher.Flush()
		}
		return
	}

	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		event := StreamEvent{Type: "error", Message: "error: " + err.Error()}
		b, _ := json.Marshal(event)
		fmt.Fprintf(c.Writer, "data: %s\n\n", b)
		c.Writer.Flush()
		return
	}

	c.Header("X-Accel-Buffering", "no")

	// sendEvent is the emit hook for SSE events. It respects task
	// cancellation — baseCtx is the parent task context and
	// ErrTaskCancelled suppresses spurious error events during stop.
	var baseCtx context.Context

	clientDisconnected := false
	// shared with sseKeepalive: ResponseWriter, chunked (ERR_INVALID_CHUNKED_ENCODING).
	var sseWriteMu sync.Mutex
	sendEvent := func(eventType, message string, data interface{}) {
		if clientDisconnected {
			return
		}
		// Cancellation squelch: when the operator stops the task, the
		// orchestrator's error channel also fires an eventType=="error"
		// event. Suppressing it here keeps the UI from rendering a
		// spurious "error + cancelled" pair — the "cancelled" status
		// is the authoritative outcome.
		if eventType == "error" && baseCtx != nil && errors.Is(context.Cause(baseCtx), ErrTaskCancelled) {
			return
		}
		select {
		case <-c.Request.Context().Done():
			clientDisconnected = true
			return
		default:
		}
		ev := StreamEvent{Type: eventType, Message: message, Data: data}
		b, _ := json.Marshal(ev)
		sseWriteMu.Lock()
		_, err := fmt.Fprintf(c.Writer, "data: %s\n\n", b)
		if err != nil {
			sseWriteMu.Unlock()
			clientDisconnected = true
			return
		}
		if flusher, ok := c.Writer.(http.Flusher); ok {
			flusher.Flush()
		} else {
			c.Writer.Flush()
		}
		sseWriteMu.Unlock()
	}

	h.logger.Info("multi-agent orchestrator: received streaming request",
		zap.String("conversationId", req.ConversationID),
	)

	prep, err := h.prepareMultiAgentSession(&req)
	if err != nil {
		sendEvent("error", err.Error(), nil)
		sendEvent("done", "", nil)
		return
	}
	if prep.CreatedNew {
		sendEvent("conversation", "created", map[string]interface{}{
			"conversationId": prep.ConversationID,
		})
	}

	conversationID := prep.ConversationID
	assistantMessageID := prep.AssistantMessageID

	progressCallback := h.createProgressCallback(conversationID, assistantMessageID, sendEvent)

	baseCtx, cancelWithCause := context.WithCancelCause(context.Background())
	taskCtx, timeoutCancel := context.WithTimeout(baseCtx, 600*time.Minute)
	defer timeoutCancel()
	defer cancelWithCause(nil)

	if _, err := h.tasks.StartTask(conversationID, req.Message, cancelWithCause); err != nil {
		var errorMsg string
		if errors.Is(err, ErrTaskAlreadyRunning) {
			errorMsg = "⚠️ This session already has a running task. Say \"stop\" to cancel it."
			sendEvent("error", errorMsg, map[string]interface{}{
				"conversationId": conversationID,
				"errorType":      "task_already_running",
			})
		} else {
			errorMsg = "❌ Failed to start task: " + err.Error()
			sendEvent("error", errorMsg, nil)
		}
		if assistantMessageID != "" {
			_, _ = h.db.Exec("UPDATE messages SET content = ? WHERE id = ?", errorMsg, assistantMessageID)
		}
		sendEvent("done", "", map[string]interface{}{"conversationId": conversationID})
		return
	}

	taskStatus := "completed"
	defer h.tasks.FinishTask(conversationID, taskStatus)

	sendEvent("progress", "starting multi-agent orchestrator...", map[string]interface{}{
		"conversationId": conversationID,
	})

	stopKeepalive := make(chan struct{})
	go sseKeepalive(c, stopKeepalive, &sseWriteMu)
	defer close(stopKeepalive)

	result, runErr := multiagent.RunOrchestrator(
		taskCtx,
		h.config,
		&h.config.MultiAgent,
		h.agent,
		h.logger,
		conversationID,
		prep.FinalMessage,
		prep.History,
		prep.RoleTools,
		progressCallback,
		h.agentsMarkdownDir,
		h.debugSink,
	)

	if runErr != nil {
		cause := context.Cause(baseCtx)
		if errors.Is(cause, ErrTaskCancelled) {
			taskStatus = "cancelled"
			h.tasks.UpdateTaskStatus(conversationID, taskStatus)
			cancelMsg := "Task cancelled by user; stopping."
			if assistantMessageID != "" {
				_, _ = h.db.Exec("UPDATE messages SET content = ? WHERE id = ?", cancelMsg, assistantMessageID)
				_ = h.db.AddProcessDetail(assistantMessageID, conversationID, "cancelled", cancelMsg, nil)
			}
			sendEvent("cancelled", cancelMsg, map[string]interface{}{
				"conversationId": conversationID,
				"messageId":      assistantMessageID,
			})
			sendEvent("done", "", map[string]interface{}{"conversationId": conversationID})
			return
		}

		h.logger.Error("multi-agent orchestrator: execution failed", zap.Error(runErr))
		taskStatus = "failed"
		h.tasks.UpdateTaskStatus(conversationID, taskStatus)
		errMsg := "execution failed: " + runErr.Error()
		if assistantMessageID != "" {
			_, _ = h.db.Exec("UPDATE messages SET content = ? WHERE id = ?", errMsg, assistantMessageID)
			_ = h.db.AddProcessDetail(assistantMessageID, conversationID, "error", errMsg, nil)
		}
		sendEvent("error", errMsg, map[string]interface{}{
			"conversationId": conversationID,
			"messageId":      assistantMessageID,
		})
		sendEvent("done", "", map[string]interface{}{"conversationId": conversationID})
		return
	}

	if assistantMessageID != "" {
		mcpIDsJSON := ""
		if len(result.MCPExecutionIDs) > 0 {
			jsonData, _ := json.Marshal(result.MCPExecutionIDs)
			mcpIDsJSON = string(jsonData)
		}
		_, _ = h.db.Exec(
			"UPDATE messages SET content = ?, mcp_execution_ids = ? WHERE id = ?",
			result.Response,
			mcpIDsJSON,
			assistantMessageID,
		)
	}

	if result.LastReActInput != "" || result.LastReActOutput != "" {
		if err := h.db.SaveReActData(conversationID, result.LastReActInput, result.LastReActOutput); err != nil {
			h.logger.Warn("failed to save ReAct data", zap.Error(err))
		}
	}

	sendEvent("response", result.Response, map[string]interface{}{
		"mcpExecutionIds": result.MCPExecutionIDs,
		"conversationId":  conversationID,
		"messageId":       assistantMessageID,
		"agentMode":       "multi",
	})
	sendEvent("done", "", map[string]interface{}{"conversationId": conversationID})
}

// MultiAgentLoop runs a conversation through the native multi-agent
// orchestrator and returns the final result as JSON (non-streaming
// counterpart to /api/agent-loop). Requires config.multi_agent.enabled=true;
// otherwise returns HTTP 404 with a JSON error body.
func (h *AgentHandler) MultiAgentLoop(c *gin.Context) {
	if h.config == nil || !h.config.MultiAgent.Enabled {
		c.JSON(http.StatusNotFound, gin.H{"error": "multi-agent mode is not enabled. Set multi_agent.enabled: true in config.yaml and restart."})
		return
	}

	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("multi-agent orchestrator: received non-streaming request", zap.String("conversationId", req.ConversationID))

	prep, err := h.prepareMultiAgentSession(&req)
	if err != nil {
		status, msg := multiAgentHTTPErrorStatus(err)
		c.JSON(status, gin.H{"error": msg})
		return
	}

	result, runErr := multiagent.RunOrchestrator(
		c.Request.Context(),
		h.config,
		&h.config.MultiAgent,
		h.agent,
		h.logger,
		prep.ConversationID,
		prep.FinalMessage,
		prep.History,
		prep.RoleTools,
		nil,
		h.agentsMarkdownDir,
		h.debugSink,
	)
	if runErr != nil {
		h.logger.Error("multi-agent orchestrator: execution failed", zap.Error(runErr))
		errMsg := "execution failed: " + runErr.Error()
		if prep.AssistantMessageID != "" {
			_, _ = h.db.Exec("UPDATE messages SET content = ? WHERE id = ?", errMsg, prep.AssistantMessageID)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": errMsg})
		return
	}

	if prep.AssistantMessageID != "" {
		mcpIDsJSON := ""
		if len(result.MCPExecutionIDs) > 0 {
			jsonData, _ := json.Marshal(result.MCPExecutionIDs)
			mcpIDsJSON = string(jsonData)
		}
		_, _ = h.db.Exec(
			"UPDATE messages SET content = ?, mcp_execution_ids = ? WHERE id = ?",
			result.Response,
			mcpIDsJSON,
			prep.AssistantMessageID,
		)
	}

	if result.LastReActInput != "" || result.LastReActOutput != "" {
		if err := h.db.SaveReActData(prep.ConversationID, result.LastReActInput, result.LastReActOutput); err != nil {
			h.logger.Warn("failed to save ReAct data", zap.Error(err))
		}
	}

	c.JSON(http.StatusOK, ChatResponse{
		Response:        result.Response,
		MCPExecutionIDs: result.MCPExecutionIDs,
		ConversationID:  prep.ConversationID,
		Time:            time.Now(),
	})
}

func multiAgentHTTPErrorStatus(err error) (int, string) {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "conversation"):
		return http.StatusNotFound, msg
	case strings.Contains(msg, "WebShell not found"):
		return http.StatusBadRequest, msg
	case strings.Contains(msg, "maximum attachments"):
		return http.StatusBadRequest, msg
	case strings.Contains(msg, "message"), strings.Contains(msg, "conversation"):
		return http.StatusInternalServerError, msg
	case strings.Contains(msg, "failed to save uploaded file"):
		return http.StatusInternalServerError, msg
	default:
		return http.StatusBadRequest, msg
	}
}
