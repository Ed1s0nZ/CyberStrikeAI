package database

import (
	"database/sql"
	"strings"
	"time"
)

const (
	legacySuperAdminPermission    = "system.super_admin"
	canonicalSuperAdminPermission = "system.super_admin.grant"
)

// WebAccessRole represents a durable Web RBAC role and its permissions.
type WebAccessRole struct {
	ID          string
	Name        string
	Description string
	IsSystem    bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Permissions []string
}

// UpdateWebUserInput describes mutable Web user fields.
type UpdateWebUserInput struct {
	ID          string
	Username    string
	DisplayName string
	Enabled     bool
	RoleIDs     []string
}

// UpdateWebAccessRoleInput describes mutable Web access role fields.
type UpdateWebAccessRoleInput struct {
	ID          string
	Name        string
	Description string
	Permissions []string
}

// GetWebUserWithPermissionsByID returns a user with resolved roles and permissions by ID.
func (db *DB) GetWebUserWithPermissionsByID(userID string) (*WebUserWithPermissions, error) {
	user, err := db.GetWebUserByID(userID)
	if err != nil {
		return nil, err
	}

	return db.GetWebUserWithPermissionsByUsername(user.Username)
}

// ListWebUsersWithPermissions returns all web users with resolved roles and permissions.
func (db *DB) ListWebUsersWithPermissions() ([]*WebUserWithPermissions, error) {
	users, err := db.ListWebUsers()
	if err != nil {
		return nil, err
	}

	result := make([]*WebUserWithPermissions, 0, len(users))
	for _, user := range users {
		resolved, err := db.GetWebUserWithPermissionsByUsername(user.Username)
		if err != nil {
			return nil, err
		}
		result = append(result, resolved)
	}

	return result, nil
}

