package database

import (
	"database/sql"
	"errors"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

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
		Username:           "alice",
		DisplayName:        "Alice",
		PasswordHash:       "hashed-value",
		Enabled:            true,
		MustChangePassword: true,
		RoleIDs:            []string{roleID, roleID2},
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

func TestCreateWebUser_DuplicateUsernameFails(t *testing.T) {
	db := openTestWebAuthDB(t)

	roleID, err := db.CreateWebAccessRole(CreateWebAccessRoleInput{
		Name:        "dup-reader",
		Description: "dedupe role",
		Permissions: []string{"system.config.read"},
		IsSystem:    false,
	})
	if err != nil {
		t.Fatalf("CreateWebAccessRole() error = %v", err)
	}

	if _, err := db.CreateWebUser(CreateWebUserInput{
		Username:     "duplicate",
		DisplayName:  "First",
		PasswordHash: "hash",
		Enabled:      true,
		RoleIDs:      []string{roleID},
	}); err != nil {
		t.Fatalf("CreateWebUser() error = %v", err)
	}

	if _, err := db.CreateWebUser(CreateWebUserInput{
		Username:     "duplicate",
		DisplayName:  "Second",
		PasswordHash: "hash",
		Enabled:      true,
		RoleIDs:      []string{roleID},
	}); err == nil {
		t.Fatal("expected duplicate username creation to fail")
	}

	users, err := db.ListWebUsers()
	if err != nil {
		t.Fatalf("ListWebUsers() error = %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("expected only one user, got %d", len(users))
	}
}

func TestCreateWebUser_RollbackOnInvalidRole(t *testing.T) {
	db := openTestWebAuthDB(t)

	_, err := db.CreateWebUser(CreateWebUserInput{
		Username:     "rollback",
		DisplayName:  "Rollback",
		PasswordHash: "hash",
		Enabled:      true,
		RoleIDs:      []string{"missing-role"},
	})
	if err == nil {
		t.Fatal("expected CreateWebUser to fail when binding invalid role")
	}

	if _, err := db.GetWebUserWithPermissionsByUsername("rollback"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected no user persisted after rollback, got %v", err)
	}
}

func TestGetWebUserWithPermissionsByUsername_Deduplicates(t *testing.T) {
	db := openTestWebAuthDB(t)

	roleA, err := db.CreateWebAccessRole(CreateWebAccessRoleInput{
		Name:        "role-a",
		Description: "Role A",
		Permissions: []string{"perm.alpha", "perm.shared"},
		IsSystem:    false,
	})
	if err != nil {
		t.Fatalf("CreateWebAccessRole(role-a) error = %v", err)
	}

	roleB, err := db.CreateWebAccessRole(CreateWebAccessRoleInput{
		Name:        "role-b",
		Description: "Role B",
		Permissions: []string{"perm.shared", "perm.beta"},
		IsSystem:    false,
	})
	if err != nil {
		t.Fatalf("CreateWebAccessRole(role-b) error = %v", err)
	}

	roleC, err := db.CreateWebAccessRole(CreateWebAccessRoleInput{
		Name:        "role-c",
		Description: "Role C (no perms)",
		Permissions: nil,
		IsSystem:    false,
	})
	if err != nil {
		t.Fatalf("CreateWebAccessRole(role-c) error = %v", err)
	}

	if _, err := db.CreateWebUser(CreateWebUserInput{
		Username:     "dedupe-test",
		DisplayName:  "Dedupe",
		PasswordHash: "hash",
		Enabled:      true,
		RoleIDs:      []string{roleA, roleB, roleC},
	}); err != nil {
		t.Fatalf("CreateWebUser() error = %v", err)
	}

	user, err := db.GetWebUserWithPermissionsByUsername("dedupe-test")
	if err != nil {
		t.Fatalf("GetWebUserWithPermissionsByUsername() error = %v", err)
	}
	if len(user.RoleIDs) != 3 {
		t.Fatalf("expected 3 assigned roles, got %d", len(user.RoleIDs))
	}
	if len(user.RoleNames) != 3 {
		t.Fatalf("expected 3 role names, got %d", len(user.RoleNames))
	}

	expectedRoles := map[string]struct{}{roleA: {}, roleB: {}, roleC: {}}
	for _, id := range user.RoleIDs {
		delete(expectedRoles, id)
	}
	if len(expectedRoles) != 0 {
		t.Fatalf("role IDs missing or duplicated: %#v", expectedRoles)
	}

	expectedPerms := map[string]struct{}{
		"perm.alpha":  {},
		"perm.shared": {},
		"perm.beta":   {},
	}
	if len(user.Permissions) != len(expectedPerms) {
		t.Fatalf("expected %d unique permissions, got %d", len(expectedPerms), len(user.Permissions))
	}
	for _, perm := range user.Permissions {
		if _, ok := expectedPerms[perm]; !ok {
			t.Fatalf("unexpected permission %s", perm)
		}
	}
}

func TestUpdateWebUserLastLogin(t *testing.T) {
	db := openTestWebAuthDB(t)

	roleID, err := db.CreateWebAccessRole(CreateWebAccessRoleInput{
		Name:        "login-role",
		Description: "Login role",
		Permissions: []string{"perm.login"},
		IsSystem:    false,
	})
	if err != nil {
		t.Fatalf("CreateWebAccessRole() error = %v", err)
	}

	user, err := db.CreateWebUser(CreateWebUserInput{
		Username:     "time-updates",
		DisplayName:  "Time Test",
		PasswordHash: "hash",
		Enabled:      true,
		RoleIDs:      []string{roleID},
	})
	if err != nil {
		t.Fatalf("CreateWebUser() error = %v", err)
	}

	at := time.Now().UTC().Truncate(time.Second)
	if err := db.UpdateWebUserLastLogin(user.ID, at); err != nil {
		t.Fatalf("UpdateWebUserLastLogin() error = %v", err)
	}

	stored, err := db.GetWebUserByID(user.ID)
	if err != nil {
		t.Fatalf("GetWebUserByID() error = %v", err)
	}
	if !stored.LastLoginAt.Valid {
		t.Fatal("expected LastLoginAt to be set")
	}
	if stored.LastLoginAt.Time.Unix() != at.Unix() {
		t.Fatalf("expected last login %v, got %v", at, stored.LastLoginAt.Time)
	}
}

func TestUpdateWebUserLastLogin_MissingUser(t *testing.T) {
	db := openTestWebAuthDB(t)

	if err := db.UpdateWebUserLastLogin("missing-id", time.Now()); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected sql.ErrNoRows for missing user, got %v", err)
	}
}

func TestUpdateWebUserPasswordByUsername(t *testing.T) {
	db := openTestWebAuthDB(t)

	roleID, err := db.CreateWebAccessRole(CreateWebAccessRoleInput{
		Name:        "password-update-role",
		Description: "Password update role",
		Permissions: []string{"perm.update"},
		IsSystem:    false,
	})
	if err != nil {
		t.Fatalf("CreateWebAccessRole() error = %v", err)
	}

	user, err := db.CreateWebUser(CreateWebUserInput{
		Username:           "password-update-user",
		DisplayName:        "Password Update User",
		PasswordHash:       "old-hash",
		Enabled:            true,
		MustChangePassword: true,
		RoleIDs:            []string{roleID},
	})
	if err != nil {
		t.Fatalf("CreateWebUser() error = %v", err)
	}

	if err := db.UpdateWebUserPasswordByUsername(user.Username, "new-hash", false); err != nil {
		t.Fatalf("UpdateWebUserPasswordByUsername() error = %v", err)
	}

	stored, err := db.GetWebUserByID(user.ID)
	if err != nil {
		t.Fatalf("GetWebUserByID() error = %v", err)
	}
	if stored.PasswordHash != "new-hash" {
		t.Fatalf("expected password hash updated to new-hash, got %q", stored.PasswordHash)
	}
	if stored.MustChangePassword {
		t.Fatal("expected must_change_password to be false after update")
	}
}

func TestUpdateWebUserPasswordByUsername_MissingUser(t *testing.T) {
	db := openTestWebAuthDB(t)

	if err := db.UpdateWebUserPasswordByUsername("missing-user", "hash", false); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected sql.ErrNoRows for missing user, got %v", err)
	}
}

