package handler

import (
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
}

// NewAgentHandler 创建新的Agent处理器
func NewAgentHandler(agent *agent.Agent, db *database.DB, logger *zap.Logger) *AgentHandler {
	return &AgentHandler{
		agent:  agent,
		db:     db,
		logger: logger,
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
		ConversationID: conversationID,
		Time:            time.Now(),
	})
}

