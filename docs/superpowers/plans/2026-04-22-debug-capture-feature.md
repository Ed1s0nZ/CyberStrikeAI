# Debug Capture + Training-Data Export — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship an opt-in debug subsystem that captures verbatim LLM requests/responses, token usage, fine-grained timestamps, and the full SSE event stream into three sidecar SQLite tables; expose it as a post-hoc Settings → Debug tab with per-session raw JSONL and ShareGPT JSONL export.

**Architecture:** New `internal/debug/` package with a `Sink` interface (noopSink / dbSink implementations). Sink is wired through Agent and Orchestrator constructors; call sites invoke it unconditionally and noopSink returns early when off. Three new SQLite tables (`debug_sessions`, `debug_llm_calls`, `debug_events`) captured at Agent's LLM wrapper, Orchestrator's `sendProgress`, and Handler's session start/end. Six HTTP routes and one Settings tab consume the captures.

**Tech Stack:** Go 1.22, gin, SQLite (`modernc.org/sqlite`), zap, vanilla JS SPA.

## Spec reference

Spec: `docs/superpowers/specs/2026-04-22-debug-capture-feature-design.md` (commits `9e757cd` + `8b31179` + `RecordToolCall-drop`).

## Deviations from spec

1. **`Sink.RecordToolCall` dropped.** Tool dispatch is already covered by the existing `tool_executions` table plus the `tool_call`/`tool_result` events via `RecordEvent`. Spec was updated to match.
2. **Token usage may be NULL for streaming calls.** The project's streaming OpenAI path (`callOpenAIStreamWithToolCalls` in `internal/openai/openai.go:333`) does not parse `stream_options.include_usage`; Anthropic backend does (`internal/openai/anthropic.go:429-431`). v1 records tokens as NULLable — present when the client backend emits them, absent otherwise. Plan does not add a tokenizer dependency.

## File map

**Create (new files):**
- `internal/debug/sink.go` — interface + noopSink + dbSink skeleton
- `internal/debug/capture.go` — `LLMCall`, `Event` value types
- `internal/debug/session.go` — StartSession/EndSession/SweepOrphans
- `internal/debug/converter.go` — ShareGPT pure function
- `internal/debug/export.go` — Raw/ShareGPT/Bulk streaming writers
- `internal/debug/retention.go` — periodic pruner goroutine
- `internal/debug/sink_test.go`
- `internal/debug/converter_test.go`
- `internal/debug/export_test.go`
- `internal/debug/retention_test.go`
- `internal/handler/debug.go` — HTTP handlers
- `internal/handler/debug_test.go`

**Modify (existing files):**
- `internal/config/config.go` — add `DebugConfig` struct + `Debug` field on `Config`
- `config.example.yaml` — add `debug:` stanza
- `internal/database/database.go` — add three `CREATE TABLE IF NOT EXISTS`
- `cmd/server/main.go` — construct Sink, pass through
- `internal/app/app.go` — thread sink to agent + orchestrator + handler, register routes
- `internal/agent/agent.go` — accept sink, wrap LLM calls
- `internal/multiagent/orchestrator.go` — accept sink, tee `sendProgress`
- `internal/handler/agent.go` — `sink.StartSession` / `sink.EndSession`
- `web/templates/index.html` — Settings → Debug tab
- `web/static/js/settings.js` — Debug tab JS
- `web/static/css/style.css` — Debug tab styles (or inline via `<style>` in the tab)
- `web/static/i18n/en-US.json`, `web/static/i18n/uk-UA.json` — Debug tab strings

## Ordering rationale

Tasks 1–4 establish the plumbing skeleton (config, schema, sink interface, constructor threading) without changing any runtime behavior. Tasks 5–9 fill in dbSink semantics, tested in isolation against in-memory SQLite. Tasks 10–11 build the converter/exporters as pure functions. Tasks 12–14 wire the hook points. Tasks 15–19 expose HTTP routes. Tasks 20–22 build the UI. Task 23 is the final integration gate.

Each task ends in a commit. Task 4 is the first commit that requires reviewing an end-to-end code path (plumbing through Agent + Orchestrator), so that's where the first checkpoint lives.

---

## Task 1: Add DebugConfig struct and example YAML

**Files:**
- Modify: `internal/config/config.go`
- Modify: `config.example.yaml`
- Test: `internal/config/config_test.go` (existing file)

- [ ] **Step 1: Write failing test for YAML unmarshalling of DebugConfig**

Open `internal/config/config_test.go` and append:

```go
func TestLoad_DebugConfig(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.yaml")
	yaml := `
openai:
  api_key: test
  base_url: https://example.invalid
  model: test
debug:
  enabled: true
  retain_days: 7
`
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.Debug.Enabled {
		t.Fatalf("Debug.Enabled: want true, got false")
	}
	if cfg.Debug.RetainDays != 7 {
		t.Fatalf("Debug.RetainDays: want 7, got %d", cfg.Debug.RetainDays)
	}
}
```

If `filepath` / `os` are not already imported in this file, add them.

- [ ] **Step 2: Run test to verify it fails**

```
go test ./internal/config/ -run TestLoad_DebugConfig
```

