package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// CairnCanonicalSchemaVersion identifies SQLite-backed Cairn state. YAML v2 remains legacy input only.
const CairnCanonicalSchemaVersion = 1

const (
	CairnRunQueued      = "queued"
	CairnRunPlanning    = "planning"
	CairnRunExecuting   = "executing"
	CairnRunReplanning  = "replanning"
	CairnRunInterrupted = "interrupted"
	CairnRunBlocked     = "blocked"
	CairnRunCompleted   = "completed"
	CairnRunFailed      = "failed"
	CairnRunCancelled   = "cancelled"
	CairnRunTimedOut    = "timed_out"

	CairnIntentOpen        = "open"
	CairnIntentInProgress  = "in_progress"
	CairnIntentSucceeded   = "succeeded"
	CairnIntentRetryable   = "retryable"
	CairnIntentFailed      = "failed"
	CairnIntentDropped     = "dropped"
	CairnIntentCancelled   = "cancelled"
	CairnIntentInterrupted = "interrupted"
	CairnIntentSuperseded  = "superseded"
)

// CairnRun is canonical durable state for one serial Fact-Intent-Hint execution.
type CairnRun struct {
	ID                string
	ProjectID         string
	ConversationID    string
	GoalText          string
	Mode              string
	Status            string
	ScopeSnapshotJSON string
	ScopeSHA256       string
	ProjectRevision   int64
	Revision          int64
	LastEventSeq      int64
	TerminalCode      string
	TerminalDetail    string
	CancelRequestedAt *time.Time
	LeaseOwner        string
	LeaseUntil        *time.Time
	CreatedAt         time.Time
	StartedAt         *time.Time
	FinishedAt        *time.Time
	UpdatedAt         time.Time
}

// CairnIntent stores an intent by canonical object ID. Description is display-only, never identity.
type CairnIntent struct {
	ID             string
	RunID          string
	Description    string
	Status         string
	Ordinal        int
	AttemptCount   int
	LastErrorCode  string
	CreatedEventID string
	CreatedAt      time.Time
	StartedAt      *time.Time
	FinishedAt     *time.Time
	UpdatedAt      time.Time
}

// CairnFact is a validated observation. Errors, cancellation, timeout and empty output never use this type.
type CairnFact struct {
	ID              string
	RunID           string
	SourceIntentID  string
	SourceSegmentID string
	SourceEventID   string
	Statement       string
	ObservationType string
	Confidence      string
	EvidenceJSON    string
	ProvenanceJSON  string
	CreatedAt       time.Time
}

// CairnEvent is append-only audit evidence. Mutations and outbox rows commit atomically with it.
type CairnEvent struct {
	ID             string
	RunID          string
	Sequence       int64
	SegmentID      string
	IntentID       string
	InvocationID   string
	EventType      string
	IdempotencyKey string
	PayloadJSON    string
	CreatedAt      time.Time
}

// CairnOutboxItem projects canonical events to legacy project facts and YAML exports.
type CairnOutboxItem struct {
	EventID     string
	Consumer    string
	Status      string
	Attempts    int
	AvailableAt time.Time
	LockedBy    string
	LockedUntil *time.Time
	LastError   string
	DeliveredAt *time.Time
}

// CairnHint is a canonical Reason hint. It is read-only until the runtime cutover.
type CairnHint struct {
	ID             string
	RunID          string
	Content        string
	CreatorType    string
	CreatorID      string
	Status         string
	CreatedEventID string
	CreatedAt      time.Time
	ResolvedAt     *time.Time
}

// CairnEdge is a canonical Fact-Intent-Hint graph edge.
type CairnEdge struct {
	ID            string
	RunID         string
	SourceNodeID  string
	TargetNodeID  string
	EdgeType      string
	SourceEventID string
	CreatedAt     time.Time
}

// CairnStateSnapshot is a consistent, read-only view of one conversation's latest canonical run.
type CairnStateSnapshot struct {
	SchemaVersion int
	Initialized   bool
	Run           *CairnRun
	Facts         []CairnFact
	Intents       []CairnIntent
	Hints         []CairnHint
	Edges         []CairnEdge
}

func cairnNow() time.Time { return time.Now().UTC() }

func cairnJSONOrEmpty(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "{}", nil
	}
	var v interface{}
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return "", fmt.Errorf("invalid Cairn JSON: %w", err)
	}
	return raw, nil
}

func validCairnRunStatus(status string) bool {
	switch status {
	case CairnRunQueued, CairnRunPlanning, CairnRunExecuting, CairnRunReplanning, CairnRunInterrupted, CairnRunBlocked, CairnRunCompleted, CairnRunFailed, CairnRunCancelled, CairnRunTimedOut:
		return true
	default:
		return false
	}
}

func validCairnIntentStatus(status string) bool {
	switch status {
	case CairnIntentOpen, CairnIntentInProgress, CairnIntentSucceeded, CairnIntentRetryable, CairnIntentFailed, CairnIntentDropped, CairnIntentCancelled, CairnIntentInterrupted, CairnIntentSuperseded:
		return true
	default:
		return false
	}
}

func isTerminalCairnRun(status string) bool {
	switch status {
	case CairnRunCompleted, CairnRunFailed, CairnRunCancelled, CairnRunTimedOut:
		return true
	default:
		return false
	}
}

