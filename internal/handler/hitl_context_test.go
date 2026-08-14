package handler

import (
	"os"
	"path/filepath"
	"testing"

	"cyberstrike-ai/internal/database"

	"go.uber.org/zap"
)

func TestEnrichHitlApprovalPayload(t *testing.T) {
	tmp := t.TempDir()
	db, err := database.NewDB(filepath.Join(tmp, "test.sqlite"), zap.NewNop())
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	defer os.RemoveAll(tmp)

	conv, err := db.CreateConversation("hitl ctx", database.ConversationCreateMeta{})
	if err != nil {
		t.Fatalf("conv: %v", err)
	}
	if _, err := db.AddMessage(conv.ID, "user", "scan 10.0.0.1 please", nil); err != nil {
		t.Fatalf("user msg: %v", err)
	}
	asst, err := db.AddMessage(conv.ID, "assistant", "", nil)
	if err != nil {
		t.Fatalf("asst msg: %v", err)
	}
	if err := db.AddProcessDetail(asst.ID, conv.ID, "thinking", "need port scan first", nil); err != nil {
		t.Fatalf("detail: %v", err)
	}

	h := &AgentHandler{db: db, tasks: NewAgentTaskManager()}
	payload := map[string]interface{}{"toolName": "nmap", "arguments": "{}"}
	h.enrichHitlApprovalPayload(conv.ID, asst.ID, payload)

	if got := payload["userMessage"]; got != "scan 10.0.0.1 please" {
		t.Fatalf("userMessage=%v", got)
	}
	if got := payload["thinking"]; got != "need port scan first" {
		t.Fatalf("thinking=%v", got)
	}
}
