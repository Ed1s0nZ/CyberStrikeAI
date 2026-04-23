package debug

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

func TestWriteRawJSONL_MergesCallsAndEventsByTimestamp(t *testing.T) {
	db := openTestDB(t)
	mustExec := func(q string, args ...interface{}) {
		if _, err := db.Exec(q, args...); err != nil {
			t.Fatalf("exec: %v", err)
		}
	}
	// One session, one event at t=100, one LLM call at t=50, one event at t=150
	mustExec(`INSERT INTO debug_sessions (conversation_id, started_at, ended_at, outcome) VALUES ('c1', 10, 200, 'completed')`)
	mustExec(`INSERT INTO debug_events (conversation_id, seq, event_type, payload_json, started_at) VALUES ('c1', 0, 'iteration', '{"iteration":1}', 100)`)
	mustExec(`INSERT INTO debug_llm_calls (conversation_id, sent_at, agent_id, request_json, response_json) VALUES ('c1', 50, 'cyberstrike-orchestrator', '{}', '{}')`)
	mustExec(`INSERT INTO debug_events (conversation_id, seq, event_type, payload_json, started_at) VALUES ('c1', 1, 'tool_call', '{"tool":"nmap"}', 150)`)

	var buf bytes.Buffer
	if err := WriteRawJSONL(&buf, db, "c1"); err != nil {
		t.Fatalf("WriteRawJSONL: %v", err)
	}
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("want 3 lines, got %d: %s", len(lines), buf.String())
	}
	// Order: llm_call (50), event (100), event (150)
	wantSources := []string{"llm_call", "event", "event"}
	for i, line := range lines {
		var row struct {
			Source string `json:"source"`
		}
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			t.Fatalf("line %d not JSON: %v (%s)", i, err, line)
		}
		if row.Source != wantSources[i] {
			t.Fatalf("line %d source: want %q, got %q", i, wantSources[i], row.Source)
		}
	}
}

func TestWriteShareGPTJSONL_EmitsOneLineWithTrailingNewline(t *testing.T) {
	db := openTestDB(t)
	_, err := db.Exec(`INSERT INTO debug_llm_calls (conversation_id, sent_at, agent_id, request_json, response_json) VALUES ('c1', 1, 'cyberstrike-orchestrator', '{"messages":[{"role":"user","content":"hi"}]}', '{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"hello"}}]}')`)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	var buf bytes.Buffer
	if err := WriteShareGPTJSONL(&buf, db, "c1"); err != nil {
		t.Fatalf("WriteShareGPTJSONL: %v", err)
	}
	if !strings.HasSuffix(buf.String(), "\n") {
		t.Fatalf("ShareGPT output must end with newline (JSONL convention)")
	}
	trimmed := strings.TrimRight(buf.String(), "\n")
	if strings.Contains(trimmed, "\n") {
		t.Fatalf("ShareGPT should be exactly one line, got:\n%s", buf.String())
	}
}

func TestWriteBulkArchive_TarGzContainsOneEntryPerSession(t *testing.T) {
	db := openTestDB(t)
	for _, id := range []string{"a", "b"} {
		_, _ = db.Exec(`INSERT INTO debug_sessions (conversation_id, started_at, ended_at, outcome) VALUES (?, 1, 2, 'completed')`, id)
		_, _ = db.Exec(`INSERT INTO debug_llm_calls (conversation_id, sent_at, agent_id, request_json, response_json) VALUES (?, 1, 'cyberstrike-orchestrator', '{"messages":[{"role":"user","content":"hi"}]}', '{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"hi"}}]}')`, id)
	}

	var buf bytes.Buffer
	if err := WriteBulkArchive(&buf, db, "sharegpt", 0, 0); err != nil {
		t.Fatalf("WriteBulkArchive: %v", err)
	}
	gzr, err := gzip.NewReader(&buf)
	if err != nil {
		t.Fatalf("gzip: %v", err)
	}
	tr := tar.NewReader(gzr)
	seen := map[string]bool{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar.Next: %v", err)
		}
		seen[hdr.Name] = true
	}
	for _, id := range []string{"a.jsonl", "b.jsonl"} {
		if !seen[id] {
			t.Fatalf("missing %q in archive; saw %v", id, seen)
		}
	}
}

func TestWriteBulkArchive_InvalidFormatReturnsError(t *testing.T) {
	db := openTestDB(t)
	var buf bytes.Buffer
	if err := WriteBulkArchive(&buf, db, "bogus", 0, 0); err == nil {
		t.Fatalf("want error for invalid format, got nil")
	}
}