// initCairnTables is additive. It creates durable state without switching current YAML traffic.
func (db *DB) initCairnTables() error {
	ddls := []string{
		`CREATE TABLE IF NOT EXISTS cairn_schema_meta (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at DATETIME NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS cairn_runs (
			id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL,
			conversation_id TEXT NOT NULL,
			goal_text TEXT NOT NULL,
			mode TEXT NOT NULL DEFAULT 'serial' CHECK (mode = 'serial'),
			status TEXT NOT NULL CHECK (status IN ('queued','planning','executing','replanning','interrupted','blocked','completed','failed','cancelled','timed_out')),
			scope_snapshot_json TEXT NOT NULL CHECK (json_valid(scope_snapshot_json)),
			scope_sha256 TEXT NOT NULL,
			project_revision INTEGER NOT NULL DEFAULT 0,
			revision INTEGER NOT NULL DEFAULT 0,
			last_event_seq INTEGER NOT NULL DEFAULT 0,
			terminal_code TEXT,
			terminal_detail TEXT,
			cancel_requested_at DATETIME,
			lease_owner TEXT,
			lease_until DATETIME,
			created_at DATETIME NOT NULL,
			started_at DATETIME,
			finished_at DATETIME,
			updated_at DATETIME NOT NULL,
			FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE RESTRICT,
			FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE RESTRICT
		);`,
		`CREATE TABLE IF NOT EXISTS cairn_nodes (
			node_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			node_type TEXT NOT NULL CHECK (node_type IN ('fact','intent','hint')),
			created_at DATETIME NOT NULL,
			PRIMARY KEY (node_id, run_id),
			FOREIGN KEY (run_id) REFERENCES cairn_runs(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS cairn_intents (
			intent_id TEXT PRIMARY KEY,
			run_id TEXT NOT NULL,
			description TEXT NOT NULL,
			status TEXT NOT NULL CHECK (status IN ('open','in_progress','succeeded','retryable','failed','dropped','cancelled','interrupted','superseded')),
			ordinal INTEGER NOT NULL,
			attempt_count INTEGER NOT NULL DEFAULT 0,
			last_error_code TEXT,
			created_event_id TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			started_at DATETIME,
			finished_at DATETIME,
			updated_at DATETIME NOT NULL,
			FOREIGN KEY (intent_id, run_id) REFERENCES cairn_nodes(node_id, run_id) ON DELETE CASCADE,
			UNIQUE (run_id, ordinal)
		);`,
		`CREATE TABLE IF NOT EXISTS cairn_facts (
			fact_id TEXT PRIMARY KEY,
			run_id TEXT NOT NULL,
			source_intent_id TEXT NOT NULL,
			source_segment_id TEXT NOT NULL,
			source_event_id TEXT NOT NULL UNIQUE,
			statement TEXT NOT NULL,
			observation_type TEXT NOT NULL CHECK (observation_type IN ('positive','negative','derived')),
			confidence TEXT NOT NULL CHECK (confidence IN ('tentative','confirmed')),
			evidence_json TEXT NOT NULL CHECK (json_valid(evidence_json)),
			provenance_json TEXT NOT NULL CHECK (json_valid(provenance_json)),
			created_at DATETIME NOT NULL,
			FOREIGN KEY (fact_id, run_id) REFERENCES cairn_nodes(node_id, run_id) ON DELETE CASCADE,
			FOREIGN KEY (source_intent_id) REFERENCES cairn_intents(intent_id) ON DELETE RESTRICT
		);`,
		`CREATE TABLE IF NOT EXISTS cairn_hints (
			hint_id TEXT PRIMARY KEY,
			run_id TEXT NOT NULL,
			content TEXT NOT NULL,
			creator_type TEXT NOT NULL,
			creator_id TEXT,
			status TEXT NOT NULL CHECK (status IN ('active','applied','dismissed')),
			created_event_id TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			resolved_at DATETIME,
			FOREIGN KEY (hint_id, run_id) REFERENCES cairn_nodes(node_id, run_id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS cairn_edges (
			edge_id TEXT PRIMARY KEY,
			run_id TEXT NOT NULL,
			source_node_id TEXT NOT NULL,
			target_node_id TEXT NOT NULL,
			edge_type TEXT NOT NULL,
			source_event_id TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			UNIQUE (run_id, source_node_id, target_node_id, edge_type),
			FOREIGN KEY (run_id) REFERENCES cairn_runs(id) ON DELETE CASCADE,
			FOREIGN KEY (source_node_id, run_id) REFERENCES cairn_nodes(node_id, run_id) ON DELETE CASCADE,
			FOREIGN KEY (target_node_id, run_id) REFERENCES cairn_nodes(node_id, run_id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS cairn_segments (
			segment_id TEXT PRIMARY KEY,
			run_id TEXT NOT NULL,
			sequence INTEGER NOT NULL,
			phase TEXT NOT NULL CHECK (phase IN ('reason','explore')),
			intent_id TEXT,
			attempt INTEGER NOT NULL,
			status TEXT NOT NULL CHECK (status IN ('started','succeeded','failed','cancelled','timed_out','interrupted','no_fact')),
			error_code TEXT,
			error_detail TEXT,
			partial_output TEXT,
			started_at DATETIME NOT NULL,
			finished_at DATETIME,
			UNIQUE (run_id, sequence),
			FOREIGN KEY (run_id) REFERENCES cairn_runs(id) ON DELETE CASCADE,
			FOREIGN KEY (intent_id) REFERENCES cairn_intents(intent_id) ON DELETE RESTRICT
		);`,
		`CREATE TABLE IF NOT EXISTS cairn_invocations (
			invocation_id TEXT PRIMARY KEY,
			run_id TEXT NOT NULL,
			segment_id TEXT NOT NULL,
			intent_id TEXT,
			invocation_type TEXT NOT NULL CHECK (invocation_type IN ('model','tool')),
			tool_name TEXT,
			batch_sequence INTEGER,
			index_in_batch INTEGER,
			replay_of TEXT,
			idempotency_class TEXT NOT NULL CHECK (idempotency_class IN ('safe_retry','deduplicated','unsafe','unknown')),
			status TEXT NOT NULL CHECK (status IN ('started','succeeded','failed','cancelled','timed_out','interrupted','unknown')),
			request_json TEXT,
			response_json TEXT,
			error_code TEXT,
			error_detail TEXT,
			started_at DATETIME NOT NULL,
			finished_at DATETIME,
			FOREIGN KEY (run_id) REFERENCES cairn_runs(id) ON DELETE CASCADE,
			FOREIGN KEY (segment_id) REFERENCES cairn_segments(segment_id) ON DELETE CASCADE,
			FOREIGN KEY (intent_id) REFERENCES cairn_intents(intent_id) ON DELETE RESTRICT,
			FOREIGN KEY (replay_of) REFERENCES cairn_invocations(invocation_id) ON DELETE SET NULL
		);`,
		`CREATE TABLE IF NOT EXISTS cairn_checkpoints (
			checkpoint_id TEXT PRIMARY KEY,
			run_id TEXT NOT NULL,
			segment_id TEXT,
			run_revision INTEGER NOT NULL,
			boundary TEXT NOT NULL,
			state_json TEXT NOT NULL CHECK (json_valid(state_json)),
			adk_checkpoint_blob BLOB,
			created_at DATETIME NOT NULL,
			FOREIGN KEY (run_id) REFERENCES cairn_runs(id) ON DELETE CASCADE,
			FOREIGN KEY (segment_id) REFERENCES cairn_segments(segment_id) ON DELETE SET NULL
		);`,
		`CREATE TABLE IF NOT EXISTS cairn_events (
			event_id TEXT PRIMARY KEY,
			run_id TEXT NOT NULL,
			sequence INTEGER NOT NULL,
			segment_id TEXT,
			intent_id TEXT,
			invocation_id TEXT,
			event_type TEXT NOT NULL,
			idempotency_key TEXT NOT NULL UNIQUE,
			payload_json TEXT NOT NULL CHECK (json_valid(payload_json)),
			created_at DATETIME NOT NULL,
			UNIQUE (run_id, sequence),
			FOREIGN KEY (run_id) REFERENCES cairn_runs(id) ON DELETE CASCADE,
			FOREIGN KEY (segment_id) REFERENCES cairn_segments(segment_id) ON DELETE SET NULL,
			FOREIGN KEY (intent_id) REFERENCES cairn_intents(intent_id) ON DELETE SET NULL,
			FOREIGN KEY (invocation_id) REFERENCES cairn_invocations(invocation_id) ON DELETE SET NULL
		);`,
		`CREATE TABLE IF NOT EXISTS cairn_outbox (
			event_id TEXT NOT NULL,
			consumer TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','processing','delivered','dead')),
			attempts INTEGER NOT NULL DEFAULT 0,
			available_at DATETIME NOT NULL,
			locked_by TEXT,
			locked_until DATETIME,
			last_error TEXT,
			delivered_at DATETIME,
			PRIMARY KEY (event_id, consumer),
			FOREIGN KEY (event_id) REFERENCES cairn_events(event_id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS cairn_legacy_imports (
			import_id TEXT PRIMARY KEY,
			source_path TEXT NOT NULL,
			source_sha256 TEXT NOT NULL,
			schema_version INTEGER NOT NULL,
			status TEXT NOT NULL,
			report_json TEXT NOT NULL CHECK (json_valid(report_json)),
			imported_at DATETIME,
			UNIQUE (source_path, source_sha256)
		);`,
		`CREATE TABLE IF NOT EXISTS cairn_legacy_executions (
			legacy_execution_id TEXT PRIMARY KEY,
			run_id TEXT NOT NULL,
			import_id TEXT NOT NULL,
			legacy_node_id TEXT,
			raw_description TEXT NOT NULL,
			raw_source_intent TEXT,
			classification TEXT NOT NULL CHECK (classification IN ('error','stub','max_iterations','empty_result','unresolved')),
			provenance_json TEXT NOT NULL CHECK (json_valid(provenance_json)),
			created_at DATETIME NOT NULL,
			FOREIGN KEY (run_id) REFERENCES cairn_runs(id) ON DELETE CASCADE,
			FOREIGN KEY (import_id) REFERENCES cairn_legacy_imports(import_id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS cairn_migration_issues (
			issue_id TEXT PRIMARY KEY,
			import_id TEXT NOT NULL,
			severity TEXT NOT NULL,
			issue_code TEXT NOT NULL,
			legacy_node_id TEXT,
			detail_json TEXT NOT NULL CHECK (json_valid(detail_json)),
			resolved_at DATETIME,
			FOREIGN KEY (import_id) REFERENCES cairn_legacy_imports(import_id) ON DELETE CASCADE
		);`,
		`CREATE UNIQUE INDEX IF NOT EXISTS ux_cairn_active_conversation ON cairn_runs(conversation_id) WHERE status IN ('queued','planning','executing','replanning','interrupted','blocked');`,
		`CREATE UNIQUE INDEX IF NOT EXISTS ux_cairn_one_active_intent ON cairn_intents(run_id) WHERE status = 'in_progress';`,
		`CREATE INDEX IF NOT EXISTS idx_cairn_runs_project_status ON cairn_runs(project_id, status, updated_at);`,
		`CREATE INDEX IF NOT EXISTS idx_cairn_intents_run_status ON cairn_intents(run_id, status, ordinal);`,
		`CREATE INDEX IF NOT EXISTS idx_cairn_facts_run_created ON cairn_facts(run_id, created_at);`,
		`CREATE INDEX IF NOT EXISTS idx_cairn_events_run_sequence ON cairn_events(run_id, sequence);`,
		`CREATE INDEX IF NOT EXISTS idx_cairn_outbox_ready ON cairn_outbox(status, available_at);`,
	}
	for _, ddl := range ddls {
		if _, err := db.Exec(ddl); err != nil {
			return fmt.Errorf("create Cairn schema: %w", err)
		}
	}
	_, err := db.Exec(`INSERT INTO cairn_schema_meta (key, value, updated_at) VALUES ('schema_version', ?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`, fmt.Sprintf("%d", CairnCanonicalSchemaVersion), cairnNow())
	if err != nil {
		return fmt.Errorf("record Cairn schema version: %w", err)
	}
	return nil
}

