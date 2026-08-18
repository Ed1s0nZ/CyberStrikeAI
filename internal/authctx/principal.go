package authctx

import (
	"context"
	"strings"
)

// Principal is the immutable authorization identity propagated beyond the
// transport layer into Agent, MCP and background task contexts.
type Principal struct {
	UserID           string
	Username         string
	Permissions      map[string]bool
	PermissionScopes map[string]string
	Scope            string
}

type principalContextKey struct{}

func NewPrincipal(userID, username, scope string, permissions map[string]bool) Principal {
	return NewPrincipalWithScopes(userID, username, scope, permissions, nil)
}

func NewPrincipalWithScopes(userID, username, scope string, permissions map[string]bool, permissionScopes map[string]string) Principal {
	permissionCopy := make(map[string]bool, len(permissions))
	scopeCopy := make(map[string]string, len(permissionScopes))
	for permission, allowed := range permissions {
		if allowed {
			permissionCopy[permission] = true
			if permissionScope := strings.TrimSpace(permissionScopes[permission]); permissionScope != "" {
				scopeCopy[permission] = permissionScope
			}
		}
	}
	return Principal{
		UserID: strings.TrimSpace(userID), Username: strings.TrimSpace(username),
		Scope: strings.TrimSpace(scope), Permissions: permissionCopy, PermissionScopes: scopeCopy,
	}
}

func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(principal.UserID) == "" {
		return ctx
	}
	return context.WithValue(ctx, principalContextKey{}, principal)
}

func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	if ctx == nil {
		return Principal{}, false
	}
	principal, ok := ctx.Value(principalContextKey{}).(Principal)
	return principal, ok && strings.TrimSpace(principal.UserID) != ""
}

func (p Principal) HasPermission(permission string) bool {
	return p.Permissions[strings.TrimSpace(permission)]
}

// ScopeFor returns the scope attached to the permission that authorizes the
// current action. Falling back to Scope keeps explicit service principals and
// legacy callers compatible without reintroducing cross-role scope widening.
func (p Principal) ScopeFor(permission string) string {
	permission = strings.TrimSpace(permission)
	if scope := strings.TrimSpace(p.PermissionScopes[permission]); scope != "" {
		return scope
	}
	return strings.TrimSpace(p.Scope)
}
