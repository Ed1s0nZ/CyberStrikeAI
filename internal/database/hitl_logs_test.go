package database

import (
	"path/filepath"
	"testing"
	"time"

	"go.uber.org/zap"
)

func ensureHitlInterruptsTable(t *testing.T, db *DB) {
	t.Helper()
	if _, err := db.Exec(`
CREATE TABLE IF NOT EXISTS hitl_interrupts (
    id TEXT PRIMARY KEY,
    conversation_id TEXT NOT NULL,
    message_id TEXT,
    mode TEXT NOT NULL,
    tool_name TEXT NOT NULL,
    tool_call_id TEXT,
    payload TEXT,
    status TEXT NOT NULL,
    decision TEXT,
    decision_comment TEXT,
    created_at DATETIME NOT NULL,
    decided_at DATETIME
);`); err != nil {
		t.Fatalf("create hitl_interrupts: %v", err)
	}
}

func TestDeleteHitlInterruptLogsByIDs_skipsPending(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "hitl.db")
	db, err := NewDB(dbPath, zap.NewNop())
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer db.Close()
	ensureHitlInterruptsTable(t, db)

	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := db.Exec(`INSERT INTO hitl_interrupts
		(id, conversation_id, mode, tool_name, status, created_at)
		VALUES ('pending-1', 'c1', 'approval', 'exec', 'pending', ?)`, now); err != nil {
		t.Fatalf("insert pending: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO hitl_interrupts
		(id, conversation_id, mode, tool_name, status, decision, created_at, decided_at)
		VALUES ('done-1', 'c1', 'approval', 'exec', 'decided', 'approve', ?, ?)`, now, now); err != nil {
		t.Fatalf("insert decided: %v", err)
	}

	deleted, err := db.DeleteHitlInterruptLogsByIDs([]string{"pending-1", "done-1"})
	if err != nil {
		t.Fatalf("DeleteHitlInterruptLogsByIDs: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1", deleted)
	}

	var status string
	if err := db.QueryRow(`SELECT status FROM hitl_interrupts WHERE id = 'pending-1'`).Scan(&status); err != nil {
		t.Fatalf("pending row missing: %v", err)
	}
	if err := db.QueryRow(`SELECT id FROM hitl_interrupts WHERE id = 'done-1'`).Scan(new(string)); err == nil {
		t.Fatal("decided row should be deleted")
	}
}

func TestPurgeHitlInterruptLogsBefore(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "hitl.db")
	db, err := NewDB(dbPath, zap.NewNop())
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer db.Close()
	ensureHitlInterruptsTable(t, db)

	old := time.Now().AddDate(0, 0, -100).UTC().Format(time.RFC3339)
	recent := time.Now().AddDate(0, 0, -1).UTC().Format(time.RFC3339)
	for _, row := range []struct{ id, decided string }{
		{"old-1", old},
		{"new-1", recent},
	} {
		if _, err := db.Exec(`INSERT INTO hitl_interrupts
			(id, conversation_id, mode, tool_name, status, decision, created_at, decided_at)
			VALUES (?, 'c1', 'approval', 'exec', 'decided', 'approve', ?, ?)`, row.id, row.decided, row.decided); err != nil {
			t.Fatalf("insert %s: %v", row.id, err)
		}
	}

	cutoff := time.Now().AddDate(0, 0, -90)
	deleted, err := db.PurgeHitlInterruptLogsBefore(cutoff)
	if err != nil {
		t.Fatalf("PurgeHitlInterruptLogsBefore: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1", deleted)
	}
	if err := db.QueryRow(`SELECT id FROM hitl_interrupts WHERE id = 'old-1'`).Scan(new(string)); err == nil {
		t.Fatal("old row should be purged")
	}
	if err := db.QueryRow(`SELECT id FROM hitl_interrupts WHERE id = 'new-1'`).Scan(new(string)); err != nil {
		t.Fatalf("new row should remain: %v", err)
	}
}
