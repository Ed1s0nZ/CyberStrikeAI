package handler

import (
	"database/sql"
	"path/filepath"
	"testing"

	"cyberstrike-ai/internal/database"

	"go.uber.org/zap"
)

func TestEnsureSchemaCancelsPendingInterruptsAfterRestart(t *testing.T) {
	db, err := database.NewDB(filepath.Join(t.TempDir(), "hitl-restart.db"), zap.NewNop())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = db.Close() }()
	manager := NewHITLManager(db, zap.NewNop())
	if err := manager.EnsureSchema(); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO hitl_interrupts
		(id, conversation_id, mode, tool_name, tool_call_id, payload, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, 'pending', CURRENT_TIMESTAMP)`,
		"restart-pending", "conversation-1", "approval", "browser", "tool-call-1", `{}`); err != nil {
		t.Fatalf("insert pending interrupt: %v", err)
	}

	if err := manager.EnsureSchema(); err != nil {
		t.Fatalf("reconcile restart: %v", err)
	}

	var status, decision, comment, decidedBy string
	var decidedAt sql.NullTime
	if err := db.QueryRow(`SELECT status, decision, decision_comment, decided_by, decided_at
		FROM hitl_interrupts WHERE id = ?`, "restart-pending").
		Scan(&status, &decision, &comment, &decidedBy, &decidedAt); err != nil {
		t.Fatalf("query reconciled interrupt: %v", err)
	}
	if status != "cancelled" || decision != "reject" || comment != "process restarted" {
		t.Fatalf("unexpected restart decision: status=%q decision=%q comment=%q", status, decision, comment)
	}
	if decidedBy != "system" {
		t.Fatalf("decided_by=%q, want system", decidedBy)
	}
	if !decidedAt.Valid {
		t.Fatal("decided_at should be set after restart reconciliation")
	}
}
