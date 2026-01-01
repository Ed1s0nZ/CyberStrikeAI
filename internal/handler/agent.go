package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"cyberstrike-ai/internal/agent"
	"cyberstrike-ai/internal/database"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// safeTruncateString 安全截断字符串，避免在 UTF-8 字符中间截断
func safeTruncateString(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= maxLen {
		return s
	}

	// 将字符串转换为 rune 切片以正确计算字符数
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}

	// 截断到最大长度
	truncated := string(runes[:maxLen])

	// 尝试在标点符号或空格处截断，使截断更自然
	// 在截断点往前查找合适的断点（不超过20%的长度）
	searchRange := maxLen / 5
	if searchRange > maxLen {
		searchRange = maxLen
	}
	breakChars := []rune("，。、 ,.;:!?！？/\\-_")
	bestBreakPos := len(runes[:maxLen])

	for i := bestBreakPos - 1; i >= bestBreakPos-searchRange && i >= 0; i-- {
		for _, breakChar := range breakChars {
			if runes[i] == breakChar {
				bestBreakPos = i + 1 // 在标点符号后断开
				goto found
			}
		}
	}

found:
	truncated = string(runes[:bestBreakPos])
	return truncated + "..."
}

// AgentHandler Agent处理器
type AgentHandler struct {
	agent            *agent.Agent
	db               *database.DB
	logger           *zap.Logger
	tasks            *AgentTaskManager
	batchTaskManager *BatchTaskManager
	knowledgeManager interface { // 知识库管理器接口
		LogRetrieval(conversationID, messageID, query, riskType string, retrievedItems []string) error
	}
}

// NewAgentHandler 创建新的Agent处理器
func NewAgentHandler(agent *agent.Agent, db *database.DB, logger *zap.Logger) *AgentHandler {
	batchTaskManager := NewBatchTaskManager()
	batchTaskManager.SetDB(db)

	// 从数据库加载所有批量任务队列
	if err := batchTaskManager.LoadFromDB(); err != nil {
		logger.Warn("从数据库加载批量任务队列失败", zap.Error(err))
	}

	return &AgentHandler{
		agent:            agent,
		db:               db,
		logger:           logger,
		tasks:            NewAgentTaskManager(),
		batchTaskManager: batchTaskManager,
	}
}

