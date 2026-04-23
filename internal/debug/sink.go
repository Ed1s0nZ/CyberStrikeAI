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

// NewSink returns a Sink bound to db. When db is non-nil, the returned
// *dbSink honors SetEnabled for runtime toggling — boot-time enabled
// is the initial value of the atomic.Bool, not a choice of
// implementation. When db is nil (test harnesses, NewAgent's
// nil-fallback), a noopSink is returned because there's no
// destination to write to; a noopSink's SetEnabled is intentionally
// inert so it can't try to write to a nil *sql.DB.
func NewSink(enabled bool, db *sql.DB, log *zap.Logger) Sink {
	if log == nil {
		log = zap.NewNop()
	}
	if db == nil {
		return noopSink{}
	}
	s := &dbSink{db: db, log: log}
	s.enabled.Store(enabled)
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
	// conversation; entries are reaped by EndSession to bound memory
	// growth over the process lifetime.
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

func (s *dbSink) RecordEvent(conversationID, messageID string, e Event) {
	if !s.enabled.Load() {
		return
	}
	seq := s.nextSeq(conversationID)
	var finAt interface{}
	if e.FinishedAt != 0 {
		finAt = e.FinishedAt
	}
	_, err := s.db.Exec(`
		INSERT INTO debug_events
		  (conversation_id, message_id, seq, event_type, agent_id,
		   payload_json, started_at, finished_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		conversationID, nullableStr(messageID), seq, e.EventType,
		nullableStr(e.AgentID), e.PayloadJSON, e.StartedAt, finAt,
	)
	if err != nil {
		s.log.Warn("debug: RecordEvent insert failed",
			zap.String("conversation_id", conversationID),
			zap.Int64("seq", seq),
			zap.Error(err))
	}
}

// nextSeq returns the next 0-based monotonic sequence for a
// conversation. Backed by sync.Map<string, *atomic.Int64> — lazy
// populated on first call per conversation. LoadOrStore races are
// harmless: the loser's freshly-allocated *atomic.Int64 is discarded
// before any Add, so the winner's counter is authoritative.
func (s *dbSink) nextSeq(conversationID string) int64 {
	v, _ := s.seqByConv.LoadOrStore(conversationID, new(atomic.Int64))
	return v.(*atomic.Int64).Add(1) - 1
}
