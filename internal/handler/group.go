package handler

import (
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"cyberstrike-ai/internal/database"
	"cyberstrike-ai/internal/security"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// GroupHandler 分组处理器
type GroupHandler struct {
	db     *database.DB
	logger *zap.Logger
}

const (
	maxGroupNameRunes = 64
	maxGroupIconRunes = 16
)

// NewGroupHandler 创建新的分组处理器
func NewGroupHandler(db *database.DB, logger *zap.Logger) *GroupHandler {
	return &GroupHandler{
		db:     db,
		logger: logger,
	}
}

func validateGroupTextField(field, value string, maxRunes int, required bool) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		if required {
			return "", errors.New(field + "不能为空")
		}
		return "", nil
	}
	if utf8.RuneCountInString(value) > maxRunes {
		return "", errors.New(field + "过长")
	}
	for _, r := range value {
		switch r {
		case '<', '>', '"', '\'', '`':
			return "", errors.New(field + "包含非法字符")
		}
		if r < 0x20 || r == 0x7f {
			return "", errors.New(field + "包含非法控制字符")
		}
	}
	return value, nil
}

func validateGroupFields(name, icon string) (string, string, error) {
	validName, err := validateGroupTextField("分组名称", name, maxGroupNameRunes, true)
	if err != nil {
		return "", "", err
	}
	validIcon, err := validateGroupTextField("分组图标", icon, maxGroupIconRunes, false)
	if err != nil {
		return "", "", err
	}
	return validName, validIcon, nil
}

// CreateGroupRequest 创建分组请求
type CreateGroupRequest struct {
	Name string `json:"name"`
	Icon string `json:"icon"`
}

