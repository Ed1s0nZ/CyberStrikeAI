package security

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"cyberstrike-ai/internal/database"

	"github.com/gin-gonic/gin"
)

func TestRBACMiddlewareUsesMatchedFullPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(ContextSessionKey, Session{
			UserID:      "u1",
			Username:    "operator",
			Permissions: map[string]bool{"project:read": true},
			Scope:       database.RBACScopeAll,
		})
		c.Next()
	})
	router.Use(RBACMiddleware(nil))
	router.GET("/api/projects/:id", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/projects/p1", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestRBACMiddlewareRejectsMissingPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(ContextSessionKey, Session{
			UserID:      "u1",
			Username:    "viewer",
			Permissions: map[string]bool{"project:read": true},
			Scope:       database.RBACScopeAll,
		})
		c.Next()
	})
	router.Use(RBACMiddleware(nil))
	router.POST("/api/projects", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/projects", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestRBACMiddlewareRejectsUnmappedProtectedRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(ContextSessionKey, Session{
			UserID:      "u1",
			Username:    "admin",
			Permissions: allPermissions(),
			Scope:       database.RBACScopeAll,
		})
		c.Next()
	})
	router.Use(RBACMiddleware(nil))
	router.GET("/api/new-module", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/new-module", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestRBACMiddlewareMapsOpenAPISpec(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(ContextSessionKey, Session{
			UserID:      "u1",
			Username:    "viewer",
			Permissions: map[string]bool{"openapi:read": true},
			Scope:       database.RBACScopeAll,
		})
		c.Next()
	})
	router.Use(RBACMiddleware(nil))
	router.GET("/api/openapi/spec", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/openapi/spec", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestRBACResourcePickerRequiresWritePermission(t *testing.T) {
	if got := permissionForRequest(http.MethodGet, "/api/rbac/resources"); got != "rbac:write" {
		t.Fatalf("picker permission = %q, want rbac:write", got)
	}
	if got := permissionForRequest(http.MethodGet, "/api/rbac/resource-assignments"); got != "rbac:read" {
		t.Fatalf("assignment list permission = %q, want rbac:read", got)
	}
}