// CreateCairnRunWithScopeSnapshot atomically creates a serial run, its audit event and two outbox projections.
func (db *DB) CreateCairnRunWithScopeSnapshot(ctx context.Context, run *CairnRun, idempotencyKey string) (*CairnRun, error) {
	if run == nil {
		return nil, fmt.Errorf("Cairn run is nil")
	}
	run.ProjectID = strings.TrimSpace(run.ProjectID)
	run.ConversationID = strings.TrimSpace(run.ConversationID)
	run.GoalText = strings.TrimSpace(run.GoalText)
	run.ScopeSHA256 = strings.TrimSpace(run.ScopeSHA256)
	if run.ProjectID == "" || run.ConversationID == "" || run.GoalText == "" || run.ScopeSHA256 == "" {
		return nil, fmt.Errorf("Cairn run requires project, conversation, goal and scope hash")
	}
	scopeJSON, err := cairnJSONOrEmpty(run.ScopeSnapshotJSON)
	if err != nil {
		return nil, err
	}
	if run.ID == "" {
		run.ID = uuid.NewString()
	}
	if run.Mode == "" {
		run.Mode = "serial"
	}
	if run.Mode != "serial" {
		return nil, fmt.Errorf("Cairn mode %q is not supported", run.Mode)
	}
	if run.Status == "" {
		run.Status = CairnRunQueued
	}
	if !validCairnRunStatus(run.Status) || isTerminalCairnRun(run.Status) {
		return nil, fmt.Errorf("invalid initial Cairn run status %q", run.Status)
	}
	if strings.TrimSpace(idempotencyKey) == "" {
		return nil, fmt.Errorf("Cairn run idempotency key is required")
	}
	now := cairnNow()
	run.ScopeSnapshotJSON = scopeJSON
	run.CreatedAt, run.UpdatedAt = now, now

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin Cairn run transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var existingRunID string
	err = tx.QueryRowContext(ctx, `SELECT run_id FROM cairn_events WHERE idempotency_key = ?`, idempotencyKey).Scan(&existingRunID)
	if err == nil {
		existing, getErr := getCairnRunTx(ctx, tx, existingRunID)
		if getErr != nil {
			return nil, getErr
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return existing, nil
	}
	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("check Cairn run idempotency: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO cairn_runs (id, project_id, conversation_id, goal_text, mode, status, scope_snapshot_json, scope_sha256, project_revision, revision, last_event_seq, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 0, 1, ?, ?)`, run.ID, run.ProjectID, run.ConversationID, run.GoalText, run.Mode, run.Status, run.ScopeSnapshotJSON, run.ScopeSHA256, run.ProjectRevision, now, now); err != nil {
		return nil, fmt.Errorf("insert Cairn run: %w", err)
	}
	event := CairnEvent{ID: uuid.NewString(), RunID: run.ID, Sequence: 1, EventType: "run_created", IdempotencyKey: idempotencyKey, PayloadJSON: fmt.Sprintf(`{"mode":"serial","schema_version":%d}`, CairnCanonicalSchemaVersion), CreatedAt: now}
	if err := insertCairnEventTx(ctx, tx, &event, []string{"yaml_export"}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit Cairn run: %w", err)
	}
	run.LastEventSeq = 1
	return run, nil
}

// CommitCairnReasonDecision is the only entry point for Planner and Replanner intent creation.
func (db *DB) CommitCairnReasonDecision(ctx context.Context, runID string, expectedRevision int64, descriptions []string, idempotencyKey string) ([]CairnIntent, int64, error) {
	if strings.TrimSpace(runID) == "" || strings.TrimSpace(idempotencyKey) == "" {
		return nil, 0, fmt.Errorf("Cairn reason decision requires run and idempotency key")
	}
	clean := make([]string, 0, len(descriptions))
	for _, description := range descriptions {
		description = strings.TrimSpace(description)
		if description != "" {
			clean = append(clean, description)
		}
	}
	if len(clean) == 0 {
		return nil, 0, fmt.Errorf("Cairn reason decision requires at least one intent")
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = tx.Rollback() }()
	run, err := getCairnRunTx(ctx, tx, runID)
	if err != nil {
		return nil, 0, err
	}
	if run.Revision != expectedRevision {
		return nil, run.Revision, fmt.Errorf("Cairn revision conflict: expected %d, current %d", expectedRevision, run.Revision)
	}
	if isTerminalCairnRun(run.Status) {
		return nil, run.Revision, fmt.Errorf("Cairn run %s is terminal", runID)
	}
	var maxOrdinal int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(ordinal), 0) FROM cairn_intents WHERE run_id = ?`, runID).Scan(&maxOrdinal); err != nil {
		return nil, 0, err
	}
	now := cairnNow()
	eventID := uuid.NewString()
	out := make([]CairnIntent, 0, len(clean))
	for i, description := range clean {
		intent := CairnIntent{ID: uuid.NewString(), RunID: runID, Description: description, Status: CairnIntentOpen, Ordinal: maxOrdinal + i + 1, CreatedEventID: eventID, CreatedAt: now, UpdatedAt: now}
		if _, err := tx.ExecContext(ctx, `INSERT INTO cairn_nodes (node_id, run_id, node_type, created_at) VALUES (?, ?, 'intent', ?)`, intent.ID, runID, now); err != nil {
			return nil, 0, fmt.Errorf("insert Cairn intent node: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO cairn_intents (intent_id, run_id, description, status, ordinal, attempt_count, created_event_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, 0, ?, ?, ?)`, intent.ID, runID, intent.Description, intent.Status, intent.Ordinal, eventID, now, now); err != nil {
			return nil, 0, fmt.Errorf("insert Cairn intent: %w", err)
		}
		out = append(out, intent)
	}
	newRevision := run.Revision + 1
	newSeq := run.LastEventSeq + 1
	result, err := tx.ExecContext(ctx, `UPDATE cairn_runs SET status = ?, revision = ?, last_event_seq = ?, updated_at = ? WHERE id = ? AND revision = ?`, CairnRunExecuting, newRevision, newSeq, now, runID, expectedRevision)
	if err != nil {
		return nil, 0, err
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		return nil, run.Revision, fmt.Errorf("Cairn revision conflict while committing reason decision")
	}
	payload, _ := json.Marshal(map[string]interface{}{"intent_ids": intentIDs(out), "count": len(out)})
	event := CairnEvent{ID: eventID, RunID: runID, Sequence: newSeq, EventType: "reason_committed", IdempotencyKey: idempotencyKey, PayloadJSON: string(payload), CreatedAt: now}
	if err := insertCairnEventTx(ctx, tx, &event, []string{"yaml_export"}); err != nil {
		return nil, 0, err
	}
	if err := tx.Commit(); err != nil {
		return nil, 0, err
	}
	return out, newRevision, nil
}

