package security

import (
	"context"
	"testing"

	"cyberstrike-ai/internal/database"
)

func TestNormalizePersistedWebRBACPermissionsConvertsLegacyPermissions(t *testing.T) {
	db := openTestSecurityDB(t)

	roleID, err := db.CreateWebAccessRole(database.CreateWebAccessRoleInput{
		Name:        "legacy-role",
		Description: "legacy role",
		Permissions: nil,
		IsSystem:    false,
	})
	if err != nil {
		t.Fatalf("CreateWebAccessRole() error = %v", err)
	}

	for _, permission := range []string{
		PermissionSystemConfigWriteLegacy,
		PermissionSecurityUsersManageLegacy,
		PermissionSuperAdminLegacy,
		"unknown.permission",
	} {
		if _, err := db.Exec(
			`INSERT INTO web_access_role_permissions (role_id, permission) VALUES (?, ?)`,
			roleID, permission,
		); err != nil {
			t.Fatalf("insert legacy permission %q: %v", permission, err)
		}
	}

	if err := NormalizePersistedWebRBACPermissions(context.Background(), db); err != nil {
		t.Fatalf("NormalizePersistedWebRBACPermissions() error = %v", err)
	}

	got, err := listRolePermissions(db, roleID)
	if err != nil {
		t.Fatalf("listRolePermissions() error = %v", err)
	}

	want := []string{
		PermissionSystemConfigSettingsUpdate,
		PermissionSystemModelConnectivityTest,
		PermissionSystemRuntimeConfigApply,
		PermissionSuperAdminGrant,
		PermissionSystemWebUserCreate,
		PermissionSystemWebUserDelete,
		PermissionSystemWebUserRead,
		PermissionSystemWebUserUpdate,
		PermissionSystemWebUserCredentialReset,
	}
	if len(got) != len(want) {
		t.Fatalf("migrated permissions = %#v, want %#v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("migrated permissions = %#v, want %#v", got, want)
		}
	}
}

func TestNormalizePersistedWebRBACPermissionsIsIdempotent(t *testing.T) {
	db := openTestSecurityDB(t)

	roleID, err := db.CreateWebAccessRole(database.CreateWebAccessRoleInput{
		Name:        "canonical-role",
		Description: "canonical role",
		Permissions: []string{PermissionSystemWebUserRead},
		IsSystem:    false,
	})
	if err != nil {
		t.Fatalf("CreateWebAccessRole() error = %v", err)
	}

	if err := NormalizePersistedWebRBACPermissions(context.Background(), db); err != nil {
		t.Fatalf("first NormalizePersistedWebRBACPermissions() error = %v", err)
	}
	if err := NormalizePersistedWebRBACPermissions(context.Background(), db); err != nil {
		t.Fatalf("second NormalizePersistedWebRBACPermissions() error = %v", err)
	}

	got, err := listRolePermissions(db, roleID)
	if err != nil {
		t.Fatalf("listRolePermissions() error = %v", err)
	}
	if len(got) != 1 || got[0] != PermissionSystemWebUserRead {
		t.Fatalf("expected canonical role permissions unchanged, got %#v", got)
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
