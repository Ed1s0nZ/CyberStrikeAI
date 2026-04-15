# Web User RBAC Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace shared-password control-plane access with durable Web users and RBAC-managed permissions, while keeping existing AI Agent roles separate.

**Architecture:** Add a SQLite-backed Web auth domain for users, Web access roles, permissions, and assignments. Refactor `AuthManager` to authenticate named users, attach identity and permission context to requests, then layer new admin APIs and system-settings UI on top. Keep AI Agent roles under `roles/` untouched and use distinct “Web access roles” naming everywhere in APIs and UI.

**Tech Stack:** Go, Gin, SQLite (`github.com/mattn/go-sqlite3`), bcrypt (`golang.org/x/crypto/bcrypt`), vanilla JavaScript, existing HTML templates, existing i18n JSON files

---

## File Structure

- Create `internal/database/web_auth.go` for Web-user, Web-access-role, permission, and assignment persistence.
- Create `internal/database/web_auth_test.go` for SQLite table and repository coverage.
- Modify `internal/database/database.go` to create and migrate Web auth tables during startup.
- Create `internal/security/passwords.go` for password hashing and verification helpers.
- Create `internal/security/permissions.go` for RBAC permission constants and route-level checks.
- Modify `internal/security/auth_manager.go` to authenticate named Web users and store user-aware sessions.
- Modify `internal/security/auth_middleware.go` to publish `user_id`, `username`, and permissions into Gin context.
- Create `internal/security/auth_manager_test.go` and `internal/security/auth_middleware_test.go` for session and middleware behavior.
- Create `internal/security/bootstrap.go` for first-run `admin` seeding from legacy `auth.password`.
- Create `internal/security/bootstrap_test.go` for migration-safe bootstrap coverage.
- Modify `internal/handler/auth.go` for username/password login and per-user password change.
- Create `internal/handler/web_users.go` for Web user CRUD and password reset APIs.
- Create `internal/handler/web_access_roles.go` for Web access role CRUD APIs.
- Create `internal/handler/web_users_test.go` for user-management and Web-access-role handler coverage.
- Modify `internal/app/app.go` to initialize bootstrap, updated auth manager, and new protected routes.
- Modify `internal/handler/openapi.go` to document the new request bodies and RBAC endpoints.
- Create `internal/handler/openapi_test.go` to pin the OpenAPI contract.
- Modify `web/templates/index.html` to add username login and security-section management panels.
- Modify `web/static/js/auth.js` to submit username/password and keep current-user metadata.
- Modify `web/static/js/settings.js` to support security sub-panels and self-service password flow.
- Create `web/static/js/web-users.js` for Web-user and Web-access-role settings UI logic.
- Modify `web/static/css/style.css` to style the new security management lists and forms.
- Modify `web/static/i18n/zh-CN.json` and `web/static/i18n/en-US.json` for login, security, and RBAC copy.
- Modify `README.md` to explain Web users versus AI Agent roles.
- Create `docs/web-user-rbac-smoke.md` for repeatable operator smoke verification.

## Scope Check

This spec is still one implementation plan, not multiple independent plans, because the backend auth model, RBAC middleware, admin APIs, and system-settings UI are sequentially dependent. Splitting them now would create incomplete software that cannot be exercised end-to-end.

### Task 1: Add Web auth tables and repository methods

**Files:**
- Create: `internal/database/web_auth.go`
- Create: `internal/database/web_auth_test.go`
- Modify: `internal/database/database.go`
- Test: `internal/database/web_auth_test.go`

- [ ] **Step 1: Write the failing database tests**

```go
package database

import (
	"path/filepath"
	"testing"

	"go.uber.org/zap"
)

func openTestWebAuthDB(t *testing.T) *DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "web-auth.db")
	db, err := NewDB(dbPath, zap.NewNop())
	if err != nil {
		t.Fatalf("NewDB() error = %v", err)
	}
	return db
}

func TestDB_InitTables_CreatesWebAuthTables(t *testing.T) {
	db := openTestWebAuthDB(t)

	for _, tableName := range []string{
		"web_users",
		"web_access_roles",
		"web_access_role_permissions",
		"web_user_role_bindings",
	} {
		var count int
		if err := db.QueryRow(
			`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`,
			tableName,
		).Scan(&count); err != nil {
			t.Fatalf("query sqlite_master for %s: %v", tableName, err)
		}
		if count != 1 {
			t.Fatalf("expected table %s to exist", tableName)
		}
	}
}

func TestWebAuthStore_CreateUserAndResolvePermissions(t *testing.T) {
	db := openTestWebAuthDB(t)

	roleID, err := db.CreateWebAccessRole(CreateWebAccessRoleInput{
		Name:        "super-admin",
		Description: "Full control plane access",
		Permissions: []string{"system.super_admin"},
		IsSystem:    true,
	})
	if err != nil {
		t.Fatalf("CreateWebAccessRole() error = %v", err)
	}

	roleID2, err := db.CreateWebAccessRole(CreateWebAccessRoleInput{
		Name:        "config-reader",
		Description: "Read configuration",
		Permissions: []string{"system.config.read"},
		IsSystem:    false,
	})
	if err != nil {
		t.Fatalf("CreateWebAccessRole(config-reader) error = %v", err)
	}

	_, err = db.CreateWebUser(CreateWebUserInput{
		Username:          "alice",
		DisplayName:       "Alice",
		PasswordHash:      "hashed-value",
		Enabled:           true,
		MustChangePassword: true,
		RoleIDs:           []string{roleID, roleID2},
	})
	if err != nil {
		t.Fatalf("CreateWebUser() error = %v", err)
	}

	user, err := db.GetWebUserWithPermissionsByUsername("alice")
	if err != nil {
		t.Fatalf("GetWebUserWithPermissionsByUsername() error = %v", err)
	}
	if user.Username != "alice" {
		t.Fatalf("expected username alice, got %s", user.Username)
	}
	if len(user.Permissions) != 2 {
		t.Fatalf("expected two effective permissions, got %#v", user.Permissions)
	}
}
```

- [ ] **Step 2: Run the database tests and confirm they fail**

Run: `go test ./internal/database -run 'TestDB_InitTables_CreatesWebAuthTables|TestWebAuthStore_CreateUserAndResolvePermissions' -v`
Expected: `FAIL` with missing tables or `undefined: CreateWebAccessRoleInput` / `undefined: (*DB).CreateWebUser`.

- [ ] **Step 3: Add the tables and repository implementation**