Expected: FAIL with `cfg.Debug undefined` (struct field doesn't exist yet).

- [ ] **Step 3: Add DebugConfig struct and Config.Debug field**

In `internal/config/config.go`, find the `LogConfig` struct (around the top of the file) and add a sibling below it:

```go
type DebugConfig struct {
	Enabled    bool `yaml:"enabled"`
	RetainDays int  `yaml:"retain_days"`
}
```

Then find the `Config` struct (the top-level one) and add a field:

```go
Debug DebugConfig `yaml:"debug"`
```

- [ ] **Step 4: Run test to verify it passes**

```
go test ./internal/config/ -run TestLoad_DebugConfig -v
```

Expected: PASS.

- [ ] **Step 5: Add debug stanza to config.example.yaml**

Append to `config.example.yaml` (after the existing `log:` stanza):

```yaml
debug:
  enabled: false       # flip via Settings UI at runtime; this file value is the boot default
  retain_days: 0       # auto-prune debug sessions older than N days; 0 = keep forever
```

- [ ] **Step 6: go vet + full config test**

```
go vet ./internal/config/...
go test ./internal/config/...
```

Expected: both pass.

- [ ] **Step 7: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go config.example.yaml
git commit -m "config: add DebugConfig (enabled, retain_days)

Seed config field for the debug capture subsystem. Values are the
boot defaults; runtime toggle will live on an atomic.Bool owned by
the Sink implementation so it can flip without restart. retain_days=0
means keep forever.

Spec: docs/superpowers/specs/2026-04-22-debug-capture-feature-design.md

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 2: Add debug_* tables to database schema

**Files:**
- Modify: `internal/database/database.go`
- Test: `internal/database/database_test.go` (create if missing)

- [ ] **Step 1: Find the existing CREATE TABLE IF NOT EXISTS block**

Open `internal/database/database.go` and locate the init/migration function that runs the `CREATE TABLE IF NOT EXISTS conversations (...)` statements (around line 62 per the recon). All three new tables go in the same function, after the last existing `CREATE INDEX`.

- [ ] **Step 2: Write failing test for the migrations**

Create `internal/database/debug_schema_test.go`:

```go
package database

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestInit_CreatesDebugTables(t *testing.T) {
	tmp := t.TempDir() + "/test.db"
	db, err := sql.Open("sqlite", tmp)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	// The package's init/migration entrypoint (name depends on the
	// package — if it's NewDatabase or Init, adapt this call). For
	// this plan, assume a helper `runMigrations(db)` exists or is
	// reachable via NewDatabase path. If the existing boot is
	// `d, err := database.New(path)`, switch this test to that.
	d := &Database{db: db}
	if err := d.runMigrations(); err != nil {
		t.Fatalf("runMigrations: %v", err)
	}

	for _, table := range []string{"debug_sessions", "debug_llm_calls", "debug_events"} {
		var name string
		err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err != nil {
			t.Fatalf("table %s missing: %v", table, err)
		}
	}
	for _, idx := range []string{"idx_debug_llm_calls_conv", "idx_debug_events_conv"} {
		var name string
		err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='index' AND name=?", idx).Scan(&name)
		if err != nil {
			t.Fatalf("index %s missing: %v", idx, err)
		}
	}
}
```

Note: the concrete boot symbol names (`Database`, `runMigrations`, `NewDatabase`) depend on what's already in the package. Before Step 3, read `internal/database/database.go` head to find the actual entrypoint and adjust the test accordingly.

- [ ] **Step 3: Run test to verify it fails**

```
go test ./internal/database/ -run TestInit_CreatesDebugTables
```

Expected: FAIL (tables don't exist yet, or method name mismatch — in the latter case, adjust the test to match the actual method name first).

- [ ] **Step 4: Add the three CREATE TABLE IF NOT EXISTS statements**

In `internal/database/database.go`, append these to the migration runner:

```go
const createDebugSessionsTable = `
CREATE TABLE IF NOT EXISTS debug_sessions (
    conversation_id TEXT PRIMARY KEY,
    started_at INTEGER NOT NULL,
    ended_at   INTEGER,
    outcome    TEXT,
    label      TEXT
);`

const createDebugLLMCallsTable = `
CREATE TABLE IF NOT EXISTS debug_llm_calls (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    conversation_id   TEXT NOT NULL,
    message_id        TEXT,
    iteration         INTEGER,
    call_index        INTEGER,
    agent_id          TEXT,
    sent_at           INTEGER NOT NULL,
    first_token_at    INTEGER,
    finished_at       INTEGER,
    prompt_tokens     INTEGER,
    completion_tokens INTEGER,
    request_json      TEXT NOT NULL,
    response_json     TEXT NOT NULL,
    error             TEXT
);`

const createDebugLLMCallsIndex = `
CREATE INDEX IF NOT EXISTS idx_debug_llm_calls_conv
  ON debug_llm_calls(conversation_id, sent_at);`

const createDebugEventsTable = `
CREATE TABLE IF NOT EXISTS debug_events (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    conversation_id TEXT NOT NULL,
    message_id      TEXT,
    seq             INTEGER NOT NULL,
    event_type      TEXT NOT NULL,
    agent_id        TEXT,
    payload_json    TEXT NOT NULL,
    started_at      INTEGER NOT NULL,
    finished_at     INTEGER
);`

const createDebugEventsIndex = `
CREATE INDEX IF NOT EXISTS idx_debug_events_conv
  ON debug_events(conversation_id, seq);`
```

Then in the migration function, add to the statement list (after the existing last CREATE):

```go
	{"debug_sessions",          createDebugSessionsTable},
	{"debug_llm_calls",         createDebugLLMCallsTable},
	{"debug_llm_calls_index",   createDebugLLMCallsIndex},
	{"debug_events",            createDebugEventsTable},
	{"debug_events_index",      createDebugEventsIndex},
```

(Adapt to the existing migration loop shape — it may be a slice of `string`, a slice of `struct{name,sql string}`, or a sequence of `db.Exec` calls. Match whatever pattern is there.)

- [ ] **Step 5: Run test to verify it passes**

```
go test ./internal/database/ -run TestInit_CreatesDebugTables -v
```

Expected: PASS.

- [ ] **Step 6: Full database test**

```
go test ./internal/database/
```

Expected: pass (no regressions).

- [ ] **Step 7: Commit**

```bash
git add internal/database/
git commit -m "database: add debug_sessions / debug_llm_calls / debug_events

Three sidecar tables for the opt-in debug capture subsystem. Schema
frozen per spec. Indexes are on (conversation_id, sent_at) for the
LLM-calls list queries and (conversation_id, seq) for the event-
stream order.

Tables are created unconditionally at boot regardless of
debug.enabled, so GET /api/debug/sessions always has a real empty
table to hit instead of 404-ing when the feature has never been on.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 3: Create `internal/debug/` package skeleton

**Files:**
- Create: `internal/debug/sink.go`
- Create: `internal/debug/capture.go`
- Create: `internal/debug/sink_test.go`

- [ ] **Step 1: Write failing test for Sink interface contract**

Create `internal/debug/sink_test.go`:

```go
package debug

import (
	"testing"

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
```

- [ ] **Step 2: Run test to verify it fails**

```
go test ./internal/debug/
```

Expected: FAIL — package doesn't exist.

- [ ] **Step 3: Create `internal/debug/capture.go`**

```go
package debug

// LLMCall is one LLM round-trip recorded to debug_llm_calls.
// Zero-valued fields serialize to NULL columns (first_token_at,
// prompt_tokens, completion_tokens) for backends that don't report
// them — see the plan's "Deviations from spec" note on streaming
// token usage.
type LLMCall struct {
	Iteration        int
	CallIndex        int
	AgentID          string
	SentAt           int64 // unix nanos
	FirstTokenAt     int64 // 0 means unknown
	FinishedAt       int64
	PromptTokens     int64 // 0 means unknown
	CompletionTokens int64 // 0 means unknown
	RequestJSON      string
	ResponseJSON     string
	Error            string
}

// Event is one orchestrator/agent progress event recorded to
// debug_events. Seq is assigned by the sink, not the caller.
type Event struct {
	EventType   string
	AgentID     string
	PayloadJSON string
	StartedAt   int64
	FinishedAt  int64 // 0 means instant / no duration
}
```

- [ ] **Step 4: Create `internal/debug/sink.go`**

```go
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

func (noopSink) StartSession(string)                             {}
func (noopSink) EndSession(string, string)                       {}
func (noopSink) RecordLLMCall(string, string, LLMCall)           {}
func (noopSink) RecordEvent(string, string, Event)               {}
func (noopSink) SetEnabled(bool)                                 {}
func (noopSink) Enabled() bool                                   { return false }

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

// Record*/Start*/End* bodies are filled in Tasks 4-7.
func (s *dbSink) StartSession(conversationID string)                           {}
func (s *dbSink) EndSession(conversationID, outcome string)                    {}
func (s *dbSink) RecordLLMCall(conversationID, messageID string, c LLMCall)    {}
func (s *dbSink) RecordEvent(conversationID, messageID string, e Event)        {}
```

- [ ] **Step 5: Run test to verify it passes**

```
go test ./internal/debug/ -run TestNoopSink_Disabled -v
go vet ./internal/debug/
```

Expected: PASS + no vet issues.

- [ ] **Step 6: Commit**

```bash
git add internal/debug/sink.go internal/debug/capture.go internal/debug/sink_test.go
git commit -m "debug: Sink interface + noopSink + dbSink skeleton

Package skeleton for the opt-in debug capture subsystem. Sink is the
single interface the rest of the codebase calls unconditionally; off-
state noopSink returns immediately, on-state dbSink persists to
SQLite (bodies land in Tasks 4-7). Enabled state is an atomic.Bool
on dbSink, flipped at runtime by the Settings toggle endpoint.

Error handling policy: DB write failures are logged at warn and
swallowed — never propagated to the conversation path.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 4: Thread Sink through Agent and Orchestrator constructors (still noopSink — behavior unchanged)

**Files:**
- Modify: `internal/agent/agent.go` (around `NewAgent`, line ~55)
- Modify: `internal/multiagent/orchestrator.go` (around `RunOrchestrator`, line 74; and `orchestratorState`, line 208)
- Modify: `internal/handler/handlers.go` or wherever `AgentHandler` is constructed
- Modify: `cmd/server/main.go` (construct sink, pass to agent + handler)
- Modify: `internal/app/app.go` (thread sink to handler/agent/orchestrator wiring)

This task does NOT change any runtime behavior — it only adds a `debug.Sink` parameter to the constructors and plumbs a noopSink through. After this task, `go build` + all existing tests must still pass.

- [ ] **Step 1: Read current wiring to confirm exact call sites**

```
grep -n "agent.NewAgent\|NewAgentHandler\|RunOrchestrator\|&h.config.MultiAgent" internal/app/app.go cmd/server/main.go internal/handler/*.go | head -30
```

Note the concrete call sites. They will be modified in Step 4.

- [ ] **Step 2: Add sink field to Agent struct**

In `internal/agent/agent.go`, find the `Agent` struct (around L25) and add a field:

```go
	debugSink debug.Sink
```

Add the import:

```go
	"cyberstrike-ai/internal/debug"
```

Find `NewAgent` (around L56) and add `sink debug.Sink` as the last parameter, assigning it:

```go
func NewAgent(cfg *config.OpenAIConfig, agentCfg *config.AgentConfig, mcpServer *mcp.Server, externalMCPMgr *mcp.ExternalMCPManager, logger *zap.Logger, maxIterations int, sink debug.Sink) *Agent {
	// ... existing body ...
	return &Agent{
		// ... existing fields ...
		debugSink: sink,
	}
}
```

If `sink` is nil, fall through to a noop:

```go
	if sink == nil {
		sink = debug.NewSink(false, nil, logger)
	}
```

Place this right after the `maxIterations` defaulting block.

- [ ] **Step 3: Add sink field to orchestratorState**

In `internal/multiagent/orchestrator.go`, find the `orchestratorState` struct (L208) and add:

```go
	sink debug.Sink
```

Add the import:

```go
	"cyberstrike-ai/internal/debug"
```

Find `RunOrchestrator` (L74) and add `sink debug.Sink` as the final parameter:

```go
func RunOrchestrator(
	ctx context.Context,
	appCfg *config.Config,
	ma *config.MultiAgentConfig,
	ag *agent.Agent,
	logger *zap.Logger,
	conversationID string,
	userMessage string,
	history []agent.ChatMessage,
	roleTools []string,
	progress func(eventType, message string, data interface{}),
	agentsMarkdownDir string,
	sink debug.Sink,
) (*RunResult, error) {
```

In the `o := &orchestratorState{...}` block (around L192), add `sink: sink`. If sink is nil, substitute a noop the same way Agent does.

- [ ] **Step 4: Update all call sites of NewAgent and RunOrchestrator**

Search:

```
grep -rn "agent.NewAgent\|multiagent.RunOrchestrator" --include='*.go' /home/badb/CyberStrikeAI
```

Every call site of `agent.NewAgent` needs a trailing sink argument; every call of `multiagent.RunOrchestrator` needs a trailing sink argument. For this task, all callers pass the AgentHandler's sink field; AgentHandler gets constructed with a sink that comes from main.go.

- [ ] **Step 5: Add sink to AgentHandler**

Locate the `AgentHandler` struct (likely in `internal/handler/agent.go` or `internal/handler/handlers.go`) and add:

```go
	debugSink debug.Sink
```

Add to its constructor signature and call sites. The handler's `MultiAgentLoopStream` / `MultiAgentLoop` methods already pass various things to `RunOrchestrator`; adapt to also pass `h.debugSink`.

- [ ] **Step 6: Wire sink in cmd/server/main.go**

After `database.Init(...)` (or the existing DB setup) and before constructing the agent/handler, add:

```go
import (
	"cyberstrike-ai/internal/debug"
)

sink := debug.NewSink(cfg.Debug.Enabled, db, log)
```

Pass `sink` into the agent constructor and the handler constructor (and down to app.New if that's how routes are wired).

- [ ] **Step 7: go build + full test suite**

```
go build ./...
go vet ./...
go test ./internal/agent/ ./internal/multiagent/ ./internal/handler/
```

Expected: all pass. No behavior change — noopSink is invoked everywhere but doesn't write.

- [ ] **Step 8: Commit**

```bash
git add internal/agent/agent.go internal/multiagent/orchestrator.go internal/handler/ cmd/server/main.go internal/app/app.go
git commit -m "debug: thread Sink through Agent + Orchestrator + Handler

Plumbing-only change: Sink parameter added to NewAgent, RunOrchestrator,
and AgentHandler constructor; a noopSink is instantiated in
cmd/server/main.go from cfg.Debug.Enabled and passed through. No
runtime behavior change — this commit just makes the sink reachable
from every call site so Tasks 5-7 can fill in dbSink methods and
Tasks 12-14 can add the actual capture calls.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 5: Implement `dbSink.StartSession` and `dbSink.EndSession`

**Files:**
- Modify: `internal/debug/sink.go`
- Create: `internal/debug/session.go`
- Modify: `internal/debug/sink_test.go`

- [ ] **Step 1: Write failing test**

Append to `internal/debug/sink_test.go`:

```go
import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// openTestDB opens an in-memory SQLite and runs the debug-table
// DDL so tests don't depend on the database package.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "debug_test.db"))
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
```

- [ ] **Step 2: Run test to verify it fails**

```
go test ./internal/debug/ -run TestDBSink_StartEndSession_HappyPath -v
```

Expected: FAIL — StartSession / EndSession are still no-op stubs.

- [ ] **Step 3: Create `internal/debug/session.go`**

```go
package debug

import (
	"time"

	"go.uber.org/zap"
)

func (s *dbSink) StartSession(conversationID string) {
	if !s.enabled.Load() {
		return
	}
	now := time.Now().UnixNano()
	// INSERT OR REPLACE so re-enabling debug on a conversation that
	// already has a row resets the session timer. Spec: one session
	// per conversation in v1.
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
}
```

Delete the empty stubs for `StartSession` / `EndSession` from `sink.go`.

- [ ] **Step 4: Run test to verify it passes**

```
go test ./internal/debug/ -run TestDBSink_StartEndSession_HappyPath -v
go vet ./internal/debug/
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/debug/session.go internal/debug/sink.go internal/debug/sink_test.go
git commit -m "debug: implement StartSession/EndSession

First dbSink body. StartSession writes a NULL-ended row (resilient to
re-enable: INSERT OR REPLACE preserves any prior user-set label via
subselect). EndSession fills ended_at + outcome only if the row is
still live, so a double EndSession call doesn't overwrite the
initial outcome.

Test uses in-memory SQLite with the DDL replicated inline — keeps the
debug package dependency-free from internal/database so the unit
tests stay fast and don't require running the full migration loop.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 6: Implement `dbSink.RecordLLMCall`

**Files:**
- Modify: `internal/debug/sink.go`
- Modify: `internal/debug/sink_test.go`

- [ ] **Step 1: Write failing test**

Append to `internal/debug/sink_test.go`:

```go
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

	var row struct {
		iter     int64
		callIdx  int64
		agent    string
		sentAt   int64
		firstTok int64
		finAt    int64
		promptT  int64
		complT   int64
		req      string
		resp     string
		errStr   sql.NullString
	}
	err := db.QueryRow(`
		SELECT iteration, call_index, agent_id, sent_at, first_token_at,
		       finished_at, prompt_tokens, completion_tokens,
		       request_json, response_json, error
		FROM debug_llm_calls WHERE conversation_id = ? AND message_id = ?`,
		"conv-1", "msg-1").Scan(
		&row.iter, &row.callIdx, &row.agent, &row.sentAt, &row.firstTok,
		&row.finAt, &row.promptT, &row.complT, &row.req, &row.resp, &row.errStr,
	)
	if err != nil {
		t.Fatalf("QueryRow: %v", err)
	}
	if row.iter != 2 || row.callIdx != 5 || row.agent != "cyberstrike-orchestrator" {
		t.Fatalf("metadata mismatch: got %+v", row)
	}
	if row.promptT != 42 || row.complT != 13 {
		t.Fatalf("token counts mismatch")
	}
	if row.req != call.RequestJSON || row.resp != call.ResponseJSON {
		t.Fatalf("JSON payload mismatch")
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
```

- [ ] **Step 2: Run test to verify it fails**

```
go test ./internal/debug/ -run TestDBSink_RecordLLMCall -v
```

Expected: FAIL — RecordLLMCall is still a no-op stub.

- [ ] **Step 3: Implement RecordLLMCall**

In `internal/debug/sink.go`, replace the empty `RecordLLMCall` stub body with:

```go
func (s *dbSink) RecordLLMCall(conversationID, messageID string, c LLMCall) {
	if !s.enabled.Load() {
		return
	}
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

// nullableStr returns nil for empty strings so they are stored as NULL
// instead of "" — keeps SQL IS NULL filters correct on the read side.
func nullableStr(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
```

Add to the import block if missing:

```go
	"go.uber.org/zap"
```

- [ ] **Step 4: Run test to verify it passes**

```
go test ./internal/debug/ -v
```

Expected: both RecordLLMCall tests PASS, prior tests still PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/debug/sink.go internal/debug/sink_test.go
git commit -m "debug: implement RecordLLMCall with nullable token columns

RecordLLMCall INSERTs one row per LLM round-trip. Zero-valued optional
fields (FirstTokenAt, FinishedAt, PromptTokens, CompletionTokens,
Error) are written as SQL NULL instead of zero so read-side queries
like AVG(completion_tokens) WHERE completion_tokens IS NOT NULL
behave correctly for backends that don't report tokens (see the plan
deviation note on streaming usage).

Helper nullableStr keeps empty-string NOT-NULL strings out of the
table too — only for optional columns (message_id, error).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 7: Implement `dbSink.RecordEvent` with monotonic per-conversation seq

**Files:**
- Modify: `internal/debug/sink.go`
- Modify: `internal/debug/sink_test.go`

- [ ] **Step 1: Write failing tests — basic + concurrent**

Append to `internal/debug/sink_test.go`:

```go
import (
	"sync"
	"sync/atomic"
)

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
		t.Fatalf("metadata mismatch")
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
		var s int64
		_ = rows.Scan(&s)
		seen = append(seen, s)
	}
	if len(seen) != N {
		t.Fatalf("want %d rows, got %d", N, len(seen))
	}
	for i, v := range seen {
		if v != int64(i) {
			t.Fatalf("seq[%d]: want %d, got %d", i, i, v)
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
		rows, _ := db.Query(`SELECT seq FROM debug_events WHERE conversation_id = ? ORDER BY seq`, conv)
		var seqs []int64
		for rows.Next() {
			var v int64
			_ = rows.Scan(&v)
			seqs = append(seqs, v)
		}
		rows.Close()
		if len(seqs) != 3 || seqs[0] != 0 || seqs[1] != 1 || seqs[2] != 2 {
			t.Fatalf("%s: want {0,1,2}, got %v", conv, seqs)
		}
	}
}

var _ = atomic.Int64{} // keep the import if only this test file uses it
```

- [ ] **Step 2: Run tests to verify they fail**

```
go test ./internal/debug/ -run TestDBSink_RecordEvent -v
```

Expected: FAIL — `RecordEvent` body is still an empty stub.

- [ ] **Step 3: Implement `RecordEvent` + `nextSeq`**

In `internal/debug/sink.go`, replace the empty `RecordEvent` stub with:

```go
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

// nextSeq returns the next 0-based monotonic sequence for a conversation.
// sync.Map<string, *atomic.Int64>; LoadOrStore races are harmless
// (the loser's counter is discarded before any Add).
func (s *dbSink) nextSeq(conversationID string) int64 {
	v, _ := s.seqByConv.LoadOrStore(conversationID, new(atomic.Int64))
	return v.(*atomic.Int64).Add(1) - 1
}
```

- [ ] **Step 4: Run tests to verify pass under -race**

```
go test ./internal/debug/ -v -race
```

Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/debug/sink.go internal/debug/sink_test.go
git commit -m "debug: implement RecordEvent with per-conversation monotonic seq

sync.Map<conversationID, *atomic.Int64> fast-path. nextSeq returns
Add(1)-1 so the first event is seq=0, matching converter/export
expectations. -race coverage: 50 concurrent goroutines on one
conversation produce seqs {0..49} with zero dups and zero gaps.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 8: `SweepOrphans` boot helper + SetEnabled gating test

**Files:**
- Modify: `internal/debug/session.go`
- Modify: `internal/debug/sink_test.go`

- [ ] **Step 1: Write failing tests**

Append to `internal/debug/sink_test.go`:

```go
func TestDBSink_SetEnabled_GatesWrites(t *testing.T) {
	db := openTestDB(t)
	s := NewSink(true, db, nil)
	s.RecordEvent("conv-1", "", Event{EventType: "a", PayloadJSON: "{}", StartedAt: 1})
	s.SetEnabled(false)
	s.RecordEvent("conv-1", "", Event{EventType: "b", PayloadJSON: "{}", StartedAt: 2})
	s.SetEnabled(true)
	s.RecordEvent("conv-1", "", Event{EventType: "c", PayloadJSON: "{}", StartedAt: 3})

	rows, _ := db.Query(`SELECT event_type FROM debug_events WHERE conversation_id = ? ORDER BY id`, "conv-1")
	defer rows.Close()
	var types []string
	for rows.Next() {
		var ty string
		_ = rows.Scan(&ty)
		types = append(types, ty)
	}
	if len(types) != 2 || types[0] != "a" || types[1] != "c" {
		t.Fatalf("want [a c], got %v", types)
	}
}

func TestSweepOrphans_MarksLiveRowsAsInterrupted(t *testing.T) {
	db := openTestDB(t)
	mustExec := func(q string, args ...interface{}) {
		if _, err := db.Exec(q, args...); err != nil {
			t.Fatalf("exec %q: %v", q, err)
		}
	}
	mustExec(`INSERT INTO debug_sessions (conversation_id, started_at) VALUES ('orphan', 100)`)
	mustExec(`INSERT INTO debug_sessions (conversation_id, started_at, ended_at, outcome) VALUES ('done', 50, 200, 'completed')`)
	mustExec(`INSERT INTO debug_events (conversation_id, seq, event_type, payload_json, started_at, finished_at) VALUES ('orphan', 0, 'tool_call', '{}', 150, 175)`)

	if err := SweepOrphans(db, nil); err != nil {
		t.Fatalf("SweepOrphans: %v", err)
	}

	var endedAt sql.NullInt64
	var outcome sql.NullString
	if err := db.QueryRow(`SELECT ended_at, outcome FROM debug_sessions WHERE conversation_id='orphan'`).Scan(&endedAt, &outcome); err != nil {
		t.Fatalf("Query orphan: %v", err)
	}
	if outcome.String != "interrupted" {
		t.Fatalf("orphan outcome: want interrupted, got %q", outcome.String)
	}
	if !endedAt.Valid || endedAt.Int64 != 175 {
		t.Fatalf("orphan ended_at: want 175, got %v", endedAt)
	}
	if err := db.QueryRow(`SELECT ended_at, outcome FROM debug_sessions WHERE conversation_id='done'`).Scan(&endedAt, &outcome); err != nil {
		t.Fatalf("Query done: %v", err)
	}
	if outcome.String != "completed" || endedAt.Int64 != 200 {
		t.Fatalf("done row changed: outcome=%q ended_at=%v", outcome.String, endedAt)
	}
}
```

- [ ] **Step 2: Run tests**

```
go test ./internal/debug/ -run "TestDBSink_SetEnabled|TestSweepOrphans" -v
```

Expected: `SetEnabled` test passes (already wired in Task 3); `SweepOrphans` test FAILS (undefined).

- [ ] **Step 3: Add `SweepOrphans` to `internal/debug/session.go`**

Append (file already imports zap from Task 5; add `database/sql`):

```go
import (
	"database/sql"
)

// SweepOrphans marks any debug_sessions row with ended_at IS NULL as
// outcome='interrupted' with ended_at derived from the latest
// captured event timestamp. Intended to run once at server boot so
// pre-crash live sessions don't appear eternally live in the
// Settings → Debug list.
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
```

- [ ] **Step 4: Run tests to verify pass**

```
go test ./internal/debug/ -v -race
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/debug/session.go internal/debug/sink_test.go
git commit -m "debug: SweepOrphans boot helper + SetEnabled gating test

SweepOrphans(db) runs once at server start (wired in Task 22).
Finds debug_sessions with ended_at IS NULL (pre-crash live sessions)
and sets outcome='interrupted' + ended_at = latest event's
finished_at/started_at/session-start. Already-ended sessions are
untouched.

SetEnabled-gating test locks in the runtime-toggle behavior: writes
are suppressed while disabled and resume when re-enabled — no writes
during the off window.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 9: Retention worker

**Files:**
- Create: `internal/debug/retention.go`
- Create: `internal/debug/retention_test.go`

- [ ] **Step 1: Write failing test**

Create `internal/debug/retention_test.go`:

```go
package debug

import (
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestPruneOnce_DeletesOnlyOldSessions(t *testing.T) {
	db := openTestDB(t)
	nowNS := time.Now().UnixNano()
	thirtyDaysAgoNS := nowNS - int64(30*24*time.Hour)

	mustExec := func(q string, args ...interface{}) {
		if _, err := db.Exec(q, args...); err != nil {
			t.Fatalf("exec %q: %v", q, err)
		}
	}
	mustExec(`INSERT INTO debug_sessions (conversation_id, started_at, ended_at, outcome) VALUES ('old', ?, ?, 'completed')`, thirtyDaysAgoNS, thirtyDaysAgoNS+1)
	mustExec(`INSERT INTO debug_events (conversation_id, seq, event_type, payload_json, started_at) VALUES ('old', 0, 'iteration', '{}', ?)`, thirtyDaysAgoNS)
	mustExec(`INSERT INTO debug_llm_calls (conversation_id, sent_at, request_json, response_json) VALUES ('old', ?, '{}', '{}')`, thirtyDaysAgoNS)
	mustExec(`INSERT INTO debug_sessions (conversation_id, started_at, ended_at, outcome) VALUES ('fresh', ?, ?, 'completed')`, nowNS, nowNS+1)
	mustExec(`INSERT INTO debug_events (conversation_id, seq, event_type, payload_json, started_at) VALUES ('fresh', 0, 'iteration', '{}', ?)`, nowNS)
	mustExec(`INSERT INTO debug_sessions (conversation_id, started_at) VALUES ('live', ?)`, thirtyDaysAgoNS)

	if err := PruneOnce(db, 7, zap.NewNop()); err != nil {
		t.Fatalf("PruneOnce: %v", err)
	}
	for _, tbl := range []string{"debug_sessions", "debug_events", "debug_llm_calls"} {
		var n int
		_ = db.QueryRow(`SELECT COUNT(*) FROM `+tbl+` WHERE conversation_id = 'old'`).Scan(&n)
		if n != 0 {
			t.Fatalf("%s still has 'old' rows after prune: %d", tbl, n)
		}
	}
	var n int
	_ = db.QueryRow(`SELECT COUNT(*) FROM debug_sessions WHERE conversation_id IN ('fresh','live')`).Scan(&n)
	if n != 2 {
		t.Fatalf("fresh + live sessions should survive: got %d", n)
	}
}
```

- [ ] **Step 2: Run test**

```
go test ./internal/debug/ -run TestPruneOnce -v
```

Expected: FAIL — `PruneOnce` undefined.

- [ ] **Step 3: Create `internal/debug/retention.go`**

```go
package debug

import (
	"context"
	"database/sql"
	"time"

	"go.uber.org/zap"
)

// PruneOnce deletes debug rows for sessions that ended more than
// retainDays ago. Live sessions (ended_at IS NULL) are never pruned.
// Runs as one transaction so a partial failure can't leave orphan
// rows in debug_events / debug_llm_calls.
func PruneOnce(db *sql.DB, retainDays int, log *zap.Logger) error {
	if db == nil || retainDays <= 0 {
		return nil
	}
	if log == nil {
		log = zap.NewNop()
	}
	cutoffNS := time.Now().UnixNano() - int64(retainDays)*int64(24*time.Hour)
	tx, err := db.Begin()
	if err != nil {
		log.Warn("debug: retention begin failed", zap.Error(err))
		return err
	}
	defer tx.Rollback()

	for _, tbl := range []string{"debug_events", "debug_llm_calls"} {
		_, err := tx.Exec(`
			DELETE FROM `+tbl+`
			WHERE conversation_id IN (
			    SELECT conversation_id FROM debug_sessions
			    WHERE ended_at IS NOT NULL AND ended_at < ?
			)`, cutoffNS)
		if err != nil {
			log.Warn("debug: retention delete failed", zap.String("table", tbl), zap.Error(err))
			return err
		}
	}
	if _, err := tx.Exec(`DELETE FROM debug_sessions WHERE ended_at IS NOT NULL AND ended_at < ?`, cutoffNS); err != nil {
		log.Warn("debug: retention delete sessions failed", zap.Error(err))
		return err
	}
	return tx.Commit()
}

// StartRetentionWorker runs PruneOnce on start, then every interval
// (default 24h) until ctx cancels. Wired in cmd/server/main.go.
func StartRetentionWorker(ctx context.Context, db *sql.DB, retainDays int, interval time.Duration, log *zap.Logger) {
	if db == nil || retainDays <= 0 {
		return
	}
	if interval <= 0 {
		interval = 24 * time.Hour
	}
	if log == nil {
		log = zap.NewNop()
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	_ = PruneOnce(db, retainDays, log)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			_ = PruneOnce(db, retainDays, log)
		}
	}
}
```

- [ ] **Step 4: Run test**

```
go test ./internal/debug/ -run TestPruneOnce -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/debug/retention.go internal/debug/retention_test.go
git commit -m "debug: retention worker — PruneOnce + StartRetentionWorker

Deletes debug rows for sessions whose ended_at is older than
retainDays*24h. All deletes in one transaction so a partial run can't
orphan child rows. Live sessions (ended_at IS NULL) always preserved.

StartRetentionWorker kicks PruneOnce once immediately on start, then
ticks every interval until ctx cancel. Wired into main.go in Task 22
once cfg.Debug.RetainDays is reachable.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 10: ShareGPT converter (pure function)

**Files:**
- Create: `internal/debug/converter.go`
- Create: `internal/debug/converter_test.go`

- [ ] **Step 1: Write failing test with a canonical LLM-calls fixture**

Create `internal/debug/converter_test.go`:

```go
package debug

import (
	"encoding/json"
	"strings"
	"testing"
)

// orchestratorCall builds the fixture data a real orchestrator run
// would leave in debug_llm_calls.
func orchestratorCall(req, resp string) LLMCallRow {
	return LLMCallRow{AgentID: "cyberstrike-orchestrator", SentAt: 1, RequestJSON: req, ResponseJSON: resp}
}

func TestToShareGPT_PicksFirstStopCall(t *testing.T) {
	calls := []LLMCallRow{
		orchestratorCall(
			`{"messages":[{"role":"system","content":"sys"},{"role":"user","content":"hi"}]}`,
			`{"choices":[{"finish_reason":"tool_calls","message":{"role":"assistant","content":"","tool_calls":[{"id":"t1","type":"function","function":{"name":"nmap","arguments":"{}"}}]}}]}`,
		),
		orchestratorCall(
			`{"messages":[{"role":"system","content":"sys"},{"role":"user","content":"hi"},{"role":"assistant","content":"","tool_calls":[{"id":"t1","type":"function","function":{"name":"nmap","arguments":"{}"}}]},{"role":"tool","tool_call_id":"t1","content":"port 22 open"}]}`,
			`{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"SSH is open on 22."}}]}`,
		),
	}
	out, err := ToShareGPT(calls)
	if err != nil {
		t.Fatalf("ToShareGPT: %v", err)
	}
	var got struct {
		Messages []map[string]interface{} `json:"messages"`
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("unmarshal: %v, body=%s", err, out)
	}
	if len(got.Messages) != 5 {
		t.Fatalf("want 5 messages (sys,user,assistant-with-tc,tool,assistant-final), got %d: %s", len(got.Messages), out)
	}
	if got.Messages[4]["role"] != "assistant" || !strings.Contains(got.Messages[4]["content"].(string), "SSH is open") {
		t.Fatalf("final assistant missing or wrong: %v", got.Messages[4])
	}
}

func TestToShareGPT_ExcludesSubAgentCalls(t *testing.T) {
	calls := []LLMCallRow{
		orchestratorCall(
			`{"messages":[{"role":"user","content":"scan"}]}`,
			`{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"done"}}]}`,
		),
		{AgentID: "recon-subagent", SentAt: 2, RequestJSON: `{"messages":[{"role":"user","content":"inner"}]}`, ResponseJSON: `{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"inner done"}}]}`},
	}
	out, err := ToShareGPT(calls)
	if err != nil {
		t.Fatalf("ToShareGPT: %v", err)
	}
	if strings.Contains(string(out), "inner") {
		t.Fatalf("sub-agent content leaked into ShareGPT export: %s", out)
	}
}

func TestToShareGPT_FallsBackToLastWhenNoStop(t *testing.T) {
	calls := []LLMCallRow{
		orchestratorCall(
			`{"messages":[{"role":"user","content":"a"}]}`,
			`{"choices":[{"finish_reason":"tool_calls","message":{"role":"assistant","content":""}}]}`,
		),
		orchestratorCall(
			`{"messages":[{"role":"user","content":"a"},{"role":"assistant","content":""},{"role":"tool","tool_call_id":"t","content":"r"}]}`,
			`{"choices":[{"finish_reason":"length","message":{"role":"assistant","content":"partial"}}]}`,
		),
	}
	out, err := ToShareGPT(calls)
	if err != nil {
		t.Fatalf("ToShareGPT: %v", err)
	}
	if !strings.Contains(string(out), "partial") {
		t.Fatalf("fallback to last call failed: %s", out)
	}
}

func TestToShareGPT_EmptyInputReturnsError(t *testing.T) {
	if _, err := ToShareGPT(nil); err == nil {
		t.Fatalf("want error on empty input, got nil")
	}
}
```

