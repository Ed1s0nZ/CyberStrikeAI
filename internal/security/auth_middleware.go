package security

import (
	"net/http"
	"strings"

	"cyberstrike-ai/internal/authctx"
	"cyberstrike-ai/internal/database"

	"github.com/gin-gonic/gin"
)

const (
	ContextAuthTokenKey  = "authToken"
	ContextSessionExpiry = "authSessionExpiry"
	ContextUserIDKey     = "authUserID"
	ContextUsernameKey   = "authUsername"
	ContextUserScopeKey  = "authUserScope"
	ContextSessionKey    = "authSession"
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
		c.Set(ContextSessionExpiry, session.ExpiresAt)
		c.Set(ContextUserIDKey, session.UserID)
		c.Set(ContextUsernameKey, session.Username)
		c.Set(ContextUserScopeKey, session.Scope)
		c.Set(ContextSessionKey, session)
		// Gin context values do not survive into Agent/MCP/background contexts.
		// Attach an immutable principal to the request context as the canonical
		// identity for every downstream execution layer.
		principal := authctx.NewPrincipalWithScopes(session.UserID, session.Username, session.Scope, session.Permissions, session.PermissionScopes)
		c.Request = c.Request.WithContext(authctx.WithPrincipal(c.Request.Context(), principal))
		c.Next()
	}
}

func RequirePermission(permission string) gin.HandlerFunc {
	permission = strings.TrimSpace(permission)
	return func(c *gin.Context) {
		if permission == "" || SessionHasPermission(c, permission) {
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error":      "权限不足",
			"permission": permission,
		})
	}
}

func RequireAnyPermission(permissions ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		for _, permission := range permissions {
			if SessionHasPermission(c, permission) {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error":       "权限不足",
			"permissions": permissions,
		})
	}
}

func RequireResourcePermission(db *database.DB, permission, resourceType, paramName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !SessionHasPermission(c, permission) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":      "权限不足",
				"permission": permission,
			})
			return
		}
		if db == nil {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "资源鉴权服务不可用"})
			return
		}
		resourceID := strings.TrimSpace(c.Param(paramName))
		if resourceID == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "资源 ID 不能为空"})
			return
		}
		session, ok := CurrentSession(c)
		if !ok || !db.UserCanAccessResource(session.UserID, session.Scope, resourceType, resourceID) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":         "无权访问该资源",
				"resource_type": resourceType,
				"resource_id":   resourceID,
			})
			return
		}
		c.Next()
	}
}

func CurrentSession(c *gin.Context) (Session, bool) {
	if c == nil {
		return Session{}, false
	}
	v, ok := c.Get(ContextSessionKey)
	if !ok {
		return Session{}, false
	}
	session, ok := v.(Session)
	return session, ok
}

func SessionHasPermission(c *gin.Context, permission string) bool {
	session, ok := CurrentSession(c)
	if !ok {
		return false
	}
	return session.Permissions[permission]
}

func extractTokenFromRequest(c *gin.Context) string {
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		if len(authHeader) > 7 && strings.EqualFold(authHeader[0:7], "Bearer ") {
			return strings.TrimSpace(authHeader[7:])
		}
		return strings.TrimSpace(authHeader)
	}

	if token := c.Query("token"); token != "" && c.Request.Method == http.MethodGet {
		acceptsSSE := strings.Contains(strings.ToLower(c.GetHeader("Accept")), "text/event-stream")
		upgradesWebSocket := strings.EqualFold(strings.TrimSpace(c.GetHeader("Upgrade")), "websocket")
		if acceptsSSE || upgradesWebSocket {
			return strings.TrimSpace(token)
		}
	}

	if cookie, err := c.Cookie("auth_token"); err == nil {
		return strings.TrimSpace(cookie)
	}

	return ""
}
