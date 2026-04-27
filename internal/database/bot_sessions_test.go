package database

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"
)

func openBotSessionsTestDB(t *testing.T) *DB {
	t.Helper()
	tmp := filepath.Join(t.TempDir(), "bot_sessions_test.db")
	db, err := sql.Open("sqlite3", "file:"+tmp+"?_foreign_keys=1")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	logger, _ := zap.NewDevelopment()
	d := &DB{DB: db, logger: logger}
	if err := d.initTables(); err != nil {
		t.Fatalf("initTables: %v", err)
	}
	return d
}

func TestBotSession_RoundTrip(t *testing.T) {
	d := openBotSessionsTestDB(t)
	conv, err := d.CreateConversation("test")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	if err := d.UpsertBotSession("telegram", "u1", conv.ID, "multi"); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	got, err := d.GetBotSession("telegram", "u1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatalf("Get returned nil")
	}
	if got.ConversationID != conv.ID || got.CurrentMode != "multi" {
		t.Fatalf("fields mismatch: %+v", got)
	}
	if got.UpdatedAt == 0 {
		t.Fatalf("updated_at zero")
	}
}

func TestBotSession_GetMissingReturnsNilNoError(t *testing.T) {
	d := openBotSessionsTestDB(t)
	got, err := d.GetBotSession("telegram", "ghost")
	if err != nil {
		t.Fatalf("Get error on missing: %v", err)
	}
	if got != nil {
		t.Fatalf("want nil, got %+v", got)
	}
}

func TestBotSession_SetMode_PreservesConversationID(t *testing.T) {
	d := openBotSessionsTestDB(t)
	conv, _ := d.CreateConversation("c")
	_ = d.UpsertBotSession("telegram", "u1", conv.ID, "")
	if err := d.SetBotMode("telegram", "u1", "multi"); err != nil {
		t.Fatalf("SetBotMode: %v", err)
	}
	got, _ := d.GetBotSession("telegram", "u1")
	if got.CurrentMode != "multi" || got.ConversationID != conv.ID {
		t.Fatalf("expected mode=multi convID preserved, got %+v", got)
	}
}

func TestBotSession_Clear_WipesRow(t *testing.T) {
	d := openBotSessionsTestDB(t)
	conv, _ := d.CreateConversation("c")
	_ = d.UpsertBotSession("telegram", "u1", conv.ID, "multi")
	if err := d.ClearBotSession("telegram", "u1"); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	got, _ := d.GetBotSession("telegram", "u1")
	if got != nil {
		t.Fatalf("expected nil after Clear, got %+v", got)
	}
}

func TestBotSession_FKCascade_OnConversationDelete(t *testing.T) {
	d := openBotSessionsTestDB(t)
	conv, _ := d.CreateConversation("c")
	_ = d.UpsertBotSession("telegram", "u1", conv.ID, "multi")

	if _, err := d.Exec(`DELETE FROM conversations WHERE id = ?`, conv.ID); err != nil {
		t.Fatalf("delete conversation: %v", err)
	}
	got, err := d.GetBotSession("telegram", "u1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatalf("row should survive parent delete; FK is ON DELETE SET NULL")
	}
	if got.ConversationID != "" {
		t.Fatalf("conversation_id should be NULL/empty after parent delete, got %q", got.ConversationID)
	}
	if got.CurrentMode != "multi" {
		t.Fatalf("mode override should survive: %+v", got)
	}
}
