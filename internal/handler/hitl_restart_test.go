package handler

import (
	"database/sql"
	"path/filepath"
	"strings"
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

func TestAuditAgentInterruptIsNotHumanPendingWork(t *testing.T) {
	db, err := database.NewDB(filepath.Join(t.TempDir(), "hitl-reviewer.db"), zap.NewNop())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = db.Close() }()
	manager := NewHITLManager(db, zap.NewNop())
	if err := manager.EnsureSchema(); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	audit, err := manager.CreatePendingInterrupt("conversation-audit", "message-audit", "review_edit", "exec", "call-audit", `{}`, "audit_agent")
	if err != nil {
		t.Fatalf("create audit interrupt: %v", err)
	}
	human, err := manager.CreatePendingInterrupt("conversation-human", "message-human", "approval", "exec", "call-human", `{}`, "human")
	if err != nil {
		t.Fatalf("create human interrupt: %v", err)
	}

	manager.mu.RLock()
	_, auditWaitsForHuman := manager.pending[audit.InterruptID]
	_, humanWaitsForHuman := manager.pending[human.InterruptID]
	manager.mu.RUnlock()
	if auditWaitsForHuman {
		t.Fatal("audit-agent interrupt must not enter the human pending queue")
	}
	if !humanWaitsForHuman {
		t.Fatal("human interrupt should enter the human pending queue")
	}

	query, args := (&AgentHandler{}).buildHitlListQuery(false)
	if len(args) != 0 {
		t.Fatalf("unexpected pending query args: %v", args)
	}
	if !strings.Contains(query, "COALESCE(reviewer,'human') = 'human'") {
		t.Fatalf("pending query must filter out audit-agent work: %s", query)
	}
	rows, err := db.Query(query)
	if err != nil {
		t.Fatalf("query human pending interrupts: %v", err)
	}
	defer rows.Close()
	items, err := (&AgentHandler{}).scanHitlInterruptRows(rows)
	if err != nil {
		t.Fatalf("scan human pending interrupts: %v", err)
	}
	if len(items) != 1 || items[0]["id"] != human.InterruptID || items[0]["reviewer"] != "human" {
		t.Fatalf("unexpected human pending result: %#v", items)
	}
}
