package database

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"
)

func TestInit_CreatesDebugTables(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("sqlite3", tmp)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	// Create a Database struct with a test logger
	logger := zap.NewNop()
	d := &DB{DB: db, logger: logger}
	if err := d.initTables(); err != nil {
		t.Fatalf("initTables: %v", err)
	}

	for _, table := range []string{"debug_sessions", "debug_llm_calls", "debug_events"} {
		var name string
		err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err != nil {
			t.Fatalf("table %s missing: %v", table, err)
		}
	}
	for _, idx := range []string{"idx_debug_llm_calls_conv", "idx_debug_events_conv"} {
		var name string
		err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='index' AND name=?", idx).Scan(&name)
		if err != nil {
			t.Fatalf("index %s missing: %v", idx, err)
		}
	}
}

func TestCreatesDebugTables_CascadesOnConversationDelete(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "cascade_test.db")
	db, err := sql.Open("sqlite3", "file:"+tmp+"?_foreign_keys=1")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	logger := zap.NewNop()
	d := &DB{DB: db, logger: logger}
	if err := d.initTables(); err != nil {
		t.Fatalf("initTables: %v", err)
	}

	// Seed a conversation + one row in each of the three debug tables.
	if _, err := db.Exec(`INSERT INTO conversations (id, title, created_at, updated_at) VALUES ('conv-x', 't', '2026-04-22T00:00:00Z', '2026-04-22T00:00:00Z')`); err != nil {
		t.Fatalf("seed conversation: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO debug_sessions (conversation_id, started_at) VALUES ('conv-x', 1)`); err != nil {
		t.Fatalf("seed session: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO debug_llm_calls (conversation_id, sent_at, request_json, response_json) VALUES ('conv-x', 1, '{}', '{}')`); err != nil {
		t.Fatalf("seed llm_call: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO debug_events (conversation_id, seq, event_type, payload_json, started_at) VALUES ('conv-x', 0, 'iteration', '{}', 1)`); err != nil {
		t.Fatalf("seed event: %v", err)
	}

	// Deleting the conversation must cascade.
	if _, err := db.Exec(`DELETE FROM conversations WHERE id = 'conv-x'`); err != nil {
		t.Fatalf("delete conversation: %v", err)
	}

	for _, tbl := range []string{"debug_sessions", "debug_llm_calls", "debug_events"} {
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM ` + tbl + ` WHERE conversation_id = 'conv-x'`).Scan(&n); err != nil {
			t.Fatalf("count %s: %v", tbl, err)
		}
		if n != 0 {
			t.Fatalf("%s did not cascade: %d rows remain", tbl, n)
		}
	}
}
