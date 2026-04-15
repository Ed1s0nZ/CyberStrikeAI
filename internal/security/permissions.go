package security

const (
	PermissionSuperAdmin          = "system.super_admin"
	PermissionSystemConfigRead    = "system.config.read"
	PermissionSystemConfigWrite   = "system.config.write"
	PermissionSecurityUsersManage = "security.users.manage"
	PermissionSecurityRolesManage = "security.roles.manage"
)

// HasPermission returns true when the required permission is present or the user is a super admin.
func HasPermission(permissionSet map[string]struct{}, required string) bool {
	if _, ok := permissionSet[PermissionSuperAdmin]; ok {
		return true
	}

	_, ok := permissionSet[required]
	return ok
}
