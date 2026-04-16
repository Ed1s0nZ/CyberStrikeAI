package handler

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/security"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// AuthHandler handles authentication-related endpoints.
type AuthHandler struct {
	manager    *security.AuthManager
	config     *config.Config
	configPath string
	logger     *zap.Logger
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(manager *security.AuthManager, cfg *config.Config, configPath string, logger *zap.Logger) *AuthHandler {
	return &AuthHandler{
		manager:    manager,
		config:     cfg,
		configPath: configPath,
		logger:     logger,
	}
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password" binding:"required"`
}

type changePasswordRequest struct {
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

// Login verifies password and returns a session token.
func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "密码不能为空"})
		return
	}

	username := strings.TrimSpace(req.Username)
	if username == "" {
		username = "admin"
	}

	session, err := h.manager.Authenticate(c.Request.Context(), username, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, security.ErrInvalidCredentials), errors.Is(err, security.ErrUserDisabled):
			c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		default:
			if h.logger != nil {
				h.logger.Error("登录失败", zap.Error(err), zap.String("username", username))
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "登录失败，请稍后重试"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":                session.Token,
		"username":             session.Username,
		"expires_at":           session.ExpiresAt.UTC().Format(time.RFC3339),
		"must_change_password": session.MustChangePassword,
		"session_duration_hr":  h.manager.SessionDurationHours(),
	})
}

// Logout revokes the current session token.
func (h *AuthHandler) Logout(c *gin.Context) {
	token := c.GetString(security.ContextAuthTokenKey)
	if token == "" {
		authHeader := c.GetHeader("Authorization")
		if len(authHeader) > 7 && strings.EqualFold(authHeader[:7], "Bearer ") {
			token = strings.TrimSpace(authHeader[7:])
		} else {
			token = strings.TrimSpace(authHeader)
		}
	}

	h.manager.RevokeToken(token)
	c.JSON(http.StatusOK, gin.H{"message": "已退出登录"})
}

// ChangePassword updates the login password.
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数无效"})
		return
	}

	oldPassword := strings.TrimSpace(req.OldPassword)
	newPassword := strings.TrimSpace(req.NewPassword)

	if oldPassword == "" || newPassword == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "当前密码和新密码均不能为空"})
		return
	}

	if len(newPassword) < 8 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "新密码长度至少需要 8 位"})
		return
	}

	if oldPassword == newPassword {
		c.JSON(http.StatusBadRequest, gin.H{"error": "新密码不能与旧密码相同"})
		return
	}

	token := c.GetString(security.ContextAuthTokenKey)
	session, ok := h.manager.ValidateToken(token)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "会话无效"})
		return
	}

	if err := h.manager.ChangePassword(c.Request.Context(), session.UserID, oldPassword, newPassword); err != nil {
		switch {
		case errors.Is(err, security.ErrInvalidCredentials):
			c.JSON(http.StatusBadRequest, gin.H{"error": "当前密码不正确"})
		case errors.Is(err, security.ErrUserDisabled):
			c.JSON(http.StatusForbidden, gin.H{"error": "当前用户已被禁用"})
		default:
			if h.logger != nil {
				h.logger.Error("更新认证配置失败", zap.Error(err), zap.String("userID", session.UserID))
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "更新认证配置失败"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "密码已更新，请使用新密码重新登录"})
}

// Validate returns the current session status.
func (h *AuthHandler) Validate(c *gin.Context) {
	token := c.GetString(security.ContextAuthTokenKey)
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "会话无效"})
		return
	}

	session, ok := h.manager.ValidateToken(token)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "会话已过期"})
		return
	}

	rawPermissions := make([]string, 0, len(session.Permissions))
	for permission := range session.Permissions {
		rawPermissions = append(rawPermissions, permission)
	}
	permissions := security.NormalizeWebPermissions(rawPermissions)
	if permissions == nil {
		permissions = []string{}
	}

	c.JSON(http.StatusOK, gin.H{
		"token":                session.Token,
		"username":             session.Username,
		"expires_at":           session.ExpiresAt.UTC().Format(time.RFC3339),
		"must_change_password": session.MustChangePassword,
		"permissions":          permissions,
	})
}
