package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"cyberstrike-ai/internal/audit"
	"cyberstrike-ai/internal/authctx"
	"cyberstrike-ai/internal/database"
	"cyberstrike-ai/internal/security"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type RBACHandler struct {
	db     *database.DB
	logger *zap.Logger
	audit  *audit.Service
	auth   *security.AuthManager
}

func NewRBACHandler(db *database.DB, logger *zap.Logger) *RBACHandler {
	return &RBACHandler{db: db, logger: logger}
}

func (h *RBACHandler) SetAudit(s *audit.Service) {
	h.audit = s
}

func (h *RBACHandler) SetAuthManager(m *security.AuthManager) {
	h.auth = m
}

func (h *RBACHandler) Me(c *gin.Context) {
	session, _ := security.CurrentSession(c)
	resolvedScope := session.Scope
	permissionScopes := session.PermissionScopes
	if principal, ok := authctx.PrincipalFromContext(c.Request.Context()); ok {
		resolvedScope = principal.Scope
		permissionScopes = principal.PermissionScopes
	}
	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{
			"id":           session.UserID,
			"username":     session.Username,
			"display_name": session.DisplayName,
		},
		"roles":             session.Roles,
		"permissions":       permissionKeys(session.Permissions),
		"scope":             resolvedScope,
		"permission_scopes": permissionScopes,
	})
}

func (h *RBACHandler) Metadata(c *gin.Context) {
	roles, err := h.db.ListRBACRoles()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	rolePermissions := map[string][]string{}
	for _, role := range roles {
		keys, _ := h.db.ListRBACRolePermissionKeys(role.ID)
		rolePermissions[role.ID] = keys
	}
	c.JSON(http.StatusOK, gin.H{
		"permissions":      security.PermissionCatalog,
		"roles":            roles,
		"role_permissions": rolePermissions,
		"scopes":           []string{database.RBACScopeAll, database.RBACScopeAssigned, database.RBACScopeOwn},
	})
}

func (h *RBACHandler) ListRoles(c *gin.Context) {
	roles, err := h.db.ListRBACRoles()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make([]gin.H, 0, len(roles))
	for _, role := range roles {
		keys, _ := h.db.ListRBACRolePermissionKeys(role.ID)
		out = append(out, gin.H{
			"id":          role.ID,
			"name":        role.Name,
			"description": role.Description,
			"scope":       role.Scope,
			"is_system":   role.IsSystem,
			"permissions": keys,
			"created_at":  role.CreatedAt,
			"updated_at":  role.UpdatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"roles": out})
}

type upsertRBACRoleRequest struct {
	ID          string   `json:"id"`
	Name        string   `json:"name" binding:"required"`
	Description string   `json:"description"`
	Scope       string   `json:"scope"`
	Permissions []string `json:"permissions"`
}

func validateRBACPermissionKeys(keys []string) error {
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if _, ok := security.PermissionCatalog[key]; !ok {
			return fmt.Errorf("未知权限: %s", key)
		}
	}
	return nil
}

func (h *RBACHandler) CreateRole(c *gin.Context) {
	var req upsertRBACRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := validateRBACPermissionKeys(req.Permissions); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	role, err := h.db.UpsertRBACRole("", req.Name, req.Description, req.Scope, req.Permissions)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if h.audit != nil {
		h.audit.RecordOK(c, "rbac", "create_role", "创建平台角色", "role", role.ID, nil)
	}
	c.JSON(http.StatusOK, gin.H{"role": role})
}

func (h *RBACHandler) UpdateRole(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	existing, err := h.db.GetRBACRoleByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "角色不存在"})
		return
	}
	if existing.IsSystem {
		c.JSON(http.StatusBadRequest, gin.H{"error": "系统内置角色不可修改，请创建自定义角色"})
		return
	}
	var req upsertRBACRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := validateRBACPermissionKeys(req.Permissions); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	role, err := h.db.UpsertRBACRole(id, req.Name, req.Description, req.Scope, req.Permissions)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if existing.IsSystem {
		role.IsSystem = true
	}
	if h.audit != nil {
		h.audit.RecordOK(c, "rbac", "update_role", "更新平台角色", "role", id, nil)
	}
	if h.auth != nil {
		h.auth.RevokeAllSessions()
	}
	c.JSON(http.StatusOK, gin.H{"role": role})
}

func (h *RBACHandler) DeleteRole(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if err := h.db.DeleteRBACRole(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if h.audit != nil {
		h.audit.RecordOK(c, "rbac", "delete_role", "删除平台角色", "role", id, nil)
	}
	if h.auth != nil {
		h.auth.RevokeAllSessions()
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *RBACHandler) ListUsers(c *gin.Context) {
	users, err := h.db.ListRBACUsers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make([]gin.H, 0, len(users))
	for _, user := range users {
		roleIDs, _ := h.db.ListRBACUserRoleIDs(user.ID)
		out = append(out, gin.H{
			"id":           user.ID,
			"username":     user.Username,
			"display_name": user.DisplayName,
			"enabled":      user.Enabled,
			"is_builtin":   user.IsBuiltin,
			"roles":        roleIDs,
			"created_at":   user.CreatedAt,
			"updated_at":   user.UpdatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"users": out})
}

type createRBACUserRequest struct {
	Username    string   `json:"username" binding:"required"`
	DisplayName string   `json:"display_name"`
	Password    string   `json:"password" binding:"required"`
	Enabled     *bool    `json:"enabled"`
	Roles       []string `json:"roles"`
}

func (h *RBACHandler) CreateUser(c *gin.Context) {
	var req createRBACUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(strings.TrimSpace(req.Password)) < 8 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "密码长度至少需要 8 位"})
		return
	}
	hash, err := security.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	user, err := h.db.CreateRBACUser(req.Username, req.DisplayName, hash, enabled, req.Roles)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if h.audit != nil {
		h.audit.RecordOK(c, "rbac", "create_user", "创建平台用户", "user", user.ID, map[string]interface{}{"username": user.Username})
	}
	c.JSON(http.StatusOK, gin.H{"user": user})
}