func intentIDs(intents []CairnIntent) []string {
	ids := make([]string, 0, len(intents))
	for _, intent := range intents {
		ids = append(ids, intent.ID)
	}
	return ids
}

// ClaimNextCairnIntent serializes execution: the unique partial index permits one active Intent per run.
func (db *DB) ClaimNextCairnIntent(ctx context.Context, runID string, expectedRevision int64, idempotencyKey string) (*CairnIntent, int64, error) {
	if strings.TrimSpace(runID) == "" || strings.TrimSpace(idempotencyKey) == "" {
		return nil, 0, fmt.Errorf("Cairn intent claim requires run and idempotency key")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = tx.Rollback() }()
	run, err := getCairnRunTx(ctx, tx, runID)
	if err != nil {
		return nil, 0, err
	}
	if run.Revision != expectedRevision {
		return nil, run.Revision, fmt.Errorf("Cairn revision conflict: expected %d, current %d", expectedRevision, run.Revision)
	}
	if isTerminalCairnRun(run.Status) {
		return nil, run.Revision, fmt.Errorf("Cairn run %s is terminal", runID)
	}
	var activeCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM cairn_intents WHERE run_id = ? AND status = 'in_progress'`, runID).Scan(&activeCount); err != nil {
		return nil, run.Revision, err
	}
	if activeCount > 0 {
		if err := tx.Commit(); err != nil {
			return nil, run.Revision, err
		}
		return nil, run.Revision, nil
	}
	var intent CairnIntent
	var startedAt sql.NullString
	var finishedAt sql.NullString
	var createdAt, updatedAt string
	err = tx.QueryRowContext(ctx, `SELECT intent_id, run_id, description, status, ordinal, attempt_count, COALESCE(last_error_code,''), created_event_id, created_at, started_at, finished_at, updated_at FROM cairn_intents WHERE run_id = ? AND status IN ('open','retryable','interrupted') ORDER BY ordinal LIMIT 1`, runID).Scan(&intent.ID, &intent.RunID, &intent.Description, &intent.Status, &intent.Ordinal, &intent.AttemptCount, &intent.LastErrorCode, &intent.CreatedEventID, &createdAt, &startedAt, &finishedAt, &updatedAt)
	if err == sql.ErrNoRows {
		if err := tx.Commit(); err != nil {
			return nil, run.Revision, err
		}
		return nil, run.Revision, nil
	}
	if err != nil {
		return nil, run.Revision, err
	}
	intent.CreatedAt = parseDBTime(createdAt)
	intent.UpdatedAt = parseDBTime(updatedAt)
	intent.StartedAt = nullableDBTime(startedAt)
	intent.FinishedAt = nullableDBTime(finishedAt)
	now := cairnNow()
	newRevision := run.Revision + 1
	newSeq := run.LastEventSeq + 1
	if _, err := tx.ExecContext(ctx, `UPDATE cairn_intents SET status = 'in_progress', attempt_count = attempt_count + 1, started_at = ?, updated_at = ? WHERE intent_id = ? AND status IN ('open','retryable','interrupted')`, now, now, intent.ID); err != nil {
		return nil, run.Revision, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE cairn_runs SET status = ?, revision = ?, last_event_seq = ?, updated_at = ? WHERE id = ? AND revision = ?`, CairnRunExecuting, newRevision, newSeq, now, runID, expectedRevision)
	if err != nil {
		return nil, run.Revision, err
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		return nil, run.Revision, fmt.Errorf("Cairn revision conflict while claiming intent")
	}
	payload, _ := json.Marshal(map[string]string{"intent_id": intent.ID})
	event := CairnEvent{ID: uuid.NewString(), RunID: runID, Sequence: newSeq, IntentID: intent.ID, EventType: "intent_claimed", IdempotencyKey: idempotencyKey, PayloadJSON: string(payload), CreatedAt: now}
	if err := insertCairnEventTx(ctx, tx, &event, []string{"yaml_export"}); err != nil {
		return nil, run.Revision, err
	}
	if err := tx.Commit(); err != nil {
		return nil, run.Revision, err
	}
	intent.Status, intent.AttemptCount, intent.StartedAt, intent.UpdatedAt = CairnIntentInProgress, intent.AttemptCount+1, &now, now
	return &intent, newRevision, nil
}