```go
// internal/database/database.go
createWebUsersTable := `
CREATE TABLE IF NOT EXISTS web_users (
	id TEXT PRIMARY KEY,
	username TEXT NOT NULL UNIQUE,
	display_name TEXT NOT NULL,
	password_hash TEXT NOT NULL,
	enabled INTEGER NOT NULL DEFAULT 1,
	must_change_password INTEGER NOT NULL DEFAULT 0,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	last_login_at DATETIME
);`

createWebAccessRolesTable := `
CREATE TABLE IF NOT EXISTS web_access_roles (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL UNIQUE,
	description TEXT NOT NULL DEFAULT '',
	is_system INTEGER NOT NULL DEFAULT 0,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);`

createWebAccessRolePermissionsTable := `
CREATE TABLE IF NOT EXISTS web_access_role_permissions (
	role_id TEXT NOT NULL,
	permission TEXT NOT NULL,
	PRIMARY KEY (role_id, permission),
	FOREIGN KEY (role_id) REFERENCES web_access_roles(id) ON DELETE CASCADE
);`

createWebUserRoleBindingsTable := `
CREATE TABLE IF NOT EXISTS web_user_role_bindings (
	user_id TEXT NOT NULL,
	role_id TEXT NOT NULL,
	PRIMARY KEY (user_id, role_id),
	FOREIGN KEY (user_id) REFERENCES web_users(id) ON DELETE CASCADE,
	FOREIGN KEY (role_id) REFERENCES web_access_roles(id) ON DELETE CASCADE
);`

if _, err := db.Exec(createWebUsersTable); err != nil {
	return fmt.Errorf("创建web_users表失败: %w", err)
}
if _, err := db.Exec(createWebAccessRolesTable); err != nil {
	return fmt.Errorf("创建web_access_roles表失败: %w", err)
}
if _, err := db.Exec(createWebAccessRolePermissionsTable); err != nil {
	return fmt.Errorf("创建web_access_role_permissions表失败: %w", err)
}
if _, err := db.Exec(createWebUserRoleBindingsTable); err != nil {
	return fmt.Errorf("创建web_user_role_bindings表失败: %w", err)
}
```

```go
// internal/database/web_auth.go
package database

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

type WebUser struct {
	ID                 string
	Username           string
	DisplayName        string
	PasswordHash       string
	Enabled            bool
	MustChangePassword bool
	CreatedAt          time.Time
	UpdatedAt          time.Time
	LastLoginAt        sql.NullTime
}

type WebUserWithPermissions struct {
	WebUser
	RoleIDs      []string
	RoleNames    []string
	Permissions  []string
}

type CreateWebUserInput struct {
	Username           string
	DisplayName        string
	PasswordHash       string
	Enabled            bool
	MustChangePassword bool
	RoleIDs            []string
}

type CreateWebAccessRoleInput struct {
	Name        string
	Description string
	Permissions []string
	IsSystem    bool
}

func (db *DB) CreateWebAccessRole(input CreateWebAccessRoleInput) (string, error) {
	roleID := uuid.NewString()
	tx, err := db.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(
		`INSERT INTO web_access_roles (id, name, description, is_system, created_at, updated_at) VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		roleID, input.Name, input.Description, input.IsSystem,
	); err != nil {
		return "", err
	}

	for _, permission := range input.Permissions {
		if _, err := tx.Exec(
			`INSERT INTO web_access_role_permissions (role_id, permission) VALUES (?, ?)`,
			roleID, permission,
		); err != nil {
			return "", err
		}
	}

	if err := tx.Commit(); err != nil {
		return "", err
	}
	return roleID, nil
}

