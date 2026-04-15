package security

import (
	"sort"
	"strings"
)

type permissionResourceActions map[string][]string

var canonicalPermissionCatalog = map[string]permissionResourceActions{
	"intel": {
		"fofa_query": {"execute"},
	},
	"task": {
		"batch_queue":         {"read", "create", "update", "delete", "start", "stop"},
		"batch_task":          {"create", "update", "delete"},
		"conversation":        {"read", "create", "update", "delete"},
		"group":               {"read", "create", "update", "delete"},
		"execution":           {"read", "delete"},
		"attack_chain":        {"read", "regenerate"},
		"conversation_result": {"read"},
	},
	"vulnerability": {
		"record": {"read", "create", "update", "delete"},
		"stats":  {"read"},
	},
	"webshell": {
		"connection": {"read", "create", "update", "delete", "test"},
		"session":    {"read", "update"},
		"command":    {"execute"},
		"file":       {"execute"},
	},
	"file": {
		"workspace_entry":   {"read", "create", "update", "delete"},
		"workspace_content": {"read", "update"},
	},
	"mcp": {
		"gateway":         {"execute"},
		"external_server": {"read", "create", "update", "delete", "start", "stop"},
	},
	"knowledge": {
		"category":      {"read"},
		"item":          {"read", "create", "update", "delete"},
		"index":         {"read", "execute"},
		"retrieval_log": {"read", "delete"},
		"search":        {"execute"},
		"stats":         {"read"},
	},
	"skill": {
		"definition": {"read", "create", "update", "delete"},
		"binding":    {"read"},
		"stats":      {"read", "delete"},
	},
	"agent": {
		"run":            {"read", "execute", "stop"},
		"multi_run":      {"read", "execute", "stop"},
		"markdown_agent": {"read", "create", "update", "delete"},
		"robot_test":     {"execute"},
	},
	"role": {
		"agent_role": {"read", "create", "update", "delete"},
	},
	"system": {
		"config_settings":     {"read", "update"},
		"runtime_config":      {"apply"},
		"model_connectivity":  {"test"},
		"web_user":            {"read", "create", "update", "delete"},
		"web_user_credential": {"reset"},
		"web_access_role":     {"read", "create", "update", "delete"},
		"terminal":            {"execute"},
		"api_spec":            {"read"},
		"super_admin":         {"grant"},
	},
}

var canonicalWebPermissions = buildCanonicalWebPermissions()

var canonicalWebPermissionSet = func() map[string]struct{} {
	set := make(map[string]struct{}, len(canonicalWebPermissions))
	for _, permission := range canonicalWebPermissions {
		set[permission] = struct{}{}
	}
	return set
}()

func CanonicalWebPermissions() []string {
	permissions := make([]string, len(canonicalWebPermissions))
	copy(permissions, canonicalWebPermissions)
	sort.Strings(permissions)
	return permissions
}

func IsCanonicalWebPermission(permission string) bool {
	_, ok := canonicalWebPermissionSet[permission]
	return ok
}

func NormalizeWebPermissions(input []string) []string {
	normalized := make(map[string]struct{}, len(input))
	for _, permission := range input {
		for _, expanded := range expandLegacyPermission(permission) {
			normalized[expanded] = struct{}{}
		}
	}

	result := make([]string, 0, len(normalized))
	for permission := range normalized {
		result = append(result, permission)
	}
	sort.Strings(result)
	return result
}

func expandLegacyPermission(permission string) []string {
	permission = normalizePermissionToken(permission)
	if permission == "" {
		return nil
	}

	if mapped, ok := legacyPermissionMap[permission]; ok {
		expanded := make([]string, len(mapped))
		copy(expanded, mapped)
		return expanded
	}

	if IsCanonicalWebPermission(permission) {
		return []string{permission}
	}

	return nil
}

func normalizePermissionToken(permission string) string {
	return strings.TrimSpace(permission)
}

func buildCanonicalWebPermissions() []string {
	permissions := make([]string, 0, 128)
	for domain, resources := range canonicalPermissionCatalog {
		for resource, actions := range resources {
			for _, action := range actions {
				permissions = append(permissions, domain+"."+resource+"."+action)
			}
		}
	}
	sort.Strings(permissions)
	return permissions
}