// UpdateWebUser updates basic user fields and replaces role bindings.
func (db *DB) UpdateWebUser(input UpdateWebUserInput) (*WebUserWithPermissions, error) {
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	result, err := tx.Exec(
		`UPDATE web_users
		    SET username = ?, display_name = ?, enabled = ?, updated_at = CURRENT_TIMESTAMP
		  WHERE id = ?`,
		input.Username, input.DisplayName, input.Enabled, input.ID,
	)
	if err != nil {
		return nil, err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if rows == 0 {
		return nil, sql.ErrNoRows
	}

	if _, err := tx.Exec(`DELETE FROM web_user_role_bindings WHERE user_id = ?`, input.ID); err != nil {
		return nil, err
	}
	for _, roleID := range input.RoleIDs {
		if _, err := tx.Exec(
			`INSERT INTO web_user_role_bindings (user_id, role_id) VALUES (?, ?)`,
			input.ID, roleID,
		); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return db.GetWebUserWithPermissionsByID(input.ID)
}

// DeleteWebUser removes a web user record.
func (db *DB) DeleteWebUser(userID string) error {
	result, err := db.Exec(`DELETE FROM web_users WHERE id = ?`, userID)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// ListWebAccessRoles returns all web access roles and their permissions.
func (db *DB) ListWebAccessRoles() ([]*WebAccessRole, error) {
	rows, err := db.Query(`
		SELECT r.id, r.name, r.description, r.is_system, r.created_at, r.updated_at, p.permission
		  FROM web_access_roles r
		  LEFT JOIN web_access_role_permissions p ON p.role_id = r.id
		 ORDER BY r.name ASC, p.permission ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rolesByID := make(map[string]*WebAccessRole)
	order := make([]string, 0)
	for rows.Next() {
		var (
			roleID      string
			name        string
			description string
			isSystem    bool
			createdAt   time.Time
			updatedAt   time.Time
			permission  sql.NullString
		)
		if err := rows.Scan(&roleID, &name, &description, &isSystem, &createdAt, &updatedAt, &permission); err != nil {
			return nil, err
		}
		role, ok := rolesByID[roleID]
		if !ok {
			role = &WebAccessRole{
				ID:          roleID,
				Name:        name,
				Description: description,
				IsSystem:    isSystem,
				CreatedAt:   createdAt,
				UpdatedAt:   updatedAt,
			}
			rolesByID[roleID] = role
			order = append(order, roleID)
		}
		if permission.Valid {
			role.Permissions = append(role.Permissions, permission.String)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	roles := make([]*WebAccessRole, 0, len(order))
	for _, roleID := range order {
		role := rolesByID[roleID]
		role.Permissions = db.normalizeWebPermissions(role.Permissions)
		roles = append(roles, role)
	}
	return roles, nil
}

// GetWebAccessRoleByID returns a web access role by ID.
func (db *DB) GetWebAccessRoleByID(roleID string) (*WebAccessRole, error) {
	roles, err := db.ListWebAccessRoles()
	if err != nil {
		return nil, err
	}
	for _, role := range roles {
		if role.ID == roleID {
			return role, nil
		}
	}
	return nil, sql.ErrNoRows
}

// UpdateWebAccessRole updates a web access role and replaces its permissions.
func (db *DB) UpdateWebAccessRole(input UpdateWebAccessRoleInput) (*WebAccessRole, error) {
	rawPermissions := input.Permissions
	input.Permissions = db.normalizeWebPermissions(input.Permissions)
	if err := validateNonEmptyWebAccessRolePermissions(rawPermissions, input.Permissions); err != nil {
		return nil, err
	}

	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	result, err := tx.Exec(
		`UPDATE web_access_roles
		    SET name = ?, description = ?, updated_at = CURRENT_TIMESTAMP
		  WHERE id = ?`,
		input.Name, input.Description, input.ID,
	)
	if err != nil {
		return nil, err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if rows == 0 {
		return nil, sql.ErrNoRows
	}

	if _, err := tx.Exec(`DELETE FROM web_access_role_permissions WHERE role_id = ?`, input.ID); err != nil {
		return nil, err
	}
	for _, permission := range input.Permissions {
		if _, err := tx.Exec(
			`INSERT INTO web_access_role_permissions (role_id, permission) VALUES (?, ?)`,
			input.ID, permission,
		); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return db.GetWebAccessRoleByID(input.ID)
}

// DeleteWebAccessRole removes a web access role.
func (db *DB) DeleteWebAccessRole(roleID string) error {
	result, err := db.Exec(`DELETE FROM web_access_roles WHERE id = ?`, roleID)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// ListWebUserIDsByRoleIDs returns distinct user IDs currently bound to any of the given role IDs.
func (db *DB) ListWebUserIDsByRoleIDs(roleIDs []string) ([]string, error) {
	if len(roleIDs) == 0 {
		return nil, nil
	}

	placeholders := make([]string, 0, len(roleIDs))
	args := make([]any, 0, len(roleIDs))
	for _, roleID := range roleIDs {
		roleID = strings.TrimSpace(roleID)
		if roleID == "" {
			continue
		}
		placeholders = append(placeholders, "?")
		args = append(args, roleID)
	}
	if len(placeholders) == 0 {
		return nil, nil
	}

	rows, err := db.Query(`
		SELECT DISTINCT user_id
		  FROM web_user_role_bindings
		 WHERE role_id IN (`+strings.Join(placeholders, ",")+`)`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	userIDs := make([]string, 0)
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return nil, err
		}
		userIDs = append(userIDs, userID)
	}

	return userIDs, rows.Err()
}

// RoleIDsGrantPermission reports whether any of the given roles grants the permission.
func (db *DB) RoleIDsGrantPermission(roleIDs []string, permission string) (bool, error) {
	return db.RoleIDsGrantAnyPermission(roleIDs, []string{permission})
}

// RoleIDsGrantAnyPermission reports whether any of the given roles grants any candidate permission.
func (db *DB) RoleIDsGrantAnyPermission(roleIDs []string, permissions []string) (bool, error) {
	roleIDs = nonEmptyTrimmedTokens(roleIDs)
	permissions = expandEquivalentPermissions(permissions)
	if len(roleIDs) == 0 || len(permissions) == 0 {
		return false, nil
	}

	placeholders := make([]string, 0, len(roleIDs))
	args := make([]any, 0, len(roleIDs)+len(permissions))
	permissionPlaceholders := make([]string, 0, len(permissions))
	for _, permission := range permissions {
		permissionPlaceholders = append(permissionPlaceholders, "?")
		args = append(args, permission)
	}

	for _, roleID := range roleIDs {
		placeholders = append(placeholders, "?")
		args = append(args, roleID)
	}

	query := `
		SELECT COUNT(*)
		  FROM web_access_role_permissions
		 WHERE permission IN (` + strings.Join(permissionPlaceholders, ",") + `)
		   AND role_id IN (` + strings.Join(placeholders, ",") + `)`

	var count int
	if err := db.QueryRow(query, args...).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func nonEmptyTrimmedTokens(tokens []string) []string {
	normalized := make([]string, 0, len(tokens))
	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		normalized = append(normalized, token)
	}
	return normalized
}

func expandEquivalentPermissions(permissions []string) []string {
	permissions = nonEmptyTrimmedTokens(permissions)
	expanded := make([]string, 0, len(permissions)+1)
	seen := make(map[string]struct{}, len(permissions)+1)

	add := func(permission string) {
		if permission == "" {
			return
		}
		if _, ok := seen[permission]; ok {
			return
		}
		seen[permission] = struct{}{}
		expanded = append(expanded, permission)
	}

	for _, permission := range permissions {
		add(permission)
		switch permission {
		case legacySuperAdminPermission:
			add(canonicalSuperAdminPermission)
		case canonicalSuperAdminPermission:
			add(legacySuperAdminPermission)
		}
	}

	return expanded
}
