package debug

import (
	"database/sql"
	"path/filepath"
	"sync"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"
)

func TestNoopSink_Disabled(t *testing.T) {
	s := NewSink(false, nil, zap.NewNop())
	if s.Enabled() {
		t.Fatalf("NewSink(false) should return a disabled sink")
	}
	// All methods must be safe to call with nil db.
	s.StartSession("conv-a")
	s.EndSession("conv-a", "completed")
	s.RecordLLMCall("conv-a", "msg-1", LLMCall{})
	s.RecordEvent("conv-a", "msg-1", Event{})
	s.SetEnabled(true)
	if s.Enabled() {
		t.Fatalf("noopSink.SetEnabled(true) must stay disabled — only dbSink has runtime toggle")
	}
}

// openTestDB opens an in-memory SQLite and runs the debug-table DDL
// so tests don't depend on the database package.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "debug_test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	ddl := []string{
		`CREATE TABLE debug_sessions (conversation_id TEXT PRIMARY KEY, started_at INTEGER NOT NULL, ended_at INTEGER, outcome TEXT, label TEXT)`,
		`CREATE TABLE debug_llm_calls (id INTEGER PRIMARY KEY AUTOINCREMENT, conversation_id TEXT NOT NULL, message_id TEXT, iteration INTEGER, call_index INTEGER, agent_id TEXT, sent_at INTEGER NOT NULL, first_token_at INTEGER, finished_at INTEGER, prompt_tokens INTEGER, completion_tokens INTEGER, request_json TEXT NOT NULL, response_json TEXT NOT NULL, error TEXT)`,
		`CREATE TABLE debug_events (id INTEGER PRIMARY KEY AUTOINCREMENT, conversation_id TEXT NOT NULL, message_id TEXT, seq INTEGER NOT NULL, event_type TEXT NOT NULL, agent_id TEXT, payload_json TEXT NOT NULL, started_at INTEGER NOT NULL, finished_at INTEGER)`,
	}
	for _, s := range ddl {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("Exec: %v (%s)", err, s)
		}
	}
	return db
}

func TestDBSink_StartEndSession_HappyPath(t *testing.T) {
	db := openTestDB(t)
	s := NewSink(true, db, nil)

	s.StartSession("conv-1")
	time.Sleep(2 * time.Millisecond) // ensure ended_at > started_at
	s.EndSession("conv-1", "completed")

	var startedAt, endedAt sql.NullInt64
	var outcome sql.NullString
	err := db.QueryRow("SELECT started_at, ended_at, outcome FROM debug_sessions WHERE conversation_id = ?", "conv-1").
		Scan(&startedAt, &endedAt, &outcome)
	if err != nil {
		t.Fatalf("QueryRow: %v", err)
	}
	if !startedAt.Valid || startedAt.Int64 == 0 {
		t.Fatalf("started_at not populated")
	}
	if !endedAt.Valid || endedAt.Int64 <= startedAt.Int64 {
		t.Fatalf("ended_at not after started_at: start=%d end=%v", startedAt.Int64, endedAt)
	}
	if outcome.String != "completed" {
		t.Fatalf("outcome: want completed, got %q", outcome.String)
	}
}

func TestDBSink_RecordLLMCall_PersistsRow(t *testing.T) {
	db := openTestDB(t)
	s := NewSink(true, db, nil)

	call := LLMCall{
		Iteration:        2,
		CallIndex:        5,
		AgentID:          "cyberstrike-orchestrator",
		SentAt:           1000,
		FirstTokenAt:     1100,
		FinishedAt:       1500,
		PromptTokens:     42,
		CompletionTokens: 13,
		RequestJSON:      `{"messages":[{"role":"user","content":"hi"}]}`,
		ResponseJSON:     `{"choices":[{"message":{"role":"assistant","content":"hello"}}]}`,
	}
	s.RecordLLMCall("conv-1", "msg-1", call)

	var iter, callIdx, sentAt, firstTok, finAt, promptT, complT int64
	var agent, req, resp string
	var errStr sql.NullString
	err := db.QueryRow(`
		SELECT iteration, call_index, agent_id, sent_at, first_token_at,
		       finished_at, prompt_tokens, completion_tokens,
		       request_json, response_json, error
		FROM debug_llm_calls WHERE conversation_id = ? AND message_id = ?`,
		"conv-1", "msg-1").Scan(
		&iter, &callIdx, &agent, &sentAt, &firstTok,
		&finAt, &promptT, &complT, &req, &resp, &errStr,
	)
	if err != nil {
		t.Fatalf("QueryRow: %v", err)
	}
	if iter != 2 || callIdx != 5 || agent != "cyberstrike-orchestrator" {
		t.Fatalf("metadata mismatch: iter=%d callIdx=%d agent=%q", iter, callIdx, agent)
	}
	if sentAt != 1000 || firstTok != 1100 || finAt != 1500 {
		t.Fatalf("timestamps mismatch: sent=%d first=%d fin=%d", sentAt, firstTok, finAt)
	}
	if promptT != 42 || complT != 13 {
		t.Fatalf("token counts mismatch: prompt=%d completion=%d", promptT, complT)
	}
	if req != call.RequestJSON || resp != call.ResponseJSON {
		t.Fatalf("JSON payload mismatch")
	}
	if errStr.Valid {
		t.Fatalf("error column should be NULL when LLMCall.Error is empty, got %q", errStr.String)
	}
}

