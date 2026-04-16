package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sort"
	"testing"

	"cyberstrike-ai/internal/database"
	"cyberstrike-ai/internal/security"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func setupWebAuthRouter(t *testing.T) (*gin.Engine, *database.DB, *security.AuthManager) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	dbPath := filepath.Join(t.TempDir(), "web-users-handler.db")
	db, err := database.NewDB(dbPath, zap.NewNop())
	if err != nil {
		t.Fatalf("database.NewDB() error = %v", err)
	}
	if err := security.EnsureBootstrapAdmin(context.Background(), db, "LegacyPass123!"); err != nil {
		t.Fatalf("EnsureBootstrapAdmin() error = %v", err)
	}

	authManager := security.NewAuthManager(db, 12)
	authHandler := NewAuthHandler(authManager, nil, "", zap.NewNop())
	webUsersHandler := NewWebUsersHandler(db, authManager, zap.NewNop())
	webAccessRolesHandler := NewWebAccessRolesHandler(db, authManager, zap.NewNop())

	router := gin.New()
	api := router.Group("/api")
	authRoutes := api.Group("/auth")
	authRoutes.POST("/login", authHandler.Login)
	authRoutes.GET("/validate", security.AuthMiddleware(authManager), authHandler.Validate)

	protected := api.Group("")
	protected.Use(security.AuthMiddleware(authManager))
	protected.GET("/security/web-users", security.RequirePermission(security.PermissionSecurityUsersManage), webUsersHandler.ListWebUsers)
	protected.POST("/security/web-users", security.RequirePermission(security.PermissionSecurityUsersManage), webUsersHandler.CreateWebUser)
	protected.PUT("/security/web-users/:id", security.RequirePermission(security.PermissionSecurityUsersManage), webUsersHandler.UpdateWebUser)
	protected.POST("/security/web-users/:id/reset-password", security.RequirePermission(security.PermissionSecurityUsersManage), webUsersHandler.ResetWebUserPassword)
	protected.DELETE("/security/web-users/:id", security.RequirePermission(security.PermissionSecurityUsersManage), webUsersHandler.DeleteWebUser)
	protected.GET("/security/web-access-roles", security.RequirePermission(security.PermissionSecurityRolesManage), webAccessRolesHandler.ListWebAccessRoles)
	protected.POST("/security/web-access-roles", security.RequirePermission(security.PermissionSecurityRolesManage), webAccessRolesHandler.CreateWebAccessRole)
	protected.PUT("/security/web-access-roles/:id", security.RequirePermission(security.PermissionSecurityRolesManage), webAccessRolesHandler.UpdateWebAccessRole)
	protected.DELETE("/security/web-access-roles/:id", security.RequirePermission(security.PermissionSecurityRolesManage), webAccessRolesHandler.DeleteWebAccessRole)
	protected.GET("/security/web-access-roles/permission-catalog", security.RequirePermission(security.PermissionSecurityRolesManage), webAccessRolesHandler.GetPermissionCatalog)

	return router, db, authManager
}

func createWebAuthTestRole(t *testing.T, db *database.DB, name string, permissions []string) string {
	t.Helper()

	roleID, err := db.CreateWebAccessRole(database.CreateWebAccessRoleInput{
		Name:        name,
		Description: name + " description",
		Permissions: permissions,
		IsSystem:    false,
	})
	if err != nil {
		t.Fatalf("CreateWebAccessRole() error = %v", err)
	}
	return roleID
}

func createWebAuthTestUser(t *testing.T, db *database.DB, username, displayName, password string, roleIDs []string) *database.WebUserWithPermissions {
	t.Helper()

	passwordHash, err := security.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	if _, err := db.CreateWebUser(database.CreateWebUserInput{
		Username:           username,
		DisplayName:        displayName,
		PasswordHash:       passwordHash,
		Enabled:            true,
		MustChangePassword: false,
		RoleIDs:            roleIDs,
	}); err != nil {
		t.Fatalf("CreateWebUser() error = %v", err)
	}

	user, err := db.GetWebUserWithPermissionsByUsername(username)
	if err != nil {
		t.Fatalf("GetWebUserWithPermissionsByUsername(%s) error = %v", username, err)
	}
	return user
}