- [ ] **Step 2: Run test**

```
go test ./internal/debug/ -run TestToShareGPT -v
```

Expected: FAIL — `ToShareGPT` and `LLMCallRow` undefined.

- [ ] **Step 3: Create `internal/debug/converter.go`**

```go
package debug

import (
	"encoding/json"
	"errors"
)

// LLMCallRow is the row-shape read from debug_llm_calls by the
// exporter layer. Kept separate from LLMCall (which is the write-side
// value type carried by the sink) so the read path can evolve without
// breaking Sink callers.
type LLMCallRow struct {
	ID               int64
	ConversationID   string
	MessageID        string
	Iteration        int
	CallIndex        int
	AgentID          string
	SentAt           int64
	FirstTokenAt     int64
	FinishedAt       int64
	PromptTokens     int64
	CompletionTokens int64
	RequestJSON      string
	ResponseJSON     string
	Error            string
}

// shareGPTRequest is the request shape we expect: the orchestrator
// always sends {messages: [...], tools: [...]} per the openai-format
// contract. We only need messages for export.
type shareGPTRequest struct {
	Messages []json.RawMessage `json:"messages"`
}

// shareGPTResponse is the response shape.
type shareGPTResponse struct {
	Choices []struct {
		FinishReason string          `json:"finish_reason"`
		Message      json.RawMessage `json:"message"`
	} `json:"choices"`
}

// ToShareGPT produces a single JSONL line (no trailing newline) of
// {"messages": [...]} matching the OpenAI / ShareGPT / HuggingFace
// SFT loader contract.
//
// Algorithm: filter to orchestrator-level calls only; pick the first
// call whose response.choices[0].finish_reason == "stop"; if none,
// fall back to the last call in input order. The chosen call's
// request.messages is the exact history the model saw on its final
// turn (includes all earlier assistant tool_calls + tool-role
// responses interleaved by the orchestrator's message-append loop).
// Append the response's message as the final assistant turn.
func ToShareGPT(calls []LLMCallRow) ([]byte, error) {
	if len(calls) == 0 {
		return nil, errors.New("ToShareGPT: empty input")
	}
	var orch []LLMCallRow
	for _, c := range calls {
		if c.AgentID == "cyberstrike-orchestrator" {
			orch = append(orch, c)
		}
	}
	if len(orch) == 0 {
		return nil, errors.New("ToShareGPT: no orchestrator-level calls in input")
	}

	chosen := -1
	for i, c := range orch {
		var resp shareGPTResponse
		if err := json.Unmarshal([]byte(c.ResponseJSON), &resp); err != nil {
			continue
		}
		if len(resp.Choices) > 0 && resp.Choices[0].FinishReason == "stop" {
			chosen = i
			break
		}
	}
	if chosen == -1 {
		chosen = len(orch) - 1
	}
	c := orch[chosen]

	var req shareGPTRequest
	if err := json.Unmarshal([]byte(c.RequestJSON), &req); err != nil {
		return nil, err
	}
	var resp shareGPTResponse
	if err := json.Unmarshal([]byte(c.ResponseJSON), &resp); err != nil {
		return nil, err
	}

	out := struct {
		Messages []json.RawMessage `json:"messages"`
	}{Messages: req.Messages}
	if len(resp.Choices) > 0 && len(resp.Choices[0].Message) > 0 {
		out.Messages = append(out.Messages, resp.Choices[0].Message)
	}
	return json.Marshal(out)
}
```

