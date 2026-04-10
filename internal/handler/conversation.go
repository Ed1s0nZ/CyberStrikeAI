package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"cyberstrike-ai/internal/database"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ConversationHandler 对话处理器
type ConversationHandler struct {
	db     *database.DB
	logger *zap.Logger
}

// NewConversationHandler 创建新的对话处理器
func NewConversationHandler(db *database.DB, logger *zap.Logger) *ConversationHandler {
	return &ConversationHandler{
		db:     db,
		logger: logger,
	}
}

// CreateConversationRequest 创建对话请求
type CreateConversationRequest struct {
	Title string `json:"title"`
}

// CreateConversation 创建新对话
func (h *ConversationHandler) CreateConversation(c *gin.Context) {
	var req CreateConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	title := req.Title
	if title == "" {
		title = "新对话"
	}

	conv, err := h.db.CreateConversation(title)
	if err != nil {
		h.logger.Error("创建对话失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, conv)
}

// ListConversations 列出对话
func (h *ConversationHandler) ListConversations(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "50")
	offsetStr := c.DefaultQuery("offset", "0")
	search := c.Query("search") // 获取搜索参数

	limit, _ := strconv.Atoi(limitStr)
	offset, _ := strconv.Atoi(offsetStr)

	if limit <= 0 || limit > 100 {
		limit = 50
	}

	conversations, err := h.db.ListConversations(limit, offset, search)
	if err != nil {
		h.logger.Error("获取对话列表失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, conversations)
}

// GetConversation 获取对话
func (h *ConversationHandler) GetConversation(c *gin.Context) {
	id := c.Param("id")

	// 默认轻量加载，只有用户需要展开详情时再按需拉取
	// include_process_details=1/true 时返回全量 processDetails（兼容旧行为）
	includeStr := c.DefaultQuery("include_process_details", "0")
	include := includeStr == "1" || includeStr == "true" || includeStr == "yes"

	var (
		conv *database.Conversation
		err  error
	)
	if include {
		conv, err = h.db.GetConversation(id)
	} else {
		conv, err = h.db.GetConversationLite(id)
	}
	if err != nil {
		h.logger.Error("获取对话失败", zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": "对话不存在"})
		return
	}

	c.JSON(http.StatusOK, conv)
}

// GetMessageProcessDetails 获取指定消息的过程详情（按需加载）
func (h *ConversationHandler) GetMessageProcessDetails(c *gin.Context) {
	messageID := c.Param("id")
	if messageID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "message id required"})
		return
	}

	details, err := h.db.GetProcessDetails(messageID)
	if err != nil {
		h.logger.Error("获取过程详情失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 转换为前端期望的 JSON 结构（与 GetConversation 中 processDetails 结构一致）
	out := make([]map[string]interface{}, 0, len(details))
	for _, d := range details {
		var data interface{}
		if d.Data != "" {
			if err := json.Unmarshal([]byte(d.Data), &data); err != nil {
				h.logger.Warn("解析过程详情数据失败", zap.Error(err))
			}
		}
		out = append(out, map[string]interface{}{
			"id":             d.ID,
			"messageId":      d.MessageID,
			"conversationId": d.ConversationID,
			"eventType":      d.EventType,
			"message":        d.Message,
			"data":           data,
			"createdAt":      d.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{"processDetails": out})
}

// UpdateConversationRequest 更新对话请求
type UpdateConversationRequest struct {
	Title string `json:"title"`
}

// UpdateConversation 更新对话
func (h *ConversationHandler) UpdateConversation(c *gin.Context) {
	id := c.Param("id")

	var req UpdateConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "标题不能为空"})
		return
	}

	if err := h.db.UpdateConversationTitle(id, req.Title); err != nil {
		h.logger.Error("更新对话失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 返回更新后的对话
	conv, err := h.db.GetConversation(id)
	if err != nil {
		h.logger.Error("获取更新后的对话失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, conv)
}

// DeleteConversation 删除对话
func (h *ConversationHandler) DeleteConversation(c *gin.Context) {
	id := c.Param("id")

	if err := h.db.DeleteConversation(id); err != nil {
		h.logger.Error("删除对话失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// DeleteTurnRequest 删除一轮对话（POST /api/conversations/:id/delete-turn）
type DeleteTurnRequest struct {
	MessageID string `json:"messageId"`
}

// DeleteConversationTurn 删除锚点消息所在轮次（从该轮 user 到下一轮 user 之前），并清空 last_react_*。
func (h *ConversationHandler) DeleteConversationTurn(c *gin.Context) {
	conversationID := c.Param("id")
	if conversationID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "conversation id required"})
		return
	}

	var req DeleteTurnRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.MessageID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "messageId required"})
		return
	}

	if _, err := h.db.GetConversation(conversationID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "对话不存在"})
		return
	}

	deletedIDs, err := h.db.DeleteConversationTurn(conversationID, req.MessageID)
	if err != nil {
		h.logger.Warn("删除对话轮次失败",
			zap.String("conversationId", conversationID),
			zap.String("messageId", req.MessageID),
			zap.Error(err),
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"deletedMessageIds": deletedIDs,
		"message":           "ok",
	})
}

