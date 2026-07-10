package database

import (
	"strings"
	"time"
)

func (db *DB) UpsertChatUploadArtifact(relativePath, conversationID, ownerUserID string) error {
	relativePath = strings.TrimSpace(relativePath)
	conversationID = strings.TrimSpace(conversationID)
	ownerUserID = strings.TrimSpace(ownerUserID)
	if relativePath == "" || conversationID == "" || ownerUserID == "" {
		return nil
	}
	_, err := db.Exec(`
		INSERT INTO chat_upload_artifacts(relative_path, conversation_id, owner_user_id, created_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(relative_path) DO UPDATE SET conversation_id=excluded.conversation_id, owner_user_id=excluded.owner_user_id
	`, relativePath, conversationID, ownerUserID, time.Now())
	return err
}

func (db *DB) GetChatUploadArtifact(relativePath string) (conversationID, ownerUserID string, ok bool) {
	err := db.QueryRow(`SELECT conversation_id, owner_user_id FROM chat_upload_artifacts WHERE relative_path = ?`, strings.TrimSpace(relativePath)).Scan(&conversationID, &ownerUserID)
	return conversationID, ownerUserID, err == nil
}

func (db *DB) DeleteChatUploadArtifactPath(relativePath string) error {
	path := strings.Trim(strings.TrimSpace(relativePath), "/")
	if path == "" {
		return nil
	}
	_, err := db.Exec(`DELETE FROM chat_upload_artifacts WHERE relative_path = ? OR relative_path LIKE ? ESCAPE '\'`, path, escapeLikePrefix(path)+"/%")
	return err
}

func (db *DB) RenameChatUploadArtifactPath(oldPath, newPath string) error {
	oldPath = strings.Trim(strings.TrimSpace(oldPath), "/")
	newPath = strings.Trim(strings.TrimSpace(newPath), "/")
	if oldPath == "" || newPath == "" {
		return nil
	}
	_, err := db.Exec(`
		UPDATE chat_upload_artifacts
		SET relative_path = CASE
			WHEN relative_path = ? THEN ?
			ELSE ? || substr(relative_path, length(?) + 1)
		END
		WHERE relative_path = ? OR relative_path LIKE ? ESCAPE '\'
	`, oldPath, newPath, newPath, oldPath, oldPath, escapeLikePrefix(oldPath)+"/%")
	return err
}

func escapeLikePrefix(value string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(value)
}