func mustLoginToken(t *testing.T, router *gin.Engine, username, password string) string {
	t.Helper()

	w := doJSONRequest(t, router, http.MethodPost, "/api/auth/login", map[string]string{
		"username": username,
		"password": password,
	}, "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected login 200, got %d: %s", w.Code, w.Body.String())
	}

	var response struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal(login) error = %v", err)
	}
	if response.Token == "" {
		t.Fatalf("expected non-empty token, got body=%s", w.Body.String())
	}
	return response.Token
}

func doJSONRequest(t *testing.T, router *gin.Engine, method, path string, payload any, token string) *httptest.ResponseRecorder {
	t.Helper()

	var body []byte
	var err error
	if payload != nil {
		body, err = json.Marshal(payload)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
	}

	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestWebUsersHandler_CreateUser(t *testing.T) {
	router, db, _ := setupWebAuthRouter(t)
	roleID := createWebAuthTestRole(t, db, "config-reader", []string{security.PermissionSystemConfigRead})
	adminToken := mustLoginToken(t, router, "admin", "LegacyPass123!")

	w := doJSONRequest(t, router, http.MethodPost, "/api/security/web-users", map[string]any{
		"username":    "bob",
		"displayName": "Bob",
		"password":    "Secret123!",
		"roleIds":     []string{roleID},
	}, adminToken)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	if bytes.Contains(w.Body.Bytes(), []byte("password_hash")) {
		t.Fatalf("response must not expose password material: %s", w.Body.String())
	}

	user, err := db.GetWebUserWithPermissionsByUsername("bob")
	if err != nil {
		t.Fatalf("GetWebUserWithPermissionsByUsername() error = %v", err)
	}
	if !user.MustChangePassword {
		t.Fatal("expected created user to require password change")
	}
}

func TestWebUsersHandler_ResetPasswordRevokesSessions(t *testing.T) {
	router, db, _ := setupWebAuthRouter(t)
	roleID := createWebAuthTestRole(t, db, "reset-role", []string{security.PermissionSystemConfigRead})

	passwordHash, err := security.HashPassword("OldSecret123!")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	if _, err := db.CreateWebUser(database.CreateWebUserInput{
		Username:           "reset-user",
		DisplayName:        "Reset User",
		PasswordHash:       passwordHash,
		Enabled:            true,
		MustChangePassword: false,
		RoleIDs:            []string{roleID},
	}); err != nil {
		t.Fatalf("CreateWebUser() error = %v", err)
	}

	adminToken := mustLoginToken(t, router, "admin", "LegacyPass123!")
	userToken := mustLoginToken(t, router, "reset-user", "OldSecret123!")
	user, err := db.GetWebUserWithPermissionsByUsername("reset-user")
	if err != nil {
		t.Fatalf("GetWebUserWithPermissionsByUsername() error = %v", err)
	}

	reset := doJSONRequest(t, router, http.MethodPost, "/api/security/web-users/"+user.ID+"/reset-password", map[string]string{
		"password": "NewSecret123!",
	}, adminToken)
	if reset.Code != http.StatusOK {
		t.Fatalf("expected reset password 200, got %d: %s", reset.Code, reset.Body.String())
	}

	validateOld := doJSONRequest(t, router, http.MethodGet, "/api/auth/validate", nil, userToken)
	if validateOld.Code != http.StatusUnauthorized {
		t.Fatalf("expected old session to be revoked, got %d: %s", validateOld.Code, validateOld.Body.String())
	}

	loginOld := doJSONRequest(t, router, http.MethodPost, "/api/auth/login", map[string]string{
		"username": "reset-user",
		"password": "OldSecret123!",
	}, "")
	if loginOld.Code != http.StatusUnauthorized {
		t.Fatalf("expected old password login to fail, got %d: %s", loginOld.Code, loginOld.Body.String())
	}

	loginNew := doJSONRequest(t, router, http.MethodPost, "/api/auth/login", map[string]string{
		"username": "reset-user",
		"password": "NewSecret123!",
	}, "")
	if loginNew.Code != http.StatusOK {
		t.Fatalf("expected new password login to succeed, got %d: %s", loginNew.Code, loginNew.Body.String())
	}
}

func TestWebUsersHandler_DisableLastSuperAdminRejected(t *testing.T) {
	router, db, _ := setupWebAuthRouter(t)
	adminToken := mustLoginToken(t, router, "admin", "LegacyPass123!")

	admin, err := db.GetWebUserWithPermissionsByUsername("admin")
	if err != nil {
		t.Fatalf("GetWebUserWithPermissionsByUsername(admin) error = %v", err)
	}

	w := doJSONRequest(t, router, http.MethodPut, "/api/security/web-users/"+admin.ID, map[string]any{
		"username":    admin.Username,
		"displayName": admin.DisplayName,
		"enabled":     false,
		"roleIds":     admin.RoleIDs,
	}, adminToken)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when disabling last super admin, got %d: %s", w.Code, w.Body.String())
	}
}

