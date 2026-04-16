package security

import "strings"

var routePermissionRegistry = map[string]string{
	"GET /config":                                       PermissionSystemConfigSettingsRead,
	"GET /config/tools":                                 PermissionSystemConfigSettingsRead,
	"PUT /config":                                       PermissionSystemConfigSettingsUpdate,
	"POST /config/apply":                                PermissionSystemRuntimeConfigApply,
	"POST /config/test-openai":                          PermissionSystemModelConnectivityTest,
	"GET /security/web-users":                           PermissionSystemWebUserRead,
	"POST /security/web-users":                          PermissionSystemWebUserCreate,
	"PUT /security/web-users/:id":                       PermissionSystemWebUserUpdate,
	"POST /security/web-users/:id/reset-password":       PermissionSystemWebUserCredentialReset,
	"DELETE /security/web-users/:id":                    PermissionSystemWebUserDelete,
	"GET /security/web-access-roles":                    PermissionSystemWebAccessRoleRead,
	"POST /security/web-access-roles":                   PermissionSystemWebAccessRoleCreate,
	"PUT /security/web-access-roles/:id":                PermissionSystemWebAccessRoleUpdate,
	"DELETE /security/web-access-roles/:id":             PermissionSystemWebAccessRoleDelete,
	"GET /security/web-access-roles/permission-catalog": PermissionSystemWebAccessRoleRead,
}

func LookupRoutePermission(method, path string) (string, bool) {
	key := strings.ToUpper(strings.TrimSpace(method)) + " " + strings.TrimSpace(path)
	permission, ok := routePermissionRegistry[key]
	return permission, ok
}