- [ ] **Step 4: Run tests — all four pass**

```
go test ./internal/debug/ -run TestToShareGPT -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/debug/converter.go internal/debug/converter_test.go
git commit -m "debug: ShareGPT converter pure function + tests

ToShareGPT(llmCalls) reconstructs a training-ready JSONL line from
debug_llm_calls rows. Algorithm per spec §Export:
  1. Filter to orchestrator-level calls (agent_id =
     \"cyberstrike-orchestrator\"). Sub-agent traces excluded from
     training output — only available via format=raw.
  2. Pick first call with response.finish_reason=\"stop\". If none
     (maxIter hit without clean stop), fall back to the last call.
  3. Output = {\"messages\": requestMessages +
     [responseMessage]} — the terminal request's messages already
     has all earlier assistant tool_calls + tool-role responses
     interleaved by the orchestrator's history-append loop.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 11: Raw JSONL + bulk tar.gz export writers

**Files:**
- Create: `internal/debug/export.go`
- Create: `internal/debug/export_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/debug/export_test.go`:

```go
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
	// First should be llm_call @ 50, then iteration @ 100, then tool_call @ 150
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

func TestWriteShareGPTJSONL_EmitsOneLine(t *testing.T) {
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
		t.Fatalf("ShareGPT should be one line, got:\n%s", buf.String())
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
```

- [ ] **Step 2: Run tests**

```
go test ./internal/debug/ -run "TestWriteRawJSONL|TestWriteShareGPTJSONL|TestWriteBulkArchive" -v
```

Expected: FAIL — none of those writers exist yet.

- [ ] **Step 3: Create `internal/debug/export.go`**

```go
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
// Each line has a "source" tag so downstream consumers can discriminate.
// Streaming: no full buffering — one row is encoded and written
// directly to w before the next row is fetched.
func WriteRawJSONL(w io.Writer, db *sql.DB, conversationID string) error {
	// Load LLM calls.
	calls, err := loadLLMCalls(db, conversationID)
	if err != nil {
		return err
	}
	// Load events.
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
// one JSONL entry per session in [sinceNS, untilNS] (0 means unbounded).
// format must be "raw" or "sharegpt".
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
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return err
		}
		ids = append(ids, id)
	}

	for _, id := range ids {
		var body []byte
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
		body = buf.Bytes()
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
// is required; the server's peak memory is still one session (not
// one archive).
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
```

- [ ] **Step 4: Run tests**

```
go test ./internal/debug/ -v -race
```

Expected: all export tests PASS; prior tests still PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/debug/export.go internal/debug/export_test.go
git commit -m "debug: export writers — raw JSONL + ShareGPT JSONL + bulk tar.gz

Three streaming writers:
  - WriteRawJSONL merges debug_llm_calls and debug_events by
    timestamp; each output line carries a source=\"llm_call|event\"
    tag so downstream consumers discriminate. Rows encoded one at a
    time — peak memory is one row.
  - WriteShareGPTJSONL wraps the Task 10 pure converter; one line
    plus trailing newline (JSONL convention).
  - WriteBulkArchive builds a gzip-compressed tar with one entry per
    session in [since,until] (0 = unbounded). Format must be raw or
    sharegpt; invalid format returns an error. Per-entry buffering
    is required by tar's size-before-write contract; peak memory is
    still one session, not one archive.

Test coverage: timestamp ordering across interleaved calls+events,
one-line ShareGPT contract, tar entries named <convID>.jsonl.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 12: Hook LLM call capture in `internal/agent/agent.go`

**Files:**
- Modify: `internal/agent/agent.go` (around the streaming LLM call wrappers)
- Create: `internal/agent/debug_capture.go` (helper for the capture scope)
- Modify: `internal/agent/agent_test.go` (add hook test)

- [ ] **Step 1: Read the call path you're wrapping**

Before writing test or code, read `internal/agent/agent.go` lines 1561–1700 (the `callOpenAIStreamWithToolCalls` body — the outermost function that has both the full request and the full response). Also read `internal/openai/openai.go` and `internal/openai/anthropic.go` to confirm where token usage surfaces on each backend. Anthropic reports tokens via the `Usage` field (line 115 + 154); OpenAI streaming does not reliably surface tokens in the current client. Token columns stay nullable per the plan's "Deviations" note.

- [ ] **Step 2: Write failing hook test**

Append to `internal/agent/agent_test.go` (adapt the `setupTestAgent` helper if it doesn't already accept a sink):

```go
func TestAgent_LLMCallWrapper_RecordsWhenDebugOn(t *testing.T) {
	// Uses the existing httptest-based setupTestAgent to spin up a
	// mocked openai HTTP endpoint. When that helper takes a sink,
	// pass a real dbSink backed by an in-memory SQLite; when it does
	// not, extend it now (this test drives that change).
	db := openTestDebugDB(t) // helper: inline SQLite + the three DDL statements, same as internal/debug.openTestDB
	sink := debug.NewSink(true, db, zap.NewNop())
	agent, _ := setupTestAgentWithSink(t, sink)

	ctx := debug.WithCapture(context.Background(), "conv-t", "msg-t", 0, 0, "cyberstrike-orchestrator")
	msgs := []ChatMessage{{Role: "user", Content: "hi"}}
	_, err := agent.CallStreamWithToolCalls(ctx, msgs, nil, func(string) error { return nil })
	if err != nil {
		t.Fatalf("CallStreamWithToolCalls: %v", err)
	}

	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM debug_llm_calls WHERE conversation_id = ?", "conv-t").Scan(&n); err != nil {
		t.Fatalf("QueryRow: %v", err)
	}
	if n != 1 {
		t.Fatalf("want 1 debug_llm_calls row, got %d", n)
	}
}
```

`openTestDebugDB` is a local helper identical in body to the `openTestDB` helper in `internal/debug/sink_test.go` (copy its three DDL statements inline).

`setupTestAgentWithSink` is a thin wrapper over the existing `setupTestAgent` that accepts a sink parameter and calls `NewAgent` with it; add this to `agent_test.go`.

- [ ] **Step 3: Run test to verify it fails**

```
go test ./internal/agent/ -run TestAgent_LLMCallWrapper -v
```

Expected: FAIL — `debug.WithCapture` undefined; no capture call site exists yet in `callOpenAIStreamWithToolCalls`.

- [ ] **Step 4: Add capture-context helpers to `internal/debug/capture.go`**

Append to `internal/debug/capture.go`:

```go
import "context"

type captureCtxKey struct{}

// captureCtx carries the per-call capture coordinates. The Agent
// wrapper reads it to know which iteration/agent/etc. to attribute
// the LLM call to; if absent, the wrapper still calls
// sink.RecordLLMCall but with zero-valued metadata — harmless under
// noopSink, slightly less useful under dbSink but still safe.
type captureCtx struct {
	ConversationID string
	MessageID      string
	Iteration      int
	CallIndex      int
	AgentID        string
}

// WithCapture attaches capture metadata to ctx. The orchestrator sets
// this before invoking any Agent method that calls the LLM.
func WithCapture(ctx context.Context, conversationID, messageID string, iteration, callIndex int, agentID string) context.Context {
	return context.WithValue(ctx, captureCtxKey{}, captureCtx{
		ConversationID: conversationID,
		MessageID:      messageID,
		Iteration:      iteration,
		CallIndex:      callIndex,
		AgentID:        agentID,
	})
}

// CaptureFromContext returns the capture metadata on ctx, or zero
// value if none is set.
func CaptureFromContext(ctx context.Context) captureCtx {
	v, _ := ctx.Value(captureCtxKey{}).(captureCtx)
	return v
}

// Exported shim so the agent package (different package) can read the
// fields without exposing the struct itself.
func CaptureCoords(ctx context.Context) (convID, msgID string, iteration, callIndex int, agentID string) {
	c := CaptureFromContext(ctx)
	return c.ConversationID, c.MessageID, c.Iteration, c.CallIndex, c.AgentID
}
```

- [ ] **Step 5: Create `internal/agent/debug_capture.go`**

```go
package agent

import (
	"encoding/json"
	"time"

	"cyberstrike-ai/internal/debug"
)

