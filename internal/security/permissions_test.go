package security

import "testing"

func TestLegacyPermissionConstantsRemainLegacyIdentifiers(t *testing.T) {
	if PermissionSuperAdmin != PermissionSuperAdminLegacy {
		t.Fatalf("PermissionSuperAdmin = %q, want %q", PermissionSuperAdmin, PermissionSuperAdminLegacy)
	}
	if PermissionSuperAdmin == PermissionSuperAdminGrant {
		t.Fatalf("PermissionSuperAdmin should not alias canonical grant %q", PermissionSuperAdminGrant)
	}

	if PermissionSystemConfigRead != PermissionSystemConfigReadLegacy {
		t.Fatalf("PermissionSystemConfigRead = %q, want %q", PermissionSystemConfigRead, PermissionSystemConfigReadLegacy)
	}
	if PermissionSystemConfigRead == PermissionSystemConfigSettingsRead {
		t.Fatalf("PermissionSystemConfigRead should not alias canonical permission %q", PermissionSystemConfigSettingsRead)
	}
}

func TestHasPermissionLegacyCanonicalCompatibilityMatrix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		permissions map[string]struct{}
		required    string
		want        bool
	}{
		{
			name:        "legacy set with legacy required",
			permissions: permissionSetForTest(PermissionSystemConfigReadLegacy),
			required:    PermissionSystemConfigReadLegacy,
			want:        true,
		},
		{
			name:        "legacy set with canonical required",
			permissions: permissionSetForTest(PermissionSystemConfigReadLegacy),
			required:    PermissionSystemConfigSettingsRead,
			want:        true,
		},
		{
			name:        "canonical set with legacy required",
			permissions: permissionSetForTest(PermissionSystemConfigSettingsRead),
			required:    PermissionSystemConfigReadLegacy,
			want:        true,
		},
		{
			name:        "canonical set with canonical required",
			permissions: permissionSetForTest(PermissionSystemConfigSettingsRead),
			required:    PermissionSystemConfigSettingsRead,
			want:        true,
		},
		{
			name: "legacy write set satisfies canonical required action",
			permissions: permissionSetForTest(
				PermissionSystemConfigWriteLegacy,
			),
			required: PermissionSystemRuntimeConfigApply,
			want:     true,
		},
		{
			name: "canonical partial write actions do not imply legacy write",
			permissions: permissionSetForTest(
				PermissionSystemConfigSettingsUpdate,
			),
			required: PermissionSystemConfigWriteLegacy,
			want:     false,
		},
		{
			name: "canonical full write actions imply legacy write",
			permissions: permissionSetForTest(
				PermissionSystemConfigSettingsUpdate,
				PermissionSystemRuntimeConfigApply,
				PermissionSystemModelConnectivityTest,
			),
			required: PermissionSystemConfigWriteLegacy,
			want:     true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := HasPermission(tc.permissions, tc.required)
			if got != tc.want {
				t.Fatalf("HasPermission(%#v, %q) = %v, want %v", tc.permissions, tc.required, got, tc.want)
			}
		})
	}
}

func TestHasPermissionSuperAdminBypassSupportsLegacyAndCanonical(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		permissions map[string]struct{}
		required    string
	}{
		{
			name:        "legacy super admin bypasses canonical required",
			permissions: permissionSetForTest(PermissionSuperAdminLegacy),
			required:    PermissionSystemConfigSettingsRead,
		},
		{
			name:        "legacy super admin bypasses unknown required",
			permissions: permissionSetForTest(PermissionSuperAdminLegacy),
			required:    "unknown.permission",
		},
		{
			name:        "canonical super admin bypasses legacy required",
			permissions: permissionSetForTest(PermissionSuperAdminGrant),
			required:    PermissionSecurityUsersManageLegacy,
		},
		{
			name:        "canonical super admin bypasses legacy super admin required",
			permissions: permissionSetForTest(PermissionSuperAdminGrant),
			required:    PermissionSuperAdminLegacy,
		},
		{
			name:        "legacy super admin bypasses canonical super admin required",
			permissions: permissionSetForTest(PermissionSuperAdminLegacy),
			required:    PermissionSuperAdminGrant,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if !HasPermission(tc.permissions, tc.required) {
				t.Fatalf("HasPermission(%#v, %q) = false, want true", tc.permissions, tc.required)
			}
		})
	}
}

func permissionSetForTest(permissions ...string) map[string]struct{} {
	set := make(map[string]struct{}, len(permissions))
	for _, permission := range permissions {
		set[permission] = struct{}{}
	}
	return set
}
