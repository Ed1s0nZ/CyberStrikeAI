# Debug Capture + Training-Data Export — Design

Date: 2026-04-22
Status: Approved for planning
Author: badb@cybersec.org
Branch target: `main`

## Goal

Ship an opt-in debug subsystem that captures everything a conversation's LLM and tool stack did (verbatim requests/responses, token usage, fine-grained timestamps, full event stream), surfaces it as a post-hoc review tab in Settings, and exposes it as two export formats: raw JSONL (lossless) and ShareGPT JSONL (training-ready). Two consumers, one capture pipeline.

## Non-goals

- Live-view debug overlays on the chat timeline. The chat UI is unchanged.
- Capture-time redaction or secret scrubbing. Traces are stored verbatim. The operator owns the host and reviews before exporting off-box.
- Browser E2E tests of the Settings → Debug UI. Wire contract is tested; rendering is manually smoke-checked.
- A fake-LLM test harness on `*agent.Agent`. Already deferred by task #31; not revisited here.
- Replacing the existing `process_details` / `messages` / `tool_executions` persistence. Those stay as-is; debug tables are sidecars.

## Scope locked during brainstorming

| Dimension | Decision |
|---|---|
| Primary bottleneck | Both observability and training-data; single integrated capture pipeline |
| Enable mechanism | Opt-in global toggle in Settings; config.yaml seeds boot default, atomic.Bool holds live state |
| Extra signals captured | Token usage, verbatim LLM request (messages + tools + params), fine-grained per-event timestamps, decision-context state |
| Export formats | `/api/conversations/:id/export?format=raw\|sharegpt`; raw is source of truth, sharegpt is a derived view |
| Redaction | None — self-hosted, operator reviews before export |
| UI placement | Settings → Debug tab; chat timeline unchanged; purely post-hoc review |

## Architecture

Debug is a self-contained subsystem in a new Go package `internal/debug/`. It exposes a single `Sink` interface with two implementations (`noopSink`, `dbSink`). Every capture call site invokes the sink unconditionally; `noopSink` returns immediately when debug is off, so there's no `if enabled {` branching scattered through the orchestrator or agent.

The `enabled` state lives as an `atomic.Bool` on the active sink. The Settings toggle endpoint flips it at runtime; the sink's existing implementation does not need to change. A conversation's capture behavior is fixed at `StartSession` time — flipping debug off mid-run does not stop capture for already-started conversations, and flipping on mid-run does not retroactively enable capture for them.

```
┌─────────────────────┐   off: no-op    ┌──────────────────────┐
│ config.debug.enabled│────────────────▶│ debug.Sink interface │
└─────────────────────┘    on: record   └──────────┬───────────┘
                                                   │
                 ┌─────────────────────────────────┼────────────────────┐
                 ▼                 ▼               ▼                    ▼
         ┌──────────────┐  ┌──────────────┐ ┌─────────────┐     ┌──────────────┐
         │ openai wrap  │  │ sendProgress │ │ handleMCP   │     │ ExecuteMCP   │
         │ (req/resp,   │  │  tee         │ │  Tool tee   │     │  ToolFor     │
         │  tokens, t)  │  │              │ │             │     │  Conversation│
         └──────┬───────┘  └──────┬───────┘ └─────┬───────┘     └──────┬───────┘
                └──────┬──────────┴───────────────┴────────────────────┘
                       ▼
           ┌───────────────────────────────┐
           │  dbSink writes debug_llm_calls│
           │  / debug_events / debug_sessions
           └───────────────┬───────────────┘
                           │
        ┌──────────────────┼─────────────────────┐
        ▼                                        ▼
┌──────────────────────────┐           ┌──────────────────────────┐
│ Settings → Debug tab     │           │ GET /api/conversations/  │
│ (sessions list,          │           │ :id/export?format=       │
│  per-session viewer,     │           │   raw|sharegpt           │
│  export buttons)         │           │                          │
└──────────────────────────┘           └──────────────────────────┘
```

## Components

### Config

New struct in `internal/config/config.go`, referenced as `Config.Debug`:

```go
type DebugConfig struct {
    Enabled    bool `yaml:"enabled"`      // default false; boot-time seed for the runtime toggle
    RetainDays int  `yaml:"retain_days"`  // 0 = keep forever; default 0
}
```