func TestWebUsersHandler_DisableLastSuperAdminRejected_WhenRolePermissionIsCanonicalGrant(t *testing.T) {
	router, db, _ := setupWebAuthRouter(t)
	adminToken := mustLoginToken(t, router, "admin", "LegacyPass123!")

	roles, err := db.ListWebAccessRoles()
	if err != nil {
		t.Fatalf("ListWebAccessRoles() error = %v", err)
	}

	var superAdminRole *database.WebAccessRole
	for _, role := range roles {
		if role.Name == "super-admin" {
			superAdminRole = role
			break
		}
	}
	if superAdminRole == nil {
		t.Fatal("expected bootstrap super-admin role to exist")
	}

	if _, err := db.UpdateWebAccessRole(database.UpdateWebAccessRoleInput{
		ID:          superAdminRole.ID,
		Name:        superAdminRole.Name,
		Description: superAdminRole.Description,
		Permissions: []string{security.PermissionSuperAdminGrant},
	}); err != nil {
		t.Fatalf("UpdateWebAccessRole() error = %v", err)
	}

	admin, err := db.GetWebUserWithPermissionsByUsername("admin")
	if err != nil {
		t.Fatalf("GetWebUserWithPermissionsByUsername(admin) error = %v", err)
	}

	w := doJSONRequest(t, router, http.MethodPut, "/api/security/web-users/"+admin.ID, map[string]any{
		"username":    admin.Username,
		"displayName": admin.DisplayName,
		"enabled":     false,
		"roleIds":     admin.RoleIDs,
	}, adminToken)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when disabling canonical-grant last super admin, got %d: %s", w.Code, w.Body.String())
	}
}

func TestWebAccessRolesHandler_CreateRole_DuplicateRejected(t *testing.T) {
	router, _, _ := setupWebAuthRouter(t)
	adminToken := mustLoginToken(t, router, "admin", "LegacyPass123!")

	w := doJSONRequest(t, router, http.MethodPost, "/api/security/web-access-roles", map[string]any{
		"name":        "super-admin",
		"description": "duplicate built-in role",
		"permissions": []string{security.PermissionSuperAdmin},
	}, adminToken)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected duplicate role name to be rejected with 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateWebAccessRoleRejectsLegacyPermission(t *testing.T) {
	router, _, _ := setupWebAuthRouter(t)
	adminToken := mustLoginToken(t, router, "admin", "LegacyPass123!")

	w := doJSONRequest(t, router, http.MethodPost, "/api/security/web-access-roles", map[string]any{
		"name":        "legacy-permission-role",
		"description": "uses retired permission identifiers",
		"permissions": []string{security.PermissionSecurityRolesManageLegacy},
	}, adminToken)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for legacy permissions, got %d: %s", w.Code, w.Body.String())
	}
}

func TestWebAccessRolesHandler_CreateRoleRejectsUnknownCanonicalPermission(t *testing.T) {
	router, _, _ := setupWebAuthRouter(t)
	adminToken := mustLoginToken(t, router, "admin", "LegacyPass123!")

	w := doJSONRequest(t, router, http.MethodPost, "/api/security/web-access-roles", map[string]any{
		"name":        "unknown-permission-role",
		"description": "uses unknown canonical-like identifier",
		"permissions": []string{"system.web_access_role.publish"},
	}, adminToken)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown canonical permission, got %d: %s", w.Code, w.Body.String())
	}
}

