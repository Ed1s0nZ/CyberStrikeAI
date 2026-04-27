# Telegram Bot Feature Parity Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bring Telegram bot to feature/observability parity with the main UI: claude-cli routing, debug capture, streaming progress edits, persisted per-user sessions, per-chat agent-mode override, conversation-list filter + badge.

**Architecture:** Approach A from spec — minimal-touch. Two new SQLite objects (`bot_sessions` table + `conversations.platform` column), `RobotHandler` implements existing `StreamingMessageHandler` interface backed by DB reads, `ProcessMessageForRobot` gains `forceMode` + `progressFn` params plus `StartSession`/`EndSession`/`WithCapture` plumbing mirroring `AgentLoopStream`.

**Tech Stack:** Go 1.22, gin, SQLite (`mattn/go-sqlite3`), zap, vanilla JS SPA. No new dependencies.

## Spec reference

`docs/superpowers/specs/2026-04-26-telegram-bot-parity-design.md` (commit `58b847b`).

## File map

**Create:**
- `internal/database/bot_sessions.go` — `BotSession` type + Get/Upsert/Clear/SetMode methods.
- `internal/database/bot_sessions_test.go` — table CRUD + FK cascade.
- `internal/handler/robot_test.go` — handler tests (or extend existing if any).
- `internal/handler/robot_progress.go` — bot-side progress filter (`MajorEventStep` translator).
- `internal/handler/robot_progress_test.go`.

**Modify:**
- `internal/database/database.go` — add bot_sessions CREATE + idempotent `conversations.platform` ALTER.
- `internal/database/conversation.go` — `CreateConversationWithPlatform` + `ListConversations` platform filter.
- `internal/handler/agent.go` — `ProcessMessageForRobot` signature/body + `wrapRunWithDebug` use.
- `internal/handler/agent.go` — `ProcessMessageForRobot` callers updated (bot path only).
- `internal/handler/robot.go` — implement `StreamingMessageHandler`, drop in-memory `sessions` map, add `mode` command, add `tasks.StartTask` lock.
- `internal/handler/conversation.go` — `?platform=` query param on list endpoint.
- `web/static/js/chat.js` (or wherever the conversation list renders) — filter dropdown + per-card badge.
- `web/templates/index.html` — dropdown markup.
- `web/static/i18n/en-US.json`, `uk-UA.json` — filter + badge keys.

## Ordering rationale

Tasks 1–3 establish DB foundation (schema + bot_sessions CRUD + platform-aware Conversation API) — independent of each other once schema lands. Task 4 modifies `ProcessMessageForRobot` signature; Tasks 5–7 fill in the new behavior (bookends/claude-cli/progressFn). Tasks 8–9 rewrite `RobotHandler` to use the new DB layer and signature. Tasks 10–13 are the UI surface — independent of 4–9 and could ship in parallel. Task 14 is the final integration gate.

---

## Task 1: DB schema for bot_sessions + conversations.platform

**Files:**
- Modify: `internal/database/database.go`
- Test: `internal/database/database_test.go` (existing or create)

- [ ] **Step 1: Read the current `initTables` body**

```
sed -n '60,90p' internal/database/database.go
sed -n '450,490p' internal/database/database.go
```

Confirm:
- `createConversationsTable` is the CREATE statement starting around L62 (with columns `id`, `title`, `created_at`, `updated_at`, plus later `last_react_input`, `last_react_output`).
- ALTER statements for legacy columns (`last_react_input`, `last_react_output`) follow the pattern: `db.Exec("ALTER TABLE conversations ADD COLUMN ...")` wrapped in error-check that ignores "duplicate column" errors. This is the pattern to mirror for the new `platform` column.

- [ ] **Step 2: Write failing test — `internal/database/database_test.go`**

```go
package database

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestInitTables_BotSessionsAndPlatform(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "init_test.db")
	db, err := sql.Open("sqlite3", "file:"+tmp+"?_foreign_keys=1")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	d := &DB{db: db}
	if err := d.initTables(); err != nil {
		t.Fatalf("initTables: %v", err)
	}

	// bot_sessions table
	var name string
	if err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='bot_sessions'").Scan(&name); err != nil {
		t.Fatalf("bot_sessions table missing: %v", err)
	}

	// conversations.platform column
	rows, err := db.Query(`PRAGMA table_info(conversations)`)
	if err != nil {
		t.Fatalf("PRAGMA: %v", err)
	}
	defer rows.Close()
	hasPlatform := false
	for rows.Next() {
		var cid int
		var n, ttype string
		var notNull, pk int
		var dflt sql.NullString
		_ = rows.Scan(&cid, &n, &ttype, &notNull, &dflt, &pk)
		if n == "platform" {
			hasPlatform = true
		}
	}
	if !hasPlatform {
		t.Fatalf("conversations.platform column missing")
	}
}
```

If `internal/database/database_test.go` already exists, append; otherwise create with the block above.

- [ ] **Step 3: Run to verify fail**

```
cd /home/badb/CyberStrikeAI && go test ./internal/database/ -run TestInitTables_BotSessionsAndPlatform -v
```

Expected: FAIL — both objects absent.

- [ ] **Step 4: Add `bot_sessions` CREATE in `initTables`**

In `internal/database/database.go`, locate the existing CREATE-table block (e.g., after `createDebugEventsTable`). Add a new constant:

```go
	createBotSessionsTable := `
	CREATE TABLE IF NOT EXISTS bot_sessions (
		platform        TEXT NOT NULL,
		user_id         TEXT NOT NULL,
		conversation_id TEXT,
		current_mode    TEXT,
		updated_at      INTEGER NOT NULL,
		PRIMARY KEY (platform, user_id),
		FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE SET NULL
	);`
```

Then append a corresponding `db.Exec` next to the existing pattern (which has `if _, err := db.Exec(createDebugEventsTable); err != nil { return err }`):

```go
	if _, err := db.Exec(createBotSessionsTable); err != nil {
		return fmt.Errorf("create bot_sessions table: %w", err)
	}
```

- [ ] **Step 5: Add idempotent `platform` column ALTER**

In the same `initTables` body, find the existing ALTER block for `last_react_input` (around L461). Mirror the pattern. The exact existing pattern probably looks like:

```go
	if _, err := db.Exec("ALTER TABLE conversations ADD COLUMN last_react_input TEXT"); err != nil {
		// Ignore duplicate-column error on re-run
		if !strings.Contains(err.Error(), "duplicate column") {
			return fmt.Errorf("...: %w", err)
		}
	}
```

Add an analogous block:

```go
	if _, err := db.Exec("ALTER TABLE conversations ADD COLUMN platform TEXT"); err != nil {
		if !strings.Contains(err.Error(), "duplicate column") {
			return fmt.Errorf("alter conversations add platform: %w", err)
		}
	}
```

Place it next to the existing ALTER blocks (so all schema-evolution code stays together).

- [ ] **Step 6: Run test**

```
cd /home/badb/CyberStrikeAI && go test ./internal/database/ -run TestInitTables_BotSessionsAndPlatform -v
```

Expected: PASS.

- [ ] **Step 7: Full database test sweep**

```
cd /home/badb/CyberStrikeAI && go test -race ./internal/database/
```

Expected: all pass.

- [ ] **Step 8: Commit**

