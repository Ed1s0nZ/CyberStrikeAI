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
	conversation, err := db.CreateConversation("restart interrupted", database.ConversationCreateMeta{})
	if err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	message, err := db.AddMessage(conversation.ID, "assistant", "处理中...", nil)
	if err != nil {
		t.Fatalf("create assistant placeholder: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO hitl_interrupts
		(id, conversation_id, message_id, mode, tool_name, tool_call_id, payload, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, 'pending', CURRENT_TIMESTAMP)`,
		"restart-pending", conversation.ID, message.ID, "approval", "browser", "tool-call-1", `{}`); err != nil {
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

	var content string
	var updatedAt sql.NullTime
	if err := db.QueryRow(`SELECT content, updated_at FROM messages WHERE id = ?`, message.ID).
		Scan(&content, &updatedAt); err != nil {
		t.Fatalf("query reconciled assistant message: %v", err)
	}
	if content != "任务因服务重启已中断，审批已取消。" {
		t.Fatalf("assistant content=%q, want restart interruption notice", content)
	}
	if !updatedAt.Valid {
		t.Fatal("assistant updated_at should be set to the interruption time")
	}
	var eventType, eventMessage string
	if err := db.QueryRow(`SELECT event_type, message FROM process_details WHERE message_id = ?`, message.ID).
		Scan(&eventType, &eventMessage); err != nil {
		t.Fatalf("query restart cancellation process detail: %v", err)
	}
	if eventType != "cancelled" || eventMessage != content {
		t.Fatalf("unexpected terminal detail: type=%q message=%q", eventType, eventMessage)
	}
}

func TestEnsureSchemaFinalizesOnlyHistoricalPlaceholdersWithTerminalEvidence(t *testing.T) {
	db, err := database.NewDB(filepath.Join(t.TempDir(), "hitl-history.db"), zap.NewNop())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = db.Close() }()
	manager := NewHITLManager(db, zap.NewNop())
	if err := manager.EnsureSchema(); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	supersededConversation, err := db.CreateConversation("superseded placeholder", database.ConversationCreateMeta{})
	if err != nil {
		t.Fatalf("create superseded conversation: %v", err)
	}
	superseded, err := db.AddMessage(supersededConversation.ID, "assistant", "处理中...", nil)
	if err != nil {
		t.Fatalf("create superseded placeholder: %v", err)
	}
	if _, err := db.AddMessage(supersededConversation.ID, "user", "继续", nil); err != nil {
		t.Fatalf("create later message: %v", err)
	}

	timeoutConversation, err := db.CreateConversation("timeout placeholder", database.ConversationCreateMeta{})
	if err != nil {
		t.Fatalf("create timeout conversation: %v", err)
	}
	timedOut, err := db.AddMessage(timeoutConversation.ID, "assistant", "处理中...", nil)
	if err != nil {
		t.Fatalf("create timeout placeholder: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO hitl_interrupts
		(id, conversation_id, message_id, mode, tool_name, status, decision, decision_comment, created_at, decided_at)
		VALUES (?, ?, ?, 'approval', 'browser', 'timeout', 'reject', 'HITL timeout auto-reject for safety', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		"timeout-interrupt", timeoutConversation.ID, timedOut.ID); err != nil {
		t.Fatalf("insert timeout interrupt: %v", err)
	}

	rejectedConversation, err := db.CreateConversation("rejected placeholder", database.ConversationCreateMeta{})
	if err != nil {
		t.Fatalf("create rejected conversation: %v", err)
	}
	rejected, err := db.AddMessage(rejectedConversation.ID, "assistant", "处理中...", nil)
	if err != nil {
		t.Fatalf("create rejected placeholder: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO hitl_interrupts
		(id, conversation_id, message_id, mode, tool_name, status, decision, decision_comment, created_at, decided_at)
		VALUES (?, ?, ?, 'approval', 'exec', 'decided', 'reject', 'user rejected', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		"rejected-interrupt", rejectedConversation.ID, rejected.ID); err != nil {
		t.Fatalf("insert rejected interrupt: %v", err)
	}

	activeConversation, err := db.CreateConversation("potentially active placeholder", database.ConversationCreateMeta{})
	if err != nil {
		t.Fatalf("create active conversation: %v", err)
	}
	potentiallyActive, err := db.AddMessage(activeConversation.ID, "assistant", "处理中...", nil)
	if err != nil {
		t.Fatalf("create potentially active placeholder: %v", err)
	}

	if err := manager.EnsureSchema(); err != nil {
		t.Fatalf("reconcile historical placeholders: %v", err)
	}

	assertTerminal := func(messageID, wantContent, wantEvent string) {
		t.Helper()
		var content, eventType string
		if err := db.QueryRow(`SELECT content FROM messages WHERE id = ?`, messageID).Scan(&content); err != nil {
			t.Fatalf("query message %s: %v", messageID, err)
		}
		if content != wantContent {
			t.Fatalf("message %s content=%q, want %q", messageID, content, wantContent)
		}
		if err := db.QueryRow(`SELECT event_type FROM process_details WHERE message_id = ?
			AND event_type IN ('cancelled', 'timeout', 'error')`, messageID).Scan(&eventType); err != nil {
			t.Fatalf("query terminal detail %s: %v", messageID, err)
		}
		if eventType != wantEvent {
			t.Fatalf("message %s event=%q, want %q", messageID, eventType, wantEvent)
		}
	}
	assertTerminal(superseded.ID, "任务因服务重启已中断。", "cancelled")
	assertTerminal(timedOut.ID, "任务等待审批超时，已自动拒绝。", "timeout")
	assertTerminal(rejected.ID, "任务审批已拒绝，执行已停止。", "cancelled")

	var activeContent string
	if err := db.QueryRow(`SELECT content FROM messages WHERE id = ?`, potentiallyActive.ID).Scan(&activeContent); err != nil {
		t.Fatalf("query potentially active message: %v", err)
	}
	if activeContent != "处理中..." {
		t.Fatalf("potentially active message was rewritten to %q", activeContent)
	}
	var terminalCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM process_details WHERE message_id = ?
		AND event_type IN ('cancelled', 'timeout', 'error')`, potentiallyActive.ID).Scan(&terminalCount); err != nil {
		t.Fatalf("count active terminal details: %v", err)
	}
	if terminalCount != 0 {
		t.Fatalf("potentially active message got %d terminal details", terminalCount)
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