`config.example.yaml`:

```yaml
debug:
  enabled: false
  retain_days: 0
```

### Go package `internal/debug/`

| File | Purpose |
|---|---|
| `sink.go` | `Sink` interface, `noopSink{}`, `dbSink{db, enabled atomic.Bool, seqByConv sync.Map}`, `NewSink(enabled bool, db *sql.DB, log *zap.Logger) Sink` |
| `capture.go` | Value types `LLMCall`, `Event` and helpers for marshaling verbatim request/response |
| `session.go` | `StartSession(convID)`, `EndSession(convID, outcome)`, boot-time `SweepOrphans(db)` |
| `converter.go` | Pure function `ToShareGPT(llmCalls []LLMCall) ([]byte, error)` |
| `export.go` | Streaming writers `WriteRawJSONL(w, convID)`, `WriteShareGPTJSONL(w, convID)`, `WriteBulkArchive(w, filter)` |
| `retention.go` | `StartRetentionWorker(db, retainDays int, interval time.Duration)` goroutine |
| `sink_test.go`, `converter_test.go`, `export_test.go`, `retention_test.go` | Unit coverage per section 5 |

`Sink` shape:

```go
type Sink interface {
    StartSession(convID string)
    EndSession(convID, outcome string)
    RecordLLMCall(convID, msgID string, c LLMCall)
    RecordEvent(convID, msgID string, e Event)
    SetEnabled(bool)
    Enabled() bool
}
```

Tool dispatch is captured without a dedicated `RecordToolCall` method: the existing `tool_executions` table already records tool name / args / result / duration, and the `tool_call` + `tool_result` SSE events route through `RecordEvent` for the fine-grained timestamp + payload capture. No third source of truth for tools.

### DB migrations

Added to the existing `CREATE TABLE IF NOT EXISTS` sweep in `internal/database/database.go`:

```sql
CREATE TABLE IF NOT EXISTS debug_sessions (
    conversation_id TEXT PRIMARY KEY,
    started_at INTEGER NOT NULL,         -- unix nanos
    ended_at   INTEGER,                  -- unix nanos; NULL while live
    outcome    TEXT,                     -- completed|cancelled|failed|interrupted|NULL-if-live
    label      TEXT                      -- optional user-set label, post-hoc
);

CREATE TABLE IF NOT EXISTS debug_llm_calls (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    conversation_id   TEXT NOT NULL,
    message_id        TEXT,
    iteration         INTEGER,
    call_index        INTEGER,
    agent_id          TEXT,                -- "cyberstrike-orchestrator" or sub-agent id
    sent_at           INTEGER NOT NULL,
    first_token_at    INTEGER,             -- NULL for non-streaming
    finished_at       INTEGER,
    prompt_tokens     INTEGER,
    completion_tokens INTEGER,
    request_json      TEXT NOT NULL,       -- full messages[] + tools[] + params
    response_json     TEXT NOT NULL,       -- full choices + finish_reason
    error             TEXT
);
CREATE INDEX idx_debug_llm_calls_conv ON debug_llm_calls(conversation_id, sent_at);

CREATE TABLE IF NOT EXISTS debug_events (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    conversation_id TEXT NOT NULL,
    message_id      TEXT,
    seq             INTEGER NOT NULL,      -- monotonic per conversation, assigned by sink
    event_type      TEXT NOT NULL,
    agent_id        TEXT,
    payload_json    TEXT NOT NULL,
    started_at      INTEGER NOT NULL,
    finished_at     INTEGER                -- for events with duration (tool span)
);
CREATE INDEX idx_debug_events_conv ON debug_events(conversation_id, seq);
```

### HTTP routes

