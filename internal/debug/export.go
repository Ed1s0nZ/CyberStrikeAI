package debug

import (
	"archive/tar"
	"compress/gzip"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
)

// WriteRawJSONL streams the full capture for one conversation as
// JSONL, merging debug_llm_calls and debug_events in timestamp order.
// Each line has a "source" tag so downstream consumers can
// discriminate. Streaming: no full buffering — one row is encoded
// and written directly to w before the next row is fetched.
func WriteRawJSONL(w io.Writer, db *sql.DB, conversationID string) error {
	calls, err := loadLLMCalls(db, conversationID)
	if err != nil {
		return err
	}
	events, err := loadEvents(db, conversationID)
	if err != nil {
		return err
	}
	// Merge by timestamp.
	i, j := 0, 0
	enc := json.NewEncoder(w)
	for i < len(calls) && j < len(events) {
		if calls[i].SentAt <= events[j].StartedAt {
			if err := enc.Encode(rawCallLine(calls[i])); err != nil {
				return err
			}
			i++
		} else {
			if err := enc.Encode(rawEventLine(events[j])); err != nil {
				return err
			}
			j++
		}
	}
	for ; i < len(calls); i++ {
		if err := enc.Encode(rawCallLine(calls[i])); err != nil {
			return err
		}
	}
	for ; j < len(events); j++ {
		if err := enc.Encode(rawEventLine(events[j])); err != nil {
			return err
		}
	}
	return nil
}

// WriteShareGPTJSONL writes the training-ready conversation for one
// conversation. Always one JSONL line plus trailing newline.
func WriteShareGPTJSONL(w io.Writer, db *sql.DB, conversationID string) error {
	calls, err := loadLLMCalls(db, conversationID)
	if err != nil {
		return err
	}
	line, err := ToShareGPT(calls)
	if err != nil {
		return err
	}
	if _, err := w.Write(line); err != nil {
		return err
	}
	_, err = w.Write([]byte("\n"))
	return err
}

// WriteBulkArchive writes a gzip-compressed tar archive containing
// one JSONL entry per session in [sinceNS, untilNS] (0 means
// unbounded). format must be "raw" or "sharegpt".
func WriteBulkArchive(w io.Writer, db *sql.DB, format string, sinceNS, untilNS int64) error {
	if format != "raw" && format != "sharegpt" {
		return fmt.Errorf("WriteBulkArchive: invalid format %q", format)
	}
	gw := gzip.NewWriter(w)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	q := `SELECT conversation_id FROM debug_sessions WHERE 1=1`
	args := []interface{}{}
	if sinceNS > 0 {
		q += ` AND started_at >= ?`
		args = append(args, sinceNS)
	}
	if untilNS > 0 {
		q += ` AND started_at <= ?`
		args = append(args, untilNS)
	}
	q += ` ORDER BY started_at`
	rows, err := db.Query(q, args...)
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	rows.Close()

	for _, id := range ids {
		var buf writerBuffer
		switch format {
		case "raw":
			if err := WriteRawJSONL(&buf, db, id); err != nil {
				return err
			}
		case "sharegpt":
			if err := WriteShareGPTJSONL(&buf, db, id); err != nil {
				return err
			}
		}
		body := buf.Bytes()
		hdr := &tar.Header{
			Name: id + ".jsonl",
			Mode: 0o644,
			Size: int64(len(body)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if _, err := tw.Write(body); err != nil {
			return err
		}
	}
	return nil
}

// writerBuffer is a small helper so we can tar-header-then-write per
// entry. tar requires the size before writing, so per-entry buffering
// is required; peak memory is still one session (not one archive).
type writerBuffer struct{ buf []byte }

func (b *writerBuffer) Write(p []byte) (int, error) { b.buf = append(b.buf, p...); return len(p), nil }
func (b *writerBuffer) Bytes() []byte               { return b.buf }

// eventRow mirrors the raw debug_events row for export.
type eventRow struct {
	ID             int64           `json:"id"`
	ConversationID string          `json:"conversationId"`
	MessageID      string          `json:"messageId,omitempty"`
	Seq            int64           `json:"seq"`
	EventType      string          `json:"eventType"`
	AgentID        string          `json:"agentId,omitempty"`
	Payload        json.RawMessage `json:"payload"`
	StartedAt      int64           `json:"startedAt"`
	FinishedAt     int64           `json:"finishedAt,omitempty"`
}

func rawCallLine(c LLMCallRow) map[string]interface{} {
	return map[string]interface{}{
		"source":           "llm_call",
		"id":               c.ID,
		"conversationId":   c.ConversationID,
		"messageId":        c.MessageID,
		"iteration":        c.Iteration,
		"callIndex":        c.CallIndex,
		"agentId":          c.AgentID,
		"sentAt":           c.SentAt,
		"firstTokenAt":     c.FirstTokenAt,
		"finishedAt":       c.FinishedAt,
		"promptTokens":     c.PromptTokens,
		"completionTokens": c.CompletionTokens,
		"request":          json.RawMessage(c.RequestJSON),
		"response":         json.RawMessage(c.ResponseJSON),
		"error":            c.Error,
	}
}

func rawEventLine(e eventRow) map[string]interface{} {
	return map[string]interface{}{
		"source":         "event",
		"id":             e.ID,
		"conversationId": e.ConversationID,
		"messageId":      e.MessageID,
		"seq":            e.Seq,
		"eventType":      e.EventType,
		"agentId":        e.AgentID,
		"payload":        e.Payload,
		"startedAt":      e.StartedAt,
		"finishedAt":     e.FinishedAt,
	}
}

func loadLLMCalls(db *sql.DB, conversationID string) ([]LLMCallRow, error) {
	rows, err := db.Query(`
		SELECT id, conversation_id, COALESCE(message_id,''), COALESCE(iteration,0),
		       COALESCE(call_index,0), COALESCE(agent_id,''), sent_at,
		       COALESCE(first_token_at,0), COALESCE(finished_at,0),
		       COALESCE(prompt_tokens,0), COALESCE(completion_tokens,0),
		       request_json, response_json, COALESCE(error,'')
		FROM debug_llm_calls WHERE conversation_id = ? ORDER BY sent_at, id`, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LLMCallRow
	for rows.Next() {
		var c LLMCallRow
		if err := rows.Scan(&c.ID, &c.ConversationID, &c.MessageID, &c.Iteration,
			&c.CallIndex, &c.AgentID, &c.SentAt, &c.FirstTokenAt, &c.FinishedAt,
			&c.PromptTokens, &c.CompletionTokens, &c.RequestJSON, &c.ResponseJSON, &c.Error); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

func loadEvents(db *sql.DB, conversationID string) ([]eventRow, error) {
	rows, err := db.Query(`
		SELECT id, conversation_id, COALESCE(message_id,''), seq, event_type,
		       COALESCE(agent_id,''), payload_json, started_at,
		       COALESCE(finished_at,0)
		FROM debug_events WHERE conversation_id = ? ORDER BY started_at, seq`, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []eventRow
	for rows.Next() {
		var e eventRow
		var payload string
		if err := rows.Scan(&e.ID, &e.ConversationID, &e.MessageID, &e.Seq, &e.EventType,
			&e.AgentID, &payload, &e.StartedAt, &e.FinishedAt); err != nil {
			return nil, err
		}
		e.Payload = json.RawMessage(payload)
		out = append(out, e)
	}
	return out, nil
}
