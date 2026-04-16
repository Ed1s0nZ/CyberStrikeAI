package security

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"cyberstrike-ai/internal/database"

	"github.com/google/uuid"
)

// EnsureBootstrapAdmin creates the initial admin account when the user table is empty.
func EnsureBootstrapAdmin(ctx context.Context, db *database.DB, legacyPassword string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if db == nil {
		return errors.New("database is nil")
	}

	legacyPassword = strings.TrimSpace(legacyPassword)
	if legacyPassword == "" {
		return errors.New("legacy password must not be empty")
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var userCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM web_users`).Scan(&userCount); err != nil {
		return err
	}
	if userCount > 0 {
		return tx.Commit()
	}

	roleID, err := ensureBootstrapRole(ctx, tx)
	if err != nil {
		return err
	}

	passwordHash, err := HashPassword(legacyPassword)
	if err != nil {
		return err
	}

	userID := uuid.NewString()
	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO web_users (id, username, display_name, password_hash, enabled, must_change_password, created_at, updated_at)
		 VALUES (?, ?, ?, ?, 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		userID, bootstrapAdminUsername, "Administrator", passwordHash,
	); err != nil {
		return err
	}

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO web_user_role_bindings (user_id, role_id) VALUES (?, ?)`,
		userID, roleID,
	); err != nil {
		return err
	}

	return tx.Commit()
}

func ensureBootstrapRole(ctx context.Context, tx *sql.Tx) (string, error) {
	var roleID string
	err := tx.QueryRowContext(ctx, `SELECT id FROM web_access_roles WHERE name = ?`, "super-admin").Scan(&roleID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		roleID = uuid.NewString()
		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO web_access_roles (id, name, description, is_system, created_at, updated_at)
			 VALUES (?, ?, ?, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
			roleID, "super-admin", "Built-in super administrator role",
		); err != nil {
			return "", err
		}
	case err != nil:
		return "", err
	}

	if _, err := tx.ExecContext(
		ctx,
		`INSERT OR IGNORE INTO web_access_role_permissions (role_id, permission) VALUES (?, ?)`,
		roleID, PermissionSuperAdminGrant,
	); err != nil {
		return "", err
	}

	return roleID, nil
}