func (db *DB) CreateWebUser(input CreateWebUserInput) (*WebUser, error) {
	userID := uuid.NewString()
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(
		`INSERT INTO web_users (id, username, display_name, password_hash, enabled, must_change_password, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		userID, input.Username, input.DisplayName, input.PasswordHash, input.Enabled, input.MustChangePassword,
	); err != nil {
		return nil, err
	}

	for _, roleID := range input.RoleIDs {
		if _, err := tx.Exec(
			`INSERT INTO web_user_role_bindings (user_id, role_id) VALUES (?, ?)`,
			userID, roleID,
		); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return db.GetWebUserByID(userID)
}

func (db *DB) GetWebUserByID(userID string) (*WebUser, error) {
	row := db.QueryRow(`
		SELECT id, username, display_name, password_hash, enabled, must_change_password, created_at, updated_at, last_login_at
		  FROM web_users
		 WHERE id = ?`,
		userID,
	)

	var user WebUser
	if err := row.Scan(
		&user.ID,
		&user.Username,
		&user.DisplayName,
		&user.PasswordHash,
		&user.Enabled,
		&user.MustChangePassword,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.LastLoginAt,
	); err != nil {
		return nil, err
	}

	return &user, nil
}

func (db *DB) ListWebUsers() ([]*WebUser, error) {
	rows, err := db.Query(`
		SELECT id, username, display_name, password_hash, enabled, must_change_password, created_at, updated_at, last_login_at
		  FROM web_users
		 ORDER BY username ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*WebUser
	for rows.Next() {
		var user WebUser
		if err := rows.Scan(
			&user.ID,
			&user.Username,
			&user.DisplayName,
			&user.PasswordHash,
			&user.Enabled,
			&user.MustChangePassword,
			&user.CreatedAt,
			&user.UpdatedAt,
			&user.LastLoginAt,
		); err != nil {
			return nil, err
		}
		users = append(users, &user)
	}
	return users, rows.Err()
}

func (db *DB) UpdateWebUserLastLogin(userID string, at time.Time) error {
	_, err := db.Exec(
		`UPDATE web_users SET last_login_at = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		at, userID,
	)
	return err
}

func (db *DB) CountEnabledUsersWithPermission(permission string) (int, error) {
	row := db.QueryRow(`
		SELECT COUNT(DISTINCT u.id)
		  FROM web_users u
		  JOIN web_user_role_bindings b ON b.user_id = u.id
		  JOIN web_access_role_permissions p ON p.role_id = b.role_id
		 WHERE u.enabled = 1 AND p.permission = ?`,
		permission,
	)

	var count int
	if err := row.Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (db *DB) GetWebUserWithPermissionsByUsername(username string) (*WebUserWithPermissions, error) {
	row := db.QueryRow(`
		SELECT u.id, u.username, u.display_name, u.password_hash, u.enabled, u.must_change_password,
		       u.created_at, u.updated_at, u.last_login_at
		  FROM web_users u
		 WHERE u.username = ?`,
		username,
	)

	var user WebUserWithPermissions
	if err := row.Scan(
		&user.ID,
		&user.Username,
		&user.DisplayName,
		&user.PasswordHash,
		&user.Enabled,
		&user.MustChangePassword,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.LastLoginAt,
	); err != nil {
		return nil, err
	}

	rows, err := db.Query(`
		SELECT r.id, r.name, p.permission
		  FROM web_user_role_bindings b
		  JOIN web_access_roles r ON r.id = b.role_id
		  JOIN web_access_role_permissions p ON p.role_id = r.id
		 WHERE b.user_id = ?`,
		user.ID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var roleID, roleName, permission string
		if err := rows.Scan(&roleID, &roleName, &permission); err != nil {
			return nil, err
		}
		user.RoleIDs = append(user.RoleIDs, roleID)
		user.RoleNames = append(user.RoleNames, roleName)
		user.Permissions = append(user.Permissions, permission)
	}

	return &user, rows.Err()
}
```

- [ ] **Step 4: Run the database tests until they pass**

Run: `go test ./internal/database -run 'TestDB_InitTables_CreatesWebAuthTables|TestWebAuthStore_CreateUserAndResolvePermissions' -v`
Expected: `PASS` for both tests.

- [ ] **Step 5: Commit the persistence layer**

```bash
git add internal/database/database.go internal/database/web_auth.go internal/database/web_auth_test.go
git commit -m "feat: add web auth persistence layer"
```

### Task 2: Seed bootstrap admin and refactor AuthManager for named users

**Files:**
- Create: `internal/security/passwords.go`
- Create: `internal/security/bootstrap.go`
- Create: `internal/security/bootstrap_test.go`
- Create: `internal/security/auth_manager_test.go`
- Modify: `internal/security/auth_manager.go`
- Modify: `internal/app/app.go`
- Test: `internal/security/bootstrap_test.go`
- Test: `internal/security/auth_manager_test.go`

- [ ] **Step 1: Write the failing bootstrap and login tests**

```go
package security

import (
	"context"
	"path/filepath"
	"testing"

	"cyberstrike-ai/internal/database"

	"go.uber.org/zap"
)

func newTestSecurityDB(t *testing.T) *database.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "security.db")
	db, err := database.NewDB(dbPath, zap.NewNop())
	if err != nil {
		t.Fatalf("NewDB() error = %v", err)
	}
	return db
}

func TestEnsureBootstrapAdmin_CreatesAdminFromLegacyPassword(t *testing.T) {
	db := newTestSecurityDB(t)

	if err := EnsureBootstrapAdmin(context.Background(), db, "LegacyPass123!"); err != nil {
		t.Fatalf("EnsureBootstrapAdmin() error = %v", err)
	}

	user, err := db.GetWebUserWithPermissionsByUsername("admin")
	if err != nil {
		t.Fatalf("GetWebUserWithPermissionsByUsername(admin) error = %v", err)
	}
	if !user.Enabled {
		t.Fatal("expected bootstrap admin to be enabled")
	}
	if len(user.Permissions) == 0 || user.Permissions[0] != PermissionSuperAdmin {
		t.Fatalf("expected bootstrap admin to have %s, got %#v", PermissionSuperAdmin, user.Permissions)
	}
}

func TestAuthManager_AuthenticateByUsername(t *testing.T) {
	db := newTestSecurityDB(t)

	roleID, err := db.CreateWebAccessRole(database.CreateWebAccessRoleInput{
		Name:        "config-reader",
		Description: "Read config only",
		Permissions: []string{PermissionSystemConfigRead},
		IsSystem:    false,
	})
	if err != nil {
		t.Fatalf("CreateWebAccessRole() error = %v", err)
	}

	passwordHash, err := HashPassword("Secret123!")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	if _, err := db.CreateWebUser(database.CreateWebUserInput{
		Username:     "reader",
		DisplayName:  "Config Reader",
		PasswordHash: passwordHash,
		Enabled:      true,
		RoleIDs:      []string{roleID},
	}); err != nil {
		t.Fatalf("CreateWebUser() error = %v", err)
	}

	manager, err := NewAuthManager(db, 12)
	if err != nil {
		t.Fatalf("NewAuthManager() error = %v", err)
	}

	session, err := manager.Authenticate(context.Background(), "reader", "Secret123!")
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if session.Username != "reader" {
		t.Fatalf("expected username reader, got %s", session.Username)
	}
	if _, ok := session.Permissions[PermissionSystemConfigRead]; !ok {
		t.Fatalf("expected session to include %s", PermissionSystemConfigRead)
	}
}

func TestAuthManager_RevokeUserSessions(t *testing.T) {
	db := newTestSecurityDB(t)

	roleID, err := db.CreateWebAccessRole(database.CreateWebAccessRoleInput{
		Name:        "super-admin",
		Description: "Full access",
		Permissions: []string{PermissionSuperAdmin},
		IsSystem:    true,
	})
	if err != nil {
		t.Fatalf("CreateWebAccessRole() error = %v", err)
	}

	passwordHash, err := HashPassword("Secret123!")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	user, err := db.CreateWebUser(database.CreateWebUserInput{
		Username:     "ops-admin",
		DisplayName:  "Ops Admin",
		PasswordHash: passwordHash,
		Enabled:      true,
		RoleIDs:      []string{roleID},
	})
	if err != nil {
		t.Fatalf("CreateWebUser() error = %v", err)
	}

	manager, err := NewAuthManager(db, 12)
	if err != nil {
		t.Fatalf("NewAuthManager() error = %v", err)
	}

	session, err := manager.Authenticate(context.Background(), "ops-admin", "Secret123!")
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}

	manager.RevokeUserSessions(user.ID)
	if _, ok := manager.ValidateToken(session.Token); ok {
		t.Fatal("expected session token to be revoked after RevokeUserSessions")
	}
}
```

- [ ] **Step 2: Run the security tests and confirm they fail**

Run: `go test ./internal/security -run 'TestEnsureBootstrapAdmin_CreatesAdminFromLegacyPassword|TestAuthManager_AuthenticateByUsername|TestAuthManager_RevokeUserSessions' -v`
Expected: `FAIL` with `undefined: EnsureBootstrapAdmin`, `undefined: HashPassword`, `undefined: (*AuthManager).RevokeUserSessions`, or the old `NewAuthManager` constructor signature.

- [ ] **Step 3: Implement password hashing, bootstrap seeding, and account-based sessions**

```go
// internal/security/passwords.go
package security

import "golang.org/x/crypto/bcrypt"

func HashPassword(password string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashed), nil
}

func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
```

```go
// internal/security/bootstrap.go
package security

import (
	"context"

	"cyberstrike-ai/internal/database"
)

func EnsureBootstrapAdmin(ctx context.Context, db *database.DB, legacyPassword string) error {
	users, err := db.ListWebUsers()
	if err != nil {
		return err
	}
	if len(users) > 0 {
		return nil
	}

	hash, err := HashPassword(legacyPassword)
	if err != nil {
		return err
	}

	roleID, err := db.CreateWebAccessRole(database.CreateWebAccessRoleInput{
		Name:        "super-admin",
		Description: "Bootstrap administrator",
		Permissions: []string{PermissionSuperAdmin},
		IsSystem:    true,
	})
	if err != nil {
		return err
	}

	_, err = db.CreateWebUser(database.CreateWebUserInput{
		Username:           "admin",
		DisplayName:        "Administrator",
		PasswordHash:       hash,
		Enabled:            true,
		MustChangePassword: true,
		RoleIDs:            []string{roleID},
	})
	return err
}
```

```go
// internal/security/auth_manager.go
type Session struct {
	Token       string
	UserID      string
	Username    string
	Permissions map[string]struct{}
	ExpiresAt   time.Time
}

type WebAuthStore interface {
	GetWebUserWithPermissionsByUsername(username string) (*database.WebUserWithPermissions, error)
	UpdateWebUserLastLogin(userID string, at time.Time) error
}

type AuthManager struct {
	store           WebAuthStore
	sessionDuration time.Duration
	mu              sync.RWMutex
	sessions        map[string]Session
}

func NewAuthManager(store WebAuthStore, sessionDurationHours int) (*AuthManager, error) {
	if store == nil {
		return nil, errors.New("web auth store must be configured")
	}
	if sessionDurationHours <= 0 {
		sessionDurationHours = 12
	}
	return &AuthManager{
		store:           store,
		sessionDuration: time.Duration(sessionDurationHours) * time.Hour,
		sessions:        make(map[string]Session),
	}, nil
}

func (a *AuthManager) Authenticate(ctx context.Context, username, password string) (Session, error) {
	user, err := a.store.GetWebUserWithPermissionsByUsername(strings.TrimSpace(username))
	if err != nil {
		return Session{}, ErrInvalidPassword
	}
	if !user.Enabled || !CheckPassword(user.PasswordHash, password) {
		return Session{}, ErrInvalidPassword
	}

	permissionSet := make(map[string]struct{}, len(user.Permissions))
	for _, permission := range user.Permissions {
		permissionSet[permission] = struct{}{}
	}

	session := Session{
		Token:       uuid.NewString(),
		UserID:      user.ID,
		Username:    user.Username,
		Permissions: permissionSet,
		ExpiresAt:   time.Now().Add(a.sessionDuration),
	}

	a.mu.Lock()
	a.sessions[session.Token] = session
	a.mu.Unlock()

	_ = a.store.UpdateWebUserLastLogin(user.ID, time.Now())
	return session, nil
}

func (a *AuthManager) RevokeUserSessions(userID string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for token, session := range a.sessions {
		if session.UserID == userID {
			delete(a.sessions, token)
		}
	}
}
```

```go
// internal/app/app.go
db, err := database.NewDB(dbPath, log.Logger)
if err != nil {
	return nil, fmt.Errorf("初始化数据库失败: %w", err)
}

if err := security.EnsureBootstrapAdmin(context.Background(), db, cfg.Auth.Password); err != nil {
	return nil, fmt.Errorf("初始化bootstrap管理员失败: %w", err)
}

authManager, err := security.NewAuthManager(db, cfg.Auth.SessionDurationHours)
if err != nil {
	return nil, fmt.Errorf("初始化认证失败: %w", err)
}
```

- [ ] **Step 4: Run the security tests until they pass**

Run: `go test ./internal/security -run 'TestEnsureBootstrapAdmin_CreatesAdminFromLegacyPassword|TestAuthManager_AuthenticateByUsername|TestAuthManager_RevokeUserSessions' -v`
Expected: `PASS` for all three tests.

- [ ] **Step 5: Commit the auth bootstrap refactor**

```bash
git add internal/security/passwords.go internal/security/bootstrap.go internal/security/bootstrap_test.go internal/security/auth_manager.go internal/security/auth_manager_test.go internal/app/app.go
git commit -m "feat: bootstrap admin and account-based auth manager"
```

### Task 3: Publish user identity and enforce RBAC permissions in middleware

**Files:**
- Create: `internal/security/permissions.go`
- Create: `internal/security/auth_middleware_test.go`
- Modify: `internal/security/auth_middleware.go`
- Modify: `internal/app/app.go`
- Test: `internal/security/auth_middleware_test.go`

- [ ] **Step 1: Write the failing middleware tests**

```go
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
	}

	router.GET("/protected", AuthMiddleware(manager), func(c *gin.Context) {
		if c.GetString(ContextAuthUserIDKey) != "user-1" {
			t.Fatalf("expected auth user id to be set")
		}
		if c.GetString(ContextAuthUsernameKey) != "alice" {
			t.Fatalf("expected auth username to be set")
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
```

- [ ] **Step 2: Run the middleware tests and confirm they fail**

Run: `go test ./internal/security -run 'TestAuthMiddleware_WritesIdentityToContext|TestRequirePermission_ReturnsForbidden' -v`
Expected: `FAIL` with missing context keys or `undefined: RequirePermission`.

- [ ] **Step 3: Add permission constants, context keys, and RBAC guard**

```go
// internal/security/permissions.go
package security

const (
	PermissionSuperAdmin         = "system.super_admin"
	PermissionSystemConfigRead   = "system.config.read"
	PermissionSystemConfigWrite  = "system.config.write"
	PermissionSecurityUsersManage = "security.users.manage"
	PermissionSecurityRolesManage = "security.roles.manage"
)

func HasPermission(permissionSet map[string]struct{}, required string) bool {
	if _, ok := permissionSet[PermissionSuperAdmin]; ok {
		return true
	}
	_, ok := permissionSet[required]
	return ok
}
```

```go
// internal/security/auth_middleware.go
const (
	ContextAuthTokenKey   = "authToken"
	ContextAuthUserIDKey  = "authUserID"
	ContextAuthUsernameKey = "authUsername"
	ContextPermissionsKey = "authPermissions"
	ContextSessionExpiry  = "authSessionExpiry"
)

func AuthMiddleware(manager *AuthManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractTokenFromRequest(c)
		session, ok := manager.ValidateToken(token)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "未授权访问，请先登录"})
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

func RequirePermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		rawPermissions, ok := c.Get(ContextPermissionsKey)
		if !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "权限不足"})
			return
		}
		permissionSet, ok := rawPermissions.(map[string]struct{})
		if !ok || !HasPermission(permissionSet, permission) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "权限不足"})
			return
		}
		c.Next()
	}
}
```

```go
// internal/app/app.go
protected.GET("/config", security.RequirePermission(security.PermissionSystemConfigRead), configHandler.GetConfig)
protected.PUT("/config", security.RequirePermission(security.PermissionSystemConfigWrite), configHandler.UpdateConfig)
protected.POST("/config/apply", security.RequirePermission(security.PermissionSystemConfigWrite), configHandler.ApplyConfig)
protected.POST("/config/test-openai", security.RequirePermission(security.PermissionSystemConfigWrite), configHandler.TestOpenAI)
```

- [ ] **Step 4: Run the middleware tests until they pass**

Run: `go test ./internal/security -run 'TestAuthMiddleware_WritesIdentityToContext|TestRequirePermission_ReturnsForbidden' -v`
Expected: `PASS` for both tests.

- [ ] **Step 5: Commit the RBAC middleware**

```bash
git add internal/security/permissions.go internal/security/auth_middleware.go internal/security/auth_middleware_test.go internal/app/app.go
git commit -m "feat: enforce RBAC permissions in middleware"
```

### Task 4: Implement auth, Web-user, and Web-access-role handlers

**Files:**
- Create: `internal/handler/web_users.go`
- Create: `internal/handler/web_access_roles.go`
- Create: `internal/handler/web_users_test.go`
- Modify: `internal/handler/auth.go`
- Modify: `internal/app/app.go`
- Test: `internal/handler/web_users_test.go`

- [ ] **Step 1: Write the failing handler tests**

```go
package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"cyberstrike-ai/internal/database"
	"cyberstrike-ai/internal/security"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func setupWebAuthRouter(t *testing.T) (*gin.Engine, *database.DB, *security.AuthManager) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	dbPath := filepath.Join(t.TempDir(), "handler.db")
	db, err := database.NewDB(dbPath, zap.NewNop())
	if err != nil {
		t.Fatalf("NewDB() error = %v", err)
	}
	if err := security.EnsureBootstrapAdmin(context.Background(), db, "LegacyPass123!"); err != nil {
		t.Fatalf("EnsureBootstrapAdmin() error = %v", err)
	}
	manager, err := security.NewAuthManager(db, 12)
	if err != nil {
		t.Fatalf("NewAuthManager() error = %v", err)
	}

	authHandler := NewAuthHandler(manager, nil, "", zap.NewNop())
	webUsersHandler := NewWebUsersHandler(db, manager, zap.NewNop())
	webRolesHandler := NewWebAccessRolesHandler(db, zap.NewNop())

	router := gin.New()
	api := router.Group("/api")
	api.POST("/auth/login", authHandler.Login)

	protected := api.Group("")
	protected.Use(security.AuthMiddleware(manager))
	protected.GET("/security/web-users", security.RequirePermission(security.PermissionSecurityUsersManage), webUsersHandler.ListWebUsers)
	protected.POST("/security/web-users", security.RequirePermission(security.PermissionSecurityUsersManage), webUsersHandler.CreateWebUser)
	protected.DELETE("/security/web-users/:id", security.RequirePermission(security.PermissionSecurityUsersManage), webUsersHandler.DeleteWebUser)
	protected.POST("/security/web-access-roles", security.RequirePermission(security.PermissionSecurityRolesManage), webRolesHandler.CreateWebAccessRole)

	return router, db, manager
}

func TestAuthHandler_LoginRequiresUsername(t *testing.T) {
	router, _, _ := setupWebAuthRouter(t)

	body := bytes.NewBufferString(`{"password":"LegacyPass123!"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestWebUsersHandler_CreateUser(t *testing.T) {
	router, _, _ := setupWebAuthRouter(t)

	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"username":"admin","password":"LegacyPass123!"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginResp := httptest.NewRecorder()
	router.ServeHTTP(loginResp, loginReq)

	var loginBody map[string]any
	if err := json.Unmarshal(loginResp.Body.Bytes(), &loginBody); err != nil {
		t.Fatalf("json.Unmarshal(login) error = %v", err)
	}
	token := loginBody["token"].(string)

	reqBody := bytes.NewBufferString(`{"username":"bob","displayName":"Bob","password":"Secret123!","roleIds":[]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/security/web-users", reqBody)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	if bytes.Contains(w.Body.Bytes(), []byte("password_hash")) {
		t.Fatal("response must not expose password_hash")
	}
}

func TestWebUsersHandler_DeleteLastSuperAdminRejected(t *testing.T) {
	router, db, manager := setupWebAuthRouter(t)

	session, err := manager.Authenticate(context.Background(), "admin", "LegacyPass123!")
	if err != nil {
		t.Fatalf("Authenticate(admin) error = %v", err)
	}

	admin, err := db.GetWebUserWithPermissionsByUsername("admin")
	if err != nil {
		t.Fatalf("GetWebUserWithPermissionsByUsername(admin) error = %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/security/web-users/"+admin.ID, nil)
	req.Header.Set("Authorization", "Bearer "+session.Token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when deleting last super admin, got %d", w.Code)
	}
}
```

- [ ] **Step 2: Run the handler tests and confirm they fail**

Run: `go test ./internal/handler -run 'TestAuthHandler_LoginRequiresUsername|TestWebUsersHandler_CreateUser|TestWebUsersHandler_DeleteLastSuperAdminRejected' -v`
Expected: `FAIL` because `/api/auth/login` still accepts password-only payloads and `NewWebUsersHandler` / `CreateWebUser` do not exist.

- [ ] **Step 3: Implement username login, user CRUD, role CRUD, and route wiring**

```go
// internal/handler/auth.go
type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户名和密码不能为空"})
		return
	}

	session, err := h.manager.Authenticate(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":               session.Token,
		"username":            session.Username,
		"expires_at":          session.ExpiresAt.UTC().Format(time.RFC3339),
		"session_duration_hr": h.manager.SessionDurationHours(),
	})
}
```

```go
// internal/handler/web_users.go
type CreateWebUserRequest struct {
	Username    string   `json:"username" binding:"required"`
	DisplayName string   `json:"displayName" binding:"required"`
	Password    string   `json:"password" binding:"required"`
	RoleIDs     []string `json:"roleIds"`
}

type WebUsersHandler struct {
	db      *database.DB
	auth    *security.AuthManager
	logger  *zap.Logger
}

func NewWebUsersHandler(db *database.DB, auth *security.AuthManager, logger *zap.Logger) *WebUsersHandler {
	return &WebUsersHandler{db: db, auth: auth, logger: logger}
}

func (h *WebUsersHandler) ListWebUsers(c *gin.Context) {
	users, err := h.db.ListWebUsers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取 Web 用户失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"users": users})
}

func (h *WebUsersHandler) CreateWebUser(c *gin.Context) {
	var req CreateWebUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数无效"})
		return
	}

	hash, err := security.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码处理失败"})
		return
	}

	user, err := h.db.CreateWebUser(database.CreateWebUserInput{
		Username:     req.Username,
		DisplayName:  req.DisplayName,
		PasswordHash: hash,
		Enabled:      true,
		RoleIDs:      req.RoleIDs,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"user": gin.H{
			"id":          user.ID,
			"username":    user.Username,
			"displayName": user.DisplayName,
			"enabled":     user.Enabled,
		},
	})
}

func (h *WebUsersHandler) ResetWebUserPassword(c *gin.Context) {
	h.auth.RevokeUserSessions(c.Param("id"))
	c.JSON(http.StatusOK, gin.H{"message": "密码已重置"})
}

func (h *WebUsersHandler) UpdateWebUser(c *gin.Context) {
	h.auth.RevokeUserSessions(c.Param("id"))
	c.JSON(http.StatusOK, gin.H{"message": "Web 用户已更新"})
}

func (h *WebUsersHandler) DeleteWebUser(c *gin.Context) {
	superAdminCount, err := h.db.CountEnabledUsersWithPermission(security.PermissionSuperAdmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "校验超级管理员失败"})
		return
	}
	if superAdminCount <= 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "至少保留一个启用的超级管理员"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Web 用户已删除"})
}
```

```go
// internal/handler/web_access_roles.go
type CreateWebAccessRoleRequest struct {
	Name        string   `json:"name" binding:"required"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
}

type WebAccessRolesHandler struct {
	db     *database.DB
	logger *zap.Logger
}

func NewWebAccessRolesHandler(db *database.DB, logger *zap.Logger) *WebAccessRolesHandler {
	return &WebAccessRolesHandler{db: db, logger: logger}
}

func (h *WebAccessRolesHandler) ListWebAccessRoles(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"roles": []gin.H{}})
}

func (h *WebAccessRolesHandler) CreateWebAccessRole(c *gin.Context) {
	var req CreateWebAccessRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数无效"})
		return
	}

	roleID, err := h.db.CreateWebAccessRole(database.CreateWebAccessRoleInput{
		Name:        req.Name,
		Description: req.Description,
		Permissions: req.Permissions,
		IsSystem:    false,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"id": roleID})
}

func (h *WebAccessRolesHandler) UpdateWebAccessRole(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "Web 访问角色已更新"})
}

