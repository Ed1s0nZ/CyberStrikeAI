package database

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestWebshellConnectionsSchemaNewDatabase(t *testing.T) {
	db := openTestDB(t)

	conn := &WebShellConnection{
		ID:        "ws_new",
		URL:       "http://example.test/shell.php",
		Password:  "pass",
		Type:      "php",
		Method:    "post",
		CmdParam:  "x",
		Protocol:  "behinder",
		UserAgent: "User-Agent: TestBrowser\r\nX-Test: 1",
		CreatedAt: time.Now(),
	}
	if err := db.CreateWebshellConnection(conn); err != nil {
		t.Fatalf("CreateWebshellConnection failed: %v", err)
	}

	got, err := db.GetWebshellConnection(conn.ID)
	if err != nil {
		t.Fatalf("GetWebshellConnection failed: %v", err)
	}
	if got == nil {
		t.Fatal("expected connection, got nil")
	}
	assertWebshellProtocolFields(t, got, "x", "behinder", "User-Agent: TestBrowser\r\nX-Test: 1")

	list, err := db.ListWebshellConnections()
	if err != nil {
		t.Fatalf("ListWebshellConnections failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 connection, got %d", len(list))
	}
	assertWebshellProtocolFields(t, &list[0], "x", "behinder", "User-Agent: TestBrowser\r\nX-Test: 1")
}

func TestWebshellConnectionsSchemaMigratesOldDatabase(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "old.db")
	raw, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open old db failed: %v", err)
	}
	_, err = raw.Exec(`
		CREATE TABLE webshell_connections (
			id TEXT PRIMARY KEY,
			url TEXT NOT NULL,
			password TEXT NOT NULL DEFAULT '',
			type TEXT NOT NULL DEFAULT 'php',
			method TEXT NOT NULL DEFAULT 'post',
			cmd_param TEXT NOT NULL DEFAULT 'cmd',
			remark TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		INSERT INTO webshell_connections (id, url, password, type, method, cmd_param, remark, created_at)
		VALUES ('ws_old', 'http://example.test/old.php', 'p', 'php', 'post', 'x', 'old', CURRENT_TIMESTAMP);
	`)
	if err != nil {
		_ = raw.Close()
		t.Fatalf("seed old db failed: %v", err)
	}
	if err := raw.Close(); err != nil {
		t.Fatalf("close old db failed: %v", err)
	}

	db, err := NewDB(dbPath, zap.NewNop())
	if err != nil {
		t.Fatalf("NewDB migrated old db failed: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	got, err := db.GetWebshellConnection("ws_old")
	if err != nil {
		t.Fatalf("GetWebshellConnection after migration failed: %v", err)
	}
	if got == nil {
		t.Fatal("expected migrated connection, got nil")
	}
	assertWebshellProtocolFields(t, got, "x", "classic", "")

	got.Protocol = "behinder"
	got.UserAgent = "User-Agent: TestBrowser"
	if err := db.UpdateWebshellConnection(got); err != nil {
		t.Fatalf("UpdateWebshellConnection after migration failed: %v", err)
	}
	updated, err := db.GetWebshellConnection("ws_old")
	if err != nil {
		t.Fatalf("GetWebshellConnection updated failed: %v", err)
	}
	assertWebshellProtocolFields(t, updated, "x", "behinder", "User-Agent: TestBrowser")
}

func openTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := NewDB(filepath.Join(t.TempDir(), "test.db"), zap.NewNop())
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	return db
}

func assertWebshellProtocolFields(t *testing.T, conn *WebShellConnection, cmdParam, protocol, userAgent string) {
	t.Helper()
	if conn.CmdParam != cmdParam {
		t.Fatalf("CmdParam mismatch: got %q want %q", conn.CmdParam, cmdParam)
	}
	if conn.Protocol != protocol {
		t.Fatalf("Protocol mismatch: got %q want %q", conn.Protocol, protocol)
	}
	if conn.UserAgent != userAgent {
		t.Fatalf("UserAgent mismatch: got %q want %q", conn.UserAgent, userAgent)
	}
}
