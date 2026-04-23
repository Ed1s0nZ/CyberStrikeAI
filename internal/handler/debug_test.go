package handler

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"
)

// openHandlerTestDB opens a fresh SQLite with the three debug tables
// so each test can hit a real DB without depending on internal/database.
func openHandlerTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "handler_test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	ddl := []string{
		`CREATE TABLE debug_sessions (conversation_id TEXT PRIMARY KEY, started_at INTEGER NOT NULL, ended_at INTEGER, outcome TEXT, label TEXT)`,
		`CREATE TABLE debug_events (id INTEGER PRIMARY KEY AUTOINCREMENT, conversation_id TEXT NOT NULL, message_id TEXT, seq INTEGER NOT NULL, event_type TEXT NOT NULL, agent_id TEXT, payload_json TEXT NOT NULL, started_at INTEGER NOT NULL, finished_at INTEGER)`,
		`CREATE TABLE debug_llm_calls (id INTEGER PRIMARY KEY AUTOINCREMENT, conversation_id TEXT NOT NULL, message_id TEXT, iteration INTEGER, call_index INTEGER, agent_id TEXT, sent_at INTEGER NOT NULL, first_token_at INTEGER, finished_at INTEGER, prompt_tokens INTEGER, completion_tokens INTEGER, request_json TEXT NOT NULL, response_json TEXT NOT NULL, error TEXT)`,
	}
	for _, s := range ddl {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("Exec: %v (%s)", err, s)
		}
	}
	return db
}

func TestListDebugSessions_EmptyReturnsArray(t *testing.T) {
	db := openHandlerTestDB(t)
	h := &DebugHandler{db: db, logger: zap.NewNop()}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/debug/sessions", h.ListSessions)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/debug/sessions", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", w.Code)
	}
	var body []interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body not JSON array: %v (%s)", err, w.Body.String())
	}
	if len(body) != 0 {
		t.Fatalf("want empty array, got %d", len(body))
	}
}

func TestListDebugSessions_WithAggregates(t *testing.T) {
	db := openHandlerTestDB(t)
	h := &DebugHandler{db: db, logger: zap.NewNop()}
	// Use realistic nanosecond timestamps: 3 second duration
	_, _ = db.Exec(`INSERT INTO debug_sessions (conversation_id, started_at, ended_at, outcome, label) VALUES ('c1', 1000000000, 4000000000, 'completed', 'nmap scan')`)
	_, _ = db.Exec(`INSERT INTO debug_llm_calls (conversation_id, iteration, sent_at, prompt_tokens, completion_tokens, request_json, response_json) VALUES ('c1', 1, 1100000000, 100, 20, '{}', '{}')`)
	_, _ = db.Exec(`INSERT INTO debug_llm_calls (conversation_id, iteration, sent_at, prompt_tokens, completion_tokens, request_json, response_json) VALUES ('c1', 2, 2200000000, 150, 30, '{}', '{}')`)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/debug/sessions", h.ListSessions)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/debug/sessions", nil)
	r.ServeHTTP(w, req)

	var body []map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body not JSON: %v (%s)", err, w.Body.String())
	}
	if len(body) != 1 {
		t.Fatalf("want 1 row, got %d", len(body))
	}
	row := body[0]
	if row["conversationId"] != "c1" {
		t.Fatalf("conversationId: got %v", row["conversationId"])
	}
	if row["label"] != "nmap scan" {
		t.Fatalf("label: got %v", row["label"])
	}
	if int(row["iterations"].(float64)) != 2 {
		t.Fatalf("iterations: want 2, got %v", row["iterations"])
	}
	if int(row["promptTokens"].(float64)) != 250 {
		t.Fatalf("promptTokens: want 250, got %v", row["promptTokens"])
	}
	if int(row["completionTokens"].(float64)) != 50 {
		t.Fatalf("completionTokens: want 50, got %v", row["completionTokens"])
	}
	if int(row["durationMs"].(float64)) != 3000 {
		t.Fatalf("durationMs: want 3000, got %v", row["durationMs"])
	}
}