// captureLLMCall wraps the LLM request/response marshaling for the
// debug sink. callFn is the actual openai call; it returns the
// response we'll marshal. If the sink is a noopSink, the marshaling
// and time calls still run but the sink's Record is free, which
// keeps the hot path branch-free.
//
// requestPayload is whatever the caller would have handed the openai
// client — a map[string]interface{}, a typed struct, etc. — that
// serializes into the API request body.
func (a *Agent) captureLLMCall(
	ctx context.Context,
	requestPayload interface{},
	callFn func() (response interface{}, promptTokens, completionTokens int64, err error),
) (interface{}, error) {
	sentAt := time.Now().UnixNano()
	resp, pt, ct, err := callFn()
	finishedAt := time.Now().UnixNano()

	convID, msgID, iteration, callIndex, agentID := debug.CaptureCoords(ctx)
	if convID == "" {
		// No capture coordinates — skip recording. Happens in tests
		// that exercise the raw LLM path without orchestrator.
		return resp, err
	}
	reqJSON, _ := json.Marshal(requestPayload)
	var respJSON []byte
	if resp != nil {
		respJSON, _ = json.Marshal(resp)
	}
	errStr := ""
	if err != nil {
		errStr = err.Error()
	}
	a.debugSink.RecordLLMCall(convID, msgID, debug.LLMCall{
		Iteration:        iteration,
		CallIndex:        callIndex,
		AgentID:          agentID,
		SentAt:           sentAt,
		FinishedAt:       finishedAt,
		PromptTokens:     pt,
		CompletionTokens: ct,
		RequestJSON:      string(reqJSON),
		ResponseJSON:     string(respJSON),
		Error:            errStr,
	})
	return resp, err
}
```

Add at top of file:

```go
import "context"
```

- [ ] **Step 6: Adapt the streaming LLM call to use `captureLLMCall`**

This is the most intricate edit. Open `internal/agent/agent.go` around line 1561 (`callOpenAIStreamWithToolCalls`). The function currently constructs a request, calls `a.openAIClient.ChatCompletionStreamWithToolCalls(ctx, reqBody, onContentDelta)` (around L1474), and returns the streamed response.

Refactor so the `captureLLMCall` wrapper is the outermost call:

```go
func (a *Agent) callOpenAIStreamWithToolCalls(
	ctx context.Context,
	messages []ChatMessage,
	tools []Tool,
	onContentDelta func(delta string) error,
) (*ChatCompletionResponse, error) {
	reqBody := a.buildChatCompletionRequest(messages, tools, true)
	var capturedFirstToken int64
	wrappedOnDelta := func(delta string) error {
		if capturedFirstToken == 0 && delta != "" {
			capturedFirstToken = time.Now().UnixNano()
		}
		return onContentDelta(delta)
	}

	respIface, err := a.captureLLMCall(ctx, reqBody, func() (interface{}, int64, int64, error) {
		content, streamToolCalls, finishReason, cerr := a.openAIClient.ChatCompletionStreamWithToolCalls(ctx, reqBody, wrappedOnDelta)
		if cerr != nil {
			// Retry path from original code stays here — preserve it.
			content, streamToolCalls, finishReason, cerr = a.openAIClient.ChatCompletionStreamWithToolCalls(ctx, reqBody, wrappedOnDelta)
		}
		if cerr != nil {
			return nil, 0, 0, cerr
		}
		resp := &ChatCompletionResponse{
			Choices: []Choice{{
				Message: ChatMessage{
					Role:      "assistant",
					Content:   content,
					ToolCalls: streamToolCalls,
				},
				FinishReason: finishReason,
			}},
		}
		return resp, 0, 0, nil // streaming openai path: token counts unknown (plan deviation #2)
	})
	if err != nil {
		return nil, err
	}
	resp, _ := respIface.(*ChatCompletionResponse)
	// TODO(post-capture): update first_token_at on the most-recent
	// debug_llm_calls row. For v1 this is post-hoc and optional; if
	// it's noisy, defer.
	_ = capturedFirstToken
	return resp, nil
}
```

Note: the exact shape of `buildChatCompletionRequest` / `ChatCompletionResponse` / `Choice` / `streamToolCalls` needs to match the current file. Read lines 1561-1700 verbatim and adapt the structure — the goal is preserving the existing return type and the retry behavior while slotting the `captureLLMCall` wrapper in as the outermost call. If the existing code does more than one retry, preserve the retry count exactly.

Also adapt `callOpenAIStreamText` and `callOpenAISingleStreamWithToolCalls` to the same pattern for completeness. If any variant is unused from any capture-on code path (grep to confirm), skip it for v1.

- [ ] **Step 7: Run all agent tests**

```
go test ./internal/agent/ -v -race
```

Expected: `TestAgent_LLMCallWrapper_RecordsWhenDebugOn` PASSES; prior tests still PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/agent/agent.go internal/agent/debug_capture.go internal/agent/agent_test.go internal/debug/capture.go
git commit -m "agent: hook LLM calls into debug Sink

Wraps callOpenAIStreamWithToolCalls (and the text variants) with a
captureLLMCall scope that:
  - Snapshots request pre-call (marshaled to JSON for verbatim
    replay).
  - Records sent_at before the call, finished_at after.
  - Captures first-token timestamp from the streaming delta callback.
  - Hands everything to a.debugSink.RecordLLMCall.

debug.WithCapture(ctx, ...) attaches per-call coordinates
(conversation_id, message_id, iteration, call_index, agent_id) the
orchestrator will set before each call in Task 13. Without those
coordinates the wrapper silently skips recording — keeps the path
safe in unit tests that exercise the raw LLM path.

Streaming token counts remain NULL for the OpenAI backend per the
plan's deviation note; Anthropic backend fills them in via its own
ratelimit/usage extraction path (unchanged).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 13: Tee `sendProgress` + wrap LLM calls in orchestrator

**Files:**
- Modify: `internal/multiagent/orchestrator.go`
- Modify: `internal/multiagent/orchestrator_test.go`

- [ ] **Step 1: Write failing test**

Append to `internal/multiagent/orchestrator_test.go`:

```go
func TestOrchestrator_sendProgress_TeesToSink(t *testing.T) {
	db := openOrchestratorTestDB(t) // same inline-DDL helper as sink_test.go
	sink := debug.NewSink(true, db, nil)
	o := &orchestratorState{
		ctx:            context.Background(),
		conversationID: "conv-t",
		sink:           sink,
		progress:       func(string, string, interface{}) {},
	}

	o.sendProgress("iteration", "", map[string]interface{}{
		"iteration":      1,
		"agent":          "cyberstrike-orchestrator",
		"conversationId": "conv-t",
	})

	var n int
	var evType, payload string
	err := db.QueryRow(`SELECT COUNT(*), COALESCE(MAX(event_type),''), COALESCE(MAX(payload_json),'') FROM debug_events WHERE conversation_id = ?`, "conv-t").Scan(&n, &evType, &payload)
	if err != nil {
		t.Fatalf("QueryRow: %v", err)
	}
	if n != 1 {
		t.Fatalf("want 1 debug_events row, got %d", n)
	}
	if evType != "iteration" {
		t.Fatalf("event_type: want iteration, got %q", evType)
	}
	if !strings.Contains(payload, `"iteration":1`) && !strings.Contains(payload, `"iteration": 1`) {
		t.Fatalf("payload missing iteration field: %s", payload)
	}
}
```

Also add the `openOrchestratorTestDB` helper (inline DDL) at the top of the test file, or import it from the debug package via a test-only helper. Since unit tests within the same package can use internal helpers freely, just inline:

```go
func openOrchestratorTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "orch_test.db"))
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
```

- [ ] **Step 2: Run test to verify it fails**

```
go test ./internal/multiagent/ -run TestOrchestrator_sendProgress_TeesToSink -v
```

Expected: FAIL — `sendProgress` currently only fires the progress callback, does not tee to sink.

- [ ] **Step 3: Modify `sendProgress`**

In `internal/multiagent/orchestrator.go` (L271 in the current code), replace the method with:

```go
func (o *orchestratorState) sendProgress(eventType, message string, data interface{}) {
	// User-facing callback: unchanged.
	if o.progress != nil {
		o.progress(eventType, message, data)
	}
	// Debug tee: fire-and-forget into the sink. Noop when debug is off.
	if o.sink == nil {
		return
	}
	now := time.Now().UnixNano()
	// Extract agent id from the data map if present; fall back to
	// orchestrator default so every event has an agentId for queries.
	agentID := "cyberstrike-orchestrator"
	var messageID string
	if m, ok := data.(map[string]interface{}); ok {
		if v, _ := m["agent"].(string); v != "" {
			agentID = v
		}
		if v, _ := m["messageId"].(string); v != "" {
			messageID = v
		}
	}
	payload, _ := json.Marshal(data)
	o.sink.RecordEvent(o.conversationID, messageID, debug.Event{
		EventType:   eventType,
		AgentID:     agentID,
		PayloadJSON: string(payload),
		StartedAt:   now,
	})
}
```

If `encoding/json` isn't already imported, it should be (the file already uses it extensively).

- [ ] **Step 4: Wrap LLM call sites with `debug.WithCapture`**

In the two places the orchestrator invokes the Agent's streaming LLM method (main loop around L415 and sub-agent loop around L695), replace the ctx argument with a capture-wrapped ctx:

Main loop:

```go
callIdx := /* the cumulative orchestrator+subagent index maintained on o.mu + o.callSeq */
captureCtx := debug.WithCapture(o.ctx, o.conversationID, /*messageID*/ "", i+1, callIdx, "cyberstrike-orchestrator")
response, err := o.ag.CallStreamWithToolCalls(captureCtx, messages, allTools, ...)
```

Sub-agent loop (inside `runSubAgent`):

```go
captureCtx := debug.WithCapture(o.ctx, o.conversationID, "", i+1, callIdx, agentName)
response, err := o.ag.CallStreamWithToolCalls(captureCtx, messages, subTools, ...)
```

Add a `callSeq` counter to `orchestratorState` (plain int, bumped under `o.mu`) so `call_index` is monotonic per conversation across main + sub-agent spans. Increment on each LLM dispatch.

- [ ] **Step 5: Run tests**

```
go test ./internal/multiagent/ -v -race
```

Expected: all pass. Existing multiagent tests unaffected; new tee test passes.

- [ ] **Step 6: Commit**

```bash
git add internal/multiagent/orchestrator.go internal/multiagent/orchestrator_test.go
git commit -m "multiagent: tee sendProgress to debug Sink + wrap LLM calls

sendProgress keeps its existing user-facing progress callback and
additionally calls sink.RecordEvent with a timestamp + JSON payload.
agent_id is extracted from the data map's \"agent\" field when
present; falls back to the orchestrator's canonical id so every
debug_events row has a non-null agentId for queries.

LLM call sites in the main loop and runSubAgent wrap ctx with
debug.WithCapture so the agent-level wrapper (Task 12) knows the
conversation/iteration/call-index/agent-id for the recorded LLM
call. A new o.callSeq counter under o.mu tracks cumulative call
index across orchestrator + sub-agents.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 14: Handler `StartSession` / `EndSession` hooks

**Files:**
- Modify: `internal/handler/multi_agent.go` (both `MultiAgentLoopStream` and `MultiAgentLoop`)
- Modify: `internal/handler/multi_agent_test.go` (or create if missing)

- [ ] **Step 1: Write failing test**

The existing `multi_agent.go` streaming handler is hard to exercise end-to-end without the full Agent and orchestrator. The minimal test uses a fake sink that records calls in-memory:

Create `internal/handler/multi_agent_debug_test.go`:

```go
package handler

import (
	"testing"

	"cyberstrike-ai/internal/debug"
)

// fakeSink captures StartSession/EndSession call args so we can
// assert the handler's session boundaries without running a real
// orchestrator.
type fakeSink struct {
	starts []string
	ends   []struct{ id, outcome string }
}

func (f *fakeSink) StartSession(id string)                                 { f.starts = append(f.starts, id) }
func (f *fakeSink) EndSession(id, outcome string)                          { f.ends = append(f.ends, struct{ id, outcome string }{id, outcome}) }
func (f *fakeSink) RecordLLMCall(string, string, debug.LLMCall)            {}
func (f *fakeSink) RecordEvent(string, string, debug.Event)                {}
func (f *fakeSink) SetEnabled(bool)                                        {}
func (f *fakeSink) Enabled() bool                                          { return true }

func TestHandler_debugSessionBoundaries_CalledOncePerRun(t *testing.T) {
	// NOTE: this test exercises a thin helper the refactor introduces —
	// `wrapRunWithDebug(sink, convID, runFn)` — so we don't need to
	// spin up a full fake orchestrator to test the boundary semantics.
	fake := &fakeSink{}
	outcome, _ := wrapRunWithDebug(fake, "conv-x", func() (string, error) {
		return "completed", nil
	})
	if outcome != "completed" {
		t.Fatalf("outcome: want completed, got %q", outcome)
	}
	if len(fake.starts) != 1 || fake.starts[0] != "conv-x" {
		t.Fatalf("StartSession not called once with conv-x: %v", fake.starts)
	}
	if len(fake.ends) != 1 || fake.ends[0].id != "conv-x" || fake.ends[0].outcome != "completed" {
		t.Fatalf("EndSession not called once with conv-x/completed: %v", fake.ends)
	}
}

func TestHandler_debugSessionBoundaries_OnError(t *testing.T) {
	fake := &fakeSink{}
	outcome, _ := wrapRunWithDebug(fake, "conv-err", func() (string, error) {
		return "failed", errTestOrchestrator
	})
	if outcome != "failed" {
		t.Fatalf("outcome: want failed, got %q", outcome)
	}
	if fake.ends[0].outcome != "failed" {
		t.Fatalf("EndSession outcome on error: want failed, got %q", fake.ends[0].outcome)
	}
}

var errTestOrchestrator = errTest("orchestrator boom")

type errTest string

func (e errTest) Error() string { return string(e) }
```

- [ ] **Step 2: Run test to verify failure**

```
go test ./internal/handler/ -run TestHandler_debugSessionBoundaries -v
```

Expected: FAIL — `wrapRunWithDebug` undefined.

- [ ] **Step 3: Add `wrapRunWithDebug` helper + wire into both handlers**

Create `internal/handler/debug_lifecycle.go`:

```go
package handler

import (
	"cyberstrike-ai/internal/debug"
)

// wrapRunWithDebug calls sink.StartSession before runFn and
// sink.EndSession after (with the returned outcome). Centralizes the
// boundary logic so MultiAgentLoop and MultiAgentLoopStream share
// exactly one code path for the capture bookends.
//
// runFn returns (outcome, err) where outcome is one of
// "completed"|"cancelled"|"failed". If the orchestrator returned an
// error but the caller already classified the outcome, that
// classification wins; otherwise "failed" is used.
func wrapRunWithDebug(sink debug.Sink, conversationID string, runFn func() (string, error)) (string, error) {
	if sink != nil {
		sink.StartSession(conversationID)
	}
	outcome, err := runFn()
	if outcome == "" {
		if err != nil {
			outcome = "failed"
		} else {
			outcome = "completed"
		}
	}
	if sink != nil {
		sink.EndSession(conversationID, outcome)
	}
	return outcome, err
}
```

Now in `internal/handler/multi_agent.go`:

**`MultiAgentLoopStream`** (around line 151 where `RunOrchestrator` is called): replace the direct `result, runErr := multiagent.RunOrchestrator(...)` with a `wrapRunWithDebug` invocation. The existing logic already tracks `taskStatus`, which is the outcome string — pass it through:

```go
var result *multiagent.RunResult
var runErr error
_, _ = wrapRunWithDebug(h.debugSink, conversationID, func() (string, error) {
	result, runErr = multiagent.RunOrchestrator(
		taskCtx, h.config, &h.config.MultiAgent, h.agent,
		h.logger, conversationID, prep.FinalMessage,
		prep.History, prep.RoleTools, progressCallback,
		h.agentsMarkdownDir, h.debugSink,
	)
	// taskStatus is updated in the existing error-handling branches.
	// Pass it through via closure so the boundary records the final
	// label. If still "completed" / classified externally, let that
	// stand; otherwise default to failed on runErr.
	return taskStatus, runErr
})
```

