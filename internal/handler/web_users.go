package handler

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"cyberstrike-ai/internal/database"
	"cyberstrike-ai/internal/security"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type createWebUserRequest struct {
	Username    string   `json:"username"`
	DisplayName string   `json:"displayName"`
	Password    string   `json:"password"`
	RoleIDs     []string `json:"roleIds"`
}

type updateWebUserRequest struct {
	Username    string   `json:"username"`
	DisplayName string   `json:"displayName"`
	Enabled     *bool    `json:"enabled"`
	RoleIDs     []string `json:"roleIds"`
}

type resetWebUserPasswordRequest struct {
	Password string `json:"password"`
}

// WebUsersHandler manages Web user CRUD APIs.
type WebUsersHandler struct {
	db     *database.DB
	auth   *security.AuthManager
	logger *zap.Logger
}

// NewWebUsersHandler creates a WebUsersHandler.
func NewWebUsersHandler(db *database.DB, auth *security.AuthManager, logger *zap.Logger) *WebUsersHandler {
	return &WebUsersHandler{db: db, auth: auth, logger: logger}
}

// ListWebUsers returns all Web users with resolved role and permission information.
func (h *WebUsersHandler) ListWebUsers(c *gin.Context) {
	users, err := h.db.ListWebUsersWithPermissions()
	if err != nil {
		h.writeServerError(c, "获取 Web 用户失败", err)
		return
	}

	items := make([]gin.H, 0, len(users))
	for _, user := range users {
		items = append(items, webUserResponse(user))
	}

	c.JSON(http.StatusOK, gin.H{"users": items})
}

// CreateWebUser creates a Web user with initial password and role assignments.
func (h *WebUsersHandler) CreateWebUser(c *gin.Context) {
	var req createWebUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数无效"})
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	req.DisplayName = strings.TrimSpace(req.DisplayName)
	if req.Username == "" || req.DisplayName == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户名、显示名称、密码不能为空"})
		return
	}
	if len(req.Password) < 8 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "密码长度至少需要 8 位"})
		return
	}
	if len(req.RoleIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "至少分配一个 Web 访问角色"})
		return
	}

	passwordHash, err := security.HashPassword(req.Password)
	if err != nil {
		h.writeServerError(c, "密码处理失败", err)
		return
	}

	user, err := h.db.CreateWebUser(database.CreateWebUserInput{
		Username:           req.Username,
		DisplayName:        req.DisplayName,
		PasswordHash:       passwordHash,
		Enabled:            true,
		MustChangePassword: true,
		RoleIDs:            req.RoleIDs,
	})
	if err != nil {
		h.writeBadRequestOrServerError(c, "创建 Web 用户失败", err)
		return
	}

	resolved, err := h.db.GetWebUserWithPermissionsByID(user.ID)
	if err != nil {
		h.writeServerError(c, "获取创建后的 Web 用户失败", err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"user": webUserResponse(resolved)})
}

// UpdateWebUser updates Web user profile, enablement, and role assignments.
func (h *WebUsersHandler) UpdateWebUser(c *gin.Context) {
	var req updateWebUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数无效"})
		return
	}

	userID := c.Param("id")
	req.Username = strings.TrimSpace(req.Username)
	req.DisplayName = strings.TrimSpace(req.DisplayName)
	if userID == "" || req.Username == "" || req.DisplayName == "" || req.Enabled == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数无效"})
		return
	}
	if len(req.RoleIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "至少分配一个 Web 访问角色"})
		return
	}

	existing, err := h.db.GetWebUserWithPermissionsByID(userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Web 用户不存在"})
			return
		}
		h.writeServerError(c, "获取 Web 用户失败", err)
		return
	}

	nextHasSuperAdmin, err := h.db.RoleIDsGrantPermission(req.RoleIDs, security.PermissionSuperAdmin)
	if err != nil {
		h.writeServerError(c, "校验 Web 访问角色失败", err)
		return
	}
	superAdminCount, err := h.db.CountEnabledUsersWithPermission(security.PermissionSuperAdmin)
	if err != nil {
		h.writeServerError(c, "校验超级管理员失败", err)
		return
	}
	if wouldRemoveLastSuperAdmin(existing, *req.Enabled, nextHasSuperAdmin, superAdminCount) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "至少保留一个启用的超级管理员"})
		return
	}

	updated, err := h.db.UpdateWebUser(database.UpdateWebUserInput{
		ID:          userID,
		Username:    req.Username,
		DisplayName: req.DisplayName,
		Enabled:     *req.Enabled,
		RoleIDs:     req.RoleIDs,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Web 用户不存在"})
			return
		}
		h.writeBadRequestOrServerError(c, "更新 Web 用户失败", err)
		return
	}

	h.auth.RevokeUserSessions(userID)
	c.JSON(http.StatusOK, gin.H{"user": webUserResponse(updated)})
}