func TestUpdateWebUserPasswordByID(t *testing.T) {
	db := openTestWebAuthDB(t)

	roleID, err := db.CreateWebAccessRole(CreateWebAccessRoleInput{
		Name:        "password-update-id-role",
		Description: "Password update by ID role",
		Permissions: []string{"perm.update.id"},
		IsSystem:    false,
	})
	if err != nil {
		t.Fatalf("CreateWebAccessRole() error = %v", err)
	}

	user, err := db.CreateWebUser(CreateWebUserInput{
		Username:           "password-update-by-id-user",
		DisplayName:        "Password Update By ID User",
		PasswordHash:       "old-id-hash",
		Enabled:            true,
		MustChangePassword: true,
		RoleIDs:            []string{roleID},
	})
	if err != nil {
		t.Fatalf("CreateWebUser() error = %v", err)
	}

	if err := db.UpdateWebUserPasswordByID(user.ID, "new-id-hash", false); err != nil {
		t.Fatalf("UpdateWebUserPasswordByID() error = %v", err)
	}

	stored, err := db.GetWebUserByID(user.ID)
	if err != nil {
		t.Fatalf("GetWebUserByID() error = %v", err)
	}
	if stored.PasswordHash != "new-id-hash" {
		t.Fatalf("expected password hash updated to new-id-hash, got %q", stored.PasswordHash)
	}
	if stored.MustChangePassword {
		t.Fatal("expected must_change_password to be false after update by id")
	}
}