```bash
git add internal/database/database.go internal/database/database_test.go
git commit -m "database: add bot_sessions table + conversations.platform column

bot_sessions has primary key (platform, user_id) + FK to
conversations(id) ON DELETE SET NULL. Replaces RobotHandler's
in-memory sessions map (Task 7) with persistent storage.

conversations.platform column added via idempotent ALTER (matches
existing pattern for last_react_input/last_react_output columns).
NULL = web origin; 'telegram' = bot origin. Used by the conversation
list filter (Tasks 9-11) and the per-card badge.

Schema only — no Go behavior change. CRUD methods land in Task 2.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 2: `internal/database/bot_sessions.go` CRUD methods

**Files:**
- Create: `internal/database/bot_sessions.go`
- Create: `internal/database/bot_sessions_test.go`

- [ ] **Step 1: Write failing tests — `internal/database/bot_sessions_test.go`**

```go
package database

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func openBotSessionsTestDB(t *testing.T) *DB {
	t.Helper()
	tmp := filepath.Join(t.TempDir(), "bot_sessions_test.db")
	db, err := sql.Open("sqlite3", "file:"+tmp+"?_foreign_keys=1")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	d := &DB{db: db}
	if err := d.initTables(); err != nil {
		t.Fatalf("initTables: %v", err)
	}
	return d
}

func TestBotSession_RoundTrip(t *testing.T) {
	d := openBotSessionsTestDB(t)
	conv, err := d.CreateConversation("test")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	if err := d.UpsertBotSession("telegram", "u1", conv.ID, "multi"); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	got, err := d.GetBotSession("telegram", "u1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatalf("Get returned nil")
	}
	if got.ConversationID != conv.ID || got.CurrentMode != "multi" {
		t.Fatalf("fields mismatch: %+v", got)
	}
	if got.UpdatedAt == 0 {
		t.Fatalf("updated_at zero")
	}
}

func TestBotSession_GetMissingReturnsNilNoError(t *testing.T) {
	d := openBotSessionsTestDB(t)
	got, err := d.GetBotSession("telegram", "ghost")
	if err != nil {
		t.Fatalf("Get error on missing: %v", err)
	}
	if got != nil {
		t.Fatalf("want nil, got %+v", got)
	}
}

func TestBotSession_SetMode_PreservesConversationID(t *testing.T) {
	d := openBotSessionsTestDB(t)
	conv, _ := d.CreateConversation("c")
	_ = d.UpsertBotSession("telegram", "u1", conv.ID, "")
	if err := d.SetBotMode("telegram", "u1", "multi"); err != nil {
		t.Fatalf("SetBotMode: %v", err)
	}
	got, _ := d.GetBotSession("telegram", "u1")
	if got.CurrentMode != "multi" || got.ConversationID != conv.ID {
		t.Fatalf("expected mode=multi convID preserved, got %+v", got)
	}
}

func TestBotSession_Clear_WipesRow(t *testing.T) {
	d := openBotSessionsTestDB(t)
	conv, _ := d.CreateConversation("c")
	_ = d.UpsertBotSession("telegram", "u1", conv.ID, "multi")
	if err := d.ClearBotSession("telegram", "u1"); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	got, _ := d.GetBotSession("telegram", "u1")
	if got != nil {
		t.Fatalf("expected nil after Clear, got %+v", got)
	}
}

func TestBotSession_FKCascade_OnConversationDelete(t *testing.T) {
	d := openBotSessionsTestDB(t)
	conv, _ := d.CreateConversation("c")
	_ = d.UpsertBotSession("telegram", "u1", conv.ID, "multi")

	if _, err := d.db.Exec(`DELETE FROM conversations WHERE id = ?`, conv.ID); err != nil {
		t.Fatalf("delete conversation: %v", err)
	}
	got, err := d.GetBotSession("telegram", "u1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatalf("row should survive parent delete; FK is ON DELETE SET NULL")
	}
	if got.ConversationID != "" {
		t.Fatalf("conversation_id should be NULL/empty after parent delete, got %q", got.ConversationID)
	}
	if got.CurrentMode != "multi" {
		t.Fatalf("mode override should survive: %+v", got)
	}
	_ = time.Now()
}
```

- [ ] **Step 2: Run to verify fail**

```
cd /home/badb/CyberStrikeAI && go test ./internal/database/ -run TestBotSession -v
```

Expected: FAIL — none of the methods exist.

- [ ] **Step 3: Create `internal/database/bot_sessions.go`**

```go
package database

import (
	"database/sql"
	"fmt"
	"time"
)

// BotSession is the persistent per-user session record for bot
// platforms. Replaces the in-memory map RobotHandler used to keep
// (which evaporated on every restart). Mode override and current
// conversation are both stored here; ON DELETE SET NULL on the
// conversation_id FK lets bot users keep their mode override even
// when the operator deletes the conversation through the web UI.
type BotSession struct {
	Platform       string
	UserID         string
	ConversationID string // empty string when conversation was deleted (FK SET NULL)
	CurrentMode    string // "" = inherit global; "single" | "multi"
	UpdatedAt      int64
}

// GetBotSession returns the session row for (platform, user_id), or
// (nil, nil) if no row exists. Errors are reserved for actual DB
// failures.
func (db *DB) GetBotSession(platform, userID string) (*BotSession, error) {
	row := db.db.QueryRow(`
		SELECT platform, user_id, COALESCE(conversation_id, ''),
		       COALESCE(current_mode, ''), updated_at
		FROM bot_sessions
		WHERE platform = ? AND user_id = ?`, platform, userID)
	var s BotSession
	if err := row.Scan(&s.Platform, &s.UserID, &s.ConversationID, &s.CurrentMode, &s.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get bot session: %w", err)
	}
	return &s, nil
}

// UpsertBotSession creates or updates the row keyed by (platform,
// user_id). conversationID empty string is stored as SQL NULL. mode
// empty string is stored as SQL NULL (= inherit global default).
func (db *DB) UpsertBotSession(platform, userID, conversationID, mode string) error {
	now := time.Now().UnixNano()
	var convArg interface{}
	if conversationID != "" {
		convArg = conversationID
	}
	var modeArg interface{}
	if mode != "" {
		modeArg = mode
	}
	_, err := db.db.Exec(`
		INSERT INTO bot_sessions (platform, user_id, conversation_id, current_mode, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(platform, user_id) DO UPDATE SET
		    conversation_id = excluded.conversation_id,
		    current_mode    = excluded.current_mode,
		    updated_at      = excluded.updated_at`,
		platform, userID, convArg, modeArg, now)
	if err != nil {
		return fmt.Errorf("upsert bot session: %w", err)
	}
	return nil
}

// ClearBotSession wipes the row entirely. Used by the `clear` slash
// command — operator-stated semantics in the spec: clear is "I want
// a fresh start"; mode override is also reset to global default.
func (db *DB) ClearBotSession(platform, userID string) error {
	_, err := db.db.Exec(`DELETE FROM bot_sessions WHERE platform = ? AND user_id = ?`, platform, userID)
	if err != nil {
		return fmt.Errorf("clear bot session: %w", err)
	}
	return nil
}

