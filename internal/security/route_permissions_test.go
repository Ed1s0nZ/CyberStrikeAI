package security

import "testing"

func TestLookupRoutePermissionForSystemRoutes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		method     string
		path       string
		permission string
	}{
		{method: "GET", path: "/config", permission: PermissionSystemConfigSettingsRead},
		{method: "GET", path: "/config/tools", permission: PermissionSystemConfigSettingsRead},
		{method: "PUT", path: "/config", permission: PermissionSystemConfigSettingsUpdate},
		{method: "POST", path: "/config/apply", permission: PermissionSystemRuntimeConfigApply},
		{method: "POST", path: "/config/test-openai", permission: PermissionSystemModelConnectivityTest},
		{method: "GET", path: "/security/web-users", permission: PermissionSystemWebUserRead},
		{method: "POST", path: "/security/web-users", permission: PermissionSystemWebUserCreate},
		{method: "PUT", path: "/security/web-users/:id", permission: PermissionSystemWebUserUpdate},
		{method: "POST", path: "/security/web-users/:id/reset-password", permission: PermissionSystemWebUserCredentialReset},
		{method: "DELETE", path: "/security/web-users/:id", permission: PermissionSystemWebUserDelete},
		{method: "GET", path: "/security/web-access-roles", permission: PermissionSystemWebAccessRoleRead},
		{method: "POST", path: "/security/web-access-roles", permission: PermissionSystemWebAccessRoleCreate},
		{method: "PUT", path: "/security/web-access-roles/:id", permission: PermissionSystemWebAccessRoleUpdate},
		{method: "DELETE", path: "/security/web-access-roles/:id", permission: PermissionSystemWebAccessRoleDelete},
		{method: "GET", path: "/security/web-access-roles/permission-catalog", permission: PermissionSystemWebAccessRoleRead},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			t.Parallel()
			got, ok := LookupRoutePermission(tc.method, tc.path)
			if !ok {
				t.Fatalf("expected route permission for %s %s", tc.method, tc.path)
			}
			if got != tc.permission {
				t.Fatalf("LookupRoutePermission(%q, %q) = %q, want %q", tc.method, tc.path, got, tc.permission)
			}
		})
	}
}
