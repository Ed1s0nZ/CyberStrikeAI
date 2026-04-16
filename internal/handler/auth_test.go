package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/database"
	"cyberstrike-ai/internal/security"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func setupAuthHandlerTest(t *testing.T) (*gin.Engine, *database.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	dbPath := filepath.Join(t.TempDir(), "auth-handler.db")
	db, err := database.NewDB(dbPath, zap.NewNop())
	if err != nil {
		t.Fatalf("database.NewDB() error = %v", err)
	}

	if err := security.EnsureBootstrapAdmin(context.Background(), db, "AdminPass123!"); err != nil {
		t.Fatalf("EnsureBootstrapAdmin() error = %v", err)
	}

	authManager := security.NewAuthManager(db, 12)
	authHandler := NewAuthHandler(authManager, &config.Config{
		Auth: config.AuthConfig{
			SessionDurationHours: 12,
		},
	}, "", zap.NewNop())

	router := gin.New()
	authRoutes := router.Group("/api/auth")
	authRoutes.POST("/login", authHandler.Login)
	authRoutes.POST("/change-password", security.AuthMiddleware(authManager), authHandler.ChangePassword)
	authRoutes.GET("/validate", security.AuthMiddleware(authManager), authHandler.Validate)

	return router, db
}

func createAuthTestUser(t *testing.T, db *database.DB, username, password string, mustChangePassword bool) {
	t.Helper()

	roleID, err := db.CreateWebAccessRole(database.CreateWebAccessRoleInput{
		Name:        "role-" + username,
		Description: "auth test role for " + username,
		Permissions: []string{security.PermissionSystemConfigRead},
		IsSystem:    false,
	})
	if err != nil {
		t.Fatalf("CreateWebAccessRole() error = %v", err)
	}

	passwordHash, err := security.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	if _, err := db.CreateWebUser(database.CreateWebUserInput{
		Username:           username,
		DisplayName:        username,
		PasswordHash:       passwordHash,
		Enabled:            true,
		MustChangePassword: mustChangePassword,
		RoleIDs:            []string{roleID},
	}); err != nil {
		t.Fatalf("CreateWebUser() error = %v", err)
	}
}

func doAuthJSONRequest(t *testing.T, router *gin.Engine, method, path string, payload any, token string) *httptest.ResponseRecorder {
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

func extractLoginToken(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()

	var response struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal(login response) error = %v", err)
	}
	if response.Token == "" {
		t.Fatalf("expected login token in response, got body=%s", w.Body.String())
	}
	return response.Token
}

func TestAuthHandler_Login_AllowsNamedNonAdminUser(t *testing.T) {
	router, db := setupAuthHandlerTest(t)
	createAuthTestUser(t, db, "reader", "ReaderPass123!", false)

	w := doAuthJSONRequest(t, router, http.MethodPost, "/api/auth/login", map[string]string{
		"username": "reader",
		"password": "ReaderPass123!",
	}, "")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	extractLoginToken(t, w)
}

func TestAuthHandler_ChangePassword_UsesCurrentAuthenticatedUser(t *testing.T) {
	router, db := setupAuthHandlerTest(t)
	createAuthTestUser(t, db, "analyst", "AnalystOld123!", true)

	login := doAuthJSONRequest(t, router, http.MethodPost, "/api/auth/login", map[string]string{
		"username": "analyst",
		"password": "AnalystOld123!",
	}, "")
	if login.Code != http.StatusOK {
		t.Fatalf("expected login 200, got %d: %s", login.Code, login.Body.String())
	}
	token := extractLoginToken(t, login)

	change := doAuthJSONRequest(t, router, http.MethodPost, "/api/auth/change-password", map[string]string{
		"oldPassword": "AnalystOld123!",
		"newPassword": "AnalystNew123!",
	}, token)
	if change.Code != http.StatusOK {
		t.Fatalf("expected change-password 200, got %d: %s", change.Code, change.Body.String())
	}

	adminLogin := doAuthJSONRequest(t, router, http.MethodPost, "/api/auth/login", map[string]string{
		"username": "admin",
		"password": "AdminPass123!",
	}, "")
	if adminLogin.Code != http.StatusOK {
		t.Fatalf("expected admin login 200 after user password change, got %d: %s", adminLogin.Code, adminLogin.Body.String())
	}
}