All on the existing `/api` group:

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/api/debug/sessions` | paginated session list; per-row summary fields (`iterations`, `promptTokens`, `completionTokens`, `durationMs`) are computed at query time by SQL aggregates over `debug_llm_calls` (COUNT DISTINCT iteration, SUM tokens, finished_at - started_at) — they are not stored on `debug_sessions` |
| `GET` | `/api/debug/sessions/:id` | full capture for one conversation: session row + llm_calls + events |
| `DELETE` | `/api/debug/sessions/:id` | purge debug rows for one session; does not touch messages / conversations |
| `PATCH` | `/api/debug/sessions/:id` | set `label` |
| `GET` | `/api/conversations/:id/export?format=raw\|sharegpt` | single-session JSONL download |
| `GET` | `/api/debug/export-bulk?since=...&until=...&format=...` | tar.gz archive; one JSONL per session |

The existing settings-save handler gains fields for `debug.enabled` and `debug.retain_days`; on save it both flips the live atomic.Bool on the sink and writes the new values into `config.yaml` (logging a warning if the yaml write fails — the live toggle still works for this process lifetime).

### Wire-in points

Three small hooks into code already touched in tasks #27 and #30:

1. `internal/agent/agent.go` — around `callOpenAIStreamWithToolCalls` (and the non-streaming / text variants): wrap the openai.Client call with a capture scope that snapshots request pre-call, captures first-token and completion timestamps, extracts token usage from the response, and calls `sink.RecordLLMCall`.
2. `internal/multiagent/orchestrator.go` — inside `sendProgress`: tee every emission into `sink.RecordEvent` with a monotonic seq and the current `agent_id`.
3. `internal/handler/agent.go` — in `MultiAgentLoopStream` (and the non-streaming `MultiAgentLoop` counterpart): `sink.StartSession(convID)` after `prepareMultiAgentSession`, `sink.EndSession(convID, outcome)` in the defer that finalizes `taskStatus`.

### UI — Settings → Debug tab

Single new tab inside the existing settings modal in `web/templates/index.html` + handler in `web/static/js/settings.js`:

- Toggle row: "Enable debug capture" checkbox, retention-days number input, Save posts to existing settings endpoint.
- Sessions table: columns = `started_at`, `label` (editable inline), `outcome`, `iterations`, `prompt + completion tokens`, `duration`, per-row actions (View, Export raw, Export ShareGPT, Delete).
- Per-session viewer: slide-over panel reusing existing modal patterns. Renders each `debug_event` in chronological order (merged with `debug_llm_calls` by timestamp), with expandable "LLM request / response" blocks per LLM call row.

No changes to chat UI.

## Data Flow

### Boot

`cmd/server/main.go` after config load and `database.Init()`:

```go
sink := debug.NewSink(cfg.Debug.Enabled, db, logger)
debug.SweepOrphans(db, logger)      // mark pre-existing NULL-ended sessions as "interrupted"
if cfg.Debug.RetainDays > 0 {
    go debug.StartRetentionWorker(db, cfg.Debug.RetainDays, 24*time.Hour)
}
// sink passed to: agentHandler, agent.NewAgent, multiagent.RunOrchestrator
```

### Runtime toggle flip

Settings UI save → handler updates in-memory config → calls `sink.SetEnabled(newValue)` → writes new `config.yaml`. If yaml write fails, log loudly; the live toggle still works until restart.

### Per-conversation capture (main path)

`MultiAgentLoopStream`:

```
1. prepareMultiAgentSession(req)        → convID, assistantMessageID
2. sink.StartSession(convID)
3. tasks.StartTask(convID, ...)
4. RunOrchestrator(ctx, ..., sink)
5. defer sink.EndSession(convID, outcome)
```

Inside `RunOrchestrator`, iteration `i`:

```
a. o.sendProgress("iteration", ...)
     ↳ sink.RecordEvent(seq=N, type="iteration", payload=data)
b. ag.CallStreamWithToolCalls(ctx, msgs, tools, cb)
     ↳ [agent.go openai wrapper]
       sink.RecordLLMCall({sent_at, first_token_at, finished_at,
                          prompt_tokens, completion_tokens,
                          request_json, response_json})
c. for each tc in toolCalls:
     o.sendProgress("tool_call", ...)             → sink.RecordEvent(seq++, type="tool_call")
     result := o.handleMCPTool(tc, ...)
       ↳ sink.RecordToolCall({args, result, duration})
     o.sendProgress("tool_result", ...)           → sink.RecordEvent(seq++, type="tool_result")
