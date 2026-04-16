package security

import (
	"context"
	"errors"

	"cyberstrike-ai/internal/database"
)

// NormalizePersistedWebRBACPermissions migrates persisted legacy Web RBAC permissions to canonical identifiers.
func NormalizePersistedWebRBACPermissions(ctx context.Context, db *database.DB) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if db == nil {
		return errors.New("database is nil")
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, `
		SELECT role_id, permission
		  FROM web_access_role_permissions
		 ORDER BY role_id ASC, permission ASC`,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	permissionsByRole := make(map[string][]string)
	order := make([]string, 0)
	for rows.Next() {
		var roleID, permission string
		if err := rows.Scan(&roleID, &permission); err != nil {
			return err
		}
		if _, ok := permissionsByRole[roleID]; !ok {
			order = append(order, roleID)
		}
		permissionsByRole[roleID] = append(permissionsByRole[roleID], permission)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, roleID := range order {
		if err := ctx.Err(); err != nil {
			return err
		}

		normalized := NormalizeWebPermissions(permissionsByRole[roleID])
		if _, err := tx.ExecContext(ctx, `DELETE FROM web_access_role_permissions WHERE role_id = ?`, roleID); err != nil {
			return err
		}
		for _, permission := range normalized {
			if _, err := tx.ExecContext(
				ctx,
				`INSERT INTO web_access_role_permissions (role_id, permission) VALUES (?, ?)`,
				roleID, permission,
			); err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}
