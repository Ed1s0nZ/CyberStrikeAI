package database

import "strings"

// SetWebPermissionNormalizer configures role permission normalization for Web RBAC persistence.
// Passing nil disables custom normalization and falls back to trimmed de-duplication.
func (db *DB) SetWebPermissionNormalizer(normalizer func([]string) []string) {
	db.webPermissionNormalizer = normalizer
}

func (db *DB) normalizeWebPermissions(input []string) []string {
	if len(input) == 0 {
		return nil
	}

	if db.webPermissionNormalizer == nil {
		return dedupeTrimmedPermissions(input)
	}

	return dedupeTrimmedPermissions(db.webPermissionNormalizer(input))
}

func dedupeTrimmedPermissions(input []string) []string {
	result := make([]string, 0, len(input))
	seen := make(map[string]struct{}, len(input))
	for _, permission := range input {
		permission = strings.TrimSpace(permission)
		if permission == "" {
			continue
		}
		if _, ok := seen[permission]; ok {
			continue
		}
		seen[permission] = struct{}{}
		result = append(result, permission)
	}
	return result
}
