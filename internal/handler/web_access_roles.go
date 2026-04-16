package handler

import (
	"database/sql"
	"errors"
	"net/http"
	"sort"
	"strings"

	"cyberstrike-ai/internal/database"
	"cyberstrike-ai/internal/security"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type webAccessRoleRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
}

// WebAccessRolesHandler manages Web RBAC role APIs.
type WebAccessRolesHandler struct {
	db     *database.DB
	auth   *security.AuthManager
	logger *zap.Logger
}

// NewWebAccessRolesHandler creates a WebAccessRolesHandler.
func NewWebAccessRolesHandler(db *database.DB, auth *security.AuthManager, logger *zap.Logger) *WebAccessRolesHandler {
	return &WebAccessRolesHandler{db: db, auth: auth, logger: logger}
}

// ListWebAccessRoles returns all Web access roles.
func (h *WebAccessRolesHandler) ListWebAccessRoles(c *gin.Context) {
	roles, err := h.db.ListWebAccessRoles()
	if err != nil {
		h.writeServerError(c, "获取 Web 访问角色失败", err)
		return
	}

	items := make([]gin.H, 0, len(roles))
	for _, role := range roles {
		items = append(items, webAccessRoleResponse(role))
	}
	c.JSON(http.StatusOK, gin.H{"roles": items})
}

// CreateWebAccessRole creates a new Web access role.
func (h *WebAccessRolesHandler) CreateWebAccessRole(c *gin.Context) {
	var req webAccessRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数无效"})
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	if req.Name == "" || len(req.Permissions) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "角色名称和权限不能为空"})
		return
	}

	validatedPermissions, err := validateCanonicalRolePermissions(req.Permissions)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	roleID, err := h.db.CreateWebAccessRole(database.CreateWebAccessRoleInput{
		Name:        req.Name,
		Description: req.Description,
		Permissions: validatedPermissions,
		IsSystem:    false,
	})
	if err != nil {
		h.writeBadRequestOrServerError(c, "创建 Web 访问角色失败", err)
		return
	}

	role, err := h.db.GetWebAccessRoleByID(roleID)
	if err != nil {
		h.writeServerError(c, "获取创建后的 Web 访问角色失败", err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"role": webAccessRoleResponse(role)})
}

// UpdateWebAccessRole updates a Web access role.
func (h *WebAccessRolesHandler) UpdateWebAccessRole(c *gin.Context) {
	var req webAccessRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数无效"})
		return
	}

	roleID := c.Param("id")
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	if roleID == "" || req.Name == "" || len(req.Permissions) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "角色名称和权限不能为空"})
		return
	}

	validatedPermissions, err := validateCanonicalRolePermissions(req.Permissions)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	existing, err := h.db.GetWebAccessRoleByID(roleID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Web 访问角色不存在"})
			return
		}
		h.writeServerError(c, "获取 Web 访问角色失败", err)
		return
	}
	if existing.IsSystem {
		c.JSON(http.StatusBadRequest, gin.H{"error": "系统内置 Web 访问角色不允许修改"})
		return
	}

	affectedUserIDs, err := h.db.ListWebUserIDsByRoleIDs([]string{roleID})
	if err != nil {
		h.writeServerError(c, "获取受影响 Web 用户失败", err)
		return
	}

	role, err := h.db.UpdateWebAccessRole(database.UpdateWebAccessRoleInput{
		ID:          roleID,
		Name:        req.Name,
		Description: req.Description,
		Permissions: validatedPermissions,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Web 访问角色不存在"})
			return
		}
		h.writeBadRequestOrServerError(c, "更新 Web 访问角色失败", err)
		return
	}

	for _, userID := range affectedUserIDs {
		h.auth.RevokeUserSessions(userID)
	}
	c.JSON(http.StatusOK, gin.H{"role": webAccessRoleResponse(role)})
}

// DeleteWebAccessRole deletes a Web access role.
func (h *WebAccessRolesHandler) DeleteWebAccessRole(c *gin.Context) {
	roleID := c.Param("id")
	if roleID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Web 访问角色 ID 不能为空"})
		return
	}

	role, err := h.db.GetWebAccessRoleByID(roleID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Web 访问角色不存在"})
			return
		}
		h.writeServerError(c, "获取 Web 访问角色失败", err)
		return
	}
	if role.IsSystem {
		c.JSON(http.StatusBadRequest, gin.H{"error": "系统内置 Web 访问角色不允许删除"})
		return
	}

	affectedUserIDs, err := h.db.ListWebUserIDsByRoleIDs([]string{roleID})
	if err != nil {
		h.writeServerError(c, "获取受影响 Web 用户失败", err)
		return
	}

	if err := h.db.DeleteWebAccessRole(roleID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Web 访问角色不存在"})
			return
		}
		h.writeServerError(c, "删除 Web 访问角色失败", err)
		return
	}

	for _, userID := range affectedUserIDs {
		h.auth.RevokeUserSessions(userID)
	}
	c.JSON(http.StatusOK, gin.H{"message": "Web 访问角色已删除"})
}

// GetPermissionCatalog returns canonical grouped permissions for RBAC role editing.
func (h *WebAccessRolesHandler) GetPermissionCatalog(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"domains": security.CanonicalWebPermissionCatalog(),
	})
}

func webAccessRoleResponse(role *database.WebAccessRole) gin.H {
	return gin.H{
		"id":          role.ID,
		"name":        role.Name,
		"description": role.Description,
		"isSystem":    role.IsSystem,
		"permissions": role.Permissions,
		"createdAt":   role.CreatedAt,
		"updatedAt":   role.UpdatedAt,
	}
}

func (h *WebAccessRolesHandler) writeBadRequestOrServerError(c *gin.Context, message string, err error) {
	if isConstraintError(err) {
		c.JSON(http.StatusBadRequest, gin.H{"error": message + ": " + err.Error()})
		return
	}
	h.writeServerError(c, message, err)
}

func (h *WebAccessRolesHandler) writeServerError(c *gin.Context, message string, err error) {
	if h.logger != nil {
		h.logger.Error(message, zap.Error(err))
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": message})
}

func validateCanonicalRolePermissions(input []string) ([]string, error) {
	if len(input) == 0 {
		return nil, database.ErrWebAccessRolePermissionsEmpty
	}

	canonical := make([]string, 0, len(input))
	seen := make(map[string]struct{}, len(input))
	retired := make([]string, 0)
	unapproved := make([]string, 0)

	for _, permission := range input {
		permission = strings.TrimSpace(permission)
		if permission == "" {
			unapproved = append(unapproved, permission)
			continue
		}

		if security.IsCanonicalWebPermission(permission) {
			if _, ok := seen[permission]; ok {
				continue
			}
			seen[permission] = struct{}{}
			canonical = append(canonical, permission)
			continue
		}

		if len(security.NormalizeWebPermissions([]string{permission})) > 0 {
			retired = append(retired, permission)
			continue
		}

		unapproved = append(unapproved, permission)
	}

	if len(retired) > 0 {
		sort.Strings(retired)
		return nil, errors.New("permissions 包含已废弃标识: " + strings.Join(retired, ", "))
	}
	if len(unapproved) > 0 {
		sort.Strings(unapproved)
		return nil, errors.New("permissions 包含未注册标识: " + strings.Join(unapproved, ", "))
	}
	if len(canonical) == 0 {
		return nil, database.ErrWebAccessRolePermissionsEmpty
	}

	sort.Strings(canonical)
	return canonical, nil
}