// CompleteCairnIntentWithFact records a validated observation and makes the intent successful atomically.
func (db *DB) CompleteCairnIntentWithFact(ctx context.Context, runID string, expectedRevision int64, fact *CairnFact, idempotencyKey string) (*CairnFact, int64, error) {
	if fact == nil {
		return nil, 0, fmt.Errorf("Cairn fact is nil")
	}
	fact.RunID = strings.TrimSpace(runID)
	fact.SourceIntentID = strings.TrimSpace(fact.SourceIntentID)
	fact.SourceSegmentID = strings.TrimSpace(fact.SourceSegmentID)
	fact.SourceEventID = strings.TrimSpace(fact.SourceEventID)
	fact.Statement = strings.TrimSpace(fact.Statement)
	if fact.SourceIntentID == "" || fact.SourceSegmentID == "" || fact.SourceEventID == "" || fact.Statement == "" || strings.TrimSpace(idempotencyKey) == "" {
		return nil, 0, fmt.Errorf("Cairn fact requires source intent, segment, event, statement and idempotency key")
	}
	if fact.ObservationType == "" {
		fact.ObservationType = "positive"
	}
	if fact.Confidence == "" {
		fact.Confidence = "tentative"
	}
	if fact.ObservationType != "positive" && fact.ObservationType != "negative" && fact.ObservationType != "derived" {
		return nil, 0, fmt.Errorf("invalid Cairn observation type %q", fact.ObservationType)
	}
	if fact.Confidence != "tentative" && fact.Confidence != "confirmed" {
		return nil, 0, fmt.Errorf("invalid Cairn fact confidence %q", fact.Confidence)
	}
	var err error
	if fact.EvidenceJSON, err = cairnJSONOrEmpty(fact.EvidenceJSON); err != nil {
		return nil, 0, err
	}
	if fact.ProvenanceJSON, err = cairnJSONOrEmpty(fact.ProvenanceJSON); err != nil {
		return nil, 0, err
	}
	if fact.ID == "" {
		fact.ID = uuid.NewString()
	}
	fact.CreatedAt = cairnNow()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = tx.Rollback() }()
	run, err := getCairnRunTx(ctx, tx, runID)
	if err != nil {
		return nil, 0, err
	}
	if run.Revision != expectedRevision {
		return nil, run.Revision, fmt.Errorf("Cairn revision conflict: expected %d, current %d", expectedRevision, run.Revision)
	}
	var intentStatus string
	if err := tx.QueryRowContext(ctx, `SELECT status FROM cairn_intents WHERE intent_id = ? AND run_id = ?`, fact.SourceIntentID, runID).Scan(&intentStatus); err != nil {
		return nil, run.Revision, fmt.Errorf("Cairn source intent: %w", err)
	}
	if intentStatus != CairnIntentInProgress {
		return nil, run.Revision, fmt.Errorf("Cairn source intent %s is %s, not in_progress", fact.SourceIntentID, intentStatus)
	}
	now := fact.CreatedAt
	if _, err := tx.ExecContext(ctx, `INSERT INTO cairn_nodes (node_id, run_id, node_type, created_at) VALUES (?, ?, 'fact', ?)`, fact.ID, runID, now); err != nil {
		return nil, run.Revision, fmt.Errorf("insert Cairn fact node: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO cairn_facts (fact_id, run_id, source_intent_id, source_segment_id, source_event_id, statement, observation_type, confidence, evidence_json, provenance_json, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, fact.ID, runID, fact.SourceIntentID, fact.SourceSegmentID, fact.SourceEventID, fact.Statement, fact.ObservationType, fact.Confidence, fact.EvidenceJSON, fact.ProvenanceJSON, now); err != nil {
		return nil, run.Revision, fmt.Errorf("insert Cairn fact: %w", err)
	}
	newRevision := run.Revision + 1
	newSeq := run.LastEventSeq + 1
	if _, err := tx.ExecContext(ctx, `UPDATE cairn_intents SET status = 'succeeded', finished_at = ?, updated_at = ? WHERE intent_id = ?`, now, now, fact.SourceIntentID); err != nil {
		return nil, run.Revision, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE cairn_runs SET status = ?, revision = ?, last_event_seq = ?, updated_at = ? WHERE id = ? AND revision = ?`, CairnRunReplanning, newRevision, newSeq, now, runID, expectedRevision)
	if err != nil {
		return nil, run.Revision, err
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		return nil, run.Revision, fmt.Errorf("Cairn revision conflict while recording fact")
	}
	payload, _ := json.Marshal(map[string]string{"fact_id": fact.ID, "intent_id": fact.SourceIntentID})
	// Segment persistence lands with the Explorer bridge. Fact provenance retains source_segment_id now;
	// event.segment_id stays empty until that segment row exists, preserving FK integrity during additive rollout.
	event := CairnEvent{ID: uuid.NewString(), RunID: runID, Sequence: newSeq, IntentID: fact.SourceIntentID, EventType: "fact_created", IdempotencyKey: idempotencyKey, PayloadJSON: string(payload), CreatedAt: now}
	if err := insertCairnEventTx(ctx, tx, &event, []string{"project_fact_projection", "yaml_export"}); err != nil {
		return nil, run.Revision, err
	}
	if err := tx.Commit(); err != nil {
		return nil, run.Revision, err
	}
	return fact, newRevision, nil
}

// FailCairnIntentExecution records an execution terminal state. It deliberately cannot create a Fact.
func (db *DB) FailCairnIntentExecution(ctx context.Context, runID, intentID string, expectedRevision int64, status, errorCode, errorDetail, idempotencyKey string) (int64, error) {
	if status != CairnIntentRetryable && status != CairnIntentFailed && status != CairnIntentCancelled && status != CairnIntentInterrupted {
		return 0, fmt.Errorf("invalid failed Cairn intent status %q", status)
	}
	if strings.TrimSpace(runID) == "" || strings.TrimSpace(intentID) == "" || strings.TrimSpace(idempotencyKey) == "" {
		return 0, fmt.Errorf("Cairn failure requires run, intent and idempotency key")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	run, err := getCairnRunTx(ctx, tx, runID)
	if err != nil {
		return 0, err
	}
	if run.Revision != expectedRevision {
		return run.Revision, fmt.Errorf("Cairn revision conflict: expected %d, current %d", expectedRevision, run.Revision)
	}
	if isTerminalCairnRun(run.Status) {
		return run.Revision, fmt.Errorf("Cairn run %s is terminal", runID)
	}
	var currentStatus string
	if err := tx.QueryRowContext(ctx, `SELECT status FROM cairn_intents WHERE intent_id = ? AND run_id = ?`, intentID, runID).Scan(&currentStatus); err != nil {
		return run.Revision, err
	}
	if currentStatus != CairnIntentInProgress {
		return run.Revision, fmt.Errorf("Cairn intent %s is %s, not in_progress", intentID, currentStatus)
	}
	now := cairnNow()
	newRevision := run.Revision + 1
	newSeq := run.LastEventSeq + 1
	if _, err := tx.ExecContext(ctx, `UPDATE cairn_intents SET status = ?, last_error_code = ?, finished_at = ?, updated_at = ? WHERE intent_id = ?`, status, nullIfEmpty(errorCode), now, now, intentID); err != nil {
		return run.Revision, err
	}
	nextRunStatus := CairnRunReplanning
	if status == CairnIntentCancelled {
		nextRunStatus = CairnRunCancelled
	}
	result, err := tx.ExecContext(ctx, `UPDATE cairn_runs SET status = ?, revision = ?, last_event_seq = ?, terminal_code = CASE WHEN ? = 'cancelled' THEN ? ELSE terminal_code END, terminal_detail = CASE WHEN ? = 'cancelled' THEN ? ELSE terminal_detail END, finished_at = CASE WHEN ? = 'cancelled' THEN ? ELSE finished_at END, updated_at = ? WHERE id = ? AND revision = ?`, nextRunStatus, newRevision, newSeq, status, errorCode, status, errorDetail, status, now, now, runID, expectedRevision)
	if err != nil {
		return run.Revision, err
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		return run.Revision, fmt.Errorf("Cairn revision conflict while recording execution failure")
	}
	payload, _ := json.Marshal(map[string]string{"intent_id": intentID, "status": status, "error_code": errorCode, "error_detail": errorDetail})
	event := CairnEvent{ID: uuid.NewString(), RunID: runID, Sequence: newSeq, IntentID: intentID, EventType: "intent_execution_" + status, IdempotencyKey: idempotencyKey, PayloadJSON: string(payload), CreatedAt: now}
	if err := insertCairnEventTx(ctx, tx, &event, []string{"yaml_export"}); err != nil {
		return run.Revision, err
	}
	if err := tx.Commit(); err != nil {
		return run.Revision, err
	}
	return newRevision, nil
}

func insertCairnEventTx(ctx context.Context, tx *sql.Tx, event *CairnEvent, consumers []string) error {
	if event == nil || strings.TrimSpace(event.ID) == "" || strings.TrimSpace(event.RunID) == "" || strings.TrimSpace(event.IdempotencyKey) == "" {
		return fmt.Errorf("invalid Cairn event")
	}
	payload, err := cairnJSONOrEmpty(event.PayloadJSON)
	if err != nil {
		return err
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = cairnNow()
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO cairn_events (event_id, run_id, sequence, segment_id, intent_id, invocation_id, event_type, idempotency_key, payload_json, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, event.ID, event.RunID, event.Sequence, nullIfEmpty(event.SegmentID), nullIfEmpty(event.IntentID), nullIfEmpty(event.InvocationID), event.EventType, event.IdempotencyKey, payload, event.CreatedAt); err != nil {
		return fmt.Errorf("insert Cairn event: %w", err)
	}
	for _, consumer := range consumers {
		consumer = strings.TrimSpace(consumer)
		if consumer == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO cairn_outbox (event_id, consumer, status, attempts, available_at) VALUES (?, ?, 'pending', 0, ?)`, event.ID, consumer, event.CreatedAt); err != nil {
			return fmt.Errorf("insert Cairn outbox %s: %w", consumer, err)
		}
	}
	return nil
}