func TestWebAccessRolesHandler_UpdateRoleRejectsLegacyPermission(t *testing.T) {
	router, db, _ := setupWebAuthRouter(t)
	adminToken := mustLoginToken(t, router, "admin", "LegacyPass123!")
	roleID := createWebAuthTestRole(t, db, "update-role-legacy-reject", []string{security.PermissionSystemWebAccessRoleRead})

	w := doJSONRequest(t, router, http.MethodPut, "/api/security/web-access-roles/"+roleID, map[string]any{
		"name":        "update-role-legacy-reject",
		"description": "attempt with retired permission",
		"permissions": []string{security.PermissionSecurityRolesManageLegacy},
	}, adminToken)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for legacy permissions on update, got %d: %s", w.Code, w.Body.String())
	}
}

func TestWebAccessRolesHandler_UpdateRoleRejectsUnknownCanonicalPermission(t *testing.T) {
	router, db, _ := setupWebAuthRouter(t)
	adminToken := mustLoginToken(t, router, "admin", "LegacyPass123!")
	roleID := createWebAuthTestRole(t, db, "update-role-unknown-reject", []string{security.PermissionSystemWebAccessRoleRead})

	testCases := []struct {
		name        string
		permissions []string
	}{
		{
			name:        "unknown canonical-like identifier",
			permissions: []string{"system.web_access_role.publish"},
		},
		{
			name:        "unapproved identifier token",
			permissions: []string{"unapproved.permission.token"},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			w := doJSONRequest(t, router, http.MethodPut, "/api/security/web-access-roles/"+roleID, map[string]any{
				"name":        "update-role-unknown-reject",
				"description": "attempt with unknown/unapproved permission",
				"permissions": tc.permissions,
			}, adminToken)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected 400 for %s, got %d: %s", tc.name, w.Code, w.Body.String())
			}
			if !bytes.Contains(w.Body.Bytes(), []byte("未注册")) {
				t.Fatalf("expected unapproved permission error for %s, got: %s", tc.name, w.Body.String())
			}
		})
	}
}

