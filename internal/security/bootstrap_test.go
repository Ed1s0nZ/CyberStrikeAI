package security

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"cyberstrike-ai/internal/database"

	"go.uber.org/zap"
)

func openTestSecurityDB(t *testing.T) *database.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "security.db")
	db, err := database.NewDB(dbPath, zap.NewNop())
	if err != nil {
		t.Fatalf("database.NewDB() error = %v", err)
	}
	return db
}

func TestEnsureBootstrapAdmin_CreatesAdminFromLegacyPassword(t *testing.T) {
	db := openTestSecurityDB(t)

	if err := EnsureBootstrapAdmin(context.Background(), db, "LegacyPass123!"); err != nil {
		t.Fatalf("EnsureBootstrapAdmin() error = %v", err)
	}

	admin, err := db.GetWebUserWithPermissionsByUsername("admin")
	if err != nil {
		t.Fatalf("GetWebUserWithPermissionsByUsername(admin) error = %v", err)
	}

	if !admin.Enabled {
		t.Fatal("expected admin to be enabled")
	}

	if _, ok := permissionSet(admin.Permissions)[PermissionSuperAdmin]; !ok {
		t.Fatalf("expected admin to include %q permission, got %#v", PermissionSuperAdmin, admin.Permissions)
	}
}

func TestEnsureBootstrapAdmin_NoOpWhenUsersAlreadyExist(t *testing.T) {
	db := openTestSecurityDB(t)

	roleID, err := db.CreateWebAccessRole(database.CreateWebAccessRoleInput{
		Name:        "existing-role",
		Description: "existing role",
		Permissions: []string{PermissionSystemConfigRead},
		IsSystem:    false,
	})
	if err != nil {
		t.Fatalf("CreateWebAccessRole() error = %v", err)
	}

	passwordHash, err := HashPassword("ExistingPass123!")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	if _, err := db.CreateWebUser(database.CreateWebUserInput{
		Username:           "existing-user",
		DisplayName:        "Existing User",
		PasswordHash:       passwordHash,
		Enabled:            true,
		MustChangePassword: false,
		RoleIDs:            []string{roleID},
	}); err != nil {
		t.Fatalf("CreateWebUser() error = %v", err)
	}

	if err := EnsureBootstrapAdmin(context.Background(), db, "LegacyPass123!"); err != nil {
		t.Fatalf("EnsureBootstrapAdmin() error = %v", err)
	}

	if _, err := db.GetWebUserWithPermissionsByUsername("admin"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected admin not to be bootstrapped when users exist, got err=%v", err)
	}

	users, err := db.ListWebUsers()
	if err != nil {
		t.Fatalf("ListWebUsers() error = %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("expected exactly 1 existing user, got %d", len(users))
	}
}

func permissionSet(items []string) map[string]struct{} {
	set := make(map[string]struct{}, len(items))
	for _, item := range items {
		set[item] = struct{}{}
	}
	return set
}