Note the existing code mutates `taskStatus` in several branches (cancelled, failed); the closure needs access to the enclosing variable. Adjust by declaring `taskStatus` above the wrapRunWithDebug call and letting the closure read it.

Do the equivalent surgery in `MultiAgentLoop` (non-streaming variant). For that handler there's no explicit taskStatus — the outcome is "completed" on success, "failed" on error.

- [ ] **Step 4: Run tests**

```
go test ./internal/handler/ -v
```

Expected: boundary tests pass; existing handler tests unaffected.

- [ ] **Step 5: Commit**

```bash
git add internal/handler/multi_agent.go internal/handler/debug_lifecycle.go internal/handler/multi_agent_debug_test.go
git commit -m "handler: StartSession/EndSession bookends for debug capture

wrapRunWithDebug centralizes the debug session boundary. Both
MultiAgentLoop and MultiAgentLoopStream now record:
  - StartSession(convID) before RunOrchestrator dispatch
  - EndSession(convID, outcome) after, with outcome taken from the
    existing taskStatus variable (completed | cancelled | failed).

If the caller did not set an outcome by the time the run returns,
the helper defaults to \"completed\" on nil error or \"failed\"
otherwise.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 15: HTTP route — `GET /api/debug/sessions` (list + aggregates)

**Files:**
- Create: `internal/handler/debug.go` (new handler file)
- Create: `internal/handler/debug_test.go`
- Modify: `internal/app/app.go` (register route)

- [ ] **Step 1: Write failing test**

Create `internal/handler/debug_test.go`:

```go
package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	_ "modernc.org/sqlite"
	"go.uber.org/zap"
)

func openHandlerTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "handler_test.db"))
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
	_, _ = db.Exec(`INSERT INTO debug_sessions (conversation_id, started_at, ended_at, outcome, label) VALUES ('c1', 1000, 4000, 'completed', 'nmap scan')`)
	_, _ = db.Exec(`INSERT INTO debug_llm_calls (conversation_id, iteration, sent_at, prompt_tokens, completion_tokens, request_json, response_json) VALUES ('c1', 1, 1100, 100, 20, '{}', '{}')`)
	_, _ = db.Exec(`INSERT INTO debug_llm_calls (conversation_id, iteration, sent_at, prompt_tokens, completion_tokens, request_json, response_json) VALUES ('c1', 2, 2200, 150, 30, '{}', '{}')`)

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
	if int(row["durationMs"].(float64)) <= 0 {
		t.Fatalf("durationMs should be >0, got %v", row["durationMs"])
	}
}
```

- [ ] **Step 2: Run test**

```
go test ./internal/handler/ -run TestListDebugSessions -v
```

Expected: FAIL — `DebugHandler` and `ListSessions` undefined.

- [ ] **Step 3: Create `internal/handler/debug.go`**

```go
package handler

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// DebugHandler hosts the /api/debug/* routes.
type DebugHandler struct {
	db     *sql.DB
	logger *zap.Logger
}

func NewDebugHandler(db *sql.DB, logger *zap.Logger) *DebugHandler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &DebugHandler{db: db, logger: logger}
}

// sessionSummary is the list row shape.
type sessionSummary struct {
	ConversationID   string  `json:"conversationId"`
	Label            string  `json:"label,omitempty"`
	StartedAt        int64   `json:"startedAt"`
	EndedAt          *int64  `json:"endedAt,omitempty"`
	Outcome          string  `json:"outcome,omitempty"`
	Iterations       int64   `json:"iterations"`
	PromptTokens     int64   `json:"promptTokens"`
	CompletionTokens int64   `json:"completionTokens"`
	DurationMs       int64   `json:"durationMs"`
}

// ListSessions handles GET /api/debug/sessions.
// Per-row aggregates (iterations, tokens, durationMs) are computed at
// query time — they're not stored columns on debug_sessions.
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
			r.DurationMs = (endedAt.Int64 - r.StartedAt) / 1_000_000
		}
		out = append(out, r)
	}
	c.JSON(http.StatusOK, out)
}
```

- [ ] **Step 4: Register the route in `internal/app/app.go`**

In the route-registration block (around the `protected.POST(...)` calls at L695+), add:

```go
debugHandler := handler.NewDebugHandler(db, logger)
protected.GET("/debug/sessions", debugHandler.ListSessions)
```

Where `db` is the same `*sql.DB` used elsewhere and `logger` is the zap logger.

- [ ] **Step 5: Run tests**

```
go build ./...
go test ./internal/handler/ -v
```

Expected: build passes, new tests pass, existing tests unaffected.

- [ ] **Step 6: Commit**

```bash
git add internal/handler/debug.go internal/handler/debug_test.go internal/app/app.go
git commit -m "handler: GET /api/debug/sessions with aggregated columns

DebugHandler.ListSessions returns the recent 200 debug_sessions rows
with per-row computed aggregates (iterations, promptTokens,
completionTokens, durationMs) joined from debug_llm_calls. Empty
result is [] not 404. endedAt is omitted when NULL; durationMs is
only computed when endedAt is set.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 16: HTTP routes — `GET` / `DELETE` / `PATCH /api/debug/sessions/:id`

**Files:**
- Modify: `internal/handler/debug.go`
- Modify: `internal/handler/debug_test.go`
- Modify: `internal/app/app.go` (register 3 more routes)

- [ ] **Step 1: Write failing tests for all three**

Append to `internal/handler/debug_test.go`:

```go
import (
	"strings"
)

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
	if len(body["llmCalls"].([]interface{})) != 1 {
		t.Fatalf("want 1 llmCalls, got %v", body["llmCalls"])
	}
	if len(body["events"].([]interface{})) != 1 {
		t.Fatalf("want 1 event, got %v", body["events"])
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
```

- [ ] **Step 2: Run tests**

```
go test ./internal/handler/ -run "TestGetDebugSession|TestDeleteDebugSession|TestPatchDebugSession" -v
```

Expected: FAIL — none of the new methods exist.

- [ ] **Step 3: Add the three methods to `internal/handler/debug.go`**

```go
import (
	"cyberstrike-ai/internal/debug"
)

// GetSession handles GET /api/debug/sessions/:id and returns the full
// capture payload: session row + all llm_calls + all events.
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
func (h *DebugHandler) DeleteSession(c *gin.Context) {
	id := c.Param("id")
	// Existence check first so we can 404 cleanly.
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

// PatchSession handles PATCH /api/debug/sessions/:id. Only the label
// field is mutable in v1.
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
```

In `internal/debug/export.go`, export `loadLLMCalls` and `loadEvents` so the handler can read them (rename to `LoadLLMCallsExported` / `LoadEventsExported` and add public doc comments):

```go
// LoadLLMCallsExported is the exported form for consumers outside the
// debug package (the HTTP handler). Prefer this over the unexported
// loadLLMCalls when you're crossing package boundaries.
func LoadLLMCallsExported(db *sql.DB, conversationID string) ([]LLMCallRow, error) {
	return loadLLMCalls(db, conversationID)
}

// LoadEventsExported is the exported form for handler consumption.
// Returns []map[string]interface{} for direct JSON marshaling on the
// wire — keeps the eventRow struct internal.
func LoadEventsExported(db *sql.DB, conversationID string) ([]map[string]interface{}, error) {
	rows, err := loadEvents(db, conversationID)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]interface{}, 0, len(rows))
	for _, e := range rows {
		out = append(out, rawEventLine(e))
	}
	return out, nil
}
```

- [ ] **Step 4: Register routes in `internal/app/app.go`**

```go
protected.GET("/debug/sessions/:id",    debugHandler.GetSession)
protected.DELETE("/debug/sessions/:id", debugHandler.DeleteSession)
protected.PATCH("/debug/sessions/:id",  debugHandler.PatchSession)
```

- [ ] **Step 5: Run tests**

```
go build ./...
go test ./internal/handler/ -v -race
```

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/handler/debug.go internal/handler/debug_test.go internal/debug/export.go internal/app/app.go
git commit -m "handler: GET/DELETE/PATCH /api/debug/sessions/:id

GetSession returns {session, llmCalls, events} nested JSON; 404 on
unknown id. DeleteSession wraps the three-table purge in one
transaction; 204 on success, 404 on unknown id, no effect on
messages/conversations (core chat history untouched). PatchSession
is v1-only for label mutation; 400 when label missing from body,
404 on unknown id.

Task also exports LoadLLMCallsExported and LoadEventsExported from
the debug package so the handler doesn't need to re-implement the
row-loader queries.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 17: HTTP route — `GET /api/conversations/:id/export?format=raw|sharegpt`

**Files:**
- Modify: `internal/handler/debug.go`
- Modify: `internal/handler/debug_test.go`
- Modify: `internal/app/app.go`

- [ ] **Step 1: Write failing test**

Append to `internal/handler/debug_test.go`:

```go
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
```

- [ ] **Step 2: Run tests**

```
go test ./internal/handler/ -run TestExportConversation -v
```

Expected: FAIL — `ExportConversation` undefined.

- [ ] **Step 3: Add `ExportConversation` method**

In `internal/handler/debug.go`:

```go
// ExportConversation handles GET /api/conversations/:id/export.
// Query param format=raw (default) or format=sharegpt.
func (h *DebugHandler) ExportConversation(c *gin.Context) {
	id := c.Param("id")
	format := c.DefaultQuery("format", "raw")

	c.Header("Content-Type", "application/jsonl")
	c.Header("Content-Disposition", `attachment; filename="`+id+`.jsonl"`)

	switch format {
	case "raw":
		if err := debug.WriteRawJSONL(c.Writer, h.db, id); err != nil {
			h.logger.Warn("debug: ExportConversation raw failed", zap.Error(err))
			// Can't easily change status mid-stream; log only.
		}
	case "sharegpt":
		if err := debug.WriteShareGPTJSONL(c.Writer, h.db, id); err != nil {
			h.logger.Warn("debug: ExportConversation sharegpt failed", zap.Error(err))
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "format must be raw or sharegpt"})
	}
}
```

- [ ] **Step 4: Register in `internal/app/app.go`**

```go
protected.GET("/conversations/:id/export", debugHandler.ExportConversation)
```

- [ ] **Step 5: Run tests**

```
go test ./internal/handler/ -v
```

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/handler/debug.go internal/handler/debug_test.go internal/app/app.go
git commit -m "handler: GET /api/conversations/:id/export?format=raw|sharegpt

Thin wrapper over debug.WriteRawJSONL / debug.WriteShareGPTJSONL.
Sets Content-Type: application/jsonl and Content-Disposition with a
download filename derived from the conversation id. Streaming: the
writer is handed c.Writer directly so peak memory is one row.

Invalid format returns 400 before any bytes are written. Errors
mid-stream are logged but can't change the HTTP status code after
headers have been sent.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 18: HTTP route — `GET /api/debug/export-bulk`

**Files:**
- Modify: `internal/handler/debug.go`
- Modify: `internal/handler/debug_test.go`
- Modify: `internal/app/app.go`

- [ ] **Step 1: Write failing test**

Append to `internal/handler/debug_test.go`:

```go
import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
)

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
```

- [ ] **Step 2: Run test**

```
go test ./internal/handler/ -run TestExportBulk -v
```

Expected: FAIL.

- [ ] **Step 3: Add `ExportBulk` method**

In `internal/handler/debug.go`:

```go
import (
	"strconv"
	"time"
)

// ExportBulk handles GET /api/debug/export-bulk.
// Query params: format=raw|sharegpt (default sharegpt),
//               since=unix_ms (optional), until=unix_ms (optional).
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
	c.Header("Content-Type", "application/gzip")
	c.Header("Content-Disposition", `attachment; filename="debug-export.tar.gz"`)

	if err := debug.WriteBulkArchive(c.Writer, h.db, format, sinceNS, untilNS); err != nil {
		h.logger.Warn("debug: ExportBulk failed", zap.Error(err))
	}
}
```

- [ ] **Step 4: Register route**

In `internal/app/app.go`:

```go
protected.GET("/debug/export-bulk", debugHandler.ExportBulk)
```

- [ ] **Step 5: Run tests**

```
go test ./internal/handler/ -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/handler/debug.go internal/handler/debug_test.go internal/app/app.go
git commit -m "handler: GET /api/debug/export-bulk (tar.gz archive)

Wraps debug.WriteBulkArchive. Accepts format=raw|sharegpt (default
sharegpt) and optional since/until unix-ms window. Streams directly
to c.Writer; Content-Disposition filename debug-export.tar.gz.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 19: Settings endpoint — debug toggle persistence

**Files:**
- Modify: `internal/handler/settings.go` (or wherever the settings POST lives)
- Modify: `internal/handler/settings_test.go` (or create)
- Modify: whatever struct holds the runtime Sink reference (the AgentHandler likely)

- [ ] **Step 1: Locate settings save handler**

```
grep -rn "debug.enabled\|Debug.Enabled\|ApplySettings\|SaveSettings" internal/handler/ | head -20
```

Most projects have a `settings.go` handler that marshals the in-memory config back to yaml. Find it.

- [ ] **Step 2: Write failing test**

Add/append to `internal/handler/settings_test.go`:

```go
func TestSettings_TogglesDebugSink(t *testing.T) {
	// Start with sink disabled.
	fake := &fakeSink{}
	fake.SetEnabled(false) // fakeSink.SetEnabled stores the flag
	h := &SettingsHandler{debugSink: fake /*, + whatever other deps */}
	// ... build a POST body that flips debug.enabled=true
	// ... invoke h.Save
	// assert fake.Enabled() == true after the call
}
```

Adapt the fakeSink from Task 14 to actually track a boolean:

```go
type fakeSink struct {
	// ... existing fields ...
	enabled bool
}
func (f *fakeSink) SetEnabled(v bool) { f.enabled = v }
func (f *fakeSink) Enabled() bool     { return f.enabled }
```

- [ ] **Step 3: Wire `sink.SetEnabled` into the settings save path**

Find the settings POST handler. After it unmarshals the new config and writes config.yaml, add:

```go
if h.debugSink != nil {
	h.debugSink.SetEnabled(newCfg.Debug.Enabled)
}
```

The yaml file update is belt-and-suspenders: the atomic-bool flip is the source of truth for the running process. If yaml write fails, log loudly but return 200 — the live toggle is still in effect.

- [ ] **Step 4: Run test**

```
go test ./internal/handler/ -run TestSettings -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/handler/settings.go internal/handler/settings_test.go
git commit -m "settings: toggle dbSink.enabled on save

