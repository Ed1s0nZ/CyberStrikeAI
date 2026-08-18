package database

import (
	"strings"
	"time"
)

func (db *DB) RecordC2PayloadArtifact(filename, payloadID, listenerID, ownerUserID string) error {
	filename = strings.TrimSpace(filename)
	if filename == "" || strings.TrimSpace(listenerID) == "" || strings.TrimSpace(ownerUserID) == "" {
		return nil
	}
	_, err := db.Exec(`
		INSERT INTO c2_payload_artifacts(filename, payload_id, listener_id, owner_user_id, created_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(filename) DO UPDATE SET payload_id=excluded.payload_id, listener_id=excluded.listener_id, owner_user_id=excluded.owner_user_id, created_at=excluded.created_at
	`, filename, payloadID, listenerID, ownerUserID, time.Now())
	return err
}

func (db *DB) UserCanAccessC2Payload(userID, scope, filename string) bool {
	if scope == RBACScopeAll {
		return true
	}
	var listenerID, ownerUserID string
	if err := db.QueryRow(`SELECT listener_id, owner_user_id FROM c2_payload_artifacts WHERE filename = ?`, strings.TrimSpace(filename)).Scan(&listenerID, &ownerUserID); err != nil {
		return false
	}
	return ownerUserID == strings.TrimSpace(userID) || db.UserCanAccessResource(userID, scope, "c2_listener", listenerID)
}