func TestUpdateWebUserPasswordByID_MissingUser(t *testing.T) {
	db := openTestWebAuthDB(t)

	if err := db.UpdateWebUserPasswordByID("missing-id", "hash", false); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected sql.ErrNoRows for missing user id, got %v", err)
	}
}

func TestRoleIDsGrantPermission_LegacySuperAdminMatchesCanonicalGrant(t *testing.T) {
	db := openTestWebAuthDB(t)

	roleID, err := db.CreateWebAccessRole(CreateWebAccessRoleInput{
		Name:        "canonical-super-admin-role",
		Description: "canonical super admin role",
		Permissions: []string{"system.super_admin.grant"},
		IsSystem:    false,
	})
	if err != nil {
		t.Fatalf("CreateWebAccessRole() error = %v", err)
	}

	granted, err := db.RoleIDsGrantPermission([]string{roleID}, "system.super_admin")
	if err != nil {
		t.Fatalf("RoleIDsGrantPermission() error = %v", err)
	}
	if !granted {
		t.Fatal("expected canonical system.super_admin.grant to satisfy legacy system.super_admin check")
	}
}

func TestCountEnabledUsersWithPermission_LegacySuperAdminCountsCanonicalGrant(t *testing.T) {
	db := openTestWebAuthDB(t)

	roleID, err := db.CreateWebAccessRole(CreateWebAccessRoleInput{
		Name:        "canonical-super-admin-count-role",
		Description: "canonical super admin count role",
		Permissions: []string{"system.super_admin.grant"},
		IsSystem:    false,
	})
	if err != nil {
		t.Fatalf("CreateWebAccessRole() error = %v", err)
	}

	if _, err := db.CreateWebUser(CreateWebUserInput{
		Username:     "canonical-super-admin-user",
		DisplayName:  "Canonical Super Admin User",
		PasswordHash: "hash",
		Enabled:      true,
		RoleIDs:      []string{roleID},
	}); err != nil {
		t.Fatalf("CreateWebUser() error = %v", err)
	}

	count, err := db.CountEnabledUsersWithPermission("system.super_admin")
	if err != nil {
		t.Fatalf("CountEnabledUsersWithPermission() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("expected canonical system.super_admin.grant user to count as legacy super admin, got %d", count)
	}
}

func TestListWebAccessRolesNormalizesLegacyPermissions(t *testing.T) {
	db := openTestWebAuthDB(t)
	db.SetWebPermissionNormalizer(testNormalizeWebPermissions)

	roleID, err := db.CreateWebAccessRole(CreateWebAccessRoleInput{
		Name:        "legacy-normalize-role",
		Description: "role with mixed legacy and canonical permissions",
		Permissions: []string{
			"system.config.write",
			"system.super_admin",
			"system.web_user.read",
			"unknown.permission",
		},
		IsSystem: false,
	})
	if err != nil {
		t.Fatalf("CreateWebAccessRole() error = %v", err)
	}

	want := []string{
		"system.config_settings.update",
		"system.model_connectivity.test",
		"system.runtime_config.apply",
		"system.super_admin.grant",
		"system.web_user.read",
	}

	gotPersisted, err := listPersistedRolePermissions(db, roleID)
	if err != nil {
		t.Fatalf("listPersistedRolePermissions() error = %v", err)
	}
	if !equalStringSlices(gotPersisted, want) {
		t.Fatalf("persisted permissions = %#v, want %#v", gotPersisted, want)
	}

	roles, err := db.ListWebAccessRoles()
	if err != nil {
		t.Fatalf("ListWebAccessRoles() error = %v", err)
	}

	var gotRole *WebAccessRole
	for _, role := range roles {
		if role.ID == roleID {
			gotRole = role
			break
		}
	}
	if gotRole == nil {
		t.Fatalf("expected role %q in list", roleID)
	}
	if !equalStringSlices(gotRole.Permissions, want) {
		t.Fatalf("ListWebAccessRoles() permissions = %#v, want %#v", gotRole.Permissions, want)
	}

	byID, err := db.GetWebAccessRoleByID(roleID)
	if err != nil {
		t.Fatalf("GetWebAccessRoleByID() error = %v", err)
	}
	if !equalStringSlices(byID.Permissions, want) {
		t.Fatalf("GetWebAccessRoleByID() permissions = %#v, want %#v", byID.Permissions, want)
	}
}

func TestGetWebUserWithPermissionsByUsernameNormalizesLegacyPermissions(t *testing.T) {
	db := openTestWebAuthDB(t)
	db.SetWebPermissionNormalizer(testNormalizeWebPermissions)

	roleID, err := db.CreateWebAccessRole(CreateWebAccessRoleInput{
		Name:        "legacy-permissions-role",
		Description: "role to test resolved canonical permissions",
		Permissions: nil,
		IsSystem:    false,
	})
	if err != nil {
		t.Fatalf("CreateWebAccessRole() error = %v", err)
	}

	// Bypass role write-path normalization to emulate persisted legacy data.
	for _, permission := range []string{
		"system.config.read",
		"security.users.manage",
		"system.web_user.read",
		"unknown.permission",
	} {
		if _, err := db.Exec(
			`INSERT INTO web_access_role_permissions (role_id, permission) VALUES (?, ?)`,
			roleID, permission,
		); err != nil {
			t.Fatalf("insert legacy permission %q: %v", permission, err)
		}
	}

	if _, err := db.CreateWebUser(CreateWebUserInput{
		Username:     "legacy-reader",
		DisplayName:  "Legacy Reader",
		PasswordHash: "hash",
		Enabled:      true,
		RoleIDs:      []string{roleID},
	}); err != nil {
		t.Fatalf("CreateWebUser() error = %v", err)
	}

	user, err := db.GetWebUserWithPermissionsByUsername("legacy-reader")
	if err != nil {
		t.Fatalf("GetWebUserWithPermissionsByUsername() error = %v", err)
	}

	want := []string{
		"system.config_settings.read",
		"system.web_user.create",
		"system.web_user.delete",
		"system.web_user.read",
		"system.web_user.update",
		"system.web_user_credential.reset",
	}
	if !equalStringSlices(user.Permissions, want) {
		t.Fatalf("resolved user permissions = %#v, want %#v", user.Permissions, want)
	}
}

func TestCreateWebAccessRoleRejectsEmptyPermissionsAfterNormalization(t *testing.T) {
	db := openTestWebAuthDB(t)
	db.SetWebPermissionNormalizer(testNormalizeWebPermissions)

	_, err := db.CreateWebAccessRole(CreateWebAccessRoleInput{
		Name:        "normalized-empty-role",
		Description: "all provided permissions are filtered out",
		Permissions: []string{"unknown.permission", " ", "\t"},
		IsSystem:    false,
	})
	if err == nil {
		t.Fatal("expected CreateWebAccessRole to fail when normalized permissions are empty")
	}
	if !errors.Is(err, ErrWebAccessRolePermissionsEmpty) {
		t.Fatalf("expected ErrWebAccessRolePermissionsEmpty, got %v", err)
	}

	roles, err := db.ListWebAccessRoles()
	if err != nil {
		t.Fatalf("ListWebAccessRoles() error = %v", err)
	}
	if len(roles) != 0 {
		t.Fatalf("expected no role persisted on failed create, got %d", len(roles))
	}
}

func TestUpdateWebAccessRoleRejectsEmptyPermissionsAfterNormalization(t *testing.T) {
	db := openTestWebAuthDB(t)

	roleID, err := db.CreateWebAccessRole(CreateWebAccessRoleInput{
		Name:        "web-user-reader",
		Description: "canonical read permission",
		Permissions: []string{"system.web_user.read"},
		IsSystem:    false,
	})
	if err != nil {
		t.Fatalf("CreateWebAccessRole() error = %v", err)
	}

	db.SetWebPermissionNormalizer(testNormalizeWebPermissions)
	_, err = db.UpdateWebAccessRole(UpdateWebAccessRoleInput{
		ID:          roleID,
		Name:        "web-user-reader-updated",
		Description: "attempt to clear permissions through normalization",
		Permissions: []string{"unknown.permission", " ", "\n"},
	})
	if err == nil {
		t.Fatal("expected UpdateWebAccessRole to fail when normalized permissions are empty")
	}
	if !errors.Is(err, ErrWebAccessRolePermissionsEmpty) {
		t.Fatalf("expected ErrWebAccessRolePermissionsEmpty, got %v", err)
	}

	persisted, err := listPersistedRolePermissions(db, roleID)
	if err != nil {
		t.Fatalf("listPersistedRolePermissions() error = %v", err)
	}
	want := []string{"system.web_user.read"}
	if !equalStringSlices(persisted, want) {
		t.Fatalf("persisted permissions after failed update = %#v, want %#v", persisted, want)
	}
}

func listPersistedRolePermissions(db *DB, roleID string) ([]string, error) {
	rows, err := db.Query(
		`SELECT permission FROM web_access_role_permissions WHERE role_id = ? ORDER BY permission ASC`,
		roleID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	permissions := make([]string, 0)
	for rows.Next() {
		var permission string
		if err := rows.Scan(&permission); err != nil {
			return nil, err
		}
		permissions = append(permissions, permission)
	}

	return permissions, rows.Err()
}

func equalStringSlices(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func testNormalizeWebPermissions(input []string) []string {
	legacy := map[string][]string{
		"system.config.read": {
			"system.config_settings.read",
		},
		"system.config.write": {
			"system.config_settings.update",
			"system.runtime_config.apply",
			"system.model_connectivity.test",
		},
		"security.users.manage": {
			"system.web_user.read",
			"system.web_user.create",
			"system.web_user.update",
			"system.web_user.delete",
			"system.web_user_credential.reset",
		},
		"security.roles.manage": {
			"system.web_access_role.read",
			"system.web_access_role.create",
			"system.web_access_role.update",
			"system.web_access_role.delete",
		},
		"system.super_admin": {
			"system.super_admin.grant",
		},
	}

	canonical := map[string]struct{}{
		"system.config_settings.read":      {},
		"system.config_settings.update":    {},
		"system.runtime_config.apply":      {},
		"system.model_connectivity.test":   {},
		"system.web_user.read":             {},
		"system.web_user.create":           {},
		"system.web_user.update":           {},
		"system.web_user.delete":           {},
		"system.web_user_credential.reset": {},
		"system.web_access_role.read":      {},
		"system.web_access_role.create":    {},
		"system.web_access_role.update":    {},
		"system.web_access_role.delete":    {},
		"system.super_admin.grant":         {},
	}

	seen := make(map[string]struct{}, len(input))
	add := func(permission string) {
		permission = strings.TrimSpace(permission)
		if permission == "" {
			return
		}
		if _, ok := canonical[permission]; !ok {
			return
		}
		seen[permission] = struct{}{}
	}

	for _, permission := range input {
		permission = strings.TrimSpace(permission)
		if permission == "" {
			continue
		}
		if expanded, ok := legacy[permission]; ok {
			for _, candidate := range expanded {
				add(candidate)
			}
			continue
		}
		add(permission)
	}

	result := make([]string, 0, len(seen))
	for permission := range seen {
		result = append(result, permission)
	}
	sort.Strings(result)
	return result
}
