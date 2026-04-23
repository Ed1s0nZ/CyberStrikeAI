package debug

import (
	"database/sql"
	"sync"
	"sync/atomic"

	"go.uber.org/zap"
)

// Sink is the debug-capture extension point. Every call site in the
// Agent / Orchestrator / Handler invokes a Sink unconditionally;
// noopSink short-circuits when debug is off, dbSink persists when on.
// The enabled flag lives on dbSink as an atomic.Bool and can be
// flipped at runtime by the Settings toggle endpoint.
type Sink interface {
	StartSession(conversationID string)
	EndSession(conversationID, outcome string)
	RecordLLMCall(conversationID, messageID string, c LLMCall)
	RecordEvent(conversationID, messageID string, e Event)
	SetEnabled(bool)
	Enabled() bool
}

// NewSink returns a dbSink when enabled at construction, otherwise a
// noopSink. The SetEnabled runtime toggle only flips writes for an
// already-dbSink; a noopSink stays a noopSink for the process lifetime
// (the handler wires a single Sink at boot).
func NewSink(enabled bool, db *sql.DB, log *zap.Logger) Sink {
	if log == nil {
		log = zap.NewNop()
	}
	if !enabled {
		return noopSink{}
	}
	s := &dbSink{db: db, log: log}
	s.enabled.Store(true)
	return s
}

// noopSink is the off-state sink. Every method returns immediately.
// Do not touch db; callers pass nil when debug is off.
type noopSink struct{}

func (noopSink) StartSession(string)                   {}
func (noopSink) EndSession(string, string)             {}
func (noopSink) RecordLLMCall(string, string, LLMCall) {}
func (noopSink) RecordEvent(string, string, Event)     {}
func (noopSink) SetEnabled(bool)                       {}
func (noopSink) Enabled() bool                         { return false }

// dbSink writes to SQLite. Writes are best-effort: any DB error is
// logged at warn and swallowed, so a debug-subsystem failure never
// takes down a user-facing conversation.
type dbSink struct {
	db      *sql.DB
	log     *zap.Logger
	enabled atomic.Bool

	// seqByConv[conversationID] is a *atomic.Int64 holding the next
	// event seq. Lazily populated on first RecordEvent for the
	// conversation; never deleted (bounded by retention sweep).
	seqByConv sync.Map
}

func (s *dbSink) SetEnabled(v bool) { s.enabled.Store(v) }
func (s *dbSink) Enabled() bool     { return s.enabled.Load() }

// RecordLLMCall and RecordEvent bodies are filled in Tasks 6-7.
func (s *dbSink) RecordLLMCall(conversationID, messageID string, c LLMCall) {
	if !s.enabled.Load() {
		return
	}
	// Zero-valued optional columns (FirstTokenAt, FinishedAt,
	// PromptTokens, CompletionTokens, Error) become SQL NULL instead
	// of 0 / "" so read-side aggregates like AVG(completion_tokens)
	// WHERE ... IS NOT NULL behave correctly for backends that don't
	// report token usage (e.g. the OpenAI streaming path).
	var firstTok, finAt, promptT, complT interface{}
	if c.FirstTokenAt != 0 {
		firstTok = c.FirstTokenAt
	}
	if c.FinishedAt != 0 {
		finAt = c.FinishedAt
	}
	if c.PromptTokens != 0 {
		promptT = c.PromptTokens
	}
	if c.CompletionTokens != 0 {
		complT = c.CompletionTokens
	}
	var errVal interface{}
	if c.Error != "" {
		errVal = c.Error
	}
	_, err := s.db.Exec(`
		INSERT INTO debug_llm_calls
		  (conversation_id, message_id, iteration, call_index, agent_id,
		   sent_at, first_token_at, finished_at, prompt_tokens,
		   completion_tokens, request_json, response_json, error)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		conversationID, nullableStr(messageID), c.Iteration, c.CallIndex, c.AgentID,
		c.SentAt, firstTok, finAt, promptT, complT,
		c.RequestJSON, c.ResponseJSON, errVal,
	)
	if err != nil {
		s.log.Warn("debug: RecordLLMCall insert failed",
			zap.String("conversation_id", conversationID),
			zap.Error(err))
	}
}

// nullableStr returns nil for empty strings so they're stored as SQL NULL
// instead of "" — keeps IS NULL filters correct on the read side.
func nullableStr(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func (s *dbSink) RecordEvent(conversationID, messageID string, e Event) {}