func getCairnRunTx(ctx context.Context, tx *sql.Tx, runID string) (*CairnRun, error) {
	var run CairnRun
	var cancelRequestedAt, leaseUntil, startedAt, finishedAt sql.NullString
	var createdAt, updatedAt string
	err := tx.QueryRowContext(ctx, `SELECT id, project_id, conversation_id, goal_text, mode, status, scope_snapshot_json, scope_sha256, project_revision, revision, last_event_seq, COALESCE(terminal_code,''), COALESCE(terminal_detail,''), cancel_requested_at, COALESCE(lease_owner,''), lease_until, created_at, started_at, finished_at, updated_at FROM cairn_runs WHERE id = ?`, runID).Scan(&run.ID, &run.ProjectID, &run.ConversationID, &run.GoalText, &run.Mode, &run.Status, &run.ScopeSnapshotJSON, &run.ScopeSHA256, &run.ProjectRevision, &run.Revision, &run.LastEventSeq, &run.TerminalCode, &run.TerminalDetail, &cancelRequestedAt, &run.LeaseOwner, &leaseUntil, &createdAt, &startedAt, &finishedAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("Cairn run not found")
		}
		return nil, err
	}
	run.CancelRequestedAt = nullableDBTime(cancelRequestedAt)
	run.LeaseUntil = nullableDBTime(leaseUntil)
	run.CreatedAt = parseDBTime(createdAt)
	run.StartedAt = nullableDBTime(startedAt)
	run.FinishedAt = nullableDBTime(finishedAt)
	run.UpdatedAt = parseDBTime(updatedAt)
	return &run, nil
}

// GetLatestCairnStateSnapshot returns latest canonical run for a project-bound conversation.
// No run is a normal empty state, not an error. Reads occur in one transaction so UI never
// renders a mixed revision across run, nodes and edges.
func (db *DB) GetLatestCairnStateSnapshot(ctx context.Context, projectID, conversationID string) (*CairnStateSnapshot, error) {
	projectID = strings.TrimSpace(projectID)
	conversationID = strings.TrimSpace(conversationID)
	if projectID == "" || conversationID == "" {
		return nil, fmt.Errorf("Cairn snapshot requires project and conversation")
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("begin Cairn snapshot: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	snapshot := &CairnStateSnapshot{
		SchemaVersion: CairnCanonicalSchemaVersion,
		Facts:         []CairnFact{},
		Intents:       []CairnIntent{},
		Hints:         []CairnHint{},
		Edges:         []CairnEdge{},
	}
	var runID string
	err = tx.QueryRowContext(ctx, `SELECT id FROM cairn_runs WHERE project_id = ? AND conversation_id = ? ORDER BY created_at DESC, id DESC LIMIT 1`, projectID, conversationID).Scan(&runID)
	if err == sql.ErrNoRows {
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("commit empty Cairn snapshot: %w", err)
		}
		return snapshot, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query latest Cairn run: %w", err)
	}
	run, err := getCairnRunTx(ctx, tx, runID)
	if err != nil {
		return nil, err
	}
	snapshot.Initialized = true
	snapshot.Run = run

	intentRows, err := tx.QueryContext(ctx, `SELECT intent_id, run_id, description, status, ordinal, attempt_count, COALESCE(last_error_code,''), created_event_id, created_at, started_at, finished_at, updated_at FROM cairn_intents WHERE run_id = ? ORDER BY ordinal`, runID)
	if err != nil {
		return nil, fmt.Errorf("query Cairn intents: %w", err)
	}
	for intentRows.Next() {
		var intent CairnIntent
		var createdAt, updatedAt string
		var startedAt, finishedAt sql.NullString
		if err := intentRows.Scan(&intent.ID, &intent.RunID, &intent.Description, &intent.Status, &intent.Ordinal, &intent.AttemptCount, &intent.LastErrorCode, &intent.CreatedEventID, &createdAt, &startedAt, &finishedAt, &updatedAt); err != nil {
			_ = intentRows.Close()
			return nil, fmt.Errorf("scan Cairn intent: %w", err)
		}
		intent.CreatedAt = parseDBTime(createdAt)
		intent.StartedAt = nullableDBTime(startedAt)
		intent.FinishedAt = nullableDBTime(finishedAt)
		intent.UpdatedAt = parseDBTime(updatedAt)
		snapshot.Intents = append(snapshot.Intents, intent)
	}
	if err := intentRows.Err(); err != nil {
		_ = intentRows.Close()
		return nil, fmt.Errorf("iterate Cairn intents: %w", err)
	}
	_ = intentRows.Close()

	factRows, err := tx.QueryContext(ctx, `SELECT fact_id, run_id, source_intent_id, source_segment_id, source_event_id, statement, observation_type, confidence, evidence_json, provenance_json, created_at FROM cairn_facts WHERE run_id = ? ORDER BY created_at, fact_id`, runID)
	if err != nil {
		return nil, fmt.Errorf("query Cairn facts: %w", err)
	}
	for factRows.Next() {
		var fact CairnFact
		var createdAt string
		if err := factRows.Scan(&fact.ID, &fact.RunID, &fact.SourceIntentID, &fact.SourceSegmentID, &fact.SourceEventID, &fact.Statement, &fact.ObservationType, &fact.Confidence, &fact.EvidenceJSON, &fact.ProvenanceJSON, &createdAt); err != nil {
			_ = factRows.Close()
			return nil, fmt.Errorf("scan Cairn fact: %w", err)
		}
		fact.CreatedAt = parseDBTime(createdAt)
		snapshot.Facts = append(snapshot.Facts, fact)
	}
	if err := factRows.Err(); err != nil {
		_ = factRows.Close()
		return nil, fmt.Errorf("iterate Cairn facts: %w", err)
	}
	_ = factRows.Close()

	hintRows, err := tx.QueryContext(ctx, `SELECT hint_id, run_id, content, creator_type, COALESCE(creator_id,''), status, created_event_id, created_at, resolved_at FROM cairn_hints WHERE run_id = ? ORDER BY created_at, hint_id`, runID)
	if err != nil {
		return nil, fmt.Errorf("query Cairn hints: %w", err)
	}
	for hintRows.Next() {
		var hint CairnHint
		var createdAt string
		var resolvedAt sql.NullString
		if err := hintRows.Scan(&hint.ID, &hint.RunID, &hint.Content, &hint.CreatorType, &hint.CreatorID, &hint.Status, &hint.CreatedEventID, &createdAt, &resolvedAt); err != nil {
			_ = hintRows.Close()
			return nil, fmt.Errorf("scan Cairn hint: %w", err)
		}
		hint.CreatedAt = parseDBTime(createdAt)
		hint.ResolvedAt = nullableDBTime(resolvedAt)
		snapshot.Hints = append(snapshot.Hints, hint)
	}
	if err := hintRows.Err(); err != nil {
		_ = hintRows.Close()
		return nil, fmt.Errorf("iterate Cairn hints: %w", err)
	}
	_ = hintRows.Close()

	edgeRows, err := tx.QueryContext(ctx, `SELECT edge_id, run_id, source_node_id, target_node_id, edge_type, source_event_id, created_at FROM cairn_edges WHERE run_id = ? ORDER BY created_at, edge_id`, runID)
	if err != nil {
		return nil, fmt.Errorf("query Cairn edges: %w", err)
	}
	for edgeRows.Next() {
		var edge CairnEdge
		var createdAt string
		if err := edgeRows.Scan(&edge.ID, &edge.RunID, &edge.SourceNodeID, &edge.TargetNodeID, &edge.EdgeType, &edge.SourceEventID, &createdAt); err != nil {
			_ = edgeRows.Close()
			return nil, fmt.Errorf("scan Cairn edge: %w", err)
		}
		edge.CreatedAt = parseDBTime(createdAt)
		snapshot.Edges = append(snapshot.Edges, edge)
	}
	if err := edgeRows.Err(); err != nil {
		_ = edgeRows.Close()
		return nil, fmt.Errorf("iterate Cairn edges: %w", err)
	}
	_ = edgeRows.Close()
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit Cairn snapshot: %w", err)
	}
	return snapshot, nil
}