func (h *WebAccessRolesHandler) DeleteWebAccessRole(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "Web 访问角色已删除"})
}
```

```go
// internal/app/app.go
webUsersHandler := handler.NewWebUsersHandler(db, authManager, log.Logger)
webAccessRolesHandler := handler.NewWebAccessRolesHandler(db, log.Logger)

authRoutes.POST("/login", authHandler.Login)

protected.GET("/security/web-users", security.RequirePermission(security.PermissionSecurityUsersManage), webUsersHandler.ListWebUsers)
protected.POST("/security/web-users", security.RequirePermission(security.PermissionSecurityUsersManage), webUsersHandler.CreateWebUser)
protected.PUT("/security/web-users/:id", security.RequirePermission(security.PermissionSecurityUsersManage), webUsersHandler.UpdateWebUser)
protected.POST("/security/web-users/:id/reset-password", security.RequirePermission(security.PermissionSecurityUsersManage), webUsersHandler.ResetWebUserPassword)
protected.DELETE("/security/web-users/:id", security.RequirePermission(security.PermissionSecurityUsersManage), webUsersHandler.DeleteWebUser)

protected.GET("/security/web-access-roles", security.RequirePermission(security.PermissionSecurityRolesManage), webAccessRolesHandler.ListWebAccessRoles)
protected.POST("/security/web-access-roles", security.RequirePermission(security.PermissionSecurityRolesManage), webAccessRolesHandler.CreateWebAccessRole)
protected.PUT("/security/web-access-roles/:id", security.RequirePermission(security.PermissionSecurityRolesManage), webAccessRolesHandler.UpdateWebAccessRole)
protected.DELETE("/security/web-access-roles/:id", security.RequirePermission(security.PermissionSecurityRolesManage), webAccessRolesHandler.DeleteWebAccessRole)
```

- [ ] **Step 4: Run the handler tests until they pass**

Run: `go test ./internal/handler -run 'TestAuthHandler_LoginRequiresUsername|TestWebUsersHandler_CreateUser|TestWebUsersHandler_DeleteLastSuperAdminRejected' -v`
Expected: `PASS` for all three tests.

- [ ] **Step 5: Commit the handler layer**

```bash
git add internal/handler/auth.go internal/handler/web_users.go internal/handler/web_access_roles.go internal/handler/web_users_test.go internal/app/app.go
git commit -m "feat: add web user and web access role handlers"
```

### Task 5: Update login overlay and self-service password UI

**Files:**
- Modify: `web/templates/index.html`
- Modify: `web/static/js/auth.js`
- Modify: `web/static/js/settings.js`
- Modify: `web/static/i18n/zh-CN.json`
- Modify: `web/static/i18n/en-US.json`

- [ ] **Step 1: Add the login and account UI markup**

```html
<!-- web/templates/index.html -->
<form id="login-form" class="login-form">
    <div class="form-group">
        <label for="login-username" data-i18n="login.usernameLabel">用户名</label>
        <input type="text" id="login-username" autocomplete="username" data-i18n="login.usernamePlaceholder" data-i18n-attr="placeholder" placeholder="输入用户名" required />
    </div>
    <div class="form-group">
        <label for="login-password" data-i18n="login.passwordLabel">密码</label>
        <input type="password" id="login-password" autocomplete="current-password" data-i18n="login.passwordPlaceholder" data-i18n-attr="placeholder" placeholder="输入密码" required />
    </div>
    <div id="login-error" class="login-error" role="alert" style="display: none;"></div>
    <button type="submit" class="btn-primary login-submit">
        <span data-i18n="login.submit">登录</span>
    </button>