POST /api/settings (or equivalent) now flips the debug Sink's atomic
bool alongside the yaml write. Yaml failure is logged but non-fatal —
the live toggle stays the source of truth until process restart.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 20: Settings → Debug tab — HTML scaffold + CSS

**Files:**
- Modify: `web/templates/index.html`
- Modify: `web/static/css/style.css`
- Modify: `web/static/i18n/en-US.json`
- Modify: `web/static/i18n/uk-UA.json`

- [ ] **Step 1: Locate the settings modal tabs**

```
grep -n "settings-tab\|data-settings-tab\|settingsBasic\|settingsKnowledge" web/templates/index.html | head -20
```

Find the pattern used for existing settings sub-tabs (Basic / Agent / Knowledge). Copy that shape for a new "Debug" tab.

- [ ] **Step 2: Add Debug tab HTML**

Inside the settings modal, append a new tab button:

```html
<button class="settings-tab" data-settings-tab="debug" data-i18n="settingsDebug.tabTitle">Debug</button>
```

And a new tab body:

```html
<div class="settings-tab-body" data-settings-tab-body="debug" hidden>
    <div class="form-group">
        <label class="checkbox-label">
            <input type="checkbox" id="debug-enabled" class="modern-checkbox" />
            <span class="checkbox-custom"></span>
            <span class="checkbox-text" data-i18n="settingsDebug.enableToggle">Enable debug capture</span>
        </label>
        <small class="form-hint" data-i18n="settingsDebug.enableHint">
            Captures verbatim LLM requests, token usage, timestamps, and the full SSE event stream for each new conversation. No redaction — self-hosted only.
        </small>
    </div>
    <div class="form-group">
        <label for="debug-retain-days" data-i18n="settingsDebug.retainDays">Retention (days)</label>
        <input type="number" id="debug-retain-days" min="0" step="1" value="0" />
        <small class="form-hint" data-i18n="settingsDebug.retainDaysHint">
            0 = keep forever. Sessions older than N days are auto-pruned daily.
        </small>
    </div>
    <hr />
    <h4 data-i18n="settingsDebug.sessionsHeading">Captured sessions</h4>
    <div id="debug-sessions-table-wrap">
        <table id="debug-sessions-table" class="data-table">
            <thead>
                <tr>
                    <th data-i18n="settingsDebug.colStarted">Started</th>
                    <th data-i18n="settingsDebug.colLabel">Label</th>
                    <th data-i18n="settingsDebug.colOutcome">Outcome</th>
                    <th data-i18n="settingsDebug.colIterations">Iterations</th>
                    <th data-i18n="settingsDebug.colTokens">Tokens (in/out)</th>
                    <th data-i18n="settingsDebug.colDuration">Duration</th>
                    <th data-i18n="settingsDebug.colActions">Actions</th>
                </tr>
            </thead>
            <tbody id="debug-sessions-tbody"></tbody>
        </table>
    </div>
    <div class="settings-actions">
        <button class="btn-secondary" id="debug-refresh-btn" data-i18n="settingsDebug.refresh">Refresh</button>
        <button class="btn-secondary" id="debug-export-bulk-btn" data-i18n="settingsDebug.exportBulk">Export all (tar.gz)</button>
    </div>
    <div id="debug-viewer-panel" class="slide-over" hidden>
        <div class="slide-over-header">
            <span id="debug-viewer-title"></span>
            <button class="btn-icon" id="debug-viewer-close" aria-label="Close">×</button>
        </div>
        <div id="debug-viewer-body"></div>
    </div>
</div>
```

- [ ] **Step 3: Add minimal CSS**

Append to `web/static/css/style.css`:

```css
.data-table { width: 100%; border-collapse: collapse; font-size: 0.9em; }
.data-table th, .data-table td { padding: 6px 10px; border-bottom: 1px solid var(--border-color); text-align: left; }
.data-table tbody tr:hover { background: var(--hover-bg); cursor: pointer; }

.slide-over {
    position: fixed;
    top: 0; right: 0; bottom: 0;
    width: min(780px, 100vw);
    background: var(--panel-bg);
    box-shadow: -4px 0 16px rgba(0,0,0,0.22);
    z-index: 9000;
    overflow: auto;
}
.slide-over-header {
    display: flex; justify-content: space-between; align-items: center;
    padding: 12px 16px; border-bottom: 1px solid var(--border-color);
    background: var(--panel-header-bg);
}

#debug-viewer-body .debug-event   { padding: 8px 12px; border-bottom: 1px solid var(--border-subtle); font-family: monospace; font-size: 0.85em; }
#debug-viewer-body .debug-llmcall { padding: 8px 12px; border-bottom: 1px solid var(--border-subtle); background: var(--panel-alt-bg); }
#debug-viewer-body .debug-llmcall details summary { cursor: pointer; font-weight: 600; }
#debug-viewer-body pre { white-space: pre-wrap; word-break: break-word; max-height: 240px; overflow: auto; }
```