// SetKnowledgeManager 设置知识库管理器（用于记录检索日志）
func (h *AgentHandler) SetKnowledgeManager(manager interface {
	LogRetrieval(conversationID, messageID, query, riskType string, retrievedItems []string) error
}) {
	h.knowledgeManager = manager
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
		title := safeTruncateString(req.Message, 50)
		conv, err := h.db.CreateConversation(title)
		if err != nil {
			h.logger.Error("创建对话失败", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		conversationID = conv.ID
	}

	// 优先尝试从保存的ReAct数据恢复历史上下文
	agentHistoryMessages, err := h.loadHistoryFromReActData(conversationID)
	if err != nil {
		h.logger.Warn("从ReAct数据加载历史消息失败，使用消息表", zap.Error(err))
		// 回退到使用数据库消息表
		historyMessages, err := h.db.GetMessages(conversationID)
		if err != nil {
			h.logger.Warn("获取历史消息失败", zap.Error(err))
			agentHistoryMessages = []agent.ChatMessage{}
		} else {
			// 将数据库消息转换为Agent消息格式
			agentHistoryMessages = make([]agent.ChatMessage, 0, len(historyMessages))
			for _, msg := range historyMessages {
				agentHistoryMessages = append(agentHistoryMessages, agent.ChatMessage{
					Role:    msg.Role,
					Content: msg.Content,
				})
			}
			h.logger.Info("从消息表加载历史消息", zap.Int("count", len(agentHistoryMessages)))
		}
	} else {
		h.logger.Info("从ReAct数据恢复历史上下文", zap.Int("count", len(agentHistoryMessages)))
	}

	// 保存用户消息
	_, err = h.db.AddMessage(conversationID, "user", req.Message, nil)
	if err != nil {
		h.logger.Error("保存用户消息失败", zap.Error(err))
	}

	// 执行Agent Loop，传入历史消息和对话ID
	result, err := h.agent.AgentLoopWithConversationID(c.Request.Context(), req.Message, agentHistoryMessages, conversationID)
	if err != nil {
		h.logger.Error("Agent Loop执行失败", zap.Error(err))

		// 即使执行失败，也尝试保存ReAct数据（如果result中有）
		if result != nil && (result.LastReActInput != "" || result.LastReActOutput != "") {
			if saveErr := h.db.SaveReActData(conversationID, result.LastReActInput, result.LastReActOutput); saveErr != nil {
				h.logger.Warn("保存失败任务的ReAct数据失败", zap.Error(saveErr))
			} else {
				h.logger.Info("已保存失败任务的ReAct数据", zap.String("conversationId", conversationID))
			}
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 保存助手回复
	_, err = h.db.AddMessage(conversationID, "assistant", result.Response, result.MCPExecutionIDs)
	if err != nil {
		h.logger.Error("保存助手消息失败", zap.Error(err))
	}

	// 保存最后一轮ReAct的输入和输出
	if result.LastReActInput != "" || result.LastReActOutput != "" {
		if err := h.db.SaveReActData(conversationID, result.LastReActInput, result.LastReActOutput); err != nil {
			h.logger.Warn("保存ReAct数据失败", zap.Error(err))
		} else {
			h.logger.Info("已保存ReAct数据", zap.String("conversationId", conversationID))
		}
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

// createProgressCallback 创建进度回调函数，用于保存processDetails
// sendEventFunc: 可选的流式事件发送函数，如果为nil则不发送流式事件
func (h *AgentHandler) createProgressCallback(conversationID, assistantMessageID string, sendEventFunc func(eventType, message string, data interface{})) agent.ProgressCallback {
	// 用于保存tool_call事件中的参数，以便在tool_result时使用
	toolCallCache := make(map[string]map[string]interface{}) // toolCallId -> arguments

	return func(eventType, message string, data interface{}) {
		// 如果提供了sendEventFunc，发送流式事件
		if sendEventFunc != nil {
			sendEventFunc(eventType, message, data)
		}

		// 保存tool_call事件中的参数
		if eventType == "tool_call" {
			if dataMap, ok := data.(map[string]interface{}); ok {
				toolName, _ := dataMap["toolName"].(string)
				if toolName == "search_knowledge_base" {
					if toolCallId, ok := dataMap["toolCallId"].(string); ok && toolCallId != "" {
						if argumentsObj, ok := dataMap["argumentsObj"].(map[string]interface{}); ok {
							toolCallCache[toolCallId] = argumentsObj
						}
					}
				}
			}
		}

		// 处理知识检索日志记录
		if eventType == "tool_result" && h.knowledgeManager != nil {
			if dataMap, ok := data.(map[string]interface{}); ok {
				toolName, _ := dataMap["toolName"].(string)
				if toolName == "search_knowledge_base" {
					// 提取检索信息
					query := ""
					riskType := ""
					var retrievedItems []string

					// 首先尝试从tool_call缓存中获取参数
					if toolCallId, ok := dataMap["toolCallId"].(string); ok && toolCallId != "" {
						if cachedArgs, exists := toolCallCache[toolCallId]; exists {
							if q, ok := cachedArgs["query"].(string); ok && q != "" {
								query = q
							}
							if rt, ok := cachedArgs["risk_type"].(string); ok && rt != "" {
								riskType = rt
							}
							// 使用后清理缓存
							delete(toolCallCache, toolCallId)
						}
					}

					// 如果缓存中没有，尝试从argumentsObj中提取
					if query == "" {
						if arguments, ok := dataMap["argumentsObj"].(map[string]interface{}); ok {
							if q, ok := arguments["query"].(string); ok && q != "" {
								query = q
							}
							if rt, ok := arguments["risk_type"].(string); ok && rt != "" {
								riskType = rt
							}
						}
					}

					// 如果query仍然为空，尝试从result中提取（从结果文本的第一行）
					if query == "" {
						if result, ok := dataMap["result"].(string); ok && result != "" {
							// 尝试从结果中提取查询内容（如果结果包含"未找到与查询 'xxx' 相关的知识"）
							if strings.Contains(result, "未找到与查询 '") {
								start := strings.Index(result, "未找到与查询 '") + len("未找到与查询 '")
								end := strings.Index(result[start:], "'")
								if end > 0 {
									query = result[start : start+end]
								}
							}
						}
						// 如果还是为空，使用默认值
						if query == "" {
							query = "未知查询"
						}
					}

					// 从工具结果中提取检索到的知识项ID
					// 结果格式："找到 X 条相关知识：\n\n--- 结果 1 (相似度: XX.XX%) ---\n来源: [分类] 标题\n...\n<!-- METADATA: {...} -->"
					if result, ok := dataMap["result"].(string); ok && result != "" {
						// 尝试从元数据中提取知识项ID
						metadataMatch := strings.Index(result, "<!-- METADATA:")
						if metadataMatch > 0 {
							// 提取元数据JSON
							metadataStart := metadataMatch + len("<!-- METADATA: ")
							metadataEnd := strings.Index(result[metadataStart:], " -->")
							if metadataEnd > 0 {
								metadataJSON := result[metadataStart : metadataStart+metadataEnd]
								var metadata map[string]interface{}
								if err := json.Unmarshal([]byte(metadataJSON), &metadata); err == nil {
									if meta, ok := metadata["_metadata"].(map[string]interface{}); ok {
										if ids, ok := meta["retrievedItemIDs"].([]interface{}); ok {
											retrievedItems = make([]string, 0, len(ids))
											for _, id := range ids {
												if idStr, ok := id.(string); ok {
													retrievedItems = append(retrievedItems, idStr)
												}
											}
										}
									}
								}
							}
						}

						// 如果没有从元数据中提取到，但结果包含"找到 X 条"，至少标记为有结果
						if len(retrievedItems) == 0 && strings.Contains(result, "找到") && !strings.Contains(result, "未找到") {
							// 有结果，但无法准确提取ID，使用特殊标记
							retrievedItems = []string{"_has_results"}
						}
					}

					// 记录检索日志（异步，不阻塞）
					go func() {
						if err := h.knowledgeManager.LogRetrieval(conversationID, assistantMessageID, query, riskType, retrievedItems); err != nil {
							h.logger.Warn("记录知识检索日志失败", zap.Error(err))
						}
					}()

					// 添加知识检索事件到processDetails
					if assistantMessageID != "" {
						retrievalData := map[string]interface{}{
							"query":    query,
							"riskType": riskType,
							"toolName": toolName,
						}
						if err := h.db.AddProcessDetail(assistantMessageID, conversationID, "knowledge_retrieval", fmt.Sprintf("检索知识: %s", query), retrievalData); err != nil {
							h.logger.Warn("保存知识检索详情失败", zap.Error(err))
						}
					}
				}
			}
		}

		// 保存过程详情到数据库（排除response和done事件，它们会在后面单独处理）
		if assistantMessageID != "" && eventType != "response" && eventType != "done" {
			if err := h.db.AddProcessDetail(assistantMessageID, conversationID, eventType, message, data); err != nil {
				h.logger.Warn("保存过程详情失败", zap.Error(err), zap.String("eventType", eventType))
			}
		}
	}
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
		title := safeTruncateString(req.Message, 50)
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

	// 优先尝试从保存的ReAct数据恢复历史上下文
	agentHistoryMessages, err := h.loadHistoryFromReActData(conversationID)
	if err != nil {
		h.logger.Warn("从ReAct数据加载历史消息失败，使用消息表", zap.Error(err))
		// 回退到使用数据库消息表
		historyMessages, err := h.db.GetMessages(conversationID)
		if err != nil {
			h.logger.Warn("获取历史消息失败", zap.Error(err))
			agentHistoryMessages = []agent.ChatMessage{}
		} else {
			// 将数据库消息转换为Agent消息格式
			agentHistoryMessages = make([]agent.ChatMessage, 0, len(historyMessages))
			for _, msg := range historyMessages {
				agentHistoryMessages = append(agentHistoryMessages, agent.ChatMessage{
					Role:    msg.Role,
					Content: msg.Content,
				})
			}
			h.logger.Info("从消息表加载历史消息", zap.Int("count", len(agentHistoryMessages)))
		}
	} else {
		h.logger.Info("从ReAct数据恢复历史上下文", zap.Int("count", len(agentHistoryMessages)))
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

	// 创建进度回调函数，复用统一逻辑
	progressCallback := h.createProgressCallback(conversationID, assistantMessageID, sendEvent)

	// 创建一个独立的上下文用于任务执行，不随HTTP请求取消
	// 这样即使客户端断开连接（如刷新页面），任务也能继续执行
	baseCtx, cancelWithCause := context.WithCancelCause(context.Background())
	taskCtx, timeoutCancel := context.WithTimeout(baseCtx, 600*time.Minute)
	defer timeoutCancel()
	defer cancelWithCause(nil)

	if _, err := h.tasks.StartTask(conversationID, req.Message, cancelWithCause); err != nil {
		var errorMsg string
		if errors.Is(err, ErrTaskAlreadyRunning) {
			errorMsg = "⚠️ 当前会话已有任务正在执行中，请等待当前任务完成或点击「停止任务」按钮后再尝试。"
			sendEvent("error", errorMsg, map[string]interface{}{
				"conversationId": conversationID,
				"errorType":      "task_already_running",
			})
		} else {
			errorMsg = "❌ 无法启动任务: " + err.Error()
			sendEvent("error", errorMsg, map[string]interface{}{
				"conversationId": conversationID,
				"errorType":      "task_start_failed",
			})
		}

		// 更新助手消息内容并保存错误详情到数据库
		if assistantMessageID != "" {
			if _, updateErr := h.db.Exec(
				"UPDATE messages SET content = ? WHERE id = ?",
				errorMsg,
				assistantMessageID,
			); updateErr != nil {
				h.logger.Warn("更新错误后的助手消息失败", zap.Error(updateErr))
			}
			// 保存错误详情到数据库
			if err := h.db.AddProcessDetail(assistantMessageID, conversationID, "error", errorMsg, map[string]interface{}{
				"errorType": func() string {
					if errors.Is(err, ErrTaskAlreadyRunning) {
						return "task_already_running"
					}
					return "task_start_failed"
				}(),
			}); err != nil {
				h.logger.Warn("保存错误详情失败", zap.Error(err))
			}
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
	result, err := h.agent.AgentLoopWithProgress(taskCtx, req.Message, agentHistoryMessages, conversationID, progressCallback)
	if err != nil {
		h.logger.Error("Agent Loop执行失败", zap.Error(err))
		cause := context.Cause(baseCtx)

		// 检查是否是用户取消：context的cause是ErrTaskCancelled
		// 如果cause是ErrTaskCancelled，无论错误是什么类型（包括context.Canceled），都视为用户取消
		// 这样可以正确处理在API调用过程中被取消的情况
		isCancelled := errors.Is(cause, ErrTaskCancelled)

		switch {
		case isCancelled:
			taskStatus = "cancelled"
			cancelMsg := "任务已被用户取消，后续操作已停止。"

			// 在发送事件前更新任务状态，确保前端能及时看到状态变化
			h.tasks.UpdateTaskStatus(conversationID, taskStatus)

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

			// 即使任务被取消，也尝试保存ReAct数据（如果result中有）
			if result != nil && (result.LastReActInput != "" || result.LastReActOutput != "") {
				if err := h.db.SaveReActData(conversationID, result.LastReActInput, result.LastReActOutput); err != nil {
					h.logger.Warn("保存取消任务的ReAct数据失败", zap.Error(err))
				} else {
					h.logger.Info("已保存取消任务的ReAct数据", zap.String("conversationId", conversationID))
				}
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

			// 在发送事件前更新任务状态，确保前端能及时看到状态变化
			h.tasks.UpdateTaskStatus(conversationID, taskStatus)

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

			// 即使任务超时，也尝试保存ReAct数据（如果result中有）
			if result != nil && (result.LastReActInput != "" || result.LastReActOutput != "") {
				if err := h.db.SaveReActData(conversationID, result.LastReActInput, result.LastReActOutput); err != nil {
					h.logger.Warn("保存超时任务的ReAct数据失败", zap.Error(err))
				} else {
					h.logger.Info("已保存超时任务的ReAct数据", zap.String("conversationId", conversationID))
				}
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

			// 在发送事件前更新任务状态，确保前端能及时看到状态变化
			h.tasks.UpdateTaskStatus(conversationID, taskStatus)

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

			// 即使任务失败，也尝试保存ReAct数据（如果result中有）
			if result != nil && (result.LastReActInput != "" || result.LastReActOutput != "") {
				if err := h.db.SaveReActData(conversationID, result.LastReActInput, result.LastReActOutput); err != nil {
					h.logger.Warn("保存失败任务的ReAct数据失败", zap.Error(err))
				} else {
					h.logger.Info("已保存失败任务的ReAct数据", zap.String("conversationId", conversationID))
				}
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

	// 保存最后一轮ReAct的输入和输出
	if result.LastReActInput != "" || result.LastReActOutput != "" {
		if err := h.db.SaveReActData(conversationID, result.LastReActInput, result.LastReActOutput); err != nil {
			h.logger.Warn("保存ReAct数据失败", zap.Error(err))
		} else {
			h.logger.Info("已保存ReAct数据", zap.String("conversationId", conversationID))
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

// ListCompletedTasks 列出最近完成的任务历史
func (h *AgentHandler) ListCompletedTasks(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"tasks": h.tasks.GetCompletedTasks(),
	})
}

// BatchTaskRequest 批量任务请求
type BatchTaskRequest struct {
	Tasks []string `json:"tasks" binding:"required"` // 任务列表，每行一个任务
}

// CreateBatchQueue 创建批量任务队列
func (h *AgentHandler) CreateBatchQueue(c *gin.Context) {
	var req BatchTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(req.Tasks) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "任务列表不能为空"})
		return
	}

	// 过滤空任务
	validTasks := make([]string, 0, len(req.Tasks))
	for _, task := range req.Tasks {
		if task != "" {
			validTasks = append(validTasks, task)
		}
	}

	if len(validTasks) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "没有有效的任务"})
		return
	}

	queue := h.batchTaskManager.CreateBatchQueue(validTasks)
	c.JSON(http.StatusOK, gin.H{
		"queueId": queue.ID,
		"queue":   queue,
	})
}

// GetBatchQueue 获取批量任务队列
func (h *AgentHandler) GetBatchQueue(c *gin.Context) {
	queueID := c.Param("queueId")
	queue, exists := h.batchTaskManager.GetBatchQueue(queueID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "队列不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"queue": queue})
}

// ListBatchQueuesResponse 批量任务队列列表响应
type ListBatchQueuesResponse struct {
	Queues     []*BatchTaskQueue `json:"queues"`
	Total      int               `json:"total"`
	Page       int               `json:"page"`
	PageSize   int               `json:"page_size"`
	TotalPages int               `json:"total_pages"`
}

// ListBatchQueues 列出所有批量任务队列（支持筛选和分页）
func (h *AgentHandler) ListBatchQueues(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "10")
	offsetStr := c.DefaultQuery("offset", "0")
	pageStr := c.Query("page")
	status := c.Query("status")
	keyword := c.Query("keyword")

	limit, _ := strconv.Atoi(limitStr)
	offset, _ := strconv.Atoi(offsetStr)
	page := 1

	// 如果提供了page参数，优先使用page计算offset
	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
			offset = (page - 1) * limit
		}
	}

	// 限制pageSize范围
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	if offset < 0 {
		offset = 0
	}

	// 默认status为"all"
	if status == "" {
		status = "all"
	}

	// 获取队列列表和总数
	queues, total, err := h.batchTaskManager.ListQueues(limit, offset, status, keyword)
	if err != nil {
		h.logger.Error("获取批量任务队列列表失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 计算总页数
	totalPages := (total + limit - 1) / limit
	if totalPages == 0 {
		totalPages = 1
	}

	// 如果使用offset计算page，需要重新计算
	if pageStr == "" {
		page = (offset / limit) + 1
	}

	response := ListBatchQueuesResponse{
		Queues:     queues,
		Total:      total,
		Page:       page,
		PageSize:   limit,
		TotalPages: totalPages,
	}

	c.JSON(http.StatusOK, response)
}

// StartBatchQueue 开始执行批量任务队列
func (h *AgentHandler) StartBatchQueue(c *gin.Context) {
	queueID := c.Param("queueId")
	queue, exists := h.batchTaskManager.GetBatchQueue(queueID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "队列不存在"})
		return
	}

	if queue.Status != "pending" && queue.Status != "paused" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "队列状态不允许启动"})
		return
	}

	// 在后台执行批量任务
	go h.executeBatchQueue(queueID)

	h.batchTaskManager.UpdateQueueStatus(queueID, "running")
	c.JSON(http.StatusOK, gin.H{"message": "批量任务已开始执行", "queueId": queueID})
}

// PauseBatchQueue 暂停批量任务队列
func (h *AgentHandler) PauseBatchQueue(c *gin.Context) {
	queueID := c.Param("queueId")
	success := h.batchTaskManager.PauseQueue(queueID)
	if !success {
		c.JSON(http.StatusNotFound, gin.H{"error": "队列不存在或无法暂停"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "批量任务已暂停"})
}

// DeleteBatchQueue 删除批量任务队列
func (h *AgentHandler) DeleteBatchQueue(c *gin.Context) {
	queueID := c.Param("queueId")
	success := h.batchTaskManager.DeleteQueue(queueID)
	if !success {
		c.JSON(http.StatusNotFound, gin.H{"error": "队列不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "批量任务队列已删除"})
}

// UpdateBatchTask 更新批量任务消息
func (h *AgentHandler) UpdateBatchTask(c *gin.Context) {
	queueID := c.Param("queueId")
	taskID := c.Param("taskId")

	var req struct {
		Message string `json:"message" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数: " + err.Error()})
		return
	}

	if req.Message == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "任务消息不能为空"})
		return
	}

	err := h.batchTaskManager.UpdateTaskMessage(queueID, taskID, req.Message)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 返回更新后的队列信息
	queue, exists := h.batchTaskManager.GetBatchQueue(queueID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "队列不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "任务已更新", "queue": queue})
}

// AddBatchTask 添加任务到批量任务队列
func (h *AgentHandler) AddBatchTask(c *gin.Context) {
	queueID := c.Param("queueId")

	var req struct {
		Message string `json:"message" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数: " + err.Error()})
		return
	}

	if req.Message == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "任务消息不能为空"})
		return
	}

	task, err := h.batchTaskManager.AddTaskToQueue(queueID, req.Message)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 返回更新后的队列信息
	queue, exists := h.batchTaskManager.GetBatchQueue(queueID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "队列不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "任务已添加", "task": task, "queue": queue})
}

// DeleteBatchTask 删除批量任务
func (h *AgentHandler) DeleteBatchTask(c *gin.Context) {
	queueID := c.Param("queueId")
	taskID := c.Param("taskId")

	err := h.batchTaskManager.DeleteTask(queueID, taskID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 返回更新后的队列信息
	queue, exists := h.batchTaskManager.GetBatchQueue(queueID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "队列不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "任务已删除", "queue": queue})
}

// executeBatchQueue 执行批量任务队列
func (h *AgentHandler) executeBatchQueue(queueID string) {
	h.logger.Info("开始执行批量任务队列", zap.String("queueId", queueID))

	for {
		// 检查队列状态
		queue, exists := h.batchTaskManager.GetBatchQueue(queueID)
		if !exists || queue.Status == "cancelled" || queue.Status == "completed" || queue.Status == "paused" {
			break
		}

		// 获取下一个任务
		task, hasNext := h.batchTaskManager.GetNextTask(queueID)
		if !hasNext {
			// 所有任务完成
			h.batchTaskManager.UpdateQueueStatus(queueID, "completed")
			h.logger.Info("批量任务队列执行完成", zap.String("queueId", queueID))
			break
		}

		// 更新任务状态为运行中
		h.batchTaskManager.UpdateTaskStatus(queueID, task.ID, "running", "", "")

		// 创建新对话
		title := safeTruncateString(task.Message, 50)
		conv, err := h.db.CreateConversation(title)
		var conversationID string
		if err != nil {
			h.logger.Error("创建对话失败", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.Error(err))
			h.batchTaskManager.UpdateTaskStatus(queueID, task.ID, "failed", "", "创建对话失败: "+err.Error())
			h.batchTaskManager.MoveToNextTask(queueID)
			continue
		}
		conversationID = conv.ID

		// 保存conversationId到任务中（即使是运行中状态也要保存，以便查看对话）
		h.batchTaskManager.UpdateTaskStatusWithConversationID(queueID, task.ID, "running", "", "", conversationID)

		// 保存用户消息
		_, err = h.db.AddMessage(conversationID, "user", task.Message, nil)
		if err != nil {
			h.logger.Error("保存用户消息失败", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.String("conversationId", conversationID), zap.Error(err))
		}

		// 预先创建助手消息，以便关联过程详情
		assistantMsg, err := h.db.AddMessage(conversationID, "assistant", "处理中...", nil)
		if err != nil {
			h.logger.Error("创建助手消息失败", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.String("conversationId", conversationID), zap.Error(err))
			// 如果创建失败，继续执行但不保存过程详情
			assistantMsg = nil
		}

		// 创建进度回调函数，复用统一逻辑（批量任务不需要流式事件，所以传入nil）
		var assistantMessageID string
		if assistantMsg != nil {
			assistantMessageID = assistantMsg.ID
		}
		progressCallback := h.createProgressCallback(conversationID, assistantMessageID, nil)

		// 执行任务
		h.logger.Info("执行批量任务", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.String("message", task.Message), zap.String("conversationId", conversationID))

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		// 存储取消函数，以便在取消队列时能够取消当前任务
		h.batchTaskManager.SetTaskCancel(queueID, cancel)
		result, err := h.agent.AgentLoopWithProgress(ctx, task.Message, []agent.ChatMessage{}, conversationID, progressCallback)
		// 任务执行完成，清理取消函数
		h.batchTaskManager.SetTaskCancel(queueID, nil)
		cancel()

		if err != nil {
			// 检查是否是取消错误
			// 1. 直接检查是否是 context.Canceled（包括包装后的错误）
			// 2. 检查错误消息中是否包含"context canceled"或"cancelled"关键字
			// 3. 检查 result.Response 中是否包含取消相关的消息
			errStr := err.Error()
			isCancelled := errors.Is(err, context.Canceled) ||
				strings.Contains(strings.ToLower(errStr), "context canceled") ||
				strings.Contains(strings.ToLower(errStr), "context cancelled") ||
				(result != nil && result.Response != "" && (strings.Contains(result.Response, "任务已被取消") || strings.Contains(result.Response, "任务执行中断")))

			if isCancelled {
				h.logger.Info("批量任务被取消", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.String("conversationId", conversationID))
				cancelMsg := "任务已被用户取消，后续操作已停止。"
				// 如果result中有更具体的取消消息，使用它
				if result != nil && result.Response != "" && (strings.Contains(result.Response, "任务已被取消") || strings.Contains(result.Response, "任务执行中断")) {
					cancelMsg = result.Response
				}
				// 更新助手消息内容
				if assistantMessageID != "" {
					if _, updateErr := h.db.Exec(
						"UPDATE messages SET content = ? WHERE id = ?",
						cancelMsg,
						assistantMessageID,
					); updateErr != nil {
						h.logger.Warn("更新取消后的助手消息失败", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.Error(updateErr))
					}
					// 保存取消详情到数据库
					if err := h.db.AddProcessDetail(assistantMessageID, conversationID, "cancelled", cancelMsg, nil); err != nil {
						h.logger.Warn("保存取消详情失败", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.Error(err))
					}
				} else {
					// 如果没有预先创建的助手消息，创建一个新的
					_, errMsg := h.db.AddMessage(conversationID, "assistant", cancelMsg, nil)
					if errMsg != nil {
						h.logger.Warn("保存取消消息失败", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.Error(errMsg))
					}
				}
				// 保存ReAct数据（如果存在）
				if result != nil && (result.LastReActInput != "" || result.LastReActOutput != "") {
					if err := h.db.SaveReActData(conversationID, result.LastReActInput, result.LastReActOutput); err != nil {
						h.logger.Warn("保存取消任务的ReAct数据失败", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.Error(err))
					}
				}
				h.batchTaskManager.UpdateTaskStatusWithConversationID(queueID, task.ID, "cancelled", cancelMsg, "", conversationID)
			} else {
				h.logger.Error("批量任务执行失败", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.String("conversationId", conversationID), zap.Error(err))
				errorMsg := "执行失败: " + err.Error()
				// 更新助手消息内容
				if assistantMessageID != "" {
					if _, updateErr := h.db.Exec(
						"UPDATE messages SET content = ? WHERE id = ?",
						errorMsg,
						assistantMessageID,
					); updateErr != nil {
						h.logger.Warn("更新失败后的助手消息失败", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.Error(updateErr))
					}
					// 保存错误详情到数据库
					if err := h.db.AddProcessDetail(assistantMessageID, conversationID, "error", errorMsg, nil); err != nil {
						h.logger.Warn("保存错误详情失败", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.Error(err))
					}
				}
				h.batchTaskManager.UpdateTaskStatus(queueID, task.ID, "failed", "", err.Error())
			}
		} else {
			h.logger.Info("批量任务执行成功", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.String("conversationId", conversationID))

			// 更新助手消息内容
			if assistantMessageID != "" {
				mcpIDsJSON := ""
				if len(result.MCPExecutionIDs) > 0 {
					jsonData, _ := json.Marshal(result.MCPExecutionIDs)
					mcpIDsJSON = string(jsonData)
				}
				if _, updateErr := h.db.Exec(
					"UPDATE messages SET content = ?, mcp_execution_ids = ? WHERE id = ?",
					result.Response,
					mcpIDsJSON,
					assistantMessageID,
				); updateErr != nil {
					h.logger.Warn("更新助手消息失败", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.Error(updateErr))
					// 如果更新失败，尝试创建新消息
					_, err = h.db.AddMessage(conversationID, "assistant", result.Response, result.MCPExecutionIDs)
					if err != nil {
						h.logger.Error("保存助手消息失败", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.String("conversationId", conversationID), zap.Error(err))
					}
				}
			} else {
				// 如果没有预先创建的助手消息，创建一个新的
				_, err = h.db.AddMessage(conversationID, "assistant", result.Response, result.MCPExecutionIDs)
				if err != nil {
					h.logger.Error("保存助手消息失败", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.String("conversationId", conversationID), zap.Error(err))
				}
			}

			// 保存ReAct数据
			if result.LastReActInput != "" || result.LastReActOutput != "" {
				if err := h.db.SaveReActData(conversationID, result.LastReActInput, result.LastReActOutput); err != nil {
					h.logger.Warn("保存ReAct数据失败", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.Error(err))
				} else {
					h.logger.Info("已保存ReAct数据", zap.String("queueId", queueID), zap.String("taskId", task.ID), zap.String("conversationId", conversationID))
				}
			}

			// 保存结果
			h.batchTaskManager.UpdateTaskStatusWithConversationID(queueID, task.ID, "completed", result.Response, "", conversationID)
		}

		// 移动到下一个任务
		h.batchTaskManager.MoveToNextTask(queueID)

		// 检查是否被取消或暂停
		queue, _ = h.batchTaskManager.GetBatchQueue(queueID)
		if queue.Status == "cancelled" || queue.Status == "paused" {
			break
		}
	}
}

// loadHistoryFromReActData 从保存的ReAct数据恢复历史消息上下文
// 采用与攻击链生成类似的拼接逻辑：优先使用保存的last_react_input和last_react_output，若不存在则回退到消息表
func (h *AgentHandler) loadHistoryFromReActData(conversationID string) ([]agent.ChatMessage, error) {
	// 获取保存的ReAct输入和输出
	reactInputJSON, reactOutput, err := h.db.GetReActData(conversationID)
	if err != nil {
		return nil, fmt.Errorf("获取ReAct数据失败: %w", err)
	}

	// 如果last_react_input为空，回退到使用消息表（与攻击链生成逻辑一致）
	if reactInputJSON == "" {
		return nil, fmt.Errorf("ReAct数据为空，将使用消息表")
	}

	dataSource := "database_last_react_input"

	// 解析JSON格式的messages数组
	var messagesArray []map[string]interface{}
	if err := json.Unmarshal([]byte(reactInputJSON), &messagesArray); err != nil {
		return nil, fmt.Errorf("解析ReAct输入JSON失败: %w", err)
	}

	messageCount := len(messagesArray)

	h.logger.Info("使用保存的ReAct数据恢复历史上下文",
		zap.String("conversationId", conversationID),
		zap.String("dataSource", dataSource),
		zap.Int("reactInputSize", len(reactInputJSON)),
		zap.Int("messageCount", messageCount),
		zap.Int("reactOutputSize", len(reactOutput)),
	)
	// fmt.Println("messagesArray:", messagesArray)//debug

	// 转换为Agent消息格式
	agentMessages := make([]agent.ChatMessage, 0, len(messagesArray))
	for _, msgMap := range messagesArray {
		msg := agent.ChatMessage{}

		// 解析role
		if role, ok := msgMap["role"].(string); ok {
			msg.Role = role
		} else {
			continue // 跳过无效消息
		}

		// 跳过system消息（AgentLoop会重新添加）
		if msg.Role == "system" {
			continue
		}

		// 解析content
		if content, ok := msgMap["content"].(string); ok {
			msg.Content = content
		}

		// 解析tool_calls（如果存在）
		if toolCallsRaw, ok := msgMap["tool_calls"]; ok && toolCallsRaw != nil {
			if toolCallsArray, ok := toolCallsRaw.([]interface{}); ok {
				msg.ToolCalls = make([]agent.ToolCall, 0, len(toolCallsArray))
				for _, tcRaw := range toolCallsArray {
					if tcMap, ok := tcRaw.(map[string]interface{}); ok {
						toolCall := agent.ToolCall{}

						// 解析ID
						if id, ok := tcMap["id"].(string); ok {
							toolCall.ID = id
						}

						// 解析Type
						if toolType, ok := tcMap["type"].(string); ok {
							toolCall.Type = toolType
						}

						// 解析Function
						if funcMap, ok := tcMap["function"].(map[string]interface{}); ok {
							toolCall.Function = agent.FunctionCall{}

							// 解析函数名
							if name, ok := funcMap["name"].(string); ok {
								toolCall.Function.Name = name
							}

							// 解析arguments（可能是字符串或对象）
							if argsRaw, ok := funcMap["arguments"]; ok {
								if argsStr, ok := argsRaw.(string); ok {
									// 如果是字符串，解析为JSON
									var argsMap map[string]interface{}
									if err := json.Unmarshal([]byte(argsStr), &argsMap); err == nil {
										toolCall.Function.Arguments = argsMap
									}
								} else if argsMap, ok := argsRaw.(map[string]interface{}); ok {
									// 如果已经是对象，直接使用
									toolCall.Function.Arguments = argsMap
								}
							}
						}

						if toolCall.ID != "" {
							msg.ToolCalls = append(msg.ToolCalls, toolCall)
						}
					}
				}
			}
		}

		// 解析tool_call_id（tool角色消息）
		if toolCallID, ok := msgMap["tool_call_id"].(string); ok {
			msg.ToolCallID = toolCallID
		}

		agentMessages = append(agentMessages, msg)
	}

	// 如果存在last_react_output，需要将其作为最后一条assistant消息
	// 因为last_react_input是在迭代开始前保存的，不包含最后一轮的最终输出
	if reactOutput != "" {
		// 检查最后一条消息是否是assistant消息且没有tool_calls
		// 如果有tool_calls，说明后面应该还有tool消息和最终的assistant回复
		if len(agentMessages) > 0 {
			lastMsg := &agentMessages[len(agentMessages)-1]
			if strings.EqualFold(lastMsg.Role, "assistant") && len(lastMsg.ToolCalls) == 0 {
				// 最后一条是assistant消息且没有tool_calls，用最终输出更新其content
				lastMsg.Content = reactOutput
			} else {
				// 最后一条不是assistant消息，或者有tool_calls，添加最终输出作为新的assistant消息
				agentMessages = append(agentMessages, agent.ChatMessage{
					Role:    "assistant",
					Content: reactOutput,
				})
			}
		} else {
			// 如果没有消息，直接添加最终输出
			agentMessages = append(agentMessages, agent.ChatMessage{
				Role:    "assistant",
				Content: reactOutput,
			})
		}
	}

	if len(agentMessages) == 0 {
		return nil, fmt.Errorf("从ReAct数据解析的消息为空")
	}

	// 修复可能存在的失配tool消息，避免OpenAI报错
	// 这可以防止出现"messages with role 'tool' must be a response to a preceeding message with 'tool_calls'"错误
	if h.agent != nil {
		if fixed := h.agent.RepairOrphanToolMessages(&agentMessages); fixed {
			h.logger.Info("修复了从ReAct数据恢复的历史消息中的失配tool消息",
				zap.String("conversationId", conversationID),
			)
		}
	}

	h.logger.Info("从ReAct数据恢复历史消息完成",
		zap.String("conversationId", conversationID),
		zap.String("dataSource", dataSource),
		zap.Int("originalMessageCount", messageCount),
		zap.Int("finalMessageCount", len(agentMessages)),
		zap.Bool("hasReactOutput", reactOutput != ""),
	)
	fmt.Println("agentMessages:", agentMessages) //debug
	return agentMessages, nil
}
