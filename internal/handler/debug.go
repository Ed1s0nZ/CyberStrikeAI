package handler

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"cyberstrike-ai/internal/debug"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// DebugHandler hosts the /api/debug/* routes (Tasks 15-18).
type DebugHandler struct {
	db     *sql.DB
	logger *zap.Logger
}

// NewDebugHandler constructs the handler. Nil logger defaults to nop.
func NewDebugHandler(db *sql.DB, logger *zap.Logger) *DebugHandler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &DebugHandler{db: db, logger: logger}
}

// sessionSummary is the per-row shape returned by ListSessions.
type sessionSummary struct {
	ConversationID   string `json:"conversationId"`
	Label            string `json:"label,omitempty"`
	StartedAt        int64  `json:"startedAt"`
	EndedAt          *int64 `json:"endedAt,omitempty"`
	Outcome          string `json:"outcome,omitempty"`
	Iterations       int64  `json:"iterations"`
	PromptTokens     int64  `json:"promptTokens"`
	CompletionTokens int64  `json:"completionTokens"`
	DurationMs       int64  `json:"durationMs"`
}

// ListSessions handles GET /api/debug/sessions.
// Per-row aggregates (iterations, tokens, durationMs) are computed
// at query time via SQL aggregates over debug_llm_calls; they're
// not stored columns on debug_sessions. Empty result returns [] with
// 200, not 404.
func (h *DebugHandler) ListSessions(c *gin.Context) {
	rows, err := h.db.Query(`
		SELECT
		  s.conversation_id,
		  COALESCE(s.label, ''),
		  s.started_at,
		  s.ended_at,
		  COALESCE(s.outcome, ''),
		  COALESCE((SELECT COUNT(DISTINCT iteration) FROM debug_llm_calls WHERE conversation_id = s.conversation_id), 0) AS iterations,
		  COALESCE((SELECT SUM(prompt_tokens)        FROM debug_llm_calls WHERE conversation_id = s.conversation_id), 0) AS prompt_tokens,
		  COALESCE((SELECT SUM(completion_tokens)    FROM debug_llm_calls WHERE conversation_id = s.conversation_id), 0) AS completion_tokens
		FROM debug_sessions s
		ORDER BY s.started_at DESC
		LIMIT 200
	`)
	if err != nil {
		h.logger.Warn("debug: ListSessions query failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "debug query failed"})
		return
	}
	defer rows.Close()

	out := make([]sessionSummary, 0, 16)
	for rows.Next() {
		var r sessionSummary
		var endedAt sql.NullInt64
		if err := rows.Scan(&r.ConversationID, &r.Label, &r.StartedAt, &endedAt, &r.Outcome, &r.Iterations, &r.PromptTokens, &r.CompletionTokens); err != nil {
			h.logger.Warn("debug: ListSessions scan failed", zap.Error(err))
			continue
		}
		if endedAt.Valid {
			r.EndedAt = &endedAt.Int64
			durationNs := endedAt.Int64 - r.StartedAt
			r.DurationMs = durationNs / 1_000_000
		}
		out = append(out, r)
	}
	c.JSON(http.StatusOK, out)
}

// GetSession handles GET /api/debug/sessions/:id and returns the
// full capture payload: session row + all llm_calls + all events.
func (h *DebugHandler) GetSession(c *gin.Context) {
	id := c.Param("id")

	var sess sessionSummary
	var endedAt sql.NullInt64
	err := h.db.QueryRow(`
		SELECT conversation_id, COALESCE(label,''), started_at, ended_at, COALESCE(outcome,'')
		FROM debug_sessions WHERE conversation_id = ?`, id).
		Scan(&sess.ConversationID, &sess.Label, &sess.StartedAt, &endedAt, &sess.Outcome)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "debug session not found"})
		return
	}
	if err != nil {
		h.logger.Warn("debug: GetSession session query failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "debug query failed"})
		return
	}
	if endedAt.Valid {
		sess.EndedAt = &endedAt.Int64
		sess.DurationMs = (endedAt.Int64 - sess.StartedAt) / 1_000_000
	}

	llmCalls, err := debug.LoadLLMCallsExported(h.db, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	events, err := debug.LoadEventsExported(h.db, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"session":  sess,
		"llmCalls": llmCalls,
		"events":   events,
	})
}