type updateRBACUserRequest struct {
	DisplayName *string   `json:"display_name"`
	Password    *string   `json:"password"`
	Enabled     *bool     `json:"enabled"`
	Roles       *[]string `json:"roles"`
}

func (h *RBACHandler) UpdateUser(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	user, err := h.db.GetRBACUserByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	var req updateRBACUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	displayName := user.DisplayName
	if req.DisplayName != nil {
		displayName = *req.DisplayName
	}
	if err := h.db.UpdateRBACUser(id, displayName, req.Enabled, req.Roles); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Password != nil && strings.TrimSpace(*req.Password) != "" {
		if len(strings.TrimSpace(*req.Password)) < 8 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "密码长度至少需要 8 位"})
			return
		}
		hash, err := security.HashPassword(*req.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if err := h.db.UpdateRBACUserPassword(id, hash); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	if h.audit != nil {
		h.audit.RecordOK(c, "rbac", "update_user", "更新平台用户", "user", id, nil)
	}
	if h.auth != nil {
		h.auth.RevokeUserSessions(id)
	}
	updated, _ := h.db.GetRBACUserByID(id)
	c.JSON(http.StatusOK, gin.H{"user": updated})
}

func (h *RBACHandler) DeleteUser(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if err := h.db.DeleteRBACUser(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if h.audit != nil {
		h.audit.RecordOK(c, "rbac", "delete_user", "删除平台用户", "user", id, nil)
	}
	if h.auth != nil {
		h.auth.RevokeUserSessions(id)
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

type assignResourceRequest struct {
	UserID       string   `json:"user_id" binding:"required"`
	ResourceType string   `json:"resource_type" binding:"required"`
	ResourceID   string   `json:"resource_id"`
	ResourceIDs  []string `json:"resource_ids"`
	AutoDetect   bool     `json:"auto_detect"`
}

func (h *RBACHandler) AssignResource(c *gin.Context) {
	var req assignResourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resourceIDs := append([]string(nil), req.ResourceIDs...)
	if strings.TrimSpace(req.ResourceID) != "" {
		resourceIDs = append(resourceIDs, req.ResourceID)
	}
	if len(resourceIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "至少需要一个资源 ID"})
		return
	}
	var created int64
	var detectedTypes map[string]string
	var err error
	if req.AutoDetect {
		created, detectedTypes, err = h.db.AssignResourcesToUserAuto(req.UserID, resourceIDs)
	} else {
		created, err = h.db.AssignResourcesToUser(req.UserID, req.ResourceType, resourceIDs)
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if h.audit != nil {
		for _, resourceID := range resourceIDs {
			resourceType := req.ResourceType
			if detectedTypes != nil {
				resourceType = detectedTypes[strings.TrimSpace(resourceID)]
			}
			h.audit.RecordOK(c, "rbac", "assign_resource", "授权资源访问", resourceType, strings.TrimSpace(resourceID), map[string]interface{}{"user_id": req.UserID})
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"success":        true,
		"requested":      len(resourceIDs),
		"created":        created,
		"skipped":        int64(len(resourceIDs)) - created,
		"detected_types": detectedTypes,
	})
}

func (h *RBACHandler) ListResourceAssignments(c *gin.Context) {
	rows, err := h.db.ListRBACResourceAssignments(c.Query("user_id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"assignments": rows})
}

func (h *RBACHandler) ListAssignableResources(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if limit <= 0 || limit > 50 {
		limit = 50
	}
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if offset < 0 {
		offset = 0
	}
	resources, err := h.db.ListAssignableRBACResourcesPage(c.Query("type"), c.Query("q"), limit+1, offset)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	hasMore := len(resources) > limit
	if hasMore {
		resources = resources[:limit]
	}
	total, err := h.db.CountAssignableRBACResources(c.Query("type"), c.Query("q"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"resources": resources,
		"has_more":  hasMore,
		"limit":     limit,
		"offset":    offset,
		"total":     total,
	})
}

func (h *RBACHandler) DeleteResourceAssignment(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	assignment, err := h.db.DeleteRBACResourceAssignmentWithDetails(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if h.audit != nil {
		h.audit.RecordOK(c, "rbac", "delete_resource_assignment", "撤销资源授权", assignment.ResourceType, assignment.ResourceID, map[string]interface{}{
			"user_id":       assignment.UserID,
			"assignment_id": assignment.ID,
		})
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}
