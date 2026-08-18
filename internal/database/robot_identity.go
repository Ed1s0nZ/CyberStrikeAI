package database

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// RobotUserBinding maps one tenant-scoped platform identity to one RBAC user.
// external_user_id must be derived from the verified platform event, never
// from user-controlled message content.
type RobotUserBinding struct {
	ID             string    `json:"id"`
	Platform       string    `json:"platform"`
	ExternalUserID string    `json:"externalUserId"`
	RBACUserID     string    `json:"rbacUserId"`
	Enabled        bool      `json:"enabled"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

func normalizeRobotIdentity(platform, externalUserID string) (string, string, error) {
	platform = strings.ToLower(strings.TrimSpace(platform))
	externalUserID = strings.TrimSpace(externalUserID)
	if platform == "" || externalUserID == "" {
		return "", "", fmt.Errorf("robot platform and external user identity are required")
	}
	return platform, externalUserID, nil
}

func (db *DB) CreateRobotBindingCode(userID, codeHash string, expiresAt time.Time) error {
	userID = strings.TrimSpace(userID)
	codeHash = strings.TrimSpace(codeHash)
	if userID == "" || codeHash == "" || !expiresAt.After(time.Now()) {
		return fmt.Errorf("invalid robot binding code")
	}
	now := time.Now()
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	// Keep only the newest active code per user and remove expired/used secrets.
	if _, err = tx.Exec(`DELETE FROM robot_binding_codes WHERE rbac_user_id = ? OR expires_at <= ? OR used_at IS NOT NULL`, userID, now); err != nil {
		return err
	}
	if _, err = tx.Exec(`INSERT INTO robot_binding_codes (code_hash, rbac_user_id, expires_at, created_at) VALUES (?, ?, ?, ?)`, codeHash, userID, expiresAt, now); err != nil {
		return err
	}
	return tx.Commit()
}

// ConsumeRobotBindingCode atomically consumes a single-use code and binds the
// verified platform identity. Existing bindings are deliberately replaced so
// users can recover from stale or incorrect associations with a fresh code.
func (db *DB) ConsumeRobotBindingCode(platform, externalUserID, codeHash string) (*RBACUser, error) {
	platform, externalUserID, err := normalizeRobotIdentity(platform, externalUserID)
	if err != nil {
		return nil, err
	}
	codeHash = strings.TrimSpace(codeHash)
	if codeHash == "" {
		return nil, fmt.Errorf("binding code is required")
	}
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var userID string
	now := time.Now()
	if err = tx.QueryRow(`
		SELECT c.rbac_user_id
		FROM robot_binding_codes c
		JOIN rbac_users u ON u.id = c.rbac_user_id
		WHERE c.code_hash = ? AND c.used_at IS NULL AND c.expires_at > ? AND u.enabled = 1
	`, codeHash, now).Scan(&userID); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("binding code is invalid or expired")
		}
		return nil, err
	}
	result, err := tx.Exec(`UPDATE robot_binding_codes SET used_at = ? WHERE code_hash = ? AND used_at IS NULL`, now, codeHash)
	if err != nil {
		return nil, err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return nil, fmt.Errorf("binding code has already been used")
	}
	if _, err = tx.Exec(`
		INSERT INTO robot_user_bindings (id, platform, external_user_id, rbac_user_id, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, 1, ?, ?)
		ON CONFLICT(platform, external_user_id) DO UPDATE SET
			rbac_user_id = excluded.rbac_user_id,
			enabled = 1,
			updated_at = excluded.updated_at
	`, uuid.New().String(), platform, externalUserID, userID, now, now); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return db.GetRBACUserByID(userID)
}

func (db *DB) ResolveRobotRBACAccess(platform, externalUserID string) (*RBACAccess, error) {
	platform, externalUserID, err := normalizeRobotIdentity(platform, externalUserID)
	if err != nil {
		return nil, err
	}
	var userID string
	err = db.QueryRow(`
		SELECT b.rbac_user_id
		FROM robot_user_bindings b
		JOIN rbac_users u ON u.id = b.rbac_user_id
		WHERE b.platform = ? AND b.external_user_id = ? AND b.enabled = 1 AND u.enabled = 1
	`, platform, externalUserID).Scan(&userID)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("robot identity is not bound")
	}
	if err != nil {
		return nil, err
	}
	return db.ResolveRBACAccess(userID)
}

func (db *DB) ListRobotUserBindings(userID string) ([]RobotUserBinding, error) {
	rows, err := db.Query(`
		SELECT id, platform, external_user_id, rbac_user_id, enabled, created_at, updated_at
		FROM robot_user_bindings WHERE rbac_user_id = ? ORDER BY updated_at DESC
	`, strings.TrimSpace(userID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RobotUserBinding
	for rows.Next() {
		var b RobotUserBinding
		var enabled int
		var createdAt, updatedAt string
		if err := rows.Scan(&b.ID, &b.Platform, &b.ExternalUserID, &b.RBACUserID, &enabled, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		b.Enabled = enabled != 0
		b.CreatedAt = parseDBTime(createdAt)
		b.UpdatedAt = parseDBTime(updatedAt)
		out = append(out, b)
	}
	return out, rows.Err()
}

func (db *DB) DeleteRobotUserBindingForUser(bindingID, userID string) error {
	result, err := db.Exec(`DELETE FROM robot_user_bindings WHERE id = ? AND rbac_user_id = ?`, strings.TrimSpace(bindingID), strings.TrimSpace(userID))
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return sql.ErrNoRows
	}
	return nil
}

func (db *DB) DeleteRobotIdentityBinding(platform, externalUserID string) error {
	platform, externalUserID, err := normalizeRobotIdentity(platform, externalUserID)
	if err != nil {
		return err
	}
	_, err = db.Exec(`DELETE FROM robot_user_bindings WHERE platform = ? AND external_user_id = ?`, platform, externalUserID)
	return err
}