```

Sub-agent dispatch (`handleTaskCall` → `runSubAgent`) runs the same pattern with `agent_id = <sub-agent-id>`. `call_index` is monotonic per conversation across the full (orchestrator + sub-agents) span so the interleaving is reconstructable by time order.

### Cancellation mid-run

`baseCtx` cancelled with `ErrTaskCancelled`. Orchestrator returns; `MultiAgentLoopStream` defer runs `sink.EndSession(convID, "cancelled")`. The messages-invariant fix from task #29 means tool-role entries for not-yet-executed tool calls are already synthesized as "Tool call cancelled" messages, so the captured trace is consistent with what the LLM observed.

### Post-hoc review

User opens Settings → Debug. `GET /api/debug/sessions` returns summary rows. Click one → `GET /api/debug/sessions/:id` returns:

```json
{
  "session":  { "conversationId": "...", "label": "...", "startedAt": ..., "endedAt": ..., "outcome": "completed" },
  "llmCalls": [ { "id": 1, "iteration": 0, "agentId": "cyberstrike-orchestrator",
                 "sentAt": ..., "firstTokenAt": ..., "finishedAt": ...,
                 "promptTokens": ..., "completionTokens": ...,
                 "request": { ... }, "response": { ... }, "error": null }, ... ],
  "events":   [ { "id": 1, "seq": 0, "type": "iteration", "agentId": "...",
                 "payload": { ... }, "startedAt": ..., "finishedAt": null }, ... ]
}
```

Viewer merges `events` and `llmCalls` by timestamp into one vertical timeline.

### Export

`GET /api/conversations/:id/export?format=sharegpt`:

1. Load `debug_llm_calls` for convID where `agent_id = "cyberstrike-orchestrator"` ordered by `sent_at`. Sub-agent LLM calls are excluded from ShareGPT output — they're internal delegation detail, not user-facing dialogue. (Operators who want the sub-agent traces use `format=raw`.)
2. Pick the terminal call: the first call whose `response.choices[0].finish_reason == "stop"`, i.e. the last round that produced a final text answer rather than more tool calls. If no such call exists (the orchestrator hit `maxIter` without a clean stop), pick the last call in `sent_at` order.
3. That call's `request.messages` is the exact history the model saw on its final turn — with all earlier assistant `tool_calls` and `tool`-role responses already interleaved by the orchestrator's message-append loop.
4. Output = `{"messages": terminalCall.request.messages + [{role: "assistant", ...terminalCall.response.choices[0].message}]}` as one JSONL line.

`?format=raw`:

1. Stream `debug_llm_calls` + `debug_events` merged by timestamp; each line is `{"source": "llm_call|event", ...row}`. Sub-agent calls included (tagged by `agent_id`).

Bulk (`/api/debug/export-bulk`) iterates sessions matching the since/until range, writes each as an entry in a `tar.gz` stream using `archive/tar` + `compress/gzip` over the response body.

### Auto-prune

`retention.StartRetentionWorker` runs daily: `DELETE FROM debug_events/debug_llm_calls/debug_sessions WHERE conversation_id IN (SELECT conversation_id FROM debug_sessions WHERE ended_at < now_nanos - retain_days*24h_in_nanos)`.

## Error Handling

### Debug write failure mid-conversation

Each `sink.Record*` wraps its DB op, logs failure at `warn`, returns without error. Consequence: event missing from capture; conversation unaffected.

### Server crash → orphan session

Row has `ended_at=NULL`, `outcome=NULL`. Boot-time `SweepOrphans` updates such rows to `outcome="interrupted"` with `ended_at` = latest event's `finished_at` for that conversation (or `started_at` if no events recorded).

### Toggle flipped mid-run

A conversation's behavior is bound at `StartSession` time. Flipping mid-run affects only new conversations.

### Concurrent writes within one conversation

SQLite's connection-pool serialization is sufficient for v1 sub-agent parallelism volumes. `seq` is assigned from an atomic counter on the sink keyed by convID (stored in `seqByConv sync.Map` → `*atomic.Int64`), preserving order regardless of DB write interleaving.

### Huge session export → memory

Handlers stream; never buffer full session. Row-level encoding directly to `http.ResponseWriter`. Peak memory = one row.

### Disk-full during retention

DELETE failure logged at warn; next day retries. Tables allowed to exceed nominal retention briefly.

### Config / live-toggle divergence

`config.yaml` = boot default only. Atomic bool on sink = source of truth during process lifetime. Yaml write on toggle save is durability belt-and-suspenders; yaml write failure is logged but does not block the toggle.

### Empty tables (feature never enabled)

`GET /api/debug/sessions` returns `[]` with 200. UI renders empty-state message. Tables always exist thanks to `CREATE TABLE IF NOT EXISTS` at boot regardless of enabled flag.

### Delete while viewer open

DELETE is atomic. Viewer's next interaction sees 404; client shows "Session deleted" toast and closes panel.

## Testing

### `internal/debug/` unit tests

- `TestNoopSink_SilentNoop` — `noopSink.Record*` works with nil DB.
- `TestDBSink_RecordLLMCall_PersistsRow` — in-memory SQLite; column round-trip.
- `TestDBSink_RecordEvent_MonotonicSeq` — 10 concurrent events, seq in {0..9} no dups.
- `TestDBSink_StartEndSession_HappyPath` — end with outcome="completed".
- `TestDBSink_SetEnabled_Runtime` — writes suppressed after `SetEnabled(false)`.
- `TestConverter_ShareGPT_RoundTrip` — fixture → expected JSONL shape; tool_calls↔tool pairing by id.
- `TestConverter_RawJSONL_StreamsInTimeOrder` — output is timestamp-sorted, each line tagged with `source`.
- `TestExport_Bulk_TarGzStructure` — tar entries named per convID, each entry is valid JSONL.
- `TestRetention_Sweep_DeletesOnlyOld` — old-dated session pruned, fresh one untouched.

### `internal/agent/` and `internal/multiagent/` hook tests

- `TestAgent_CallStreamWithToolCalls_RecordsLLMCall_WhenDebugOn` — mocked openai HTTP; one row in `debug_llm_calls`.
- `TestAgent_CallStreamWithToolCalls_NoWrite_WhenDebugOff` — `noopSink`; zero rows.
- `TestOrchestrator_sendProgress_TeesToSink` — direct `sendProgress` call; matching row in `debug_events`.
- `TestOrchestrator_EndSession_OnCancel` — `outcome="cancelled"`.

### HTTP handler tests

Table-driven via `httptest.NewServer`:

- `GET /api/debug/sessions` — empty, paginated, since/until filter.
- `GET /api/debug/sessions/:id` — 404 unknown, 200 known.
- `DELETE /api/debug/sessions/:id` — 404 unknown, 204 known; rows gone from all three tables.
- `PATCH /api/debug/sessions/:id` — label persisted.
- `GET /api/conversations/:id/export?format=raw|sharegpt` — valid JSONL; invalid format → 400.
- `GET /api/debug/export-bulk` — valid tar.gz.
- Settings POST toggling `debug.enabled` — atomic bool flipped; yaml updated.

### Orphan-sweep test

- `TestBootSweep_MarksOrphansInterrupted` — pre-seed NULL-ended row; `SweepOrphans` sets `outcome="interrupted"`, populates `ended_at`.

### Explicitly out of scope for v1

- Browser E2E of Settings → Debug UI (manual smoke).
- Full orchestrator happy-path integration with real LLM (#31 deferred, still deferred).
- Concurrent multi-conversation stress test (waits on actual parallel sub-agent dispatch).

### Toolchain gate before ship

```
go build ./...
go vet ./...
go test -race ./internal/debug/... ./internal/agent/... ./internal/multiagent/... ./internal/handler/...
go test -race ./...        # ignore the known-failing internal/security test unrelated to this work
```

Manual smoke: enable debug, run a multi-agent conversation, verify the Settings viewer renders the session, export both formats, diff raw JSONL against `jq` over the tables, confirm sharegpt JSONL loads in a HuggingFace trainer's dataset validator.

## Implementation order (sketch; detailed plan follows in writing-plans)

1. Config struct + `config.example.yaml` + yaml load/save path for the new fields.
2. DB migrations + `SweepOrphans` + retention worker.
3. `internal/debug/` package: `Sink` interface, two implementations, capture types, unit tests.
4. Converter + export writers + unit tests.
5. Hook points in `agent.go`, `orchestrator.go`, `handler/agent.go`.
6. HTTP handlers + route wiring + table-driven tests.
7. Settings → Debug UI.
8. Integration pass + toolchain gate + manual smoke.

Each step is committable on its own; steps 3–4 could be parallelized after step 2 lands.