// CreateGroup 创建分组
func (h *GroupHandler) CreateGroup(c *gin.Context) {
	var req CreateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	name, icon, err := validateGroupFields(req.Name, req.Icon)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	session, _ := security.CurrentSession(c)
	group, err := h.db.CreateGroup(name, icon, session.UserID)
	if err != nil {
		h.logger.Error("创建分组失败", zap.Error(err))
		// 如果是名称重复错误，返回400状态码
		if err.Error() == "分组名称已存在" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "分组名称已存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, group)
}

// ListGroups 列出所有分组
func (h *GroupHandler) ListGroups(c *gin.Context) {
	session, _ := security.CurrentSession(c)
	groups, err := h.db.ListGroupsForAccess(session.UserID, session.Scope)
	if err != nil {
		h.logger.Error("获取分组列表失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, groups)
}

// GetGroup 获取分组
func (h *GroupHandler) GetGroup(c *gin.Context) {
	id := c.Param("id")
	if !h.groupAllowed(c, id) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权访问该资源"})
		return
	}

	group, err := h.db.GetGroup(id)
	if err != nil {
		h.logger.Error("获取分组失败", zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": "分组不存在"})
		return
	}

	c.JSON(http.StatusOK, group)
}

// UpdateGroupRequest 更新分组请求
type UpdateGroupRequest struct {
	Name string `json:"name"`
	Icon string `json:"icon"`
}

// UpdateGroup 更新分组
func (h *GroupHandler) UpdateGroup(c *gin.Context) {
	id := c.Param("id")
	if !h.groupAllowed(c, id) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权访问该资源"})
		return
	}

	var req UpdateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	name, icon, err := validateGroupFields(req.Name, req.Icon)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.db.UpdateGroup(id, name, icon); err != nil {
		h.logger.Error("更新分组失败", zap.Error(err))
		// 如果是名称重复错误，返回400状态码
		if err.Error() == "分组名称已存在" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "分组名称已存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	group, err := h.db.GetGroup(id)
	if err != nil {
		h.logger.Error("获取更新后的分组失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, group)
}

// DeleteGroup 删除分组
func (h *GroupHandler) DeleteGroup(c *gin.Context) {
	id := c.Param("id")
	if !h.groupAllowed(c, id) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权访问该资源"})
		return
	}

	if err := h.db.DeleteGroup(id); err != nil {
		h.logger.Error("删除分组失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// AddConversationToGroupRequest 添加对话到分组请求
type AddConversationToGroupRequest struct {
	ConversationID string `json:"conversationId"`
	GroupID        string `json:"groupId"`
}

// AddConversationToGroup 将对话添加到分组
func (h *GroupHandler) AddConversationToGroup(c *gin.Context) {
	var req AddConversationToGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !h.groupConversationAllowed(c, req.ConversationID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权访问该资源"})
		return
	}
	if !h.groupAllowed(c, req.GroupID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权访问该分组"})
		return
	}

	if err := h.db.AddConversationToGroup(req.ConversationID, req.GroupID); err != nil {
		h.logger.Error("添加对话到分组失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "添加成功"})
}

// RemoveConversationFromGroup 从分组中移除对话
func (h *GroupHandler) RemoveConversationFromGroup(c *gin.Context) {
	conversationID := c.Param("conversationId")
	groupID := c.Param("id")
	if !h.groupAllowed(c, groupID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权访问该分组"})
		return
	}
	if !h.groupConversationAllowed(c, conversationID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权访问该资源"})
		return
	}

	if err := h.db.RemoveConversationFromGroup(conversationID, groupID); err != nil {
		h.logger.Error("从分组中移除对话失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "移除成功"})
}

// GroupConversation 分组对话响应结构
type GroupConversation struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Pinned      bool      `json:"pinned"`
	GroupPinned bool      `json:"groupPinned"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// GetGroupConversations 获取分组中的所有对话
func (h *GroupHandler) GetGroupConversations(c *gin.Context) {
	groupID := c.Param("id")
	if !h.groupAllowed(c, groupID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权访问该分组"})
		return
	}
	searchQuery := c.Query("search") // 获取搜索参数

	var conversations []*database.Conversation
	var err error

	// 如果有搜索关键词，使用搜索方法；否则使用普通方法
	if searchQuery != "" {
		conversations, err = h.db.SearchConversationsByGroup(groupID, searchQuery)
	} else {
		conversations, err = h.db.GetConversationsByGroup(groupID)
	}

	if err != nil {
		h.logger.Error("获取分组对话失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 获取每个对话在分组中的置顶状态
	groupConvs := make([]GroupConversation, 0, len(conversations))
	for _, conv := range conversations {
		if conv == nil || !h.groupConversationAllowed(c, conv.ID) {
			continue
		}
		// 查询分组内置顶状态
		var groupPinned int
		err := h.db.QueryRow(
			"SELECT COALESCE(pinned, 0) FROM conversation_group_mappings WHERE conversation_id = ? AND group_id = ?",
			conv.ID, groupID,
		).Scan(&groupPinned)
		if err != nil {
			h.logger.Warn("查询分组内置顶状态失败", zap.String("conversationId", conv.ID), zap.Error(err))
			groupPinned = 0
		}

		groupConvs = append(groupConvs, GroupConversation{
			ID:          conv.ID,
			Title:       conv.Title,
			Pinned:      conv.Pinned,
			GroupPinned: groupPinned != 0,
			CreatedAt:   conv.CreatedAt,
			UpdatedAt:   conv.UpdatedAt,
		})
	}

	c.JSON(http.StatusOK, groupConvs)
}

// GetAllMappings 批量获取所有分组映射（消除前端 N+1 请求）
func (h *GroupHandler) GetAllMappings(c *gin.Context) {
	mappings, err := h.db.GetAllGroupMappings()
	if err != nil {
		h.logger.Error("获取分组映射失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	filtered := mappings[:0]
	for _, mapping := range mappings {
		if h.groupConversationAllowed(c, mapping.ConversationID) && h.groupAllowed(c, mapping.GroupID) {
			filtered = append(filtered, mapping)
		}
	}

	c.JSON(http.StatusOK, filtered)
}

// UpdateConversationPinnedRequest 更新对话置顶状态请求
type UpdateConversationPinnedRequest struct {
	Pinned bool `json:"pinned"`
}

// UpdateConversationPinned 更新对话置顶状态
func (h *GroupHandler) UpdateConversationPinned(c *gin.Context) {
	conversationID := c.Param("id")
	if !h.groupConversationAllowed(c, conversationID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权访问该资源"})
		return
	}

	var req UpdateConversationPinnedRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.db.UpdateConversationPinned(conversationID, req.Pinned); err != nil {
		h.logger.Error("更新对话置顶状态失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "更新成功"})
}

// UpdateGroupPinnedRequest 更新分组置顶状态请求
type UpdateGroupPinnedRequest struct {
	Pinned bool `json:"pinned"`
}

// UpdateGroupPinned 更新分组置顶状态
func (h *GroupHandler) UpdateGroupPinned(c *gin.Context) {
	groupID := c.Param("id")
	if !h.groupAllowed(c, groupID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权访问该分组"})
		return
	}

	var req UpdateGroupPinnedRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.db.UpdateGroupPinned(groupID, req.Pinned); err != nil {
		h.logger.Error("更新分组置顶状态失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "更新成功"})
}

// UpdateConversationPinnedInGroupRequest 更新分组对话置顶状态请求
type UpdateConversationPinnedInGroupRequest struct {
	Pinned bool `json:"pinned"`
}

// UpdateConversationPinnedInGroup 更新对话在分组中的置顶状态
func (h *GroupHandler) UpdateConversationPinnedInGroup(c *gin.Context) {
	groupID := c.Param("id")
	conversationID := c.Param("conversationId")
	if !h.groupAllowed(c, groupID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权访问该分组"})
		return
	}
	if !h.groupConversationAllowed(c, conversationID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权访问该资源"})
		return
	}

	var req UpdateConversationPinnedInGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.db.UpdateConversationPinnedInGroup(conversationID, groupID, req.Pinned); err != nil {
		h.logger.Error("更新分组对话置顶状态失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "更新成功"})
}

func (h *GroupHandler) groupConversationAllowed(c *gin.Context, conversationID string) bool {
	session, ok := security.CurrentSession(c)
	if !ok {
		return false
	}
	return h.db.UserCanAccessResource(session.UserID, session.Scope, "conversation", conversationID)
}

func (h *GroupHandler) groupAllowed(c *gin.Context, groupID string) bool {
	session, ok := security.CurrentSession(c)
	return ok && h.db.UserCanAccessGroup(session.UserID, session.Scope, groupID)
}