func TestListDebugSessions_OrderedByStartedAtDesc(t *testing.T) {
	db := openHandlerTestDB(t)
	h := &DebugHandler{db: db, logger: zap.NewNop()}
	_, _ = db.Exec(`INSERT INTO debug_sessions (conversation_id, started_at) VALUES ('older', 1000000000)`)
	_, _ = db.Exec(`INSERT INTO debug_sessions (conversation_id, started_at) VALUES ('newer', 2000000000)`)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/debug/sessions", h.ListSessions)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/debug/sessions", nil)
	r.ServeHTTP(w, req)

	var body []map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if len(body) != 2 || body[0]["conversationId"] != "newer" {
		t.Fatalf("want newer first, got %v", body)
	}
}

func TestGetDebugSession_UnknownIs404(t *testing.T) {
	db := openHandlerTestDB(t)
	h := &DebugHandler{db: db, logger: zap.NewNop()}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/debug/sessions/:id", h.GetSession)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/debug/sessions/does-not-exist", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status: want 404, got %d", w.Code)
	}
}

func TestGetDebugSession_ReturnsFullCapture(t *testing.T) {
	db := openHandlerTestDB(t)
	_, _ = db.Exec(`INSERT INTO debug_sessions (conversation_id, started_at, ended_at, outcome, label) VALUES ('c1', 1000, 2000, 'completed', '')`)
	_, _ = db.Exec(`INSERT INTO debug_events (conversation_id, seq, event_type, payload_json, started_at) VALUES ('c1', 0, 'iteration', '{"iteration":1}', 1100)`)
	_, _ = db.Exec(`INSERT INTO debug_llm_calls (conversation_id, iteration, sent_at, request_json, response_json) VALUES ('c1', 1, 1200, '{"messages":[]}', '{"choices":[]}')`)

	h := &DebugHandler{db: db, logger: zap.NewNop()}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/debug/sessions/:id", h.GetSession)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/debug/sessions/c1", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d: %s", w.Code, w.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body: %v", err)
	}
	if _, ok := body["session"]; !ok {
		t.Fatalf("missing session field")
	}
	llmCalls, _ := body["llmCalls"].([]interface{})
	if len(llmCalls) != 1 {
		t.Fatalf("want 1 llmCall, got %v", llmCalls)
	}
	events, _ := body["events"].([]interface{})
	if len(events) != 1 {
		t.Fatalf("want 1 event, got %v", events)
	}
}

func TestDeleteDebugSession_PurgesAllTables(t *testing.T) {
	db := openHandlerTestDB(t)
	_, _ = db.Exec(`INSERT INTO debug_sessions (conversation_id, started_at) VALUES ('c1', 1)`)
	_, _ = db.Exec(`INSERT INTO debug_events (conversation_id, seq, event_type, payload_json, started_at) VALUES ('c1', 0, 'a', '{}', 1)`)
	_, _ = db.Exec(`INSERT INTO debug_llm_calls (conversation_id, sent_at, request_json, response_json) VALUES ('c1', 1, '{}', '{}')`)

	h := &DebugHandler{db: db, logger: zap.NewNop()}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.DELETE("/api/debug/sessions/:id", h.DeleteSession)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/api/debug/sessions/c1", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("status: want 204, got %d (%s)", w.Code, w.Body.String())
	}

	for _, tbl := range []string{"debug_sessions", "debug_events", "debug_llm_calls"} {
		var n int
		_ = db.QueryRow(`SELECT COUNT(*) FROM `+tbl+` WHERE conversation_id = 'c1'`).Scan(&n)
		if n != 0 {
			t.Fatalf("%s still has rows after delete: %d", tbl, n)
		}
	}
}

func TestDeleteDebugSession_UnknownIs404(t *testing.T) {
	db := openHandlerTestDB(t)
	h := &DebugHandler{db: db, logger: zap.NewNop()}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.DELETE("/api/debug/sessions/:id", h.DeleteSession)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/api/debug/sessions/ghost", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status: want 404, got %d", w.Code)
	}
}