func nullableDBTime(v sql.NullString) *time.Time {
	if !v.Valid || strings.TrimSpace(v.String) == "" {
		return nil
	}
	t := parseDBTime(v.String)
	if t.IsZero() {
		return nil
	}
	return &t
}

// GetCairnRunByID loads one canonical run outside a transaction.
func (db *DB) GetCairnRunByID(ctx context.Context, runID string) (*CairnRun, error) {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil, fmt.Errorf("Cairn run id is empty")
	}
	var run CairnRun
	var cancelRequestedAt, leaseUntil, startedAt, finishedAt sql.NullString
	var createdAt, updatedAt string
	err := db.QueryRowContext(ctx, `SELECT id, project_id, conversation_id, goal_text, mode, status, scope_snapshot_json, scope_sha256, project_revision, revision, last_event_seq, COALESCE(terminal_code,''), COALESCE(terminal_detail,''), cancel_requested_at, COALESCE(lease_owner,''), lease_until, created_at, started_at, finished_at, updated_at FROM cairn_runs WHERE id = ?`, runID).Scan(&run.ID, &run.ProjectID, &run.ConversationID, &run.GoalText, &run.Mode, &run.Status, &run.ScopeSnapshotJSON, &run.ScopeSHA256, &run.ProjectRevision, &run.Revision, &run.LastEventSeq, &run.TerminalCode, &run.TerminalDetail, &cancelRequestedAt, &run.LeaseOwner, &leaseUntil, &createdAt, &startedAt, &finishedAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("Cairn run not found")
		}
		return nil, err
	}
	run.CancelRequestedAt = nullableDBTime(cancelRequestedAt)
	run.LeaseUntil = nullableDBTime(leaseUntil)
	run.CreatedAt = parseDBTime(createdAt)
	run.StartedAt = nullableDBTime(startedAt)
	run.FinishedAt = nullableDBTime(finishedAt)
	run.UpdatedAt = parseDBTime(updatedAt)
	return &run, nil
}

// GetActiveCairnRunForConversation returns the single non-terminal run for a conversation, if any.
// The partial unique index guarantees at most one row matches.
func (db *DB) GetActiveCairnRunForConversation(ctx context.Context, conversationID string) (*CairnRun, error) {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return nil, fmt.Errorf("Cairn conversation id is empty")
	}
	var runID string
	err := db.QueryRowContext(ctx, `SELECT id FROM cairn_runs WHERE conversation_id = ? AND status IN ('queued','planning','executing','replanning','interrupted','blocked') ORDER BY created_at DESC LIMIT 1`, conversationID).Scan(&runID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query active Cairn run: %w", err)
	}
	return db.GetCairnRunByID(ctx, runID)
}

func cairnRunFailStatusFor(code string) string {
	switch strings.TrimSpace(code) {
	case "context_canceled", "user_canceled":
		return CairnRunCancelled
	case "timeout", "deadline_exceeded":
		return CairnRunTimedOut
	default:
		return CairnRunFailed
	}
}

// CompleteCairnRun makes a run terminal with status completed. Residual in_progress intents become interrupted.
func (db *DB) CompleteCairnRun(ctx context.Context, runID string, expectedRevision int64, code, detail, idempotencyKey string) (int64, error) {
	return db.finishCairnRun(ctx, runID, expectedRevision, CairnRunCompleted, code, detail, idempotencyKey)
}

// FailCairnRun makes a run terminal; status derives from code (cancelled/timed_out/failed).
func (db *DB) FailCairnRun(ctx context.Context, runID string, expectedRevision int64, code, detail, idempotencyKey string) (int64, error) {
	return db.finishCairnRun(ctx, runID, expectedRevision, cairnRunFailStatusFor(code), code, detail, idempotencyKey)
}

func (db *DB) finishCairnRun(ctx context.Context, runID string, expectedRevision int64, status, code, detail, idempotencyKey string) (int64, error) {
	if strings.TrimSpace(runID) == "" || strings.TrimSpace(idempotencyKey) == "" {
		return 0, fmt.Errorf("Cairn run finish requires run and idempotency key")
	}
	if !isTerminalCairnRun(status) {
		return 0, fmt.Errorf("invalid terminal Cairn run status %q", status)
	}
	now := cairnNow()
	eventType := "run_" + status
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	run, err := getCairnRunTx(ctx, tx, runID)
	if err != nil {
		return 0, err
	}
	if run.Revision != expectedRevision {
		return run.Revision, fmt.Errorf("Cairn revision conflict: expected %d, current %d", expectedRevision, run.Revision)
	}
	if isTerminalCairnRun(run.Status) {
		return run.Revision, fmt.Errorf("Cairn run %s is already terminal (%s)", runID, run.Status)
	}
	// Residual active intents: cancelled runs cancel them; other terminal states interrupt them.
	residual := CairnIntentInterrupted
	if status == CairnRunCancelled {
		residual = CairnIntentCancelled
	}
	if _, err := tx.ExecContext(ctx, `UPDATE cairn_intents SET status = ?, finished_at = ?, updated_at = ? WHERE run_id = ? AND status = 'in_progress'`, residual, now, now, runID); err != nil {
		return run.Revision, fmt.Errorf("settle residual Cairn intents: %w", err)
	}
	newRevision := run.Revision + 1
	newSeq := run.LastEventSeq + 1
	result, err := tx.ExecContext(ctx, `UPDATE cairn_runs SET status = ?, revision = ?, last_event_seq = ?, terminal_code = ?, terminal_detail = ?, finished_at = ?, updated_at = ? WHERE id = ? AND revision = ?`, status, newRevision, newSeq, nullIfEmpty(code), nullIfEmpty(detail), now, now, runID, expectedRevision)
	if err != nil {
		return run.Revision, err
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		return run.Revision, fmt.Errorf("Cairn revision conflict while finishing run")
	}
	payload, _ := json.Marshal(map[string]string{"status": status, "code": code, "detail": detail})
	event := CairnEvent{ID: uuid.NewString(), RunID: runID, Sequence: newSeq, EventType: eventType, IdempotencyKey: idempotencyKey, PayloadJSON: string(payload), CreatedAt: now}
	if err := insertCairnEventTx(ctx, tx, &event, []string{"project_fact_projection", "yaml_export"}); err != nil {
		return run.Revision, err
	}
	if err := tx.Commit(); err != nil {
		return run.Revision, err
	}
	return newRevision, nil
}

