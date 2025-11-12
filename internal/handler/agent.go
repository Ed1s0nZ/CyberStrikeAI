package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"cyberstrike-ai/internal/agent"
	"cyberstrike-ai/internal/database"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// AgentHandler Agent处理器
type AgentHandler struct {
	agent  *agent.Agent
	db     *database.DB
	logger *zap.Logger
	tasks  *AgentTaskManager
}

// NewAgentHandler 创建新的Agent处理器
func NewAgentHandler(agent *agent.Agent, db *database.DB, logger *zap.Logger) *AgentHandler {
	return &AgentHandler{
		agent:  agent,
		db:     db,
		logger: logger,
		tasks:  NewAgentTaskManager(),
	}
}

// ChatRequest 聊天请求
type ChatRequest struct {
	Message        string `json:"message" binding:"required"`
	ConversationID string `json:"conversationId,omitempty"`
}

// ChatResponse 聊天响应
type ChatResponse struct {
	Response        string    `json:"response"`
	MCPExecutionIDs []string  `json:"mcpExecutionIds,omitempty"` // 本次对话中执行的MCP调用ID列表
	ConversationID  string    `json:"conversationId"`            // 对话ID
	Time            time.Time `json:"time"`
}

// AgentLoop 处理Agent Loop请求
func (h *AgentHandler) AgentLoop(c *gin.Context) {
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("收到Agent Loop请求",
		zap.String("message", req.Message),
		zap.String("conversationId", req.ConversationID),
	)

	// 如果没有对话ID，创建新对话
	conversationID := req.ConversationID
	if conversationID == "" {
		title := req.Message
		if len(title) > 50 {
			title = title[:50] + "..."
		}
		conv, err := h.db.CreateConversation(title)
		if err != nil {
			h.logger.Error("创建对话失败", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		conversationID = conv.ID
	}

	// 获取历史消息（排除当前消息，因为还没保存）
	historyMessages, err := h.db.GetMessages(conversationID)
	if err != nil {
		h.logger.Warn("获取历史消息失败", zap.Error(err))
		historyMessages = []database.Message{}
	}

	h.logger.Info("获取历史消息",
		zap.String("conversationId", conversationID),
		zap.Int("count", len(historyMessages)),
	)

	// 将数据库消息转换为Agent消息格式
	agentHistoryMessages := make([]agent.ChatMessage, 0, len(historyMessages))
	for i, msg := range historyMessages {
		agentHistoryMessages = append(agentHistoryMessages, agent.ChatMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
		contentPreview := msg.Content
		if len(contentPreview) > 50 {
			contentPreview = contentPreview[:50] + "..."
		}
		h.logger.Info("添加历史消息",
			zap.Int("index", i),
			zap.String("role", msg.Role),
			zap.String("content", contentPreview),
		)
	}

	h.logger.Info("历史消息转换完成",
		zap.Int("originalCount", len(historyMessages)),
		zap.Int("convertedCount", len(agentHistoryMessages)),
	)

	// 保存用户消息
	_, err = h.db.AddMessage(conversationID, "user", req.Message, nil)
	if err != nil {
		h.logger.Error("保存用户消息失败", zap.Error(err))
	}

	// 执行Agent Loop，传入历史消息
	result, err := h.agent.AgentLoop(c.Request.Context(), req.Message, agentHistoryMessages)
	if err != nil {
		h.logger.Error("Agent Loop执行失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 保存助手回复
	_, err = h.db.AddMessage(conversationID, "assistant", result.Response, result.MCPExecutionIDs)
	if err != nil {
		h.logger.Error("保存助手消息失败", zap.Error(err))
	}

	c.JSON(http.StatusOK, ChatResponse{
		Response:        result.Response,
		MCPExecutionIDs: result.MCPExecutionIDs,
		ConversationID:  conversationID,
		Time:            time.Now(),
	})
}

// StreamEvent 流式事件
type StreamEvent struct {
	Type    string      `json:"type"`    // conversation, progress, tool_call, tool_result, response, error, cancelled, done
	Message string      `json:"message"` // 显示消息
	Data    interface{} `json:"data,omitempty"`
}

// AgentLoopStream 处理Agent Loop流式请求
func (h *AgentHandler) AgentLoopStream(c *gin.Context) {
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 对于流式请求，也发送SSE格式的错误
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		event := StreamEvent{
			Type:    "error",
			Message: "请求参数错误: " + err.Error(),
		}
		eventJSON, _ := json.Marshal(event)
		fmt.Fprintf(c.Writer, "data: %s\n\n", eventJSON)
		c.Writer.Flush()
		return
	}

	h.logger.Info("收到Agent Loop流式请求",
		zap.String("message", req.Message),
		zap.String("conversationId", req.ConversationID),
	)

	// 设置SSE响应头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no") // 禁用nginx缓冲

	// 发送初始事件
	// 用于跟踪客户端是否已断开连接
	clientDisconnected := false

	sendEvent := func(eventType, message string, data interface{}) {
		// 如果客户端已断开，不再发送事件
		if clientDisconnected {
			return
		}

		// 检查请求上下文是否被取消（客户端断开）
		select {
		case <-c.Request.Context().Done():
			clientDisconnected = true
			return
		default:
		}

		event := StreamEvent{
			Type:    eventType,
			Message: message,
			Data:    data,
		}
		eventJSON, _ := json.Marshal(event)

		// 尝试写入事件，如果失败则标记客户端断开
		if _, err := fmt.Fprintf(c.Writer, "data: %s\n\n", eventJSON); err != nil {
			clientDisconnected = true
			h.logger.Debug("客户端断开连接，停止发送SSE事件", zap.Error(err))
			return
		}

		// 刷新响应，如果失败则标记客户端断开
		if flusher, ok := c.Writer.(http.Flusher); ok {
			flusher.Flush()
		} else {
			c.Writer.Flush()
		}
	}

	// 如果没有对话ID，创建新对话
	conversationID := req.ConversationID
	if conversationID == "" {
		title := req.Message
		if len(title) > 50 {
			title = title[:50] + "..."
		}
		conv, err := h.db.CreateConversation(title)
		if err != nil {
			h.logger.Error("创建对话失败", zap.Error(err))
			sendEvent("error", "创建对话失败: "+err.Error(), nil)
			return
		}
		conversationID = conv.ID
	}

	sendEvent("conversation", "会话已创建", map[string]interface{}{
		"conversationId": conversationID,
	})

	// 获取历史消息
	historyMessages, err := h.db.GetMessages(conversationID)
	if err != nil {
		h.logger.Warn("获取历史消息失败", zap.Error(err))
		historyMessages = []database.Message{}
	}

	// 将数据库消息转换为Agent消息格式
	agentHistoryMessages := make([]agent.ChatMessage, 0, len(historyMessages))
	for _, msg := range historyMessages {
		agentHistoryMessages = append(agentHistoryMessages, agent.ChatMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	// 保存用户消息
	_, err = h.db.AddMessage(conversationID, "user", req.Message, nil)
	if err != nil {
		h.logger.Error("保存用户消息失败", zap.Error(err))
	}

	// 预先创建助手消息，以便关联过程详情
	assistantMsg, err := h.db.AddMessage(conversationID, "assistant", "处理中...", nil)
	if err != nil {
		h.logger.Error("创建助手消息失败", zap.Error(err))
		// 如果创建失败，继续执行但不保存过程详情
		assistantMsg = nil
	}

	// 创建进度回调函数，同时保存到数据库
	var assistantMessageID string
	if assistantMsg != nil {
		assistantMessageID = assistantMsg.ID
	}

	progressCallback := func(eventType, message string, data interface{}) {
		sendEvent(eventType, message, data)

		// 保存过程详情到数据库（排除response和done事件，它们会在后面单独处理）
		if assistantMessageID != "" && eventType != "response" && eventType != "done" {
			if err := h.db.AddProcessDetail(assistantMessageID, conversationID, eventType, message, data); err != nil {
				h.logger.Warn("保存过程详情失败", zap.Error(err), zap.String("eventType", eventType))
			}
		}
	}

	// 创建一个独立的上下文用于任务执行，不随HTTP请求取消
	// 这样即使客户端断开连接（如刷新页面），任务也能继续执行
	baseCtx, cancelWithCause := context.WithCancelCause(context.Background())
	taskCtx, timeoutCancel := context.WithTimeout(baseCtx, 30*time.Minute)
	defer timeoutCancel()
	defer cancelWithCause(nil)

	if _, err := h.tasks.StartTask(conversationID, req.Message, cancelWithCause); err != nil {
		if errors.Is(err, ErrTaskAlreadyRunning) {
			sendEvent("error", "当前会话已有任务正在执行，请先停止后再尝试。", map[string]interface{}{
				"conversationId": conversationID,
			})
		} else {
			sendEvent("error", "无法启动任务: "+err.Error(), map[string]interface{}{
				"conversationId": conversationID,
			})
		}
		sendEvent("done", "", map[string]interface{}{
			"conversationId": conversationID,
		})
		return
	}

	taskStatus := "completed"
	defer h.tasks.FinishTask(conversationID, taskStatus)

	// 执行Agent Loop，传入独立的上下文，确保任务不会因客户端断开而中断
	sendEvent("progress", "正在分析您的请求...", nil)
	result, err := h.agent.AgentLoopWithProgress(taskCtx, req.Message, agentHistoryMessages, progressCallback)
	if err != nil {
		h.logger.Error("Agent Loop执行失败", zap.Error(err))
		cause := context.Cause(baseCtx)

		switch {
		case errors.Is(cause, ErrTaskCancelled):
			taskStatus = "cancelled"
			cancelMsg := "任务已被用户取消，后续操作已停止。"
			if assistantMessageID != "" {
				if _, updateErr := h.db.Exec(
					"UPDATE messages SET content = ? WHERE id = ?",
					cancelMsg,
					assistantMessageID,
				); updateErr != nil {
					h.logger.Warn("更新取消后的助手消息失败", zap.Error(updateErr))
				}
				h.db.AddProcessDetail(assistantMessageID, conversationID, "cancelled", cancelMsg, nil)
			}
			sendEvent("cancelled", cancelMsg, map[string]interface{}{
				"conversationId": conversationID,
				"messageId":      assistantMessageID,
			})
			sendEvent("done", "", map[string]interface{}{
				"conversationId": conversationID,
			})
			return
		case errors.Is(err, context.DeadlineExceeded) || errors.Is(cause, context.DeadlineExceeded):
			taskStatus = "timeout"
			timeoutMsg := "任务执行超时，已自动终止。"
			if assistantMessageID != "" {
				if _, updateErr := h.db.Exec(
					"UPDATE messages SET content = ? WHERE id = ?",
					timeoutMsg,
					assistantMessageID,
				); updateErr != nil {
					h.logger.Warn("更新超时后的助手消息失败", zap.Error(updateErr))
				}
				h.db.AddProcessDetail(assistantMessageID, conversationID, "timeout", timeoutMsg, nil)
			}
			sendEvent("error", timeoutMsg, map[string]interface{}{
				"conversationId": conversationID,
				"messageId":      assistantMessageID,
			})
			sendEvent("done", "", map[string]interface{}{
				"conversationId": conversationID,
			})
			return
		default:
			taskStatus = "failed"
			errorMsg := "执行失败: " + err.Error()
			if assistantMessageID != "" {
				if _, updateErr := h.db.Exec(
					"UPDATE messages SET content = ? WHERE id = ?",
					errorMsg,
					assistantMessageID,
				); updateErr != nil {
					h.logger.Warn("更新失败后的助手消息失败", zap.Error(updateErr))
				}
				h.db.AddProcessDetail(assistantMessageID, conversationID, "error", errorMsg, nil)
			}
			sendEvent("error", errorMsg, map[string]interface{}{
				"conversationId": conversationID,
				"messageId":      assistantMessageID,
			})
			sendEvent("done", "", map[string]interface{}{
				"conversationId": conversationID,
			})
		}
		return
	}

	// 更新助手消息内容
	if assistantMsg != nil {
		_, err = h.db.Exec(
			"UPDATE messages SET content = ?, mcp_execution_ids = ? WHERE id = ?",
			result.Response,
			func() string {
				if len(result.MCPExecutionIDs) > 0 {
					jsonData, _ := json.Marshal(result.MCPExecutionIDs)
					return string(jsonData)
				}
				return ""
			}(),
			assistantMessageID,
		)
		if err != nil {
			h.logger.Error("更新助手消息失败", zap.Error(err))
		}
	} else {
		// 如果之前创建失败，现在创建
		_, err = h.db.AddMessage(conversationID, "assistant", result.Response, result.MCPExecutionIDs)
		if err != nil {
			h.logger.Error("保存助手消息失败", zap.Error(err))
		}
	}

	// 发送最终响应
	sendEvent("response", result.Response, map[string]interface{}{
		"mcpExecutionIds": result.MCPExecutionIDs,
		"conversationId":  conversationID,
		"messageId":       assistantMessageID, // 包含消息ID，以便前端关联过程详情
	})
	sendEvent("done", "", map[string]interface{}{
		"conversationId": conversationID,
	})
}

// CancelAgentLoop 取消正在执行的任务
func (h *AgentHandler) CancelAgentLoop(c *gin.Context) {
	var req struct {
		ConversationID string `json:"conversationId" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ok, err := h.tasks.CancelTask(req.ConversationID, ErrTaskCancelled)
	if err != nil {
		h.logger.Error("取消任务失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "未找到正在执行的任务"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":         "cancelling",
		"conversationId": req.ConversationID,
		"message":        "已提交取消请求，任务将在当前步骤完成后停止。",
	})
}

// ListAgentTasks 列出所有运行中的任务
func (h *AgentHandler) ListAgentTasks(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"tasks": h.tasks.GetActiveTasks(),
	})
}
