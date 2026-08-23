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

func TestRBACMiddlewareMapsTokenUsageStatsToDashboardRead(t *testing.T) {
	if got := permissionForRequest(http.MethodGet, "/api/usage/tokens"); got != "dashboard:read" {
		t.Fatalf("token usage permission = %q, want dashboard:read", got)
	}
}

func TestMCPInvocationPermissionIsSeparateFromMCPAdministration(t *testing.T) {
	if got := permissionForRequest(http.MethodPost, "/api/mcp"); got != "mcp:execute" {
		t.Fatalf("MCP invocation permission = %q, want mcp:execute", got)
	}
	if got := permissionForRequest(http.MethodPut, "/api/external-mcp/example"); got != "mcp:write" {
		t.Fatalf("external MCP admin permission = %q, want mcp:write", got)
	}
}

func TestConfigToolsReadAllowsMCPReadWithoutConfigRead(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(ContextSessionKey, Session{
			UserID:      "viewer",
			Username:    "viewer",
			Permissions: map[string]bool{"mcp:read": true},
			Scope:       database.RBACScopeAssigned,
		})
		c.Next()
	})
	router.Use(RBACMiddleware(nil))
	router.GET("/api/config/tools", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"tools": []any{}})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/config/tools", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestWorkflowRunPermissionIsSeparateFromDefinitionManagement(t *testing.T) {
	if got := permissionForRequest(http.MethodPost, "/api/workflows/runs/run-1/resume"); got != "workflow:execute" {
		t.Fatalf("resume permission = %q, want workflow:execute", got)
	}
	if got := permissionForRequest(http.MethodPost, "/api/workflows/generate-draft"); got != "workflow:write" {
		t.Fatalf("generate draft permission = %q, want workflow:write", got)
	}
	if got := permissionForRequest(http.MethodPut, "/api/workflows/workflow-1"); got != "workflow:write" {
		t.Fatalf("definition permission = %q, want workflow:write", got)
	}
	if isProcessGlobalMutationPath("/workflows/generate-draft") {
		t.Fatalf("generate draft should not be treated as a process-global mutation")
	}
}

func TestRBACDenyHookReceivesDeniedDecision(t *testing.T) {
	gin.SetMode(gin.TestMode)
	called := false
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(ContextSessionKey, Session{UserID: "viewer", Permissions: map[string]bool{"project:read": true}, Scope: database.RBACScopeAssigned})
		c.Next()
	})
	router.Use(RBACMiddlewareWithDenyHook(nil, func(_ *gin.Context, reason, permission string) {
		called = reason == "permission_denied" && permission == "project:write"
	}))
	router.POST("/api/projects", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/projects", nil))
	if w.Code != http.StatusForbidden || !called {
		t.Fatalf("denial = status %d, hook called %v", w.Code, called)
	}
}

func TestRBACMiddlewareBindsPermissionSpecificScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(ContextSessionKey, Session{
			UserID: "mixed", Scope: database.RBACScopeAll,
			Permissions:      map[string]bool{"project:read": true, "project:write": true},
			PermissionScopes: map[string]string{"project:read": database.RBACScopeAll, "project:write": database.RBACScopeOwn},
		})
		c.Next()
	})
	router.Use(RBACMiddleware(nil))
	handler := func(c *gin.Context) {
		session, _ := CurrentSession(c)
		c.String(http.StatusOK, session.Scope)
	}
	router.GET("/api/projects/:id", handler)
	router.PUT("/api/projects/:id", handler)

	for _, tc := range []struct{ method, want string }{
		{http.MethodGet, database.RBACScopeAll},
		{http.MethodPut, database.RBACScopeOwn},
	} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(tc.method, "/api/projects/p1", nil))
		if w.Code != http.StatusOK || w.Body.String() != tc.want {
			t.Fatalf("%s scope response = %d/%q, want 200/%q", tc.method, w.Code, w.Body.String(), tc.want)
		}
	}
}

func TestRBACMiddlewareRejectsAssignedScopeForGlobalMonitorAggregates(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		method     string
		path       string
		permission string
	}{
		{method: http.MethodGet, path: "/api/monitor/stats", permission: "monitor:read"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set(ContextSessionKey, Session{
					UserID: "assigned-user", Permissions: map[string]bool{tc.permission: true}, Scope: database.RBACScopeAssigned,
				})
				c.Next()
			})
			router.Use(RBACMiddleware(&database.DB{}))
			router.Handle(tc.method, tc.path, func(c *gin.Context) { c.Status(http.StatusOK) })

			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
			if w.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want %d", w.Code, http.StatusForbidden)
			}
		})
	}
}

func TestAssignedScopeCannotMutateProcessGlobalAssets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, path := range []string{"/api/roles/demo", "/api/skills/demo", "/api/external-mcp/demo", "/api/workflows/demo", "/api/knowledge/items/demo"} {
		t.Run(path, func(t *testing.T) {
			permission := permissionForRequest(http.MethodPut, path)
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set(ContextSessionKey, Session{UserID: "operator", Scope: database.RBACScopeAssigned, Permissions: map[string]bool{permission: true}, PermissionScopes: map[string]string{permission: database.RBACScopeAssigned}})
				c.Next()
			})
			router.Use(RBACMiddleware(&database.DB{}))
			router.PUT(path, func(c *gin.Context) { c.Status(http.StatusNoContent) })
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(http.MethodPut, path, nil))
			if w.Code != http.StatusForbidden {
				t.Fatalf("global mutation status = %d, want 403", w.Code)
			}
		})
	}
}