func TestPatchDebugSession_SetsLabel(t *testing.T) {
	db := openHandlerTestDB(t)
	_, _ = db.Exec(`INSERT INTO debug_sessions (conversation_id, started_at) VALUES ('c1', 1)`)
	h := &DebugHandler{db: db, logger: zap.NewNop()}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.PATCH("/api/debug/sessions/:id", h.PatchSession)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("PATCH", "/api/debug/sessions/c1", strings.NewReader(`{"label":"nmap run 2"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", w.Code)
	}
	var label string
	_ = db.QueryRow(`SELECT label FROM debug_sessions WHERE conversation_id='c1'`).Scan(&label)
	if label != "nmap run 2" {
		t.Fatalf("label not persisted: got %q", label)
	}
}

func TestExportConversation_RawFormat(t *testing.T) {
	db := openHandlerTestDB(t)
	_, _ = db.Exec(`INSERT INTO debug_sessions (conversation_id, started_at, ended_at, outcome) VALUES ('c1', 1, 2, 'completed')`)
	_, _ = db.Exec(`INSERT INTO debug_llm_calls (conversation_id, sent_at, agent_id, request_json, response_json) VALUES ('c1', 1, 'cyberstrike-orchestrator', '{"messages":[]}', '{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"ok"}}]}')`)

	h := &DebugHandler{db: db, logger: zap.NewNop()}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/conversations/:id/export", h.ExportConversation)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/conversations/c1/export?format=raw", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/jsonl" {
		t.Fatalf("content-type: want application/jsonl, got %q", ct)
	}
	if !strings.Contains(w.Body.String(), `"source":"llm_call"`) {
		t.Fatalf("missing llm_call source tag: %s", w.Body.String())
	}
}

func TestExportConversation_ShareGPT(t *testing.T) {
	db := openHandlerTestDB(t)
	_, _ = db.Exec(`INSERT INTO debug_llm_calls (conversation_id, sent_at, agent_id, request_json, response_json) VALUES ('c1', 1, 'cyberstrike-orchestrator', '{"messages":[{"role":"user","content":"hi"}]}', '{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"hello"}}]}')`)

	h := &DebugHandler{db: db, logger: zap.NewNop()}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/conversations/:id/export", h.ExportConversation)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/conversations/c1/export?format=sharegpt", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d body: %s", w.Code, w.Body.String())
	}
	if !strings.HasSuffix(w.Body.String(), "\n") {
		t.Fatalf("ShareGPT body must end with newline")
	}
}

func TestExportConversation_InvalidFormat(t *testing.T) {
	db := openHandlerTestDB(t)
	h := &DebugHandler{db: db, logger: zap.NewNop()}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/conversations/:id/export", h.ExportConversation)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/conversations/c1/export?format=bogus", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: want 400, got %d", w.Code)
	}
}

func TestExportBulk_TarGzValid(t *testing.T) {
	db := openHandlerTestDB(t)
	for _, id := range []string{"a", "b"} {
		_, _ = db.Exec(`INSERT INTO debug_sessions (conversation_id, started_at, ended_at, outcome) VALUES (?, 1, 2, 'completed')`, id)
		_, _ = db.Exec(`INSERT INTO debug_llm_calls (conversation_id, sent_at, agent_id, request_json, response_json) VALUES (?, 1, 'cyberstrike-orchestrator', '{"messages":[{"role":"user","content":"hi"}]}', '{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"ok"}}]}')`, id)
	}
	h := &DebugHandler{db: db, logger: zap.NewNop()}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/debug/export-bulk", h.ExportBulk)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/debug/export-bulk?format=sharegpt", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/gzip" {
		t.Fatalf("content-type: got %q", ct)
	}
	gzr, err := gzip.NewReader(bytes.NewReader(w.Body.Bytes()))
	if err != nil {
		t.Fatalf("gzip: %v", err)
	}
	tr := tar.NewReader(gzr)
	names := map[string]bool{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar: %v", err)
		}
		names[hdr.Name] = true
	}
	if !names["a.jsonl"] || !names["b.jsonl"] {
		t.Fatalf("missing entries: %v", names)
	}
}

func TestExportBulk_InvalidFormatReturns400(t *testing.T) {
	db := openHandlerTestDB(t)
	h := &DebugHandler{db: db, logger: zap.NewNop()}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/debug/export-bulk", h.ExportBulk)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/debug/export-bulk?format=bogus", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: want 400, got %d", w.Code)
	}
}