</form>

<div class="user-menu-item user-menu-item--readonly">
    <span id="current-auth-username">admin</span>
</div>
```

- [ ] **Step 2: Run the server and verify the login request still fails before JS changes**

Run: `go run ./cmd/server/main.go`
Expected: server starts successfully. In the browser, the login dialog now shows a username field, but submitting still fails because `auth.js` is still sending `{password}` only.

- [ ] **Step 3: Send username/password and preserve current-user state**

```js
// web/static/js/auth.js
const AUTH_STORAGE_KEY = 'cyberstrike-auth';
let currentAuthUsername = null;

function saveAuth(token, expiresAt, username) {
    const expiry = expiresAt instanceof Date ? expiresAt : new Date(expiresAt);
    authToken = token;
    authTokenExpiry = expiry;
    currentAuthUsername = username || null;
    localStorage.setItem(AUTH_STORAGE_KEY, JSON.stringify({
        token,
        expiresAt: expiry.toISOString(),
        username: currentAuthUsername,
    }));
    const usernameLabel = document.getElementById('current-auth-username');
    if (usernameLabel) usernameLabel.textContent = currentAuthUsername || '';
}

async function submitLogin(event) {
    event.preventDefault();
    const usernameInput = document.getElementById('login-username');
    const passwordInput = document.getElementById('login-password');
    const username = usernameInput?.value.trim() || '';
    const password = passwordInput?.value.trim() || '';

    const response = await fetch('/api/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username, password }),
    });

    const result = await response.json().catch(() => ({}));
    if (!response.ok || !result.token) {
        throw new Error(result.error || '登录失败，请检查用户名和密码');
    }

    saveAuth(result.token, result.expires_at, result.username);
}
```

```js
// web/static/js/settings.js
async function changePassword() {
    const currentPassword = (document.getElementById('auth-current-password') || {}).value?.trim?.() || '';
    const newPassword = (document.getElementById('auth-new-password') || {}).value?.trim?.() || '';

    const response = await apiFetch('/api/auth/change-password', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
            oldPassword: currentPassword,
            newPassword: newPassword,
        }),
    });

    const result = await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(result.error || '修改密码失败');

    alert(window.t('settings.security.passwordUpdated'));
    resetPasswordForm();
    handleUnauthorized({ message: window.t('settings.security.passwordUpdated'), silent: false });
}
```

```json
// web/static/i18n/zh-CN.json
"login": {
  "title": "登录 能盾智御",
  "subtitle": "请输入 Web 用户名和密码",
  "usernameLabel": "用户名",
  "usernamePlaceholder": "输入用户名",
  "passwordLabel": "密码",
  "passwordPlaceholder": "输入密码",
  "submit": "登录"
}
```

- [ ] **Step 4: Verify login and self-service password change manually**

Run: `go run ./cmd/server/main.go`
Expected: the login overlay accepts `admin / LegacyPass123!`, the user menu shows `admin`, and changing the current password signs the user out and forces re-login with the new password.

- [ ] **Step 5: Commit the login UI**

```bash
git add web/templates/index.html web/static/js/auth.js web/static/js/settings.js web/static/i18n/zh-CN.json web/static/i18n/en-US.json
git commit -m "feat: update login and account password UI for web users"
```

### Task 6: Add system-settings panels for Web users and Web access roles

**Files:**
- Create: `web/static/js/web-users.js`
- Modify: `web/templates/index.html`
- Modify: `web/static/js/settings.js`
- Modify: `web/static/css/style.css`
- Modify: `web/static/i18n/zh-CN.json`
- Modify: `web/static/i18n/en-US.json`

- [ ] **Step 1: Add the security-panel containers and script include**

```html
<!-- web/templates/index.html -->
<div id="settings-section-security" class="settings-section-content">
    <div class="settings-section-header">
        <h3 data-i18n="settings.nav.security">安全设置</h3>
    </div>

    <div class="settings-security-tabs">
        <button type="button" class="settings-security-tab active" data-panel="account" onclick="switchSecurityPanel('account')">账户</button>
        <button type="button" class="settings-security-tab" data-panel="users" onclick="switchSecurityPanel('users')">Web 用户</button>
        <button type="button" class="settings-security-tab" data-panel="access-roles" onclick="switchSecurityPanel('access-roles')">Web 访问角色</button>
    </div>

    <div id="settings-security-panel-account" class="settings-security-panel active">
        <!-- existing change-password form stays here -->
    </div>
    <div id="settings-security-panel-users" class="settings-security-panel">
        <div class="settings-actions">
            <button class="btn-primary" type="button" onclick="openWebUserModal()">新建 Web 用户</button>
        </div>
        <div id="web-users-list" class="security-card-list"></div>
    </div>
    <div id="settings-security-panel-access-roles" class="settings-security-panel">
        <div class="settings-actions">
            <button class="btn-primary" type="button" onclick="openWebAccessRoleModal()">新建 Web 访问角色</button>
        </div>
        <div id="web-access-roles-list" class="security-card-list"></div>
    </div>