// SetBotMode updates only the current_mode column on the row,
// preserving conversation_id and updated_at semantics. Empty string
// for mode means "inherit global default" — stored as NULL.
func (db *DB) SetBotMode(platform, userID, mode string) error {
	now := time.Now().UnixNano()
	var modeArg interface{}
	if mode != "" {
		modeArg = mode
	}
	// Need to upsert: setting mode for a (platform, user_id) without
	// an existing session is valid (e.g., user runs `mode multi`
	// before any chat message).
	_, err := db.db.Exec(`
		INSERT INTO bot_sessions (platform, user_id, conversation_id, current_mode, updated_at)
		VALUES (?, ?, NULL, ?, ?)
		ON CONFLICT(platform, user_id) DO UPDATE SET
		    current_mode = excluded.current_mode,
		    updated_at   = excluded.updated_at`,
		platform, userID, modeArg, now)
	if err != nil {
		return fmt.Errorf("set bot mode: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Run tests**

```
cd /home/badb/CyberStrikeAI && go test -race ./internal/database/ -run TestBotSession -v
```

Expected: all 5 pass.

- [ ] **Step 5: Commit**

```bash
git add internal/database/bot_sessions.go internal/database/bot_sessions_test.go
git commit -m "database: bot_sessions CRUD methods

Get returns (nil, nil) on miss for callsite simplicity. Upsert and
SetBotMode use ON CONFLICT DO UPDATE so the bot's two write paths
(new chat / mode-only command) collapse into single statements.
Clear wipes the row entirely — semantically 'fresh start' includes
resetting any mode override to global default per the spec.

Empty-string mode is stored as SQL NULL so 'inherit global' is
distinguishable from 'explicitly single' on read.

FK cascade test verifies ON DELETE SET NULL semantics: deleting the
parent conversation row leaves the bot session intact with NULL
conversation_id but preserved mode override.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 3: Conversation API platform-aware extensions

**Files:**
- Modify: `internal/database/conversation.go`
- Test: `internal/database/conversation_test.go` (extend)

- [ ] **Step 1: Read current shape**

```
sed -n '30,60p' internal/database/conversation.go
sed -n '300,350p' internal/database/conversation.go
```

Note `CreateConversation(title string) (*Conversation, error)` at L35 and `ListConversations(limit, offset int, search string)` at L307.

- [ ] **Step 2: Write failing tests**

Append to `internal/database/conversation_test.go` (create file if absent — pattern matches existing test files):

```go
package database

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func openConvTestDB(t *testing.T) *DB {
	t.Helper()
	tmp := filepath.Join(t.TempDir(), "conv_test.db")
	db, err := sql.Open("sqlite3", "file:"+tmp+"?_foreign_keys=1")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	d := &DB{db: db}
	if err := d.initTables(); err != nil {
		t.Fatalf("initTables: %v", err)
	}
	return d
}

func TestCreateConversationWithPlatform(t *testing.T) {
	d := openConvTestDB(t)
	conv, err := d.CreateConversationWithPlatform("scan", "telegram")
	if err != nil {
		t.Fatalf("CreateConversationWithPlatform: %v", err)
	}
	var platform sql.NullString
	if err := d.db.QueryRow(`SELECT platform FROM conversations WHERE id = ?`, conv.ID).Scan(&platform); err != nil {
		t.Fatalf("Query platform: %v", err)
	}
	if !platform.Valid || platform.String != "telegram" {
		t.Fatalf("platform: want 'telegram', got %v", platform)
	}
}

func TestCreateConversation_PlatformIsNULL(t *testing.T) {
	d := openConvTestDB(t)
	conv, err := d.CreateConversation("web chat")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	var platform sql.NullString
	_ = d.db.QueryRow(`SELECT platform FROM conversations WHERE id = ?`, conv.ID).Scan(&platform)
	if platform.Valid {
		t.Fatalf("expected NULL platform on web-origin conversation, got %v", platform)
	}
}

func TestListConversations_FilterByPlatform(t *testing.T) {
	d := openConvTestDB(t)
	_, _ = d.CreateConversation("web1")
	_, _ = d.CreateConversation("web2")
	_, _ = d.CreateConversationWithPlatform("tg1", "telegram")

	all, err := d.ListConversations(100, 0, "")
	if err != nil {
		t.Fatalf("ListConversations: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("all: want 3, got %d", len(all))
	}

	tgOnly, err := d.ListConversationsByPlatform(100, 0, "", "telegram")
	if err != nil {
		t.Fatalf("ListConversationsByPlatform: %v", err)
	}
	if len(tgOnly) != 1 || tgOnly[0].Title != "tg1" {
		t.Fatalf("tg-only: want [tg1], got %+v", tgOnly)
	}
}
```

- [ ] **Step 3: Run to verify fail**

```
cd /home/badb/CyberStrikeAI && go test ./internal/database/ -run "TestCreateConversation\|TestListConversations_FilterByPlatform" -v
```

Expected: FAIL — `CreateConversationWithPlatform` and `ListConversationsByPlatform` undefined.

- [ ] **Step 4: Implement helpers in `internal/database/conversation.go`**

Locate `CreateConversation` at L35 and read its body to learn the existing INSERT pattern. Then add a sibling:

```go
// CreateConversationWithPlatform tags the conversation row with a
// non-NULL platform string ("telegram", future: "dingtalk", etc.).
// Used by RobotHandler when creating a conversation on first message
// from a bot user.
func (db *DB) CreateConversationWithPlatform(title, platform string) (*Conversation, error) {
	c, err := db.CreateConversation(title)
	if err != nil {
		return nil, err
	}
	if _, err := db.db.Exec(`UPDATE conversations SET platform = ? WHERE id = ?`, platform, c.ID); err != nil {
		return nil, fmt.Errorf("set platform: %w", err)
	}
	return c, nil
}
```

(Two-statement implementation keeps the platform optional without forking `CreateConversation`'s INSERT signature — small write-amplification cost is acceptable.)

Add `ListConversationsByPlatform` next to `ListConversations`:

```go
// ListConversationsByPlatform returns conversations filtered by
// platform tag. Empty platform = NULL match (web-origin only). Use
// the existing ListConversations for "all platforms".
func (db *DB) ListConversationsByPlatform(limit, offset int, search, platform string) ([]*Conversation, error) {
	// Read existing ListConversations query/scan logic and add a
	// `WHERE platform = ?` (or `IS NULL` for empty) clause to it.
	// The exact SQL depends on what ListConversations does with
	// search; mirror that and add the platform constraint.
	//
	// Sketch (adapt to actual query in ListConversations):
	q := `SELECT id, title, created_at, updated_at,
	             COALESCE(last_react_input, ''), COALESCE(last_react_output, ''),
	             pinned
	      FROM conversations
	      WHERE 1=1`
	args := []interface{}{}
	if platform == "" {
		q += " AND platform IS NULL"
	} else {
		q += " AND platform = ?"
		args = append(args, platform)
	}
	if search != "" {
		q += " AND title LIKE ?"
		args = append(args, "%"+search+"%")
	}
	q += " ORDER BY updated_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := db.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("list conversations by platform: %w", err)
	}
	defer rows.Close()
	var out []*Conversation
	for rows.Next() {
		c := &Conversation{}
		if err := rows.Scan(&c.ID, &c.Title, &c.CreatedAt, &c.UpdatedAt,
			&c.LastReActInput, &c.LastReActOutput, &c.Pinned); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}
```

**Critical**: read the actual `ListConversations` body at L307 and mirror its column list + scan order EXACTLY. The sketch above guesses at the columns; the existing function is authoritative.

- [ ] **Step 5: Run tests**

```
cd /home/badb/CyberStrikeAI && go test -race ./internal/database/ -v 2>&1 | tail -20
```

Expected: all conversation tests pass + the prior bot_sessions tests still pass.

- [ ] **Step 6: Commit**

```bash
git add internal/database/conversation.go internal/database/conversation_test.go
git commit -m "database: platform-aware conversation API

CreateConversationWithPlatform tags new conversations with a
platform string (\"telegram\" today). Web-origin conversations stay
NULL — distinguishable on read for filter/badge logic in the UI.

ListConversationsByPlatform mirrors ListConversations' shape with an
added WHERE clause. Empty platform argument matches NULL (web-only);
non-empty matches the literal value. Caller chooses ListConversations
(all platforms) vs ListConversationsByPlatform (filter) by API.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 4: `ProcessMessageForRobot` signature + bookends

**Files:**
- Modify: `internal/handler/agent.go` (around L610)
- Modify: callers in `internal/handler/robot.go` (one site at this point)

- [ ] **Step 1: Read current `ProcessMessageForRobot`**

```
cd /home/badb/CyberStrikeAI && sed -n '610,720p' internal/handler/agent.go
```

Confirm the signature `ProcessMessageForRobot(ctx context.Context, conversationID, message, role string) (response string, convID string, err error)` and inspect the body — note where the multi-vs-single decision is made (around L664: `useRobotMulti := h.config != nil && h.config.MultiAgent.Enabled && h.config.MultiAgent.RobotUseMultiAgent`).

- [ ] **Step 2: Find the single caller**

```
cd /home/badb/CyberStrikeAI && grep -n "ProcessMessageForRobot" internal/handler/*.go
```

Expect exactly one production caller in `internal/handler/robot.go` plus possibly tests.

- [ ] **Step 3: Update the signature**

In `internal/handler/agent.go`, change:

```go
func (h *AgentHandler) ProcessMessageForRobot(ctx context.Context, conversationID, message, role string) (response string, convID string, err error) {
```

to:

```go
// ProcessMessageForRobot drives an agent loop on behalf of a chat
// platform (Telegram today). forceMode overrides the global
// RobotUseMultiAgent flag for this single invocation:
//   - "multi"  → orchestrator path
//   - "single" → AgentLoopWithProgress path
//   - "" or anything else → fall back to global
// progressFn (nil = silent) receives bot-side step strings as the
// agent loop runs — the caller is responsible for any throttling.
func (h *AgentHandler) ProcessMessageForRobot(
	ctx context.Context,
	conversationID, message, role, forceMode string,
	progressFn func(step string),
) (response string, convID string, err error) {
```

- [ ] **Step 4: Replace the existing `useRobotMulti` line**

Find `useRobotMulti := h.config != nil && h.config.MultiAgent.Enabled && h.config.MultiAgent.RobotUseMultiAgent` and replace with:

```go
	var useRobotMulti bool
	switch forceMode {
	case "multi":
		useRobotMulti = h.config != nil && h.config.MultiAgent.Enabled
	case "single":
		useRobotMulti = false
	default:
		useRobotMulti = h.config != nil && h.config.MultiAgent.Enabled && h.config.MultiAgent.RobotUseMultiAgent
	}
```

`forceMode == "multi"` still requires `MultiAgent.Enabled` — the user's per-chat override can't override the operator's global enable flag.

- [ ] **Step 5: Add `StartSession`/`EndSession` bookends**

Inside the function body, BEFORE the agent-loop-running branch, add:

```go
	taskStatus := "completed"
	h.debugSink.StartSession(conversationID)
	defer func() {
		if err != nil {
			taskStatus = "failed"
		}
		h.debugSink.EndSession(conversationID, taskStatus)
	}()
```

(The `err` is the named return value, so the defer reads its final state via closure capture.)

- [ ] **Step 6: Update caller in `internal/handler/robot.go`**

The single existing call site (around L132 in `RobotHandler.HandleMessage`) currently passes 4 args to `ProcessMessageForRobot`. Add the two new args. For now, pass `""` (no force) and `nil` (no progress):

```go
	resp, convID, err := h.agentHandler.ProcessMessageForRobot(ctx, sessionConv, text, role, "", nil)
```

This keeps Task 4 a behavior-neutral signature change. The actual streaming and force-mode wiring happens in Task 8.

- [ ] **Step 7: Build + test**

```
cd /home/badb/CyberStrikeAI && go build ./... && go vet ./... && go test -race ./internal/handler/ -v 2>&1 | tail -10
```

Expected: clean build, all existing tests pass, no behavior change yet.

- [ ] **Step 8: Commit**

```bash
git add internal/handler/agent.go internal/handler/robot.go
git commit -m "handler: ProcessMessageForRobot signature + debug bookends

Adds forceMode + progressFn parameters; both unused by the single
existing caller (passes \"\" and nil) until Task 8 wires the bot
streaming path. forceMode 'multi'/'single' overrides the global
RobotUseMultiAgent flag for the current invocation; multi-agent
override still respects the operator's MultiAgent.Enabled flag.

debugSink.StartSession + deferred EndSession (closure capturing the
named-return err to set taskStatus) close the audit-finding gap
where bot conversations were dark in Settings → Debug despite
LLM-call rows landing orphaned in debug_llm_calls.

No behavior change for users yet — this is the structural
prerequisite for Tasks 5-8.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 5: `ProcessMessageForRobot` claude-cli branch

**Files:**
- Modify: `internal/handler/agent.go` (`ProcessMessageForRobot` body)

- [ ] **Step 1: Read the existing claude-cli branch in `AgentLoopStream`**

```
cd /home/badb/CyberStrikeAI && sed -n '1260,1300p' internal/handler/agent.go
```

This is the reference implementation. Note: `h.claudeAdapter.RunPrompt(taskCtx, finalMessage, systemPrompt, conversationID, roleTools, sendEvent)` is the call. Adapt for the bot context.

- [ ] **Step 2: Add the branch in `ProcessMessageForRobot`**

In `ProcessMessageForRobot`, BEFORE the existing `useRobotMulti`-driven branch, add:

```go
	if h.config != nil && h.config.EffectiveProvider() == "claude-cli" && h.claudeAdapter != nil {
		// Claude CLI runs its own internal tool loop; we don't get
		// per-event progress signals. Emit a single placeholder via
		// progressFn so the bot user sees something change.
		if progressFn != nil {
			progressFn("Running through Claude CLI…")
		}
		// Build a sendEvent shim so RunPrompt's progress events also
		// flow into the debug sink (RunPrompt writes its own
		// debug_events via the adapter — verify against the existing
		// AgentLoopStream branch for parity).
		sendEvent := func(eventType, msg string, data interface{}) {}
		systemPrompt := "" // bot path doesn't currently apply orchestrator prompt; keep empty
		resultText, _, runErr := h.claudeAdapter.RunPrompt(ctx, message, systemPrompt, conversationID, /*roleTools=*/ nil, sendEvent)
		if runErr != nil {
			return "", conversationID, runErr
		}
		return resultText, conversationID, nil
	}
```

If the existing AgentLoopStream branch passes `roleTools` (the role's tool restriction list) as a non-nil arg, mirror that — the bot path resolves `role` to a `*config.Role` earlier in `ProcessMessageForRobot`; check the existing body for how `roleTools` is computed there.

- [ ] **Step 3: Build + test**

```
cd /home/badb/CyberStrikeAI && go build ./... && go vet ./...
```

Expected: clean.

- [ ] **Step 4: Commit**

```bash
git add internal/handler/agent.go
git commit -m "handler: ProcessMessageForRobot claude-cli branch

Mirrors AgentLoopStream's claude-cli branch: when Provider ==
'claude-cli' and the adapter is configured, route bot messages
through h.claudeAdapter.RunPrompt instead of the OpenAI/multi-agent
loop. Emits a single 'Running through Claude CLI…' progress event
because the CLI subprocess doesn't surface per-event tool progress.

Closes audit-finding gap 1 — bot path used to ignore the global
provider toggle and silently fall back to OpenAI even when the
operator had configured claude-cli.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 6: Bot-side `progressFn` filter (`MajorEventStep`)

**Files:**
- Create: `internal/handler/robot_progress.go`
- Create: `internal/handler/robot_progress_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/handler/robot_progress_test.go`:

```go
package handler

import (
	"testing"
)

func TestMajorEventStep_ReturnsEmptyForFilteredEvents(t *testing.T) {
	cases := []string{"thinking_stream_start", "thinking_stream_delta", "tool_result_delta", "response_delta", "done"}
	for _, ev := range cases {
		got := MajorEventStep(ev, "", nil)
		if got != "" {
			t.Errorf("event %q should be filtered (return ''), got %q", ev, got)
		}
	}
}

func TestMajorEventStep_IterationFormatsRoundNumber(t *testing.T) {
	got := MajorEventStep("iteration", "", map[string]interface{}{"iteration": 2})
	want := "🤔 Round 2: thinking…"
	if got != want {
		t.Fatalf("want %q, got %q", want, got)
	}
}

func TestMajorEventStep_ToolCallShowsToolName(t *testing.T) {
	got := MajorEventStep("tool_call", "", map[string]interface{}{
		"toolName":  "nmap",
		"iteration": 3,
	})
	want := "🔧 Round 3: calling nmap…"
	if got != want {
		t.Fatalf("want %q, got %q", want, got)
	}
}

func TestMajorEventStep_ToolResultBranchesOnSuccess(t *testing.T) {
	ok := MajorEventStep("tool_result", "", map[string]interface{}{
		"toolName":  "nmap",
		"iteration": 3,
		"success":   true,
	})
	if ok != "✅ Round 3: nmap done" {
		t.Fatalf("success: got %q", ok)
	}
	fail := MajorEventStep("tool_result", "", map[string]interface{}{
		"toolName":  "nmap",
		"iteration": 3,
		"success":   false,
	})
	if fail != "❌ Round 3: nmap failed" {
		t.Fatalf("failure: got %q", fail)
	}
}

func TestMajorEventStep_ResponseStart(t *testing.T) {
	got := MajorEventStep("response_start", "", nil)
	if got != "✍️ Drafting answer…" {
		t.Fatalf("got %q", got)
	}
}
```

- [ ] **Step 2: Run to verify fail**

```
cd /home/badb/CyberStrikeAI && go test ./internal/handler/ -run TestMajorEventStep -v
```

Expected: FAIL — `MajorEventStep` undefined.

- [ ] **Step 3: Create `internal/handler/robot_progress.go`**

```go
package handler

import "fmt"

// MajorEventStep translates the agent loop's progress event tuple
// (eventType, message, data) into a short single-line "step" string
// suitable for editing into a Telegram placeholder message. Returns
// the empty string for events that should be silently filtered (the
// telegram.go throttler treats empty as no-op).
//
// Rule set is fixed to per-major-event verbosity per the spec:
//   - iteration       → 🤔 Round N: thinking…
//   - tool_call       → 🔧 Round N: calling {tool}…
//   - tool_result     → ✅/❌ Round N: {tool} done|failed
//   - response_start  → ✍️ Drafting answer…
//   - everything else → "" (filtered)
//
// Throttling is the caller's responsibility — telegram.go already
// enforces a wall-clock minimum interval between editMessageText
// calls (telegramEditThrottle).
func MajorEventStep(eventType, message string, data map[string]interface{}) string {
	switch eventType {
	case "iteration":
		n := intFromData(data, "iteration", 0)
		return fmt.Sprintf("🤔 Round %d: thinking…", n)
	case "tool_call":
		tool := stringFromData(data, "toolName", "?")
		n := intFromData(data, "iteration", 0)
		return fmt.Sprintf("🔧 Round %d: calling %s…", n, tool)
	case "tool_result":
		tool := stringFromData(data, "toolName", "?")
		n := intFromData(data, "iteration", 0)
		success, _ := data["success"].(bool)
		if success {
			return fmt.Sprintf("✅ Round %d: %s done", n, tool)
		}
		return fmt.Sprintf("❌ Round %d: %s failed", n, tool)
	case "response_start":
		return "✍️ Drafting answer…"
	}
	return ""
}

func intFromData(data map[string]interface{}, key string, fallback int) int {
	if data == nil {
		return fallback
	}
	switch v := data[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	}
	return fallback
}

func stringFromData(data map[string]interface{}, key, fallback string) string {
	if data == nil {
		return fallback
	}
	if v, ok := data[key].(string); ok && v != "" {
		return v
	}
	return fallback
}
```

- [ ] **Step 4: Run tests**

```
cd /home/badb/CyberStrikeAI && go test -race ./internal/handler/ -run TestMajorEventStep -v
```

Expected: all 5 tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/handler/robot_progress.go internal/handler/robot_progress_test.go
git commit -m "handler: MajorEventStep — bot-side progress filter

Translates the agent loop's (eventType, message, data) progress
tuple into a short Telegram-suitable step string. Filters out
high-frequency events (thinking_stream_delta, tool_result_delta,
response_delta) and emits compact step text for major boundaries:
iteration, tool_call, tool_result (success/fail), response_start.

Returns '' for filtered events; telegram.go's throttler treats
empty as no-op (verified at telegram.go:329-330). Throttling itself
stays in telegram.go, not in this filter — clean separation.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 7: Wire `progressFn` into `ProcessMessageForRobot`'s progress callback

**Files:**
- Modify: `internal/handler/agent.go` (`ProcessMessageForRobot` body)

- [ ] **Step 1: Locate the existing progress callback**

In `ProcessMessageForRobot`, the existing body builds a `progressCallback` that's handed to either `AgentLoopWithProgress` or `RunOrchestrator`. The exact line depends on where the body is now (post-Tasks 4-5 changes). Find it:

```
cd /home/badb/CyberStrikeAI && grep -n "createProgressCallback\|progressCallback\s*:=" internal/handler/agent.go | head
```

Expect a `progressCallback := h.createProgressCallback(...)` or a fresh closure inside `ProcessMessageForRobot`.

- [ ] **Step 2: Wrap the progress callback**

The existing callback writes to DB / SSE. We extend it to ALSO call `progressFn` via `MajorEventStep`. Two options:
  (a) Mutate the existing closure body to also call `progressFn(MajorEventStep(...))`.
  (b) Compose: build a wrapper that fires both the existing callback AND the bot-side filter.

Use (b) for clarity. Add right above the `useRobotMulti` branch (after the `progressCallback` assignment):

```go
	// Bot-side progress tee: forward filtered major events to the
	// caller-supplied progressFn (nil = silent, e.g. legacy callers
	// that didn't opt in).
	originalCallback := progressCallback
	progressCallback = func(eventType, message string, data interface{}) {
		if originalCallback != nil {
			originalCallback(eventType, message, data)
		}
		if progressFn != nil {
			dataMap, _ := data.(map[string]interface{})
			if step := MajorEventStep(eventType, message, dataMap); step != "" {
				progressFn(step)
			}
		}
	}
```

The exact name of the existing `progressCallback` variable may differ — adapt to whatever the function uses (`progressCb`, `cb`, etc.).

- [ ] **Step 3: Build + test**

```
cd /home/badb/CyberStrikeAI && go build ./... && go vet ./... && go test -race ./internal/handler/ 2>&1 | tail -10
```

Expected: clean.

- [ ] **Step 4: Commit**

```bash
git add internal/handler/agent.go
git commit -m "handler: tee bot-side progress through MajorEventStep filter

Wraps ProcessMessageForRobot's existing progressCallback so it ALSO
calls the caller's progressFn with filtered, formatted step strings.
Original callback (DB writes, SSE) fires unchanged; the tee is purely
additive. Nil progressFn = no bot-side emission, matching the
silent-default semantics for non-streaming callers.

Closes the F4-equivalent gap on the bot path: every-event noise is
filtered to per-major-event step strings via MajorEventStep, and
telegram.go's existing throttler enforces ≥3s between edits.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 8: `RobotHandler` implements `StreamingMessageHandler`

**Files:**
- Modify: `internal/handler/robot.go`

- [ ] **Step 1: Read current `HandleMessage` and the helpers it calls**

```
cd /home/badb/CyberStrikeAI && sed -n '40,180p' internal/handler/robot.go
```

Specifically confirm:
- `getOrCreateConversation(platform, userID, title)` — replace with DB-backed lookup.
- `setConversation(platform, userID, convID)` — replace with `UpsertBotSession`.
- `clearConversation(platform, userID)` — replace with `ClearBotSession`.
- `getRole(platform, userID)` — leave as-is (role is RAM-only per spec; out of scope).

- [ ] **Step 2: Drop the `sessions` map; add session-loader helpers**

In the `RobotHandler` struct (around L40):
- Remove `sessions map[string]string` field.
- Remove the `sessionsMu` mutex if it was the only thing the struct used it for (likely shared with `roleSessions` — keep the mutex for that).

Replace `getOrCreateConversation` body with:

```go
func (h *RobotHandler) getOrCreateConversation(platform, userID, title string) (convID string, isNew bool) {
	sess, err := h.db.GetBotSession(platform, userID)
	if err != nil {
		h.logger.Warn("GetBotSession failed; falling back to fresh conversation",
			zap.String("platform", platform), zap.String("user_id", userID), zap.Error(err))
	}
	if sess != nil && sess.ConversationID != "" {
		return sess.ConversationID, false
	}
	conv, err := h.db.CreateConversationWithPlatform(title, platform)
	if err != nil {
		h.logger.Error("CreateConversationWithPlatform failed", zap.Error(err))
		return "", true
	}
	mode := ""
	if sess != nil {
		mode = sess.CurrentMode // preserve mode override if session existed but conversation was deleted
	}
	if upErr := h.db.UpsertBotSession(platform, userID, conv.ID, mode); upErr != nil {
		h.logger.Warn("UpsertBotSession failed", zap.Error(upErr))
	}
	return conv.ID, true
}
```

Replace `setConversation`:

```go
func (h *RobotHandler) setConversation(platform, userID, convID string) {
	sess, _ := h.db.GetBotSession(platform, userID)
	mode := ""
	if sess != nil {
		mode = sess.CurrentMode
	}
	if err := h.db.UpsertBotSession(platform, userID, convID, mode); err != nil {
		h.logger.Warn("setConversation upsert failed", zap.Error(err))
	}
}
```

Replace `clearConversation`:

```go
func (h *RobotHandler) clearConversation(platform, userID string) (newConvID string) {
	if err := h.db.ClearBotSession(platform, userID); err != nil {
		h.logger.Warn("ClearBotSession failed", zap.Error(err))
	}
	// Don't pre-create a new conversation — next message creates one
	// via getOrCreateConversation. Return "" to signal "no current".
	return ""
}
```

- [ ] **Step 3: Add the streaming method**

Append to `internal/handler/robot.go`:

```go
// HandleMessageStream is the StreamingMessageHandler implementation.
// telegram.go invokes this when the streaming-handler interface is
// satisfied; the synchronous HandleMessage falls through to this
// with a no-op progressFn.
func (h *RobotHandler) HandleMessageStream(platform, userID, text string, progressFn func(step string)) string {
	// Reuse the existing HandleMessage body's command dispatch and
	// session resolution, but now passes progressFn down.
	return h.handleInternal(platform, userID, text, progressFn)
}

// HandleMessage is the synchronous wrapper for non-streaming clients.
func (h *RobotHandler) HandleMessage(platform, userID, text string) string {
	return h.handleInternal(platform, userID, text, nil)
}

// handleInternal is the shared implementation called by both the
// streaming and non-streaming entry points. It resolves session +
// dispatches commands + drives ProcessMessageForRobot.
func (h *RobotHandler) handleInternal(platform, userID, text string, progressFn func(step string)) string {
```

The body of `handleInternal` is the existing `HandleMessage` body lifted verbatim from L132-180 of the pre-Task-8 file. Before this task, that function is the public entry point; after this task, `HandleMessage` is a thin shim that calls `handleInternal(..., nil)` and `HandleMessageStream` calls `handleInternal(..., progressFn)`.

Concretely: cut the existing `HandleMessage` function body (between the curly braces, after the receiver/signature line) and paste it as the body of `handleInternal`. Then make the only behavioral change — update the call to `ProcessMessageForRobot` to pass `progressFn` (the param) instead of `nil`:

```go
	resp, convID, err := h.agentHandler.ProcessMessageForRobot(ctx, sessionConv, text, role, "", progressFn)
```

- [ ] **Step 4: Verify interface satisfaction**

Append to `internal/handler/robot_test.go` (create if absent):

```go
package handler

import (
	"cyberstrike-ai/internal/robot"
	"testing"
)

func TestRobotHandler_ImplementsStreamingMessageHandler(t *testing.T) {
	var _ robot.StreamingMessageHandler = (*RobotHandler)(nil)
}
```

- [ ] **Step 5: Run tests**

```
cd /home/badb/CyberStrikeAI && go build ./... && go vet ./... && go test -race ./internal/handler/ 2>&1 | tail -15
```

Expected: clean, including the interface-satisfaction compile check.

- [ ] **Step 6: Commit**

```bash
git add internal/handler/robot.go internal/handler/robot_test.go
git commit -m "handler: RobotHandler implements StreamingMessageHandler

The robot package's StreamingMessageHandler interface (declared at
internal/robot/conn.go) was previously unsatisfied — telegram.go's
type-assert at L365 always fell through to the synchronous path,
silencing the throttled-progress closure that was already built.
Now the streaming path fires.

In-memory sessions map is gone; getOrCreateConversation /
setConversation / clearConversation are now thin wrappers over
db.GetBotSession / UpsertBotSession / ClearBotSession from Task 2.
Bot user sessions persist across restart (gap-5 fix).

Conversations are created via CreateConversationWithPlatform from
Task 3 with platform=\"telegram\" — sets up the gap-9 filter/badge
work in Tasks 10-11.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 9: `mode` slash command + `tasks.StartTask` lock

**Files:**
- Modify: `internal/handler/robot.go`

- [ ] **Step 1: Read existing command dispatch**

```
cd /home/badb/CyberStrikeAI && sed -n '180,260p' internal/handler/robot.go
```

The existing dispatch is a switch on the first word of the input (`help`, `list`, `new`, `clear`, etc.). Find the switch.

- [ ] **Step 2: Add `mode` command handler method**

Append to `internal/handler/robot.go`:

```go
// cmdMode handles the `mode <single|multi|default>` slash command.
// Returns the reply string to send to the user. Does not invoke the
// agent loop — pure session-state mutation.
func (h *RobotHandler) cmdMode(platform, userID string, arg string) string {
	switch arg {
	case "":
		// Status query: report current effective mode + override.
		sess, _ := h.db.GetBotSession(platform, userID)
		override := ""
		if sess != nil {
			override = sess.CurrentMode
		}
		globalMulti := h.config != nil && h.config.MultiAgent.Enabled && h.config.MultiAgent.RobotUseMultiAgent
		effective := "single"
		if override == "multi" || (override == "" && globalMulti) {
			effective = "multi"
		}
		if override == "" {
			return fmt.Sprintf("Current mode: %s (inheriting global default).\nUse `mode single`, `mode multi`, or `mode default`.", effective)
		}
		return fmt.Sprintf("Current mode: %s (per-chat override).\nUse `mode default` to revert.", effective)

	case "single":
		if err := h.db.SetBotMode(platform, userID, "single"); err != nil {
			return "Failed to set mode: " + err.Error()
		}
		return "✅ This chat is now single-agent."

	case "multi":
		if h.config == nil || !h.config.MultiAgent.Enabled {
			return "Multi-agent is disabled in this deployment. Ask the operator to enable it (config: multi_agent.enabled)."
		}
		if err := h.db.SetBotMode(platform, userID, "multi"); err != nil {
			return "Failed to set mode: " + err.Error()
		}
		return "✅ This chat is now multi-agent."

	case "default":
		if err := h.db.SetBotMode(platform, userID, ""); err != nil {
			return "Failed to revert mode: " + err.Error()
		}
		globalMulti := h.config != nil && h.config.MultiAgent.Enabled && h.config.MultiAgent.RobotUseMultiAgent
		fallbackName := "single"
		if globalMulti {
			fallbackName = "multi"
		}
		return fmt.Sprintf("↩️ Reverted to global default (%s).", fallbackName)

	default:
		return fmt.Sprintf("Unknown mode '%s'. Use: mode single | mode multi | mode default", arg)
	}
}
```

- [ ] **Step 3: Wire `mode` into the dispatch switch**

In `handleInternal` (post-Task-8 name), find the switch on the first input word and add a case. Also handle the no-arg form (just `mode`):

```go
	case "mode":
		arg := strings.TrimSpace(strings.TrimPrefix(text, "mode"))
		return h.cmdMode(platform, userID, arg)
```

- [ ] **Step 4: Resolve `forceMode` from session before calling ProcessMessageForRobot**

In the non-command branch of `handleInternal`, after `sessionConv, _ := h.getOrCreateConversation(...)`:

```go
	// Resolve per-chat mode override (or "" to inherit).
	var forceMode string
	if sess, _ := h.db.GetBotSession(platform, userID); sess != nil {
		forceMode = sess.CurrentMode
	}
```

Then pass `forceMode` (instead of `""`) into `ProcessMessageForRobot`:

```go
	resp, convID, err := h.agentHandler.ProcessMessageForRobot(ctx, sessionConv, text, role, forceMode, progressFn)
```

- [ ] **Step 5: Add `tasks.StartTask` lock**

Inspect `internal/handler/multi_agent.go:121-138` for the canonical pattern. Apply to `handleInternal` in the non-command branch, BEFORE calling `ProcessMessageForRobot`:

```go
	// Per-conversation lock: refuse concurrent messages on the same
	// chat. Mirrors the web-UI semantics — second message gets a
	// synchronous reply instead of racing on shared state.
	taskCtx, cancel := context.WithCancel(ctx)
	cancelWithCause := func(cause error) { cancel() }
	if _, err := h.agentHandler.tasks.StartTask(sessionConv, text, cancelWithCause); err != nil {
		cancel()
		if errors.Is(err, ErrTaskAlreadyRunning) {
			return "⚠️ Another task is running for this chat. Say `stop` to cancel."
		}
		return "Failed to start task: " + err.Error()
	}
	taskStatus := "completed"
	defer func() { h.agentHandler.tasks.FinishTask(sessionConv, taskStatus) }()
```

(Note: `tasks.StartTask` takes a `context.CancelCauseFunc` per multi_agent.go's signature. If the simpler `cancel` we created here doesn't fit, build a small adapter. Read the signature in `task_manager.go:104` first to confirm.)

Use `taskCtx` for the rest of the agent invocation; on agent error or cancel, `taskStatus` updates accordingly via the same defer-closure pattern as elsewhere.

- [ ] **Step 6: Build + test**

```
cd /home/badb/CyberStrikeAI && go build ./... && go vet ./... && go test -race ./internal/handler/ 2>&1 | tail -15
```

Expected: clean.

- [ ] **Step 7: Commit**

```bash
git add internal/handler/robot.go
git commit -m "handler: \`mode\` command + per-conversation task lock

`mode <single|multi|default>` slash command sets the per-chat
override on bot_sessions.current_mode. mode multi requires
MultiAgent.Enabled at the global level (operator must opt the
deployment in before users can self-select). mode default reverts
the override; mode (no arg) reports current effective state.

Per-conversation tasks.StartTask lock mirrors web-UI's
MultiAgentLoopStream pattern: concurrent messages on the same chat
get a synchronous \"Another task is running… say stop to cancel.\"
reply instead of racing on shared state.

Bot now reads bot_sessions.current_mode at message time and passes
to ProcessMessageForRobot's forceMode param, so the per-chat
override actually changes routing.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 10: `?platform=` query param on conversation list endpoint

**Files:**
- Modify: `internal/handler/conversation.go` (or wherever `ListConversations` is exposed)

- [ ] **Step 1: Locate the existing handler**

```
cd /home/badb/CyberStrikeAI && grep -n "func.*ListConversations\|c.Query.*search\|router.GET.*conversation" internal/handler/*.go internal/app/app.go | head -10
```

Find the gin handler that serves `GET /api/conversations` (likely `internal/handler/conversation.go`). Read its body.

- [ ] **Step 2: Add `?platform=` handling**

In the handler body, after the existing `search := c.Query("search")` (or similar), add:

```go
	platform := c.Query("platform")
```

Then branch on whether `platform` is provided:

```go
	var conversations []*database.Conversation
	var err error
	if platform == "" {
		conversations, err = h.db.ListConversations(limit, offset, search)
	} else {
		conversations, err = h.db.ListConversationsByPlatform(limit, offset, search, platform)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
```

- [ ] **Step 3: Build + smoke**

```
cd /home/badb/CyberStrikeAI && go build ./... && go vet ./...
```

Manual smoke (run server, hit endpoint):
```
curl -s 'http://localhost:8080/api/conversations?platform=telegram' | jq '. | length'
curl -s 'http://localhost:8080/api/conversations?platform=' | jq '. | length'
```

Skip if local server not running; the unit-test layer already covered the DB path in Task 3.

- [ ] **Step 4: Commit**

```bash
git add internal/handler/conversation.go
git commit -m "handler: ?platform= filter on /api/conversations

Empty platform query (or absent) → existing ListConversations
behavior (all platforms). Non-empty → ListConversationsByPlatform
from Task 3.

Frontend filter dropdown lands in Tasks 11-12.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 11: UI filter dropdown + per-card badge

**Files:**
- Modify: `web/templates/index.html` — dropdown markup
- Modify: `web/static/js/chat.js` (or wherever conversation list renders) — fetch + render
- Modify: `web/static/css/style.css` — badge styles
- Modify: `web/static/i18n/en-US.json`, `uk-UA.json` — keys

- [ ] **Step 1: Locate the conversation list rendering**

```
cd /home/badb/CyberStrikeAI && grep -n "/api/conversations\|loadConversations\|conversation-list" web/static/js/*.js web/templates/index.html | head -15
```

Find the function that fetches `/api/conversations` and the DOM element that holds the list.

- [ ] **Step 2: Add the filter dropdown to `index.html`**

Above the conversation list element (whatever ID it has — `conversation-list`, `sidebar-conversations`, etc.), insert:

```html
<select id="conversation-platform-filter" class="filter-dropdown">
    <option value="" data-i18n="conversations.filterAll">All</option>
    <option value="" data-i18n="conversations.filterWeb">Web</option>
    <option value="telegram" data-i18n="conversations.filterTelegram">Telegram</option>
</select>
```

Note: the "Web" option needs special handling because empty platform = all in our backend. Use a separate sentinel:

```html
<select id="conversation-platform-filter" class="filter-dropdown">
    <option value="" data-i18n="conversations.filterAll">All</option>
    <option value="__web__" data-i18n="conversations.filterWeb">Web</option>
    <option value="telegram" data-i18n="conversations.filterTelegram">Telegram</option>
</select>
```

The JS then maps `__web__` → no platform-aware backend call but a client-side filter on `platform == null`. Easier alternative: extend the backend to accept `platform=null` literal.

**Choose**: extend backend. In `conversation.go` handler from Task 10, treat `platform == "null"` (string literal) as "WHERE platform IS NULL". Same fix in `database/conversation.go:ListConversationsByPlatform` (already handles empty == NULL; need to differentiate "null" sentinel vs empty).

Cleaner alternative: backend only knows about `?platform=` (empty = all platforms). Client maps the dropdown value:
- "" → no query param → all
- "__web__" → `?platform_is_null=true` (new param) — gross
- "telegram" → `?platform=telegram`

Simplest: add `?platform=__web__` and have the handler treat it as "WHERE platform IS NULL". Document the sentinel. Wireguard against the unlikely case that "__web__" becomes a real platform name.

For Task 11, go with backend sentinel. Update the handler from Task 10:

```go
	platform := c.Query("platform")
	switch platform {
	case "":
		conversations, err = h.db.ListConversations(limit, offset, search)
	case "__web__":
		conversations, err = h.db.ListConversationsByPlatform(limit, offset, search, "")  // empty = NULL match
	default:
		conversations, err = h.db.ListConversationsByPlatform(limit, offset, search, platform)
	}
```

- [ ] **Step 3: Add JS for filter change**

In whatever JS file owns `loadConversations`:

```javascript
const filterEl = document.getElementById('conversation-platform-filter');
if (filterEl) {
    filterEl.addEventListener('change', () => loadConversations());
}

// Inside loadConversations(), build the URL:
async function loadConversations() {
    const filter = (document.getElementById('conversation-platform-filter') || {}).value || '';
    const url = filter ? `/api/conversations?platform=${encodeURIComponent(filter)}` : '/api/conversations';
    const r = await fetch(url, { credentials: 'same-origin' });
    const list = await r.json();
    renderConversationList(list);
}
```

Adapt to the actual existing function shape — wrap whatever already builds the URL.

- [ ] **Step 4: Add per-card badge**

In `renderConversationList` (or wherever each conversation row's HTML is built), add the badge HTML when `conversation.platform` is non-null:

```javascript
const badge = c.platform
    ? `<span class="platform-badge platform-${c.platform}">🤖 ${c.platform}</span>`
    : '';
// Insert badge into the card template.
```

The exact insertion point depends on the existing template. The card already has a title + timestamp; insert near the title.

- [ ] **Step 5: CSS**

Append to `web/static/css/style.css`:

```css
.filter-dropdown {
    margin: 4px 0 8px 0;
    padding: 4px 6px;
    font-size: 0.85em;
    border: 1px solid var(--border-color, #ddd);
    background: var(--panel-bg, #fff);
}

.platform-badge {
    display: inline-block;
    padding: 1px 6px;
    margin-left: 6px;
    font-size: 0.7em;
    border-radius: 8px;
    background: rgba(33, 150, 243, 0.15);
    color: #1976d2;
    vertical-align: middle;
}
```

- [ ] **Step 6: i18n keys**

Append to `web/static/i18n/en-US.json` under the appropriate section (likely an existing `conversations` namespace; if absent, add):

```json
"conversations": {
  "filterAll": "All",
  "filterWeb": "Web",
  "filterTelegram": "Telegram"
}
```

And in `uk-UA.json`:

```json
"conversations": {
  "filterAll": "Усі",
  "filterWeb": "Веб",
  "filterTelegram": "Telegram"
}
```

- [ ] **Step 7: Manual smoke**

Run server. Open the UI. Send a message via Telegram → bot conversation appears. Open the conversation list:
- Default view: bot conversation shows up with 🤖 badge.
- Filter "Telegram": only bot conversations.
- Filter "Web": only web conversations (no badge).

- [ ] **Step 8: Commit**

```bash
git add web/templates/index.html web/static/js/chat.js web/static/css/style.css web/static/i18n/
git commit -m "ui: conversation list filter dropdown + per-card platform badge

Filter dropdown above the conversation list (All / Web / Telegram).
Web is signaled to backend via __web__ sentinel — handler at
/api/conversations maps to ListConversationsByPlatform with empty
platform string (matches NULL).

Per-card badge renders 🤖 telegram for non-NULL platforms; uses
existing CSS variables with a fallback so badge survives even if
theme variables aren't defined.

i18n keys live under conversations.filterAll / filterWeb /
filterTelegram in both en-US and uk-UA.

Closes audit-finding gap 9 — operators can now distinguish bot
traffic at a glance and filter for it.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 12: Final integration gate + manual smoke

**Files:**
- No code changes; verification only

- [ ] **Step 1: Toolchain gate**

```
cd /home/badb/CyberStrikeAI
go build ./...
go vet ./...
go test -race ./...
```

Expected: clean modulo the preexisting `internal/security/TestExecutor_NormalizeToolArgs_RepairsMalformedHTTPFrameworkArgs` failure (unrelated, fails on main throughout the branch).

- [ ] **Step 2: Manual smoke checklist**

Operator-run sequence (report results in commit body):

1. **Boot fresh.** Set `bot.telegram.enabled: true` + token. First message creates `bot_sessions` row + `conversations.platform="telegram"`. Verify `sqlite3 chat.db "SELECT * FROM bot_sessions"`.

2. **Mode toggle.** Send `mode multi` → reply confirms switch. Next message uses orchestrator (verify in Settings → Debug viewer: agentRole=cyberstrike-orchestrator events). `mode single` switches back. `mode default` reverts.

3. **Server restart.** Note current conversation_id from `mode` (no arg) reply or DB. Restart server. Send another message — same conversation continues; mode override preserved.

4. **Conversation deleted from UI.** Operator deletes the bot conversation in the web UI. User sends another message → bot creates new conversation; bot_sessions row's `current_mode` survives (verify with SQL).

5. **Concurrent messages.** Fire two messages back-to-back. Second gets the "Another task is running" reply. Run `stop` → first cancels → second can be retried.

6. **Filter + badge.** Web UI conversation list dropdown filters correctly; bot conversation has 🤖 telegram badge.

7. **Settings → Debug.** Bot conversation appears in the session list with iterations / token counts / duration. Click row → viewer shows merged event timeline incl. tool_call / tool_result with the bot-side step strings used for Telegram edits. Export raw + ShareGPT both work.

8. **Provider switch to Claude CLI.** Toggle in Settings → Provider (assumes prereqs green). Bot's next message uses claude-cli adapter — single "🤔 Running through Claude CLI…" then final answer. Settings → Debug shows the session.

9. **Streaming progress visible.** Long agent run with multiple tool calls → placeholder edits at each major event with ≥3 s spacing. Final answer replaces placeholder.

- [ ] **Step 3: Commit smoke results (optional, only if any drift surfaced)**

If any smoke item failed, file a follow-up task and create a fix commit. If all passed, no commit needed — Task 12 is verification only.

```bash
# only if a fix is needed:
git add ...
git commit -m "bot parity: post-smoke fix for X"
```

---

## Self-review checklist

- [ ] Every task has exact file paths, complete code blocks (no "TODO" / "similar to X" / "write tests for the above").
- [ ] Every task ends in a commit step (or is verification-only and noted).
- [ ] Type names + method signatures consistent across tasks:
  - `BotSession`, `GetBotSession`, `UpsertBotSession`, `ClearBotSession`, `SetBotMode` (Task 2, used in Tasks 8-9).
  - `CreateConversationWithPlatform`, `ListConversationsByPlatform` (Task 3, used in Tasks 8 + 10).
  - `ProcessMessageForRobot(ctx, conversationID, message, role, forceMode string, progressFn func(step string))` returning `(response, convID string, err error)` (Task 4 sig, used in Tasks 5-9).
  - `MajorEventStep(eventType, message string, data map[string]interface{}) string` (Task 6, used in Task 7).
  - `RobotHandler.HandleMessageStream` (Task 8, satisfies `robot.StreamingMessageHandler`).
  - `cmdMode(platform, userID, arg string) string` (Task 9).
- [ ] Spec coverage:
  - Gap 1 (claude-cli routing) → Task 5.
  - Gap 2 (debug bookends) → Task 4.
  - Gap 3 (StreamingMessageHandler) → Task 8.
  - Gap 5 (persisted sessions) → Tasks 1, 2, 8.
  - Gap 6 (per-chat mode override) → Tasks 1, 2, 9.
  - Gap 9 (platform tag + filter) → Tasks 1, 3, 10, 11.
  - 4i (concurrent message lock) → Task 9.
- [ ] No undefined types referenced. (`Conversation` is from `internal/database`; verify field names match.)

## Execution handoff

Plan complete and saved to `docs/superpowers/plans/2026-04-26-telegram-bot-parity.md`. Two execution options:

**1. Subagent-Driven (recommended)** — fresh subagent per task, review between tasks, fast iteration.

**2. Inline Execution** — execute in this session via `superpowers:executing-plans`, batched checkpoints.
