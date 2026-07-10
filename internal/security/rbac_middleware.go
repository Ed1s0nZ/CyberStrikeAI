package security

import (
	"net/http"
	"strings"

	"cyberstrike-ai/internal/database"

	"github.com/gin-gonic/gin"
)

// RBACMiddleware maps protected API routes to platform permissions. It keeps
// enforcement centralized so route declarations stay readable.
func RBACMiddleware(db *database.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		permission := permissionForRequest(c.Request.Method, c.FullPath())
		if permission == "" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "未配置访问权限",
			})
			return
		}
		if SessionHasPermission(c, permission) {
			if db != nil && !resourceAllowed(c, db) {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "无权访问该资源"})
				return
			}
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error":      "权限不足",
			"permission": permission,
		})
	}
}

func permissionForRequest(method, fullPath string) string {
	path := strings.TrimPrefix(fullPath, "/api")
	switch {
	case path == "/rbac/me":
		return "auth:self"
	case path == "/rbac/resources":
		// The picker enumerates resource names and IDs and is only needed by
		// administrators who can actually create assignments.
		return "rbac:write"
	case strings.HasPrefix(path, "/rbac"):
		if method == http.MethodGet {
			return "rbac:read"
		}
		return "rbac:write"
	case strings.HasPrefix(path, "/robot/wechat/status"):
		return "robot:read"
	case strings.HasPrefix(path, "/robot"):
		return "robot:write"
	case strings.HasPrefix(path, "/eino-agent"), strings.HasPrefix(path, "/multi-agent"):
		if strings.Contains(path, "/markdown-agents") {
			return crudPermission(method, "agents")
		}
		return "agent:execute"
	case strings.HasPrefix(path, "/hitl"):
		return crudPermission(method, "hitl")
	case strings.HasPrefix(path, "/agent-loop"), strings.HasPrefix(path, "/batch-tasks"):
		return crudPermission(method, "tasks")
	case strings.HasPrefix(path, "/conversations"), strings.HasPrefix(path, "/messages"), strings.HasPrefix(path, "/process-details"):
		return crudPermission(method, "chat")
	case strings.HasPrefix(path, "/groups"):
		return crudPermission(method, "group")
	case strings.HasPrefix(path, "/monitor"):
		return crudPermission(method, "monitor")
	case strings.HasPrefix(path, "/notifications"):
		if method == http.MethodGet {
			return "notification:read"
		}
		return "notification:write"
	case strings.HasPrefix(path, "/config"):
		return crudPermission(method, "config")
	case strings.HasPrefix(path, "/terminal"):
		return "terminal:execute"
	case strings.HasPrefix(path, "/audit"):
		return crudPermission(method, "audit")
	case strings.HasPrefix(path, "/external-mcp"), path == "/mcp":
		return crudPermission(method, "mcp")
	case strings.HasPrefix(path, "/attack-chain"):
		return crudPermission(method, "attackchain")
	case strings.HasPrefix(path, "/knowledge"):
		return crudPermission(method, "knowledge")
	case strings.HasPrefix(path, "/vulnerabilities"):
		return crudPermission(method, "vulnerability")
	case strings.HasPrefix(path, "/projects"):
		return crudPermission(method, "project")
	case strings.HasPrefix(path, "/webshell"):
		return crudPermission(method, "webshell")
	case strings.HasPrefix(path, "/c2"):
		return crudPermission(method, "c2")
	case strings.HasPrefix(path, "/chat-uploads"):
		return crudPermission(method, "files")
	case strings.HasPrefix(path, "/roles"):
		return crudPermission(method, "roles")
	case strings.HasPrefix(path, "/workflows"):
		return crudPermission(method, "workflow")
	case strings.HasPrefix(path, "/skills"):
		return crudPermission(method, "skills")
	case strings.HasPrefix(path, "/openapi"):
		return "openapi:read"
	case strings.HasPrefix(path, "/fofa"):
		return "fofa:execute"
	default:
		return ""
	}
}

func crudPermission(method, module string) string {
	switch method {
	case http.MethodGet, http.MethodHead:
		return module + ":read"
	case http.MethodDelete:
		return module + ":delete"
	default:
		return module + ":write"
	}
}

func resourceAllowed(c *gin.Context, db *database.DB) bool {
	session, ok := CurrentSession(c)
	if !ok || session.Scope == database.RBACScopeAll {
		return ok
	}
	path := strings.TrimPrefix(c.FullPath(), "/api")
	switch {
	case strings.HasPrefix(path, "/projects/:id"):
		return db.UserCanAccessResource(session.UserID, session.Scope, "project", c.Param("id"))
	case strings.HasPrefix(path, "/conversations/:id"):
		return db.UserCanAccessResource(session.UserID, session.Scope, "conversation", c.Param("id"))
	case strings.HasPrefix(path, "/messages/:id/process-details"):
		return db.UserCanAccessMessage(session.UserID, session.Scope, c.Param("id"))
	case strings.HasPrefix(path, "/process-details/:id"):
		return db.UserCanAccessProcessDetail(session.UserID, session.Scope, c.Param("id"))
	case strings.HasPrefix(path, "/attack-chain/:conversationId"):
		return db.UserCanAccessResource(session.UserID, session.Scope, "conversation", c.Param("conversationId"))
	case strings.HasPrefix(path, "/webshell/connections/:id"):
		return db.UserCanAccessResource(session.UserID, session.Scope, "webshell", c.Param("id"))
	case strings.HasPrefix(path, "/batch-tasks/:queueId"):
		return db.UserCanAccessResource(session.UserID, session.Scope, "batch_task", c.Param("queueId"))
	case strings.HasPrefix(path, "/vulnerabilities/:id"):
		return db.UserCanAccessResource(session.UserID, session.Scope, "vulnerability", c.Param("id"))
	case strings.HasPrefix(path, "/c2/listeners/:id"):
		return db.UserCanAccessResource(session.UserID, session.Scope, "c2_listener", c.Param("id"))
	case strings.HasPrefix(path, "/c2/sessions/:id"):
		return db.UserCanAccessResource(session.UserID, session.Scope, "c2_session", c.Param("id"))
	case strings.HasPrefix(path, "/c2/tasks/:id"):
		return db.UserCanAccessResource(session.UserID, session.Scope, "c2_task", c.Param("id"))
	default:
		return true
	}
}