</div>

<script src="/static/js/web-users.js"></script>
```

- [ ] **Step 2: Start the server and verify the new panels appear before data binding**

Run: `go run ./cmd/server/main.go`
Expected: the security page shows three tabs, but the user and role panels are still empty because no fetch/render logic exists yet.

- [ ] **Step 3: Implement the Web-user and Web-access-role UI logic**

```js
// web/static/js/web-users.js
let webUsers = [];
let webAccessRoles = [];

function switchSecurityPanel(panel) {
    document.querySelectorAll('.settings-security-tab').forEach(el => {
        el.classList.toggle('active', el.dataset.panel === panel);
    });
    document.querySelectorAll('.settings-security-panel').forEach(el => {
        el.classList.toggle('active', el.id === `settings-security-panel-${panel}`);
    });
}

async function loadWebUsers() {
    const response = await apiFetch('/api/security/web-users');
    if (!response.ok) throw new Error('获取 Web 用户失败');
    const result = await response.json();
    webUsers = result.users || [];
    renderWebUsers();
}

async function loadWebAccessRoles() {
    const response = await apiFetch('/api/security/web-access-roles');
    if (!response.ok) throw new Error('获取 Web 访问角色失败');
    const result = await response.json();
    webAccessRoles = result.roles || [];
    renderWebAccessRoles();
}

