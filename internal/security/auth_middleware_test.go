package security

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestAuthMiddleware_WritesIdentityToContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	manager := &AuthManager{
		sessionDuration: time.Hour,
		sessions: map[string]Session{
			"token-1": {
				Token:       "token-1",
				UserID:      "user-1",
				Username:    "alice",
				Permissions: map[string]struct{}{PermissionSystemConfigRead: {}},
				ExpiresAt:   time.Now().Add(time.Hour),
			},
		},
		userSessions: map[string]map[string]struct{}{
			"user-1": {
				"token-1": {},
			},
		},
	}

	router.GET("/protected", AuthMiddleware(manager), func(c *gin.Context) {
		if c.GetString(ContextAuthUserIDKey) != "user-1" {
			t.Fatalf("expected auth user id to be set")
		}
		if c.GetString(ContextAuthUsernameKey) != "alice" {
			t.Fatalf("expected auth username to be set")
		}
		rawPermissions, ok := c.Get(ContextPermissionsKey)
		if !ok {
			t.Fatalf("expected permissions to be set in context")
		}
		permissionSet, ok := rawPermissions.(map[string]struct{})
		if !ok {
			t.Fatalf("expected permissions to be map[string]struct{}, got %T", rawPermissions)
		}
		if _, ok := permissionSet[PermissionSystemConfigRead]; !ok {
			t.Fatalf("expected permission %q in context", PermissionSystemConfigRead)
		}
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer token-1")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
}

func TestRequirePermission_ReturnsForbidden(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/protected", func(c *gin.Context) {
		c.Set(ContextPermissionsKey, map[string]struct{}{PermissionSystemConfigRead: {}})
		c.Next()
	}, RequirePermission(PermissionSecurityUsersManage), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestRequireRoutePermission_UsesCanonicalRouteBinding(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/protected", func(c *gin.Context) {
		c.Set(ContextPermissionsKey, map[string]struct{}{PermissionSystemWebUserRead: {}})
		c.Next()
	}, RequireRoutePermission(http.MethodGet, "/security/web-users"), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
}

func TestRequireRoutePermission_PanicsWhenRouteUnbound(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected RequireRoutePermission to panic for unbound route")
		}
	}()

	_ = RequireRoutePermission(http.MethodGet, "/unknown/route")
}