// GetCairnEventByID loads one audit event.
func (db *DB) GetCairnEventByID(ctx context.Context, eventID string) (*CairnEvent, error) {
	eventID = strings.TrimSpace(eventID)
	if eventID == "" {
		return nil, fmt.Errorf("Cairn event id is empty")
	}
	var event CairnEvent
	var segmentID, intentID, invocationID sql.NullString
	var createdAt string
	err := db.QueryRowContext(ctx, `SELECT event_id, run_id, sequence, segment_id, intent_id, invocation_id, event_type, idempotency_key, payload_json, created_at FROM cairn_events WHERE event_id = ?`, eventID).Scan(&event.ID, &event.RunID, &event.Sequence, &segmentID, &intentID, &invocationID, &event.EventType, &event.IdempotencyKey, &event.PayloadJSON, &createdAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("Cairn event not found")
		}
		return nil, err
	}
	event.SegmentID = segmentID.String
	event.IntentID = intentID.String
	event.InvocationID = invocationID.String
	event.CreatedAt = parseDBTime(createdAt)
	return &event, nil
}

// GetCairnFactByID loads one validated observation.
func (db *DB) GetCairnFactByID(ctx context.Context, factID string) (*CairnFact, error) {
	factID = strings.TrimSpace(factID)
	if factID == "" {
		return nil, fmt.Errorf("Cairn fact id is empty")
	}
	var fact CairnFact
	var createdAt string
	err := db.QueryRowContext(ctx, `SELECT fact_id, run_id, source_intent_id, source_segment_id, source_event_id, statement, observation_type, confidence, evidence_json, provenance_json, created_at FROM cairn_facts WHERE fact_id = ?`, factID).Scan(&fact.ID, &fact.RunID, &fact.SourceIntentID, &fact.SourceSegmentID, &fact.SourceEventID, &fact.Statement, &fact.ObservationType, &fact.Confidence, &fact.EvidenceJSON, &fact.ProvenanceJSON, &createdAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("Cairn fact not found")
		}
		return nil, err
	}
	fact.CreatedAt = parseDBTime(createdAt)
	return &fact, nil
}

const cairnOutboxLease = 90 * time.Second
const cairnOutboxMaxAttempts = 8

// ClaimCairnOutboxItems claims up to limit ready items for one consumer with a short lease.
// Stale processing rows whose lease expired are reclaimed, so a crashed worker cannot strand events.
func (db *DB) ClaimCairnOutboxItems(ctx context.Context, consumer string, limit int) ([]CairnOutboxItem, error) {
	consumer = strings.TrimSpace(consumer)
	if consumer == "" {
		return nil, fmt.Errorf("Cairn outbox consumer is empty")
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}
	now := cairnNow()
	owner := fmt.Sprintf("%s-%d", consumer, time.Now().UnixNano())
	until := now.Add(cairnOutboxLease)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	rows, err := tx.QueryContext(ctx, `SELECT event_id, consumer, status, attempts, available_at, COALESCE(locked_by,''), locked_until, COALESCE(last_error,''), delivered_at FROM cairn_outbox WHERE consumer = ? AND ((status = 'pending' AND available_at <= ?) OR (status = 'processing' AND (locked_until IS NULL OR locked_until < ?))) ORDER BY available_at, event_id LIMIT ?`, consumer, now, now, limit)
	if err != nil {
		return nil, fmt.Errorf("query Cairn outbox: %w", err)
	}
	var items []CairnOutboxItem
	var eventIDs []string
	for rows.Next() {
		var item CairnOutboxItem
		var lockedBy, lastError sql.NullString
		var lockedUntil, deliveredAt sql.NullString
		var attempts int
		if err := rows.Scan(&item.EventID, &item.Consumer, &item.Status, &attempts, &item.AvailableAt, &lockedBy, &lockedUntil, &lastError, &deliveredAt); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scan Cairn outbox: %w", err)
		}
		item.Attempts = attempts
		item.LockedBy = lockedBy.String
		item.LockedUntil = nullableDBTime(lockedUntil)
		item.LastError = lastError.String
		item.DeliveredAt = nullableDBTime(deliveredAt)
		items = append(items, item)
		eventIDs = append(eventIDs, item.EventID)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("iterate Cairn outbox: %w", err)
	}
	_ = rows.Close()
	for _, eventID := range eventIDs {
		if _, err := tx.ExecContext(ctx, `UPDATE cairn_outbox SET status = 'processing', locked_by = ?, locked_until = ? WHERE event_id = ? AND consumer = ?`, owner, until, eventID, consumer); err != nil {
			return nil, fmt.Errorf("lock Cairn outbox item: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return items, nil
}

// AckCairnOutboxItem marks one item delivered.
func (db *DB) AckCairnOutboxItem(ctx context.Context, eventID, consumer string) error {
	_, err := db.ExecContext(ctx, `UPDATE cairn_outbox SET status = 'delivered', locked_by = NULL, locked_until = NULL, delivered_at = ? WHERE event_id = ? AND consumer = ?`, cairnNow(), eventID, consumer)
	if err != nil {
		return fmt.Errorf("ack Cairn outbox item: %w", err)
	}
	return nil
}

// NackCairnOutboxItem retries with backoff or dead-letters after max attempts.
func (db *DB) NackCairnOutboxItem(ctx context.Context, eventID, consumer, errMsg string) error {
	now := cairnNow()
	attempts := 0
	if err := db.QueryRowContext(ctx, `SELECT attempts FROM cairn_outbox WHERE event_id = ? AND consumer = ?`, eventID, consumer).Scan(&attempts); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("Cairn outbox item not found")
		}
		return err
	}
	attempts++
	if attempts >= cairnOutboxMaxAttempts {
		_, err := db.ExecContext(ctx, `UPDATE cairn_outbox SET status = 'dead', locked_by = NULL, locked_until = NULL, last_error = ? WHERE event_id = ? AND consumer = ?`, nullIfEmpty(errMsg), eventID, consumer)
		if err != nil {
			return fmt.Errorf("dead-letter Cairn outbox item: %w", err)
		}
		return nil
	}
	backoff := time.Duration(1<<uint(min(attempts, 10))) * time.Second
	if _, err := db.ExecContext(ctx, `UPDATE cairn_outbox SET status = 'pending', attempts = ?, locked_by = NULL, locked_until = NULL, available_at = ?, last_error = ? WHERE event_id = ? AND consumer = ?`, attempts, now.Add(backoff), nullIfEmpty(errMsg), eventID, consumer); err != nil {
		return fmt.Errorf("nack Cairn outbox item: %w", err)
	}
	return nil
}