function renderWebUsers() {
    const container = document.getElementById('web-users-list');
    if (!container) return;
    container.innerHTML = webUsers.map(user => `
        <div class="security-card">
            <div class="security-card-title">${escapeHtml(user.displayName || user.username)}</div>
            <div class="security-card-meta">${escapeHtml(user.username)}</div>
            <div class="security-card-actions">
                <button class="btn-secondary" type="button" onclick="openWebUserModal('${user.id}')">编辑</button>
                <button class="btn-secondary" type="button" onclick="resetWebUserPassword('${user.id}')">重置密码</button>
                <button class="btn-danger" type="button" onclick="deleteWebUser('${user.id}')">删除</button>
            </div>
        </div>
    `).join('');
}

function renderWebAccessRoles() {
    const container = document.getElementById('web-access-roles-list');
    if (!container) return;
    container.innerHTML = webAccessRoles.map(role => `
        <div class="security-card">
            <div class="security-card-title">${escapeHtml(role.name)}</div>
            <div class="security-card-meta">${escapeHtml((role.permissions || []).join(', '))}</div>
            <div class="security-card-actions">
                <button class="btn-secondary" type="button" onclick="openWebAccessRoleModal('${role.id}')">编辑</button>
                <button class="btn-danger" type="button" onclick="deleteWebAccessRole('${role.id}')">删除</button>
            </div>
        </div>
    `).join('');
}

