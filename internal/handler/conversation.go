package handler

import (
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

	limit, _ := strconv.Atoi(limitStr)
	offset, _ := strconv.Atoi(offsetStr)

	if limit <= 0 || limit > 100 {
		limit = 50
	}

	conversations, err := h.db.ListConversations(limit, offset)
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

	conv, err := h.db.GetConversation(id)
	if err != nil {
		h.logger.Error("获取对话失败", zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": "对话不存在"})
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