func TestWebAccessRolesHandler_PermissionCatalog(t *testing.T) {
	router, _, _ := setupWebAuthRouter(t)
	adminToken := mustLoginToken(t, router, "admin", "LegacyPass123!")

	w := doJSONRequest(t, router, http.MethodGet, "/api/security/web-access-roles/permission-catalog", nil, adminToken)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var response struct {
		Domains []struct {
			Domain    string `json:"domain"`
			Resources []struct {
				Resource string `json:"resource"`
				Actions  []struct {
					Action     string `json:"action"`
					Permission string `json:"permission"`
				} `json:"actions"`
			} `json:"resources"`
		} `json:"domains"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if len(response.Domains) == 0 {
		t.Fatalf("expected non-empty permission catalog domains: %s", w.Body.String())
	}

	w2 := doJSONRequest(t, router, http.MethodGet, "/api/security/web-access-roles/permission-catalog", nil, adminToken)
	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200 from repeated catalog request, got %d: %s", w2.Code, w2.Body.String())
	}
	if !bytes.Equal(w.Body.Bytes(), w2.Body.Bytes()) {
		t.Fatalf("expected stable catalog order across requests, first=%s second=%s", w.Body.String(), w2.Body.String())
	}

	domainNames := make([]string, 0, len(response.Domains))
	foundSystemWebUserRead := false
	for _, domain := range response.Domains {
		domainNames = append(domainNames, domain.Domain)
		resourceNames := make([]string, 0, len(domain.Resources))
		for _, resource := range domain.Resources {
			resourceNames = append(resourceNames, resource.Resource)
			actionNames := make([]string, 0, len(resource.Actions))
			for _, action := range resource.Actions {
				actionNames = append(actionNames, action.Action)
				expectedPermission := domain.Domain + "." + resource.Resource + "." + action.Action
				if action.Permission != expectedPermission {
					t.Fatalf("expected permission %s, got %s", expectedPermission, action.Permission)
				}

				if domain.Domain == "system" && resource.Resource == "web_user" && action.Action == "read" {
					foundSystemWebUserRead = true
					if action.Permission != "system.web_user.read" {
						t.Fatalf("expected system.web_user.read sample permission, got %s", action.Permission)
					}
				}
			}
			assertSortedStrings(t, "actions under "+domain.Domain+"."+resource.Resource, actionNames)
		}
		assertSortedStrings(t, "resources under "+domain.Domain, resourceNames)
	}
	assertSortedStrings(t, "domains", domainNames)
	if !foundSystemWebUserRead {
		t.Fatalf("expected to find system.web_user.read in catalog response: %s", w.Body.String())
	}
}

func assertSortedStrings(t *testing.T, label string, values []string) {
	t.Helper()
	expected := append([]string(nil), values...)
	sort.Strings(expected)
	for i := range values {
		if values[i] != expected[i] {
			t.Fatalf("expected %s to be sorted lexicographically, got %v", label, values)
		}
	}
}

func TestWebAccessRolesHandler_UpdateRole_RevokesOnlyAffectedSessions(t *testing.T) {
	router, db, _ := setupWebAuthRouter(t)
	affectedRoleID := createWebAuthTestRole(t, db, "affected-role", []string{security.PermissionSystemConfigRead})
	createWebAuthTestUser(t, db, "affected-user", "Affected User", "AffectedPass123!", []string{affectedRoleID})

	adminToken := mustLoginToken(t, router, "admin", "LegacyPass123!")
	affectedToken := mustLoginToken(t, router, "affected-user", "AffectedPass123!")

	update := doJSONRequest(t, router, http.MethodPut, "/api/security/web-access-roles/"+affectedRoleID, map[string]any{
		"name":        "affected-role-updated",
		"description": "updated description",
		"permissions": []string{security.PermissionSystemConfigSettingsRead, security.PermissionSystemWebUserRead},
	}, adminToken)
	if update.Code != http.StatusOK {
		t.Fatalf("expected update role 200, got %d: %s", update.Code, update.Body.String())
	}

	adminValidate := doJSONRequest(t, router, http.MethodGet, "/api/auth/validate", nil, adminToken)
	if adminValidate.Code != http.StatusOK {
		t.Fatalf("expected admin session to stay valid, got %d: %s", adminValidate.Code, adminValidate.Body.String())
	}

	affectedValidate := doJSONRequest(t, router, http.MethodGet, "/api/auth/validate", nil, affectedToken)
	if affectedValidate.Code != http.StatusUnauthorized {
		t.Fatalf("expected affected user session to be revoked, got %d: %s", affectedValidate.Code, affectedValidate.Body.String())
	}
}

func TestWebAccessRolesHandler_DeleteRole_RevokesOnlyAffectedSessions(t *testing.T) {
	router, db, _ := setupWebAuthRouter(t)
	deletedRoleID := createWebAuthTestRole(t, db, "deleted-role", []string{security.PermissionSystemConfigRead})
	createWebAuthTestUser(t, db, "deleted-role-user", "Deleted Role User", "DeletedRolePass123!", []string{deletedRoleID})

	adminToken := mustLoginToken(t, router, "admin", "LegacyPass123!")
	affectedToken := mustLoginToken(t, router, "deleted-role-user", "DeletedRolePass123!")

	deleted := doJSONRequest(t, router, http.MethodDelete, "/api/security/web-access-roles/"+deletedRoleID, nil, adminToken)
	if deleted.Code != http.StatusOK {
		t.Fatalf("expected delete role 200, got %d: %s", deleted.Code, deleted.Body.String())
	}

	adminValidate := doJSONRequest(t, router, http.MethodGet, "/api/auth/validate", nil, adminToken)
	if adminValidate.Code != http.StatusOK {
		t.Fatalf("expected admin session to stay valid after deleting unrelated role, got %d: %s", adminValidate.Code, adminValidate.Body.String())
	}

	affectedValidate := doJSONRequest(t, router, http.MethodGet, "/api/auth/validate", nil, affectedToken)
	if affectedValidate.Code != http.StatusUnauthorized {
		t.Fatalf("expected affected user session to be revoked after role delete, got %d: %s", affectedValidate.Code, affectedValidate.Body.String())
	}

	usersList := doJSONRequest(t, router, http.MethodGet, "/api/security/web-users", nil, adminToken)
	if usersList.Code != http.StatusOK {
		t.Fatalf("expected admin to keep loading user list, got %d: %s", usersList.Code, usersList.Body.String())
	}

	rolesList := doJSONRequest(t, router, http.MethodGet, "/api/security/web-access-roles", nil, adminToken)
	if rolesList.Code != http.StatusOK {
		t.Fatalf("expected admin to keep loading role list, got %d: %s", rolesList.Code, rolesList.Body.String())
	}
}