// DeleteSession handles DELETE /api/debug/sessions/:id.
// Purges rows from all three debug tables in one transaction.
// Does not touch the parent messages / conversations rows.
func (h *DebugHandler) DeleteSession(c *gin.Context) {
	id := c.Param("id")
	var n int
	if err := h.db.QueryRow(`SELECT COUNT(*) FROM debug_sessions WHERE conversation_id = ?`, id).Scan(&n); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if n == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "debug session not found"})
		return
	}
	tx, err := h.db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer tx.Rollback()
	for _, tbl := range []string{"debug_events", "debug_llm_calls", "debug_sessions"} {
		if _, err := tx.Exec(`DELETE FROM `+tbl+` WHERE conversation_id = ?`, id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// PatchSession handles PATCH /api/debug/sessions/:id. Only the
// label field is mutable in v1.
func (h *DebugHandler) PatchSession(c *gin.Context) {
	id := c.Param("id")
	var body struct {
		Label *string `json:"label"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if body.Label == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "label required"})
		return
	}
	res, err := h.db.Exec(`UPDATE debug_sessions SET label = ? WHERE conversation_id = ?`, *body.Label, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "debug session not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ExportConversation handles GET /api/conversations/:id/export.
// Query param format=raw (default) or format=sharegpt.
// Streams JSONL directly to the response writer; peak memory is one row.
func (h *DebugHandler) ExportConversation(c *gin.Context) {
	id := c.Param("id")
	format := c.DefaultQuery("format", "raw")

	if format != "raw" && format != "sharegpt" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "format must be raw or sharegpt"})
		return
	}

	c.Header("Content-Type", "application/jsonl")
	c.Header("Content-Disposition", `attachment; filename="`+id+`.jsonl"`)

	switch format {
	case "raw":
		if err := debug.WriteRawJSONL(c.Writer, h.db, id); err != nil {
			h.logger.Warn("debug: ExportConversation raw failed", zap.Error(err))
		}
	case "sharegpt":
		if err := debug.WriteShareGPTJSONL(c.Writer, h.db, id); err != nil {
			h.logger.Warn("debug: ExportConversation sharegpt failed", zap.Error(err))
		}
	}
}

// ExportBulk handles GET /api/debug/export-bulk.
// Query params: format=raw|sharegpt (default sharegpt),
//               since=unix_ms (optional), until=unix_ms (optional).
// Streams a gzip-compressed tar archive with one JSONL entry per
// session in the [since, until] window.
func (h *DebugHandler) ExportBulk(c *gin.Context) {
	format := c.DefaultQuery("format", "sharegpt")
	var sinceNS, untilNS int64
	if s := c.Query("since"); s != "" {
		if ms, err := strconv.ParseInt(s, 10, 64); err == nil {
			sinceNS = ms * int64(time.Millisecond)
		}
	}
	if u := c.Query("until"); u != "" {
		if ms, err := strconv.ParseInt(u, 10, 64); err == nil {
			untilNS = ms * int64(time.Millisecond)
		}
	}

	if format != "raw" && format != "sharegpt" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "format must be raw or sharegpt"})
		return
	}

	c.Header("Content-Type", "application/gzip")
	c.Header("Content-Disposition", `attachment; filename="debug-export.tar.gz"`)

	if err := debug.WriteBulkArchive(c.Writer, h.db, format, sinceNS, untilNS); err != nil {
		h.logger.Warn("debug: ExportBulk failed", zap.Error(err))
	}
}