// ResetWebUserPassword replaces a user's password and revokes their sessions.
func (h *WebUsersHandler) ResetWebUserPassword(c *gin.Context) {
	var req resetWebUserPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数无效"})
		return
	}

	userID := c.Param("id")
	if userID == "" || len(req.Password) < 8 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "密码长度至少需要 8 位"})
		return
	}

	passwordHash, err := security.HashPassword(req.Password)
	if err != nil {
		h.writeServerError(c, "密码处理失败", err)
		return
	}

	if err := h.db.UpdateWebUserPasswordByID(userID, passwordHash, true); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Web 用户不存在"})
			return
		}
		h.writeServerError(c, "重置密码失败", err)
		return
	}

	h.auth.RevokeUserSessions(userID)
	c.JSON(http.StatusOK, gin.H{"message": "密码已重置"})
}

// DeleteWebUser deletes a Web user while protecting the last enabled super admin.
func (h *WebUsersHandler) DeleteWebUser(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Web 用户 ID 不能为空"})
		return
	}

	existing, err := h.db.GetWebUserWithPermissionsByID(userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Web 用户不存在"})
			return
		}
		h.writeServerError(c, "获取 Web 用户失败", err)
		return
	}
	superAdminCount, err := h.db.CountEnabledUsersWithPermission(security.PermissionSuperAdmin)
	if err != nil {
		h.writeServerError(c, "校验超级管理员失败", err)
		return
	}
	if wouldRemoveLastSuperAdmin(existing, false, false, superAdminCount) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "至少保留一个启用的超级管理员"})
		return
	}

	if err := h.db.DeleteWebUser(userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Web 用户不存在"})
			return
		}
		h.writeServerError(c, "删除 Web 用户失败", err)
		return
	}

	h.auth.RevokeUserSessions(userID)
	c.JSON(http.StatusOK, gin.H{"message": "Web 用户已删除"})
}

func wouldRemoveLastSuperAdmin(existing *database.WebUserWithPermissions, nextEnabled bool, nextHasSuperAdmin bool, superAdminCount int) bool {
	if existing == nil || !existing.Enabled || !hasPermission(existing.Permissions, security.PermissionSuperAdmin) {
		return false
	}

	if nextEnabled && nextHasSuperAdmin {
		return false
	}

	return superAdminCount <= 1
}

func hasPermission(permissions []string, required string) bool {
	for _, permission := range permissions {
		if permission == required {
			return true
		}
	}
	return false
}

func webUserResponse(user *database.WebUserWithPermissions) gin.H {
	response := gin.H{
		"id":                 user.ID,
		"username":           user.Username,
		"displayName":        user.DisplayName,
		"enabled":            user.Enabled,
		"mustChangePassword": user.MustChangePassword,
		"roleIds":            user.RoleIDs,
		"roleNames":          user.RoleNames,
		"permissions":        user.Permissions,
		"createdAt":          user.CreatedAt,
		"updatedAt":          user.UpdatedAt,
	}
	if user.LastLoginAt.Valid {
		response["lastLoginAt"] = user.LastLoginAt.Time
	} else {
		response["lastLoginAt"] = nil
	}
	return response
}

func (h *WebUsersHandler) writeBadRequestOrServerError(c *gin.Context, message string, err error) {
	if isConstraintError(err) {
		c.JSON(http.StatusBadRequest, gin.H{"error": message + ": " + err.Error()})
		return
	}
	h.writeServerError(c, message, err)
}

func (h *WebUsersHandler) writeServerError(c *gin.Context, message string, err error) {
	if h.logger != nil {
		h.logger.Error(message, zap.Error(err))
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": message})
}

func isConstraintError(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.Contains(message, "UNIQUE constraint failed") ||
		strings.Contains(message, "FOREIGN KEY constraint failed")
}