(Variable names like `--border-color` should match the project's existing CSS custom properties — skim style.css to match.)

- [ ] **Step 4: Add i18n keys**

`web/static/i18n/en-US.json` — add a new top-level section:

```json
"settingsDebug": {
  "tabTitle": "Debug",
  "enableToggle": "Enable debug capture",
  "enableHint": "Captures verbatim LLM requests, token usage, timestamps, and the full SSE event stream for each new conversation. No redaction — self-hosted only.",
  "retainDays": "Retention (days)",
  "retainDaysHint": "0 = keep forever. Sessions older than N days are auto-pruned daily.",
  "sessionsHeading": "Captured sessions",
  "colStarted": "Started",
  "colLabel": "Label",
  "colOutcome": "Outcome",
  "colIterations": "Iterations",
  "colTokens": "Tokens (in/out)",
  "colDuration": "Duration",
  "colActions": "Actions",
  "refresh": "Refresh",
  "exportBulk": "Export all (tar.gz)",
  "view": "View",
  "exportRaw": "Export raw",
  "exportShareGPT": "Export ShareGPT",
  "delete": "Delete",
  "emptyState": "No debug sessions yet — enable debug and run a conversation.",
  "deleteConfirm": "Delete captured session for {{id}}?",
  "deleteOk": "Session deleted",
  "labelPlaceholder": "Add a label…"
}
```

`web/static/i18n/uk-UA.json` — same keys with Ukrainian values:

```json
"settingsDebug": {
  "tabTitle": "Діагностика",
  "enableToggle": "Увімкнути захоплення налагоджувальних даних",
  "enableHint": "Захоплює повні запити до LLM, статистику токенів, часові мітки та весь потік SSE подій для кожної нової розмови. Без редагування — тільки для власного хостингу.",
  "retainDays": "Термін зберігання (днів)",
  "retainDaysHint": "0 = зберігати назавжди. Сеанси, старші за N днів, щоденно автоматично видаляються.",
  "sessionsHeading": "Захоплені сеанси",
  "colStarted": "Почато",
  "colLabel": "Мітка",
  "colOutcome": "Результат",
  "colIterations": "Ітерацій",
  "colTokens": "Токени (вх/вих)",
  "colDuration": "Тривалість",
  "colActions": "Дії",
  "refresh": "Оновити",
  "exportBulk": "Експортувати всі (tar.gz)",
  "view": "Переглянути",
  "exportRaw": "Експорт (raw)",
  "exportShareGPT": "Експорт (ShareGPT)",
  "delete": "Видалити",
  "emptyState": "Ще немає захоплених сеансів — увімкніть діагностику та запустіть розмову.",
  "deleteConfirm": "Видалити захоплений сеанс для {{id}}?",
  "deleteOk": "Сеанс видалено",
  "labelPlaceholder": "Додати мітку…"
}
```

- [ ] **Step 5: Manual smoke — open in browser**

Run the server:

```
go run ./cmd/server
```

Open `http://localhost:<port>/`, click Settings, confirm the new Debug tab renders without layout breakage. The tab table is empty at this point; that's correct — no JS yet.

- [ ] **Step 6: Commit**

```bash
git add web/templates/index.html web/static/css/style.css web/static/i18n/
git commit -m "ui: Settings → Debug tab scaffold (HTML + CSS + i18n)

New Settings tab button + body holding the enable-toggle, retention-
days input, sessions table, Refresh / Export-all buttons, and a
slide-over viewer panel. CSS adds a .data-table pattern and
.slide-over pattern for reuse.

i18n keys live under settingsDebug.* in both en-US and uk-UA. JS
wiring lands in Tasks 21-22.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 21: JS — toggle + sessions list rendering

**Files:**
- Modify: `web/static/js/settings.js` (or wherever the settings modal's JS lives)

- [ ] **Step 1: Locate the settings JS**

```
grep -rn "settingsBasic\|data-settings-tab\|applySettings" web/static/js/ | head -20
```

Find the function that swaps tab bodies when a tab button is clicked. Reuse it for the Debug tab.

- [ ] **Step 2: Wire the toggle + retention fields into settings save/load**

Add to the settings-load path (function that reads `/api/settings` response and populates the form):

```javascript
// In the function that loads settings into form fields:
if (cfg.debug) {
    document.getElementById('debug-enabled').checked = !!cfg.debug.enabled;
    document.getElementById('debug-retain-days').value = Number(cfg.debug.retain_days || 0);
}
```

Add to the settings-save path (the function that serializes form fields into a POST body):

```javascript
// When building the request body:
body.debug = {
    enabled:     document.getElementById('debug-enabled').checked,
    retain_days: parseInt(document.getElementById('debug-retain-days').value || '0', 10)
};
```

- [ ] **Step 3: Add sessions-list loader**

Append to `web/static/js/settings.js`:

```javascript
// Debug tab: sessions list + export/delete/view actions.
const debugTab = {
    async loadSessions() {
        const tbody = document.getElementById('debug-sessions-tbody');
        if (!tbody) return;
        tbody.innerHTML = '';
        let rows;
        try {
            const resp = await fetch('/api/debug/sessions', { credentials: 'same-origin' });
            if (!resp.ok) throw new Error('status ' + resp.status);
            rows = await resp.json();
        } catch (e) {
            tbody.innerHTML = '<tr><td colspan="7">' + escapeHtml(String(e)) + '</td></tr>';
            return;
        }
        if (rows.length === 0) {
            const emptyMsg = (typeof window.t === 'function') ? window.t('settingsDebug.emptyState') : 'No debug sessions yet.';
            tbody.innerHTML = '<tr><td colspan="7">' + escapeHtml(emptyMsg) + '</td></tr>';
            return;
        }
        for (const r of rows) {
            tbody.appendChild(debugTab.renderRow(r));
        }
    },

    renderRow(r) {
        const tr = document.createElement('tr');
        tr.dataset.conversationId = r.conversationId;

        const started = r.startedAt ? new Date(r.startedAt / 1_000_000).toISOString().replace('T', ' ').replace(/\..*/, '') : '-';
        const durSecs = r.durationMs ? Math.round(r.durationMs / 1000) : '-';
        const tokens  = (r.promptTokens || 0) + ' / ' + (r.completionTokens || 0);

        tr.innerHTML = `
            <td>${escapeHtml(started)}</td>
            <td><input class="debug-label-input" type="text" value="${escapeHtml(r.label || '')}" placeholder="${escapeHtml((typeof window.t==='function')?window.t('settingsDebug.labelPlaceholder'):'')}" /></td>
            <td>${escapeHtml(r.outcome || '')}</td>
            <td>${r.iterations || 0}</td>
            <td>${escapeHtml(tokens)}</td>
            <td>${durSecs}s</td>
            <td>
                <button class="btn-mini debug-view-btn">${escapeHtml((typeof window.t==='function')?window.t('settingsDebug.view'):'View')}</button>
                <button class="btn-mini debug-export-raw-btn">${escapeHtml((typeof window.t==='function')?window.t('settingsDebug.exportRaw'):'Raw')}</button>
                <button class="btn-mini debug-export-sg-btn">${escapeHtml((typeof window.t==='function')?window.t('settingsDebug.exportShareGPT'):'ShareGPT')}</button>
                <button class="btn-mini btn-danger debug-delete-btn">${escapeHtml((typeof window.t==='function')?window.t('settingsDebug.delete'):'Del')}</button>
            </td>
        `;
        const convID = r.conversationId;
        tr.querySelector('.debug-label-input').addEventListener('change', (e) => debugTab.saveLabel(convID, e.target.value));
        tr.querySelector('.debug-view-btn').addEventListener('click', () => debugTab.openViewer(convID));
        tr.querySelector('.debug-export-raw-btn').addEventListener('click', () => debugTab.download(convID, 'raw'));
        tr.querySelector('.debug-export-sg-btn').addEventListener('click', () => debugTab.download(convID, 'sharegpt'));
        tr.querySelector('.debug-delete-btn').addEventListener('click', () => debugTab.deleteRow(convID));
        return tr;
    },

    async saveLabel(convID, label) {
        try {
            await fetch('/api/debug/sessions/' + encodeURIComponent(convID), {
                method: 'PATCH',
                headers: { 'Content-Type': 'application/json' },
                credentials: 'same-origin',
                body: JSON.stringify({ label }),
            });
        } catch (e) { console.warn('saveLabel failed', e); }
    },

    download(convID, format) {
        // Use anchor click so Content-Disposition filename propagates.
        const a = document.createElement('a');
        a.href = '/api/conversations/' + encodeURIComponent(convID) + '/export?format=' + encodeURIComponent(format);
        document.body.appendChild(a);
        a.click();
        a.remove();
    },

    downloadBulk() {
        const a = document.createElement('a');
        a.href = '/api/debug/export-bulk?format=sharegpt';
        document.body.appendChild(a);
        a.click();
        a.remove();
    },

    async deleteRow(convID) {
        const msg = (typeof window.t === 'function')
            ? window.t('settingsDebug.deleteConfirm', { id: convID })
            : 'Delete ' + convID + '?';
        if (!confirm(msg)) return;
        const resp = await fetch('/api/debug/sessions/' + encodeURIComponent(convID), {
            method: 'DELETE',
            credentials: 'same-origin',
        });
        if (resp.status === 204 || resp.status === 404) {
            await debugTab.loadSessions();
        } else {
            alert('delete failed: ' + resp.status);
        }
    },
};

document.addEventListener('DOMContentLoaded', () => {
    const refreshBtn = document.getElementById('debug-refresh-btn');
    if (refreshBtn) refreshBtn.addEventListener('click', () => debugTab.loadSessions());
    const bulkBtn = document.getElementById('debug-export-bulk-btn');
    if (bulkBtn) bulkBtn.addEventListener('click', () => debugTab.downloadBulk());

    // Load sessions whenever the Debug tab becomes visible.
    document.querySelectorAll('[data-settings-tab="debug"]').forEach((btn) => {
        btn.addEventListener('click', () => debugTab.loadSessions());
    });
});
```

`escapeHtml` is a common helper in the codebase; if it's not in scope in this file, import/define it the same way it's done in `monitor.js` (there's an existing pattern).

- [ ] **Step 4: Manual smoke**

Restart the server, open Settings → Debug. Expected: toggle state matches `config.yaml`; empty-state message shows in the table; Refresh button re-queries `/api/debug/sessions` (check network tab). No console errors.

- [ ] **Step 5: Commit**

```bash
git add web/static/js/settings.js
git commit -m "ui: Debug tab — toggle wire-up + sessions list renderer

Settings save/load reads/writes cfg.debug.{enabled,retain_days}.
Sessions list loads on tab click or via the Refresh button; renders
one row per session with inline-editable label, per-row Export
raw/ShareGPT download, View (stub — landed in Task 22), and Delete
with a confirmation prompt.

Bulk export button downloads /api/debug/export-bulk?format=sharegpt.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 22: JS — session viewer panel (expandable LLM calls + merged events)

**Files:**
- Modify: `web/static/js/settings.js`

- [ ] **Step 1: Flesh out the `openViewer` function**

Append/replace into the `debugTab` object:

```javascript
debugTab.openViewer = async function(convID) {
    const panel = document.getElementById('debug-viewer-panel');
    const title = document.getElementById('debug-viewer-title');
    const body  = document.getElementById('debug-viewer-body');
    if (!panel || !title || !body) return;

    title.textContent = convID;
    body.innerHTML = '<div style="padding:16px">Loading…</div>';
    panel.hidden = false;

    let data;
    try {
        const resp = await fetch('/api/debug/sessions/' + encodeURIComponent(convID), { credentials: 'same-origin' });
        if (resp.status === 404) {
            body.innerHTML = '<div style="padding:16px">Session was deleted.</div>';
            return;
        }
        if (!resp.ok) throw new Error('status ' + resp.status);
        data = await resp.json();
    } catch (e) {
        body.innerHTML = '<div style="padding:16px">' + escapeHtml(String(e)) + '</div>';
        return;
    }

    // Merge llm_calls + events by timestamp.
    const items = [];
    for (const c of (data.llmCalls || [])) {
        items.push({ kind: 'llm_call', t: c.sentAt, row: c });
    }
    for (const e of (data.events || [])) {
        items.push({ kind: 'event', t: e.startedAt, row: e });
    }
    items.sort((a, b) => (a.t || 0) - (b.t || 0));

    const frag = document.createDocumentFragment();
    for (const it of items) {
        if (it.kind === 'event') {
            const d = document.createElement('div');
            d.className = 'debug-event';
            const when = it.t ? new Date(it.t / 1_000_000).toISOString().replace('T', ' ').replace(/\..*Z$/, '') : '';
            d.textContent = `[${when}] ${it.row.eventType} (agent=${it.row.agentId || '-'})`;
            frag.appendChild(d);
        } else {
            const d = document.createElement('div');
            d.className = 'debug-llmcall';
            const when = it.t ? new Date(it.t / 1_000_000).toISOString().replace('T', ' ').replace(/\..*Z$/, '') : '';
            const tokens = (it.row.promptTokens || 0) + '/' + (it.row.completionTokens || 0);
            d.innerHTML = `
                <details>
                    <summary>[${escapeHtml(when)}] LLM call — iter ${it.row.iteration || 0}, agent ${escapeHtml(it.row.agentId || '-')}, tokens ${escapeHtml(tokens)}</summary>
                    <div style="margin-top:8px">
                        <strong>Request:</strong>
                        <pre>${escapeHtml(JSON.stringify(it.row.request, null, 2))}</pre>
                        <strong>Response:</strong>
                        <pre>${escapeHtml(JSON.stringify(it.row.response, null, 2))}</pre>
                        ${it.row.error ? '<strong>Error:</strong><pre>' + escapeHtml(it.row.error) + '</pre>' : ''}
                    </div>
                </details>
            `;
            frag.appendChild(d);
        }
    }
    body.innerHTML = '';
    body.appendChild(frag);
};

// Close button.
document.addEventListener('DOMContentLoaded', () => {
    const closeBtn = document.getElementById('debug-viewer-close');
    if (closeBtn) {
        closeBtn.addEventListener('click', () => {
            const panel = document.getElementById('debug-viewer-panel');
            if (panel) panel.hidden = true;
        });
    }
});
```

- [ ] **Step 2: Manual smoke**

Enable debug, run one conversation (any kind), open Settings → Debug → click View on the row. Expected: slide-over panel shows the merged event+LLM-call timeline; each LLM call is a `<details>` element you can expand to see the verbatim request/response JSON. No console errors.

- [ ] **Step 3: Commit**

```bash
git add web/static/js/settings.js
git commit -m "ui: Debug viewer panel — merged timeline + expandable LLM calls

openViewer loads /api/debug/sessions/:id and merges llmCalls + events
by timestamp into one chronological list. Events render as compact
one-line entries; LLM calls render as collapsible <details> with
full request/response JSON on expand. 404 from the API (session
was deleted while the panel was open) shows a graceful message
instead of erroring.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 23: Final integration gate + main.go wiring polish + manual smoke

**Files:**
- Modify: `cmd/server/main.go` (finalize sink/retention/sweep wiring)
- No other code changes

- [ ] **Step 1: Verify all the main.go wiring**

Read `cmd/server/main.go` end-to-end. Confirm:

- `sink := debug.NewSink(cfg.Debug.Enabled, db, log)` is constructed after db init, before agent/handler construction.
- `debug.SweepOrphans(db, log)` runs once right after `db.Init()` (before any new session can start).
- If `cfg.Debug.RetainDays > 0`: `go debug.StartRetentionWorker(ctx, db, cfg.Debug.RetainDays, 24*time.Hour, log)` is started — pass the root context so it cancels on server shutdown.
- The sink is threaded through: Agent constructor, Orchestrator call sites, AgentHandler.

If any of these are missing, add them.

- [ ] **Step 2: Run the full gate**

```
go build ./...
go vet ./...
go test -race ./internal/debug/... ./internal/agent/... ./internal/multiagent/... ./internal/handler/... ./internal/config/... ./internal/database/...
go test -race ./...   # expect one preexisting security failure (TestExecutor_NormalizeToolArgs_RepairsMalformedHTTPFrameworkArgs); confirm it's the SAME one that fails on main before this branch
```

Expected: everything passes except the preexisting security test. Verify on a clean checkout of main that the same test fails there too — if it doesn't, something in this branch introduced the regression and must be diagnosed.

- [ ] **Step 3: Manual smoke test**

Checklist:

1. Fresh boot: server starts, `config.yaml` has `debug.enabled: false`. Settings → Debug tab loads; table is empty with the "no sessions" message. Toggle off.
2. Enable toggle, Save settings. Confirm: yaml file on disk now has `debug.enabled: true`; no server restart needed.
3. Start a multi-agent conversation (any prompt that drives a few iterations and tool calls). Let it complete.
4. Return to Settings → Debug. Refresh. Expected: one row for the just-finished conversation with non-zero iterations/tokens/duration and outcome=completed.
5. Click View. Expected: slide-over panel shows the merged event timeline; LLM call rows expand to show verbatim request/response JSON including the full system prompt, time_context block, and tool definitions.
6. Click Export raw on the row. Expected: file download `<conversationId>.jsonl`; open in an editor and confirm one line per event/call with `"source":"llm_call"` or `"source":"event"` tags.
7. Click Export ShareGPT. Expected: file download one line, shape `{"messages":[{"role":"system","content":"..."},{"role":"user",...},{"role":"assistant","content":"...","tool_calls":[...]},{"role":"tool","tool_call_id":"...","content":"..."},...,{"role":"assistant","content":"..."}]}`. Feed it to a HuggingFace trainer's dataset validator or `jq '.messages | length'` — should be > 0.
8. Bulk export: click "Export all". Expected: `debug-export.tar.gz` downloads; `tar -tzvf` lists one `.jsonl` per captured session.
9. Delete a session: click Delete, confirm. Expected: row disappears; `sqlite3 chat.db "SELECT COUNT(*) FROM debug_events WHERE conversation_id='<id>'"` returns 0.
10. Cancel test: start a new conversation, click Stop. Return to Settings → Debug → Refresh. Expected: new row with outcome=cancelled; viewer shows partial events up to the cancel point.
11. Retention test (optional): set `retain_days=1` in yaml, backdate a session row's `ended_at` via `sqlite3 chat.db "UPDATE debug_sessions SET ended_at = strftime('%s','now','-2 days')*1000000000 WHERE conversation_id='<id>'"`, restart server. Expected: that session + its events/llm_calls are gone after the immediate startup prune.
12. Turn debug off. Start a conversation. Refresh Debug tab. Expected: no new row (writes are suppressed by SetEnabled(false)).

If any step fails, diagnose and fix; repeat from Step 2.

- [ ] **Step 4: Final commit**

```bash
git add cmd/server/main.go
git commit -m "main: finalize debug wiring + retention worker + orphan sweep

Wires together the three debug-subsystem boot responsibilities in
cmd/server/main.go:

  1. debug.NewSink(cfg.Debug.Enabled, db, log) — constructed once,
     shared by the Agent, Orchestrator, and AgentHandler.
  2. debug.SweepOrphans(db, log) — runs once right after db init so
     any pre-crash live sessions are marked outcome='interrupted'
     before any new session can start.
  3. debug.StartRetentionWorker(ctx, db, cfg.Debug.RetainDays, 24h,
     log) — background goroutine pruning sessions older than
     retain_days, when retain_days > 0. Cancelled on root ctx done.

Final integration gate: go build/vet/test -race all green (modulo
the preexisting TestExecutor_NormalizeToolArgs_RepairsMalformed
HTTPFrameworkArgs failure in internal/security unrelated to this
work).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Self-review checklist (run before declaring the plan complete)

- [ ] Every task has exact file paths, complete code blocks (no "TODO" / "similar to X" / "write tests for the above").
- [ ] Every task ends in a commit step.
- [ ] Type names, function names, and property names are consistent across tasks:
  - `Sink`, `NewSink`, `noopSink`, `dbSink` (Task 3, used in 4-22)
  - `LLMCall`, `Event`, `LLMCallRow` (Task 3/10, used in 6/10-11/15-18)
  - `StartSession`, `EndSession`, `RecordLLMCall`, `RecordEvent`, `SetEnabled`, `Enabled` (Task 3, used in 5-8/14)
  - `WithCapture`, `CaptureCoords` (Task 12, used in 13)
  - `WriteRawJSONL`, `WriteShareGPTJSONL`, `WriteBulkArchive`, `ToShareGPT` (Tasks 10-11, used in 17-18)
  - `SweepOrphans`, `PruneOnce`, `StartRetentionWorker` (Tasks 8-9, used in 23)
  - `DebugHandler`, `NewDebugHandler`, `ListSessions`, `GetSession`, `DeleteSession`, `PatchSession`, `ExportConversation`, `ExportBulk` (Tasks 15-18)
  - `wrapRunWithDebug` (Task 14, used in handler.MultiAgentLoopStream/Loop)
- [ ] Every spec requirement (§Components, §Data Flow, §Error Handling, §Testing) has at least one task implementing it. Retention worker = Task 9+23. SweepOrphans = Task 8+23. ShareGPT sub-agent exclusion = Task 10. Purely post-hoc UI = Tasks 20-22. No redaction = zero code added (intentional).
- [ ] Every "capture at point X" site has a hook test verifying the sink is invoked (Tasks 7/12/13/14).
- [ ] Error-handling policy ("DB failure logged but never propagated") is demonstrated in each RecordX body (Tasks 5-7).
- [ ] Three commits end in checkpoints where a reviewer could stop and sanity-check: Task 4 (plumbing-only), Task 14 (session bookends), Task 23 (final gate).

Fix inline any gaps uncovered by the review.

---

## Execution handoff

Plan complete and saved to `docs/superpowers/plans/2026-04-22-debug-capture-feature.md`. Two execution options:

**1. Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration.

**2. Inline Execution** — I execute tasks in this session using `superpowers:executing-plans`, batch execution with checkpoints for review.

Which approach?

