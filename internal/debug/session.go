package debug

import (
	"database/sql"
	"time"

	"go.uber.org/zap"
)

func (s *dbSink) StartSession(conversationID string) {
	if !s.enabled.Load() {
		return
	}
	now := time.Now().UnixNano()
	// INSERT OR REPLACE so re-enabling debug on a conversation that
	// already has a row resets the session timer. Preserve any prior
	// user-set label via subselect. Spec: one session per conversation
	// in v1.
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO debug_sessions (conversation_id, started_at, ended_at, outcome, label)
		 VALUES (?, ?, NULL, NULL, (SELECT label FROM debug_sessions WHERE conversation_id = ?))`,
		conversationID, now, conversationID,
	)
	if err != nil {
		s.log.Warn("debug: StartSession insert failed",
			zap.String("conversation_id", conversationID),
			zap.Error(err))
	}
}

func (s *dbSink) EndSession(conversationID, outcome string) {
	if !s.enabled.Load() {
		return
	}
	now := time.Now().UnixNano()
	// Fill ended_at + outcome only on the currently-live row. Double
	// EndSession is a no-op (ended_at already set => WHERE clause
	// excludes the row); prevents later overwrite of the first
	// authoritative outcome label.
	_, err := s.db.Exec(
		`UPDATE debug_sessions SET ended_at = ?, outcome = ? WHERE conversation_id = ? AND ended_at IS NULL`,
		now, outcome, conversationID,
	)
	if err != nil {
		s.log.Warn("debug: EndSession update failed",
			zap.String("conversation_id", conversationID),
			zap.String("outcome", outcome),
			zap.Error(err))
	}
	s.seqByConv.Delete(conversationID)
}

// SweepOrphans marks any debug_sessions row with ended_at IS NULL as
// outcome='interrupted' with ended_at derived from the latest
// captured event timestamp. Intended to run once at server boot so
// pre-crash live sessions don't appear eternally live in the
// Settings → Debug list.
//
// Priority for the derived ended_at:
//   1. MAX(debug_events.finished_at) for that conversation
//   2. MAX(debug_events.started_at) if no finished_at present
//   3. the session's own started_at if no events captured
func SweepOrphans(db *sql.DB, log *zap.Logger) error {
	if db == nil {
		return nil
	}
	if log == nil {
		log = zap.NewNop()
	}
	_, err := db.Exec(`
		UPDATE debug_sessions
		SET ended_at = COALESCE(
		        (SELECT MAX(finished_at) FROM debug_events WHERE conversation_id = debug_sessions.conversation_id),
		        (SELECT MAX(started_at)  FROM debug_events WHERE conversation_id = debug_sessions.conversation_id),
		        started_at
		    ),
		    outcome = 'interrupted'
		WHERE ended_at IS NULL`)
	if err != nil {
		log.Warn("debug: SweepOrphans failed", zap.Error(err))
	}
	return err
}