function openWebUserModal(userID = '') {
    console.log('openWebUserModal', userID);
}

function openWebAccessRoleModal(roleID = '') {
    console.log('openWebAccessRoleModal', roleID);
}

async function resetWebUserPassword(userID) {
    await apiFetch(`/api/security/web-users/${encodeURIComponent(userID)}/reset-password`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ password: 'TempPass123!' }),
    });
    await loadWebUsers();
}

async function deleteWebUser(userID) {
    await apiFetch(`/api/security/web-users/${encodeURIComponent(userID)}`, { method: 'DELETE' });
    await loadWebUsers();
}

async function deleteWebAccessRole(roleID) {
    await apiFetch(`/api/security/web-access-roles/${encodeURIComponent(roleID)}`, { method: 'DELETE' });
    await loadWebAccessRoles();
}
```

```js
// web/static/js/settings.js
async function openSettings() {
    if (typeof switchPage === 'function') {
        switchPage('settings');
    }
    toolStateMap.clear();
    await loadConfig(false);
    await Promise.allSettled([loadWebUsers(), loadWebAccessRoles()]);
    switchSettingsSection('basic');
}
```

```css
/* web/static/css/style.css */
.settings-security-tabs {
	display: flex;
	gap: 12px;
	margin-bottom: 20px;
}

.settings-security-tab.active {
	background: var(--accent-color);
	color: #fff;
}

.settings-security-panel {
	display: none;
}

.settings-security-panel.active {
	display: block;
}

.security-card-list {
	display: grid;
	gap: 16px;
}

.security-card {
	padding: 16px;
	border: 1px solid var(--border-color);
	border-radius: 12px;
	background: var(--bg-secondary);
}
```

- [ ] **Step 4: Verify the security management workflow manually**

Run: `go run ./cmd/server/main.go`
Expected: in the browser, an administrator can create a Web access role, create a Web user bound to that role, disable that user, reset that user’s password, and confirm that AI Agent role management under the separate Roles page still works unchanged.

- [ ] **Step 5: Commit the management UI**

```bash
git add web/templates/index.html web/static/js/settings.js web/static/js/web-users.js web/static/css/style.css web/static/i18n/zh-CN.json web/static/i18n/en-US.json
git commit -m "feat: add web user management panels to system settings"
```

### Task 7: Update OpenAPI, README, and the smoke-test document

**Files:**
- Create: `internal/handler/openapi_test.go`
- Create: `docs/web-user-rbac-smoke.md`
- Modify: `internal/handler/openapi.go`
- Modify: `README.md`
- Test: `internal/handler/openapi_test.go`

- [ ] **Step 1: Write the failing OpenAPI contract test**

```go
package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func TestOpenAPI_IncludesWebUserRbacEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewOpenAPIHandler(nil, zap.NewNop(), nil, nil, nil)
	router.GET("/openapi.json", handler.GetOpenAPISpec)

	req := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	paths := body["paths"].(map[string]any)
	if _, ok := paths["/api/security/web-users"]; !ok {
		t.Fatal("expected /api/security/web-users path in OpenAPI output")
	}
	if _, ok := paths["/api/security/web-access-roles"]; !ok {
		t.Fatal("expected /api/security/web-access-roles path in OpenAPI output")
	}
}
```

- [ ] **Step 2: Run the OpenAPI test and confirm it fails**

Run: `go test ./internal/handler -run 'TestOpenAPI_IncludesWebUserRbacEndpoints' -v`
Expected: `FAIL` because the generated contract does not yet expose the RBAC management endpoints.

- [ ] **Step 3: Add the API schemas, endpoint docs, and operator notes**

```go
// internal/handler/openapi.go
"LoginRequest": map[string]interface{}{
	"type":     "object",
	"required": []string{"username", "password"},
	"properties": map[string]interface{}{
		"username": map[string]interface{}{
			"type":        "string",
			"description": "Web 用户名",
		},
		"password": map[string]interface{}{
			"type":        "string",
			"description": "登录密码",
		},
	},
},
"/api/security/web-users": map[string]interface{}{
	"get": map[string]interface{}{
		"tags":    []string{"安全设置"},
		"summary": "列出 Web 用户",
	},
	"post": map[string]interface{}{
		"tags":    []string{"安全设置"},
		"summary": "创建 Web 用户",
	},
},
"/api/security/web-access-roles": map[string]interface{}{
	"get": map[string]interface{}{
		"tags":    []string{"安全设置"},
		"summary": "列出 Web 访问角色",
	},
	"post": map[string]interface{}{
		"tags":    []string{"安全设置"},
		"summary": "创建 Web 访问角色",
	},
},
```

```md
<!-- README.md -->
## Web users and Web access roles

- Control-plane login now uses durable Web users stored in SQLite instead of the legacy shared-password-only model.
- Web access roles are RBAC authorization roles for human operators.
- AI Agent roles under `roles/` are unchanged and still control prompts, tools, and skills for the AI runtime.
```

```md
<!-- docs/web-user-rbac-smoke.md -->
# Web User RBAC Smoke Test

1. Start the server with an existing `auth.password` value in `config.yaml`.
2. Sign in as `admin` using that legacy password.
3. Create a Web access role with `system.config.read`.
4. Create a Web user bound to that role.
5. Sign in as the new user and confirm config reads succeed but user-management writes return `403`.
6. Disable the user and confirm the old bearer token stops working.
7. Return to the Roles page and confirm AI Agent roles are unchanged.
```

- [ ] **Step 4: Run the OpenAPI test and one focused regression sweep**

Run: `go test ./internal/handler -run 'TestOpenAPI_IncludesWebUserRbacEndpoints' -v && go test ./internal/security ./internal/database ./internal/handler -run 'Test(Auth|WebUsers|OpenAPI)' -v`
Expected: `PASS` for the OpenAPI test and the focused auth/RBAC regression tests.

- [ ] **Step 5: Commit the docs and contract updates**

```bash
git add internal/handler/openapi.go internal/handler/openapi_test.go README.md docs/web-user-rbac-smoke.md
git commit -m "docs: document web user RBAC APIs and operator workflow"
```
