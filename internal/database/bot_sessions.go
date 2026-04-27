package database

import (
	"database/sql"
	"fmt"
	"time"
)

// BotSession is the persistent per-user session record for bot
// platforms. Replaces the in-memory map RobotHandler used to keep
// (which evaporated on every restart). Mode override and current
// conversation are both stored here; ON DELETE SET NULL on the
// conversation_id FK lets bot users keep their mode override even
// when the operator deletes the conversation through the web UI.
type BotSession struct {
	Platform       string
	UserID         string
	ConversationID string // empty string when conversation was deleted (FK SET NULL)
	CurrentMode    string // "" = inherit global; "single" | "multi"
	UpdatedAt      int64
}

// GetBotSession returns the session row for (platform, user_id), or
// (nil, nil) if no row exists. Errors are reserved for actual DB
// failures.
func (db *DB) GetBotSession(platform, userID string) (*BotSession, error) {
	row := db.QueryRow(`
		SELECT platform, user_id, COALESCE(conversation_id, ''),
		       COALESCE(current_mode, ''), updated_at
		FROM bot_sessions
		WHERE platform = ? AND user_id = ?`, platform, userID)
	var s BotSession
	if err := row.Scan(&s.Platform, &s.UserID, &s.ConversationID, &s.CurrentMode, &s.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get bot session: %w", err)
	}
	return &s, nil
}

// UpsertBotSession creates or updates the row keyed by (platform,
// user_id). conversationID empty string is stored as SQL NULL. mode
// empty string is stored as SQL NULL (= inherit global default).
func (db *DB) UpsertBotSession(platform, userID, conversationID, mode string) error {
	now := time.Now().UnixNano()
	var convArg interface{}
	if conversationID != "" {
		convArg = conversationID
	}
	var modeArg interface{}
	if mode != "" {
		modeArg = mode
	}
	_, err := db.Exec(`
		INSERT INTO bot_sessions (platform, user_id, conversation_id, current_mode, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(platform, user_id) DO UPDATE SET
		    conversation_id = excluded.conversation_id,
		    current_mode    = excluded.current_mode,
		    updated_at      = excluded.updated_at`,
		platform, userID, convArg, modeArg, now)
	if err != nil {
		return fmt.Errorf("upsert bot session: %w", err)
	}
	return nil
}

// ClearBotSession wipes the row entirely. Used by the `clear` slash
// command — operator-stated semantics in the spec: clear is "I want
// a fresh start"; mode override is also reset to global default.
func (db *DB) ClearBotSession(platform, userID string) error {
	_, err := db.Exec(`DELETE FROM bot_sessions WHERE platform = ? AND user_id = ?`, platform, userID)
	if err != nil {
		return fmt.Errorf("clear bot session: %w", err)
	}
	return nil
}

// SetBotMode updates only the current_mode column on the row,
// preserving conversation_id and updated_at semantics. Empty string
// for mode means "inherit global default" — stored as NULL.
//
// Upserts: setting mode for a (platform, user_id) without an
// existing session is valid (e.g., user runs `mode multi` before
// any chat message).
func (db *DB) SetBotMode(platform, userID, mode string) error {
	now := time.Now().UnixNano()
	var modeArg interface{}
	if mode != "" {
		modeArg = mode
	}
	_, err := db.Exec(`
		INSERT INTO bot_sessions (platform, user_id, conversation_id, current_mode, updated_at)
		VALUES (?, ?, NULL, ?, ?)
		ON CONFLICT(platform, user_id) DO UPDATE SET
		    current_mode = excluded.current_mode,
		    updated_at   = excluded.updated_at`,
		platform, userID, modeArg, now)
	if err != nil {
		return fmt.Errorf("set bot mode: %w", err)
	}
	return nil
}
