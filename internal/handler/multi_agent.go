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

	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/multiagent"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// MultiAgentLoopStream Eino DeepAgent 流式对话（需 config.multi_agent.enabled）。
func (h *AgentHandler) MultiAgentLoopStream(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	if h.config == nil || !h.config.MultiAgent.Enabled {
		ev := StreamEvent{Type: "error", Message: "多代理未启用，请在设置或 config.yaml 中开启 multi_agent.enabled"}
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
		event := StreamEvent{Type: "error", Message: "请求参数错误: " + err.Error()}
		b, _ := json.Marshal(event)
		fmt.Fprintf(c.Writer, "data: %s\n\n", b)
		done := StreamEvent{Type: "done", Message: ""}
		db, _ := json.Marshal(done)
		fmt.Fprintf(c.Writer, "data: %s\n\n", db)
		c.Writer.Flush()
		return
	}

	c.Header("X-Accel-Buffering", "no")

	// 用于在 sendEvent 中判断是否为用户主动停止导致的取消。
	// 注意：baseCtx 会在后面创建；该变量用于闭包提前捕获引用。
	var baseCtx context.Context

	clientDisconnected := false
	// 与 sseKeepalive 共用：禁止并发写 ResponseWriter，否则会破坏 chunked 编码（ERR_INVALID_CHUNKED_ENCODING）。
	var sseWriteMu sync.Mutex
	var ssePublishConversationID string
	sendEvent := func(eventType, message string, data interface{}) {
		// 用户主动停止时，Eino 可能仍会并发上报 eventType=="error"。
		// 为避免 UI 看到“取消错误 + cancelled 文案”两条回复，这里直接丢弃取消对应的 error。
		if eventType == "error" && baseCtx != nil && errors.Is(context.Cause(baseCtx), ErrTaskCancelled) {
			return
		}
		ev := StreamEvent{Type: eventType, Message: message, Data: data}
		b, errMarshal := json.Marshal(ev)
		if errMarshal != nil {
			b = []byte(`{"type":"error","message":"marshal failed"}`)
		}
		sseLine := make([]byte, 0, len(b)+8)
		sseLine = append(sseLine, []byte("data: ")...)
		sseLine = append(sseLine, b...)
		sseLine = append(sseLine, '\n', '\n')
		if ssePublishConversationID != "" && h.taskEventBus != nil {
			h.taskEventBus.Publish(ssePublishConversationID, sseLine)
		}
		if clientDisconnected {
			return
		}
		select {
		case <-c.Request.Context().Done():
			clientDisconnected = true
			return
		default:
		}
		sseWriteMu.Lock()
		_, err := c.Writer.Write(sseLine)
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

	h.logger.Info("收到 Eino DeepAgent 流式请求",
		zap.String("conversationId", req.ConversationID),
	)

	prep, err := h.prepareMultiAgentSession(&req)
	if err != nil {
		sendEvent("error", err.Error(), nil)
		sendEvent("done", "", nil)
		return
	}
	ssePublishConversationID = prep.ConversationID
	if prep.CreatedNew {
		sendEvent("conversation", "会话已创建", map[string]interface{}{
			"conversationId": prep.ConversationID,
		})
	}

	conversationID := prep.ConversationID
	assistantMessageID := prep.AssistantMessageID
	h.activateHITLForConversation(conversationID, req.Hitl)
	if h.hitlManager != nil {
		defer h.hitlManager.DeactivateConversation(conversationID)
	}

	if prep.UserMessageID != "" {
		sendEvent("message_saved", "", map[string]interface{}{
			"conversationId": conversationID,
			"userMessageId":  prep.UserMessageID,
		})
	}

	baseCtx, cancelWithCause := context.WithCancelCause(context.Background())
	taskCtx, timeoutCancel := context.WithTimeout(baseCtx, 600*time.Minute)
	defer timeoutCancel()
	defer cancelWithCause(nil)
	progressCallback := h.createProgressCallback(taskCtx, cancelWithCause, conversationID, assistantMessageID, sendEvent)
	taskCtx = multiagent.WithHITLToolInterceptor(taskCtx, func(ctx context.Context, toolName, arguments string) (string, error) {
		return h.interceptHITLForEinoTool(ctx, cancelWithCause, conversationID, assistantMessageID, sendEvent, toolName, arguments)
	})

	if _, err := h.tasks.StartTask(conversationID, req.Message, cancelWithCause); err != nil {
		var errorMsg string
		if errors.Is(err, ErrTaskAlreadyRunning) {
			errorMsg = "⚠️ 当前会话已有任务正在执行中，请等待当前任务完成或点击「停止任务」后再尝试。"
			sendEvent("error", errorMsg, map[string]interface{}{
				"conversationId": conversationID,
				"errorType":      "task_already_running",
			})
		} else {
			errorMsg = "❌ 无法启动任务: " + err.Error()
			sendEvent("error", errorMsg, nil)
		}
		if assistantMessageID != "" {
			_, _ = h.db.Exec("UPDATE messages SET content = ?, updated_at = ? WHERE id = ?", errorMsg, time.Now(), assistantMessageID)
		}
		sendEvent("done", "", map[string]interface{}{"conversationId": conversationID})
		return
	}

	taskStatus := "completed"
	defer h.tasks.FinishTask(conversationID, taskStatus)

	sendEvent("progress", "正在启动 Eino 多代理...", map[string]interface{}{
		"conversationId": conversationID,
	})

	stopKeepalive := make(chan struct{})
	go sseKeepalive(c, stopKeepalive, &sseWriteMu)
	defer close(stopKeepalive)

	result, runErr := multiagent.RunDeepAgent(
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
		strings.TrimSpace(req.Orchestration),
	)

	if runErr != nil {
		h.persistEinoAgentTraceForResume(conversationID, result)
		cause := context.Cause(baseCtx)
		if errors.Is(cause, ErrTaskCancelled) {
			taskStatus = "cancelled"
			h.tasks.UpdateTaskStatus(conversationID, taskStatus)
			cancelMsg := "任务已被用户取消，后续操作已停止。"
			if assistantMessageID != "" {
				_, _ = h.db.Exec("UPDATE messages SET content = ?, updated_at = ? WHERE id = ?", cancelMsg, time.Now(), assistantMessageID)
				_ = h.db.AddProcessDetail(assistantMessageID, conversationID, "cancelled", cancelMsg, nil)
			}
			sendEvent("cancelled", cancelMsg, map[string]interface{}{
				"conversationId": conversationID,
				"messageId":      assistantMessageID,
			})
			sendEvent("done", "", map[string]interface{}{"conversationId": conversationID})
			return
		}

		if errors.Is(runErr, context.DeadlineExceeded) || errors.Is(context.Cause(taskCtx), context.DeadlineExceeded) {
			taskStatus = "timeout"
			h.tasks.UpdateTaskStatus(conversationID, taskStatus)
			timeoutMsg := "任务执行超时，已自动终止。"
			if assistantMessageID != "" {
				_, _ = h.db.Exec("UPDATE messages SET content = ?, updated_at = ? WHERE id = ?", timeoutMsg, time.Now(), assistantMessageID)
				_ = h.db.AddProcessDetail(assistantMessageID, conversationID, "timeout", timeoutMsg, nil)
			}
			sendEvent("error", timeoutMsg, map[string]interface{}{
				"conversationId": conversationID,
				"messageId":      assistantMessageID,
				"errorType":      "timeout",
			})
			sendEvent("done", "", map[string]interface{}{"conversationId": conversationID})
			return
		}

		h.logger.Error("Eino DeepAgent 执行失败", zap.Error(runErr))
		taskStatus = "failed"
		h.tasks.UpdateTaskStatus(conversationID, taskStatus)
		errMsg := "执行失败: " + runErr.Error()
		if assistantMessageID != "" {
			_, _ = h.db.Exec("UPDATE messages SET content = ?, updated_at = ? WHERE id = ?", errMsg, time.Now(), assistantMessageID)
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
			"UPDATE messages SET content = ?, mcp_execution_ids = ?, updated_at = ? WHERE id = ?",
			result.Response,
			mcpIDsJSON,
			time.Now(),
			assistantMessageID,
		)
	}

	if result.LastAgentTraceInput != "" || result.LastAgentTraceOutput != "" {
		if err := h.db.SaveAgentTrace(conversationID, result.LastAgentTraceInput, result.LastAgentTraceOutput); err != nil {
			h.logger.Warn("保存代理轨迹失败", zap.Error(err))
		}
	}

	effectiveOrch := config.NormalizeMultiAgentOrchestration(h.config.MultiAgent.Orchestration)
	if o := strings.TrimSpace(req.Orchestration); o != "" {
		effectiveOrch = config.NormalizeMultiAgentOrchestration(o)
	}
	sendEvent("response", result.Response, map[string]interface{}{
		"mcpExecutionIds": result.MCPExecutionIDs,
		"conversationId":  conversationID,
		"messageId":       assistantMessageID,
		"agentMode":       "eino_" + effectiveOrch,
	})
	sendEvent("done", "", map[string]interface{}{"conversationId": conversationID})
}

// MultiAgentLoop Eino DeepAgent 非流式对话（与 POST /api/agent-loop 对齐，需 multi_agent.enabled）。
func (h *AgentHandler) MultiAgentLoop(c *gin.Context) {
	if h.config == nil || !h.config.MultiAgent.Enabled {
		c.JSON(http.StatusNotFound, gin.H{"error": "多代理未启用，请在 config.yaml 中设置 multi_agent.enabled: true"})
		return
	}

	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("收到 Eino DeepAgent 非流式请求", zap.String("conversationId", req.ConversationID))

	prep, err := h.prepareMultiAgentSession(&req)
	if err != nil {
		status, msg := multiAgentHTTPErrorStatus(err)
		c.JSON(status, gin.H{"error": msg})
		return
	}
	h.activateHITLForConversation(prep.ConversationID, req.Hitl)
	if h.hitlManager != nil {
		defer h.hitlManager.DeactivateConversation(prep.ConversationID)
	}

	baseCtx, cancelWithCause := context.WithCancelCause(c.Request.Context())
	defer cancelWithCause(nil)
	taskCtx, timeoutCancel := context.WithTimeout(baseCtx, 600*time.Minute)
	defer timeoutCancel()
	progressCallback := h.createProgressCallback(taskCtx, cancelWithCause, prep.ConversationID, prep.AssistantMessageID, nil)
	taskCtx = multiagent.WithHITLToolInterceptor(taskCtx, func(ctx context.Context, toolName, arguments string) (string, error) {
		return h.interceptHITLForEinoTool(ctx, cancelWithCause, prep.ConversationID, prep.AssistantMessageID, nil, toolName, arguments)
	})

	result, runErr := multiagent.RunDeepAgent(
		taskCtx,
		h.config,
		&h.config.MultiAgent,
		h.agent,
		h.logger,
		prep.ConversationID,
		prep.FinalMessage,
		prep.History,
		prep.RoleTools,
		progressCallback,
		h.agentsMarkdownDir,
		strings.TrimSpace(req.Orchestration),
	)
	if runErr != nil {
		h.persistEinoAgentTraceForResume(prep.ConversationID, result)
		h.logger.Error("Eino DeepAgent 执行失败", zap.Error(runErr))
		errMsg := "执行失败: " + runErr.Error()
		if prep.AssistantMessageID != "" {
			_, _ = h.db.Exec("UPDATE messages SET content = ?, updated_at = ? WHERE id = ?", errMsg, time.Now(), prep.AssistantMessageID)
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
			"UPDATE messages SET content = ?, mcp_execution_ids = ?, updated_at = ? WHERE id = ?",
			result.Response,
			mcpIDsJSON,
			time.Now(),
			prep.AssistantMessageID,
		)
	}

	if result.LastAgentTraceInput != "" || result.LastAgentTraceOutput != "" {
		if err := h.db.SaveAgentTrace(prep.ConversationID, result.LastAgentTraceInput, result.LastAgentTraceOutput); err != nil {
			h.logger.Warn("保存代理轨迹失败", zap.Error(err))
		}
	}

	c.JSON(http.StatusOK, ChatResponse{
		Response:        result.Response,
		MCPExecutionIDs: result.MCPExecutionIDs,
		ConversationID:  prep.ConversationID,
		Time:            time.Now(),
	})
}

// persistEinoAgentTraceForResume 在 Eino 运行异常结束时写入代理轨迹（库列 last_react_*），供下一请求 loadHistoryFromAgentTrace 软续跑。
func (h *AgentHandler) persistEinoAgentTraceForResume(conversationID string, result *multiagent.RunResult) {
	if h == nil || result == nil {
		return
	}
	if result.LastAgentTraceInput == "" && result.LastAgentTraceOutput == "" {
		return
	}
	if err := h.db.SaveAgentTrace(conversationID, result.LastAgentTraceInput, result.LastAgentTraceOutput); err != nil {
		h.logger.Warn("保存 Eino 续跑上下文失败", zap.String("conversationId", conversationID), zap.Error(err))
	}
}

func multiAgentHTTPErrorStatus(err error) (int, string) {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "对话不存在"):
		return http.StatusNotFound, msg
	case strings.Contains(msg, "未找到该 WebShell"):
		return http.StatusBadRequest, msg
	case strings.Contains(msg, "附件最多"):
		return http.StatusBadRequest, msg
	case strings.Contains(msg, "保存用户消息失败"), strings.Contains(msg, "创建对话失败"):
		return http.StatusInternalServerError, msg
	case strings.Contains(msg, "保存上传文件失败"):
		return http.StatusInternalServerError, msg
	default:
		return http.StatusBadRequest, msg
	}
}