func TestDBSink_RecordLLMCall_NullableTokenColumns(t *testing.T) {
	db := openTestDB(t)
	s := NewSink(true, db, nil)
	// Zero-valued token fields must be stored as SQL NULL so that
	// read-side aggregates like AVG(completion_tokens) skip them
	// for backends that don't report usage.
	s.RecordLLMCall("conv-1", "msg-1", LLMCall{
		Iteration:    1,
		SentAt:       1,
		RequestJSON:  "{}",
		ResponseJSON: "{}",
		// PromptTokens, CompletionTokens, FirstTokenAt, FinishedAt all zero.
	})
	var firstTok, finAt, promptT, complT sql.NullInt64
	err := db.QueryRow(`SELECT first_token_at, finished_at, prompt_tokens, completion_tokens FROM debug_llm_calls WHERE conversation_id = ?`, "conv-1").Scan(&firstTok, &finAt, &promptT, &complT)
	if err != nil {
		t.Fatalf("QueryRow: %v", err)
	}
	if firstTok.Valid || finAt.Valid || promptT.Valid || complT.Valid {
		t.Fatalf("optional columns should be NULL: firstTok=%v finAt=%v promptT=%v complT=%v", firstTok, finAt, promptT, complT)
	}
}

func TestDBSink_RecordLLMCall_NoWriteWhenDisabled(t *testing.T) {
	db := openTestDB(t)
	s := NewSink(true, db, nil)
	s.SetEnabled(false)

	s.RecordLLMCall("conv-1", "msg-1", LLMCall{SentAt: 1, RequestJSON: "{}", ResponseJSON: "{}"})

	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM debug_llm_calls").Scan(&n); err != nil {
		t.Fatalf("QueryRow: %v", err)
	}
	if n != 0 {
		t.Fatalf("want 0 rows when sink disabled, got %d", n)
	}
}

func TestDBSink_RecordEvent_BasicRow(t *testing.T) {
	db := openTestDB(t)
	s := NewSink(true, db, nil)

	s.RecordEvent("conv-1", "msg-1", Event{
		EventType:   "iteration",
		AgentID:     "cyberstrike-orchestrator",
		PayloadJSON: `{"iteration":1}`,
		StartedAt:   1000,
	})

	var seq int64
	var evType, agent, payload string
	var startedAt int64
	err := db.QueryRow(`SELECT seq, event_type, agent_id, payload_json, started_at
		FROM debug_events WHERE conversation_id = ?`, "conv-1").
		Scan(&seq, &evType, &agent, &payload, &startedAt)
	if err != nil {
		t.Fatalf("QueryRow: %v", err)
	}
	if seq != 0 {
		t.Fatalf("first event seq: want 0, got %d", seq)
	}
	if evType != "iteration" || agent != "cyberstrike-orchestrator" {
		t.Fatalf("metadata mismatch: evType=%q agent=%q", evType, agent)
	}
	if payload != `{"iteration":1}` {
		t.Fatalf("payload mismatch: %q", payload)
	}
	if startedAt != 1000 {
		t.Fatalf("startedAt mismatch: %d", startedAt)
	}
}

func TestDBSink_RecordEvent_MonotonicSeq_Concurrent(t *testing.T) {
	db := openTestDB(t)
	s := NewSink(true, db, nil)

	const N = 50
	var wg sync.WaitGroup
	wg.Add(N)
	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()
			s.RecordEvent("conv-1", "", Event{EventType: "tool_call", PayloadJSON: "{}", StartedAt: 1})
		}()
	}
	wg.Wait()

	rows, err := db.Query(`SELECT seq FROM debug_events WHERE conversation_id = ? ORDER BY seq`, "conv-1")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	defer rows.Close()
	seen := make([]int64, 0, N)
	for rows.Next() {
		var v int64
		if err := rows.Scan(&v); err != nil {
			t.Fatalf("Scan: %v", err)
		}
		seen = append(seen, v)
	}
	if len(seen) != N {
		t.Fatalf("want %d rows, got %d", N, len(seen))
	}
	for i, v := range seen {
		if v != int64(i) {
			t.Fatalf("seq[%d]: want %d, got %d (dups or gaps)", i, i, v)
		}
	}
}

func TestDBSink_RecordEvent_SeparateConversationsIndependentSeq(t *testing.T) {
	db := openTestDB(t)
	s := NewSink(true, db, nil)
	for _, conv := range []string{"conv-a", "conv-b"} {
		for i := 0; i < 3; i++ {
			s.RecordEvent(conv, "", Event{EventType: "iteration", PayloadJSON: "{}", StartedAt: 1})
		}
	}
	for _, conv := range []string{"conv-a", "conv-b"} {
		rows, err := db.Query(`SELECT seq FROM debug_events WHERE conversation_id = ? ORDER BY seq`, conv)
		if err != nil {
			t.Fatalf("Query %s: %v", conv, err)
		}
		var seqs []int64
		for rows.Next() {
			var v int64
			_ = rows.Scan(&v)
			seqs = append(seqs, v)
		}
		rows.Close()
		if len(seqs) != 3 || seqs[0] != 0 || seqs[1] != 1 || seqs[2] != 2 {
			t.Fatalf("%s: want seq {0,1,2}, got %v", conv, seqs)
		}
	}
}