func TestAuthHandler_ChangePassword_InvalidatesOldPasswordAndSession(t *testing.T) {
	router, db := setupAuthHandlerTest(t)
	createAuthTestUser(t, db, "developer", "DevOldPass123!", false)

	loginOld := doAuthJSONRequest(t, router, http.MethodPost, "/api/auth/login", map[string]string{
		"username": "developer",
		"password": "DevOldPass123!",
	}, "")
	if loginOld.Code != http.StatusOK {
		t.Fatalf("expected initial login 200, got %d: %s", loginOld.Code, loginOld.Body.String())
	}
	oldToken := extractLoginToken(t, loginOld)

	change := doAuthJSONRequest(t, router, http.MethodPost, "/api/auth/change-password", map[string]string{
		"oldPassword": "DevOldPass123!",
		"newPassword": "DevNewPass123!",
	}, oldToken)
	if change.Code != http.StatusOK {
		t.Fatalf("expected change-password 200, got %d: %s", change.Code, change.Body.String())
	}

	validateOld := doAuthJSONRequest(t, router, http.MethodGet, "/api/auth/validate", nil, oldToken)
	if validateOld.Code != http.StatusUnauthorized {
		t.Fatalf("expected old token to be invalidated with 401, got %d: %s", validateOld.Code, validateOld.Body.String())
	}

	loginWithOldPassword := doAuthJSONRequest(t, router, http.MethodPost, "/api/auth/login", map[string]string{
		"username": "developer",
		"password": "DevOldPass123!",
	}, "")
	if loginWithOldPassword.Code != http.StatusUnauthorized {
		t.Fatalf("expected old password login 401, got %d: %s", loginWithOldPassword.Code, loginWithOldPassword.Body.String())
	}

	loginWithNewPassword := doAuthJSONRequest(t, router, http.MethodPost, "/api/auth/login", map[string]string{
		"username": "developer",
		"password": "DevNewPass123!",
	}, "")
	if loginWithNewPassword.Code != http.StatusOK {
		t.Fatalf("expected new password login 200, got %d: %s", loginWithNewPassword.Code, loginWithNewPassword.Body.String())
	}
}

func TestValidateReturnsCanonicalPermissions(t *testing.T) {
	router, db := setupAuthHandlerTest(t)

	roleID, err := db.CreateWebAccessRole(database.CreateWebAccessRoleInput{
		Name:        "role-canonical-validate",
		Description: "validate permissions role",
		Permissions: []string{
			security.PermissionSecurityUsersManageLegacy,
			security.PermissionSystemConfigReadLegacy,
		},
		IsSystem: false,
	})
	if err != nil {
		t.Fatalf("CreateWebAccessRole() error = %v", err)
	}

	passwordHash, err := security.HashPassword("ValidatePass123!")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	if _, err := db.CreateWebUser(database.CreateWebUserInput{
		Username:           "validate-user",
		DisplayName:        "Validate User",
		PasswordHash:       passwordHash,
		Enabled:            true,
		MustChangePassword: false,
		RoleIDs:            []string{roleID},
	}); err != nil {
		t.Fatalf("CreateWebUser() error = %v", err)
	}

	login := doAuthJSONRequest(t, router, http.MethodPost, "/api/auth/login", map[string]string{
		"username": "validate-user",
		"password": "ValidatePass123!",
	}, "")
	if login.Code != http.StatusOK {
		t.Fatalf("expected login 200, got %d: %s", login.Code, login.Body.String())
	}
	token := extractLoginToken(t, login)

	validate := doAuthJSONRequest(t, router, http.MethodGet, "/api/auth/validate", nil, token)
	if validate.Code != http.StatusOK {
		t.Fatalf("expected validate 200, got %d: %s", validate.Code, validate.Body.String())
	}

	var response struct {
		Permissions []string `json:"permissions"`
	}
	if err := json.Unmarshal(validate.Body.Bytes(), &response); err != nil {
		t.Fatalf("json.Unmarshal(validate response) error = %v", err)
	}

	want := []string{
		security.PermissionSystemConfigSettingsRead,
		security.PermissionSystemWebUserCreate,
		security.PermissionSystemWebUserCredentialReset,
		security.PermissionSystemWebUserDelete,
		security.PermissionSystemWebUserRead,
		security.PermissionSystemWebUserUpdate,
	}
	sort.Strings(want)

	if !reflect.DeepEqual(response.Permissions, want) {
		t.Fatalf("validate permissions = %#v, want %#v", response.Permissions, want)
	}
}
