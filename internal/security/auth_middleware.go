package security

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	ContextAuthTokenKey    = "authToken"
	ContextAuthUserIDKey   = "authUserID"
	ContextAuthUsernameKey = "authUsername"
	ContextPermissionsKey  = "authPermissions"
	ContextSessionExpiry   = "authSessionExpiry"
)

// AuthMiddleware enforces authentication on protected routes.
func AuthMiddleware(manager *AuthManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractTokenFromRequest(c)
		session, ok := manager.ValidateToken(token)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "未授权访问，请先登录",
			})
			return
		}

		c.Set(ContextAuthTokenKey, session.Token)
		c.Set(ContextAuthUserIDKey, session.UserID)
		c.Set(ContextAuthUsernameKey, session.Username)
		c.Set(ContextPermissionsKey, session.Permissions)
		c.Set(ContextSessionExpiry, session.ExpiresAt)
		c.Next()
	}
}

// RequirePermission rejects authenticated requests that lack the required RBAC permission.
func RequirePermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		rawPermissions, ok := c.Get(ContextPermissionsKey)
		if !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "权限不足",
			})
			return
		}

		permissionSet, ok := rawPermissions.(map[string]struct{})
		if !ok || !HasPermission(permissionSet, permission) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "权限不足",
			})
			return
		}

		c.Next()
	}
}

func extractTokenFromRequest(c *gin.Context) string {
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		if len(authHeader) > 7 && strings.EqualFold(authHeader[0:7], "Bearer ") {
			return strings.TrimSpace(authHeader[7:])
		}
		return strings.TrimSpace(authHeader)
	}

	if token := c.Query("token"); token != "" {
		return strings.TrimSpace(token)
	}

	if cookie, err := c.Cookie("auth_token"); err == nil {
		return strings.TrimSpace(cookie)
	}

	return ""
}
