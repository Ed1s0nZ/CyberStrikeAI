package database

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// WebUser represents a persisted web user account.
type WebUser struct {
	ID                 string
	Username           string
	DisplayName        string
	PasswordHash       string
	Enabled            bool
	MustChangePassword bool
	CreatedAt          time.Time
	UpdatedAt          time.Time
	LastLoginAt        sql.NullTime
}

// WebUserWithPermissions includes effective role and permission data for a user.
type WebUserWithPermissions struct {
	WebUser
	RoleIDs     []string
	RoleNames   []string
	Permissions []string
}

// CreateWebUserInput describes the data needed to create a web user.
type CreateWebUserInput struct {
	Username           string
	DisplayName        string
	PasswordHash       string
	Enabled            bool
	MustChangePassword bool
	RoleIDs            []string
}

// CreateWebAccessRoleInput describes the data needed to create an access role.
type CreateWebAccessRoleInput struct {
	Name        string
	Description string
	Permissions []string
	IsSystem    bool
}

// CreateWebAccessRole inserts a role with permissions.
func (db *DB) CreateWebAccessRole(input CreateWebAccessRoleInput) (string, error) {
	roleID := uuid.NewString()
	tx, err := db.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(
		`INSERT INTO web_access_roles (id, name, description, is_system, created_at, updated_at)
		 VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		roleID, input.Name, input.Description, input.IsSystem,
	); err != nil {
		return "", err
	}

	for _, permission := range input.Permissions {
		if _, err := tx.Exec(
			`INSERT INTO web_access_role_permissions (role_id, permission) VALUES (?, ?)`,
			roleID, permission,
		); err != nil {
			return "", err
		}
	}

	if err := tx.Commit(); err != nil {
		return "", err
	}

	return roleID, nil
}

// CreateWebUser inserts a user and attaches roles.
func (db *DB) CreateWebUser(input CreateWebUserInput) (*WebUser, error) {
	userID := uuid.NewString()
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(
		`INSERT INTO web_users (id, username, display_name, password_hash, enabled, must_change_password, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		userID, input.Username, input.DisplayName, input.PasswordHash, input.Enabled, input.MustChangePassword,
	); err != nil {
		return nil, err
	}

	for _, roleID := range input.RoleIDs {
		if _, err := tx.Exec(
			`INSERT INTO web_user_role_bindings (user_id, role_id) VALUES (?, ?)`,
			userID, roleID,
		); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return db.GetWebUserByID(userID)
}

// GetWebUserByID returns a user record.
func (db *DB) GetWebUserByID(userID string) (*WebUser, error) {
	row := db.QueryRow(`
		SELECT id, username, display_name, password_hash, enabled, must_change_password, created_at, updated_at, last_login_at
		  FROM web_users
		 WHERE id = ?`,
		userID,
	)

	var user WebUser
	if err := row.Scan(
		&user.ID,
		&user.Username,
		&user.DisplayName,
		&user.PasswordHash,
		&user.Enabled,
		&user.MustChangePassword,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.LastLoginAt,
	); err != nil {
		return nil, err
	}

	return &user, nil
}

// ListWebUsers returns all users ordered by username.
func (db *DB) ListWebUsers() ([]*WebUser, error) {
	rows, err := db.Query(`
		SELECT id, username, display_name, password_hash, enabled, must_change_password, created_at, updated_at, last_login_at
		  FROM web_users
		 ORDER BY username ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*WebUser
	for rows.Next() {
		var user WebUser
		if err := rows.Scan(
			&user.ID,
			&user.Username,
			&user.DisplayName,
			&user.PasswordHash,
			&user.Enabled,
			&user.MustChangePassword,
			&user.CreatedAt,
			&user.UpdatedAt,
			&user.LastLoginAt,
		); err != nil {
			return nil, err
		}
		users = append(users, &user)
	}
	return users, rows.Err()
}

// UpdateWebUserLastLogin updates last_login_at.
func (db *DB) UpdateWebUserLastLogin(userID string, at time.Time) error {
	result, err := db.Exec(
		`UPDATE web_users SET last_login_at = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		at, userID,
	)
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

// UpdateWebUserPasswordByUsername updates password_hash and must_change_password for a user.
func (db *DB) UpdateWebUserPasswordByUsername(username, passwordHash string, mustChangePassword bool) error {
	result, err := db.Exec(
		`UPDATE web_users
		    SET password_hash = ?, must_change_password = ?, updated_at = CURRENT_TIMESTAMP
		  WHERE username = ?`,
		passwordHash, mustChangePassword, username,
	)
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

// UpdateWebUserPasswordByID updates password_hash and must_change_password for a user ID.
func (db *DB) UpdateWebUserPasswordByID(userID, passwordHash string, mustChangePassword bool) error {
	result, err := db.Exec(
		`UPDATE web_users
		    SET password_hash = ?, must_change_password = ?, updated_at = CURRENT_TIMESTAMP
		  WHERE id = ?`,
		passwordHash, mustChangePassword, userID,
	)
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

// CountEnabledUsersWithPermission returns the number of enabled users that have the given permission.
func (db *DB) CountEnabledUsersWithPermission(permission string) (int, error) {
	row := db.QueryRow(`
		SELECT COUNT(DISTINCT u.id)
		  FROM web_users u
		  JOIN web_user_role_bindings b ON b.user_id = u.id
		  JOIN web_access_role_permissions p ON p.role_id = b.role_id
		 WHERE u.enabled = 1 AND p.permission = ?`,
		permission,
	)

	var count int
	if err := row.Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// GetWebUserWithPermissionsByUsername returns a user with resolved role and permission lists.
func (db *DB) GetWebUserWithPermissionsByUsername(username string) (*WebUserWithPermissions, error) {
	row := db.QueryRow(`
		SELECT id, username, display_name, password_hash, enabled, must_change_password,
		       created_at, updated_at, last_login_at
		  FROM web_users
		 WHERE username = ?`,
		username,
	)

	var user WebUserWithPermissions
	if err := row.Scan(
		&user.ID,
		&user.Username,
		&user.DisplayName,
		&user.PasswordHash,
		&user.Enabled,
		&user.MustChangePassword,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.LastLoginAt,
	); err != nil {
		return nil, err
	}

	rows, err := db.Query(`
		SELECT r.id, r.name, p.permission
		  FROM web_user_role_bindings b
		  JOIN web_access_roles r ON r.id = b.role_id
		  LEFT JOIN web_access_role_permissions p ON p.role_id = r.id
		 WHERE b.user_id = ?
		 ORDER BY r.name ASC, p.permission ASC`,
		user.ID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	roleSeen := make(map[string]bool)
	permSeen := make(map[string]bool)
	for rows.Next() {
		var roleID, roleName string
		var permission sql.NullString
		if err := rows.Scan(&roleID, &roleName, &permission); err != nil {
			return nil, err
		}
		if !roleSeen[roleID] {
			user.RoleIDs = append(user.RoleIDs, roleID)
			user.RoleNames = append(user.RoleNames, roleName)
			roleSeen[roleID] = true
		}
		if permission.Valid {
			if !permSeen[permission.String] {
				user.Permissions = append(user.Permissions, permission.String)
				permSeen[permission.String] = true
			}
		}
	}

	return &user, rows.Err()
}
