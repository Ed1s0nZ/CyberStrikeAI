package app

import (
	"context"
	"path/filepath"
	"testing"

	"cyberstrike-ai/internal/database"
	"cyberstrike-ai/internal/security"

	"go.uber.org/zap"
)

func TestBootstrapWebRBACRunsMigrationAfterBootstrap(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "app-bootstrap.db")
	db, err := database.NewDB(dbPath, zap.NewNop())
	if err != nil {
		t.Fatalf("database.NewDB() error = %v", err)
	}

	roleID, err := db.CreateWebAccessRole(database.CreateWebAccessRoleInput{
		Name:        "legacy-role",
		Description: "legacy role",
		Permissions: nil,
		IsSystem:    false,
	})
	if err != nil {
		t.Fatalf("CreateWebAccessRole() error = %v", err)
	}

	if _, err := db.Exec(
		`INSERT INTO web_access_role_permissions (role_id, permission) VALUES (?, ?)`,
		roleID, security.PermissionSystemConfigWriteLegacy,
	); err != nil {
		t.Fatalf("insert legacy permission error = %v", err)
	}

	if err := bootstrapWebRBAC(context.Background(), db, "LegacyPass123!"); err != nil {
		t.Fatalf("bootstrapWebRBAC() error = %v", err)
	}

	admin, err := db.GetWebUserWithPermissionsByUsername("admin")
	if err != nil {
		t.Fatalf("GetWebUserWithPermissionsByUsername(admin) error = %v", err)
	}
	if _, ok := permissionSet(admin.Permissions)[security.PermissionSuperAdminGrant]; !ok {
		t.Fatalf("expected admin permissions to include %q, got %#v", security.PermissionSuperAdminGrant, admin.Permissions)
	}

	got, err := listRolePermissions(db, roleID)
	if err != nil {
		t.Fatalf("listRolePermissions() error = %v", err)
	}
	want := []string{
		security.PermissionSystemConfigSettingsUpdate,
		security.PermissionSystemModelConnectivityTest,
		security.PermissionSystemRuntimeConfigApply,
	}
	if len(got) != len(want) {
		t.Fatalf("role permissions after migration = %#v, want %#v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("role permissions after migration = %#v, want %#v", got, want)
		}
	}
}

func listRolePermissions(db *database.DB, roleID string) ([]string, error) {
	rows, err := db.Query(`SELECT permission FROM web_access_role_permissions WHERE role_id = ? ORDER BY permission ASC`, roleID)
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

func permissionSet(items []string) map[string]struct{} {
	set := make(map[string]struct{}, len(items))
	for _, item := range items {
		set[item] = struct{}{}
	}
	return set
}
