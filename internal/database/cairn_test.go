package database

import (
	"context"
	"path/filepath"
	"testing"

	"go.uber.org/zap"
)

func newCairnTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := NewDB(filepath.Join(t.TempDir(), "cairn.db"), zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func newCairnTestRun(t *testing.T, db *DB) *CairnRun {
	t.Helper()
	project, err := db.CreateProject(&Project{Name: "cairn-schema-test"})
	if err != nil {
		t.Fatal(err)
	}
	conversation, err := db.CreateConversation("cairn test", ConversationCreateMeta{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SetConversationProjectID(conversation.ID, project.ID); err != nil {
		t.Fatal(err)
	}
	run, err := db.CreateCairnRunWithScopeSnapshot(context.Background(), &CairnRun{
		ProjectID:         project.ID,
		ConversationID:    conversation.ID,
		GoalText:          "verify canonical state",
		ScopeSnapshotJSON: `{"targets":["http://127.0.0.1"]}`,
		ScopeSHA256:       "scope-test",
	}, "run-create-test")
	if err != nil {
		t.Fatal(err)
	}
	return run
}

func TestNormalizeConversationAgentModeSupportsCairn(t *testing.T) {
	if got := normalizeConversationAgentMode("cairn"); got != "cairn" {
		t.Fatalf("mode=%q, want cairn", got)
	}
	if got := normalizeConversationAgentMode("CAIRN"); got != "cairn" {
		t.Fatalf("normalized mode=%q, want cairn", got)
	}
	if got := normalizeConversationAgentMode("unknown"); got != "eino_single" {
		t.Fatalf("unknown mode=%q, want eino_single", got)
	}
}

func TestCairnSchemaAndSerialFactLifecycle(t *testing.T) {
	db := newCairnTestDB(t)
	run := newCairnTestRun(t, db)

	intents, revision, err := db.CommitCairnReasonDecision(context.Background(), run.ID, run.Revision, []string{"inspect target"}, "reason-test")
	if err != nil {
		t.Fatal(err)
	}
	if len(intents) != 1 || intents[0].ID == "" {
		t.Fatalf("intents=%+v", intents)
	}

	intent, revision, err := db.ClaimNextCairnIntent(context.Background(), run.ID, revision, "claim-test")
	if err != nil {
		t.Fatal(err)
	}
	if intent == nil || intent.Status != CairnIntentInProgress {
		t.Fatalf("claimed intent=%+v", intent)
	}

	fact, revision, err := db.CompleteCairnIntentWithFact(context.Background(), run.ID, revision, &CairnFact{
		SourceIntentID:  intent.ID,
		SourceSegmentID: "segment-test",
		SourceEventID:   "observation-test",
		Statement:       "target answered HTTP 200",
		ObservationType: "positive",
		Confidence:      "confirmed",
		EvidenceJSON:    `{"tool_call_ids":["call-1"]}`,
		ProvenanceJSON:  `{"run_id":"test"}`,
	}, "fact-test")
	if err != nil {
		t.Fatal(err)
	}
	if fact.ID == "" || revision < 1 {
		t.Fatalf("fact=%+v revision=%d", fact, revision)
	}

	var factCount, eventCount, projectionCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM cairn_facts WHERE run_id = ?`, run.ID).Scan(&factCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM cairn_events WHERE run_id = ?`, run.ID).Scan(&eventCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM cairn_outbox WHERE event_id IN (SELECT event_id FROM cairn_events WHERE run_id = ?)`, run.ID).Scan(&projectionCount); err != nil {
		t.Fatal(err)
	}
	if factCount != 1 || eventCount != 4 || projectionCount != 5 {
		t.Fatalf("facts=%d events=%d outbox=%d", factCount, eventCount, projectionCount)
	}
}

func TestCairnSnapshotReturnsLatestCanonicalRun(t *testing.T) {
	db := newCairnTestDB(t)
	run := newCairnTestRun(t, db)
	intents, revision, err := db.CommitCairnReasonDecision(context.Background(), run.ID, run.Revision, []string{"inspect target"}, "snapshot-reason")
	if err != nil {
		t.Fatal(err)
	}
	intent, revision, err := db.ClaimNextCairnIntent(context.Background(), run.ID, revision, "snapshot-claim")
	if err != nil || intent == nil {
		t.Fatalf("intent=%+v err=%v", intent, err)
	}
	_, _, err = db.CompleteCairnIntentWithFact(context.Background(), run.ID, revision, &CairnFact{
		SourceIntentID:  intents[0].ID,
		SourceSegmentID: "snapshot-segment",
		SourceEventID:   "snapshot-observation",
		Statement:       "target answered HTTP 200",
		ObservationType: "positive",
		Confidence:      "confirmed",
		EvidenceJSON:    `{}`,
		ProvenanceJSON:  `{}`,
	}, "snapshot-fact")
	if err != nil {
		t.Fatal(err)
	}

	snapshot, err := db.GetLatestCairnStateSnapshot(context.Background(), run.ProjectID, run.ConversationID)
	if err != nil {
		t.Fatal(err)
	}
	if !snapshot.Initialized || snapshot.Run == nil || snapshot.Run.ID != run.ID {
		t.Fatalf("snapshot run=%+v initialized=%v", snapshot.Run, snapshot.Initialized)
	}
	if len(snapshot.Intents) != 1 || snapshot.Intents[0].ID != intent.ID {
		t.Fatalf("intents=%+v", snapshot.Intents)
	}
	if len(snapshot.Facts) != 1 || snapshot.Facts[0].SourceIntentID != intent.ID {
		t.Fatalf("facts=%+v", snapshot.Facts)
	}
}

func TestCairnSnapshotIsEmptyBeforeCanonicalRun(t *testing.T) {
	db := newCairnTestDB(t)
	project, err := db.CreateProject(&Project{Name: "empty-cairn-snapshot"})
	if err != nil {
		t.Fatal(err)
	}
	conversation, err := db.CreateConversation("empty cairn snapshot", ConversationCreateMeta{ProjectID: project.ID})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := db.GetLatestCairnStateSnapshot(context.Background(), project.ID, conversation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Initialized || snapshot.Run != nil || len(snapshot.Facts) != 0 || len(snapshot.Intents) != 0 || len(snapshot.Hints) != 0 || len(snapshot.Edges) != 0 {
		t.Fatalf("unexpected empty snapshot=%+v", snapshot)
	}
}

func TestCairnFailureDoesNotCreateFact(t *testing.T) {
	db := newCairnTestDB(t)
	run := newCairnTestRun(t, db)
	_, revision, err := db.CommitCairnReasonDecision(context.Background(), run.ID, run.Revision, []string{"run expensive scanner"}, "reason-failure-test")
	if err != nil {
		t.Fatal(err)
	}
	intent, revision, err := db.ClaimNextCairnIntent(context.Background(), run.ID, revision, "claim-failure-test")
	if err != nil {
		t.Fatal(err)
	}
	revision, err = db.FailCairnIntentExecution(context.Background(), run.ID, intent.ID, revision, CairnIntentFailed, "max_iterations", "executor exhausted budget", "failure-test")
	if err != nil {
		t.Fatal(err)
	}
	if revision < 1 {
		t.Fatalf("revision=%d", revision)
	}
	var facts int
	var status string
	if err := db.QueryRow(`SELECT COUNT(*) FROM cairn_facts WHERE run_id = ?`, run.ID).Scan(&facts); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT status FROM cairn_intents WHERE intent_id = ?`, intent.ID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if facts != 0 || status != CairnIntentFailed {
		t.Fatalf("facts=%d intent_status=%s", facts, status)
	}
}

func TestCairnOnlyOneIntentCanBeClaimed(t *testing.T) {
	db := newCairnTestDB(t)
	run := newCairnTestRun(t, db)
	_, revision, err := db.CommitCairnReasonDecision(context.Background(), run.ID, run.Revision, []string{"one", "two"}, "reason-two-intents-test")
	if err != nil {
		t.Fatal(err)
	}
	first, revision, err := db.ClaimNextCairnIntent(context.Background(), run.ID, revision, "claim-one-test")
	if err != nil || first == nil {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	second, _, err := db.ClaimNextCairnIntent(context.Background(), run.ID, revision, "claim-two-test")
	if err != nil {
		t.Fatal(err)
	}
	if second != nil {
		t.Fatalf("second claim must be nil while first is active: %+v", second)
	}
}

func TestCairnCompleteRunSettlesResidualIntent(t *testing.T) {
	db := newCairnTestDB(t)
	run := newCairnTestRun(t, db)
	_, revision, err := db.CommitCairnReasonDecision(context.Background(), run.ID, run.Revision, []string{"inspect target"}, "complete-run-reason")
	if err != nil {
		t.Fatal(err)
	}
	intent, revision, err := db.ClaimNextCairnIntent(context.Background(), run.ID, revision, "complete-run-claim")
	if err != nil || intent == nil {
		t.Fatalf("intent=%+v err=%v", intent, err)
	}
	revision, err = db.CompleteCairnRun(context.Background(), run.ID, revision, "goal_reached", "all intents satisfied", "complete-run-test")
	if err != nil {
		t.Fatal(err)
	}
	var runStatus, intentStatus string
	if err := db.QueryRow(`SELECT status FROM cairn_runs WHERE id = ?`, run.ID).Scan(&runStatus); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT status FROM cairn_intents WHERE intent_id = ?`, intent.ID).Scan(&intentStatus); err != nil {
		t.Fatal(err)
	}
	if runStatus != CairnRunCompleted || intentStatus != CairnIntentInterrupted {
		t.Fatalf("run=%s intent=%s", runStatus, intentStatus)
	}
}

func TestCairnFailRunCancelsResidualIntent(t *testing.T) {
	db := newCairnTestDB(t)
	run := newCairnTestRun(t, db)
	_, revision, err := db.CommitCairnReasonDecision(context.Background(), run.ID, run.Revision, []string{"inspect target"}, "fail-run-reason")
	if err != nil {
		t.Fatal(err)
	}
	intent, revision, err := db.ClaimNextCairnIntent(context.Background(), run.ID, revision, "fail-run-claim")
	if err != nil || intent == nil {
		t.Fatalf("intent=%+v err=%v", intent, err)
	}
	revision, err = db.FailCairnRun(context.Background(), run.ID, revision, "context_canceled", "user cancelled", "fail-run-test")
	if err != nil {
		t.Fatal(err)
	}
	var runStatus, intentStatus string
	if err := db.QueryRow(`SELECT status FROM cairn_runs WHERE id = ?`, run.ID).Scan(&runStatus); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT status FROM cairn_intents WHERE intent_id = ?`, intent.ID).Scan(&intentStatus); err != nil {
		t.Fatal(err)
	}
	if runStatus != CairnRunCancelled || intentStatus != CairnIntentCancelled {
		t.Fatalf("run=%s intent=%s", runStatus, intentStatus)
	}
}

func TestCairnOutboxClaimAckNack(t *testing.T) {
	db := newCairnTestDB(t)
	run := newCairnTestRun(t, db)
	_, revision, err := db.CommitCairnReasonDecision(context.Background(), run.ID, run.Revision, []string{"inspect target"}, "outbox-reason")
	if err != nil {
		t.Fatal(err)
	}
	claimed, revision, err := db.ClaimNextCairnIntent(context.Background(), run.ID, revision, "outbox-claim-intent")
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = db.CompleteCairnIntentWithFact(context.Background(), run.ID, revision, &CairnFact{
		SourceIntentID:  claimed.ID,
		SourceSegmentID: "outbox-segment",
		SourceEventID:   "outbox-observation",
		Statement:       "outbox fact",
		ObservationType: "positive",
		Confidence:      "tentative",
	}, "outbox-fact-key")
	if err != nil {
		t.Fatal(err)
	}

	items, err := db.ClaimCairnOutboxItems(context.Background(), "project_fact_projection", 10)
	if err != nil {
		t.Fatal(err)
	}
	var factItem *CairnOutboxItem
	for i := range items {
		event, err := db.GetCairnEventByID(context.Background(), items[i].EventID)
		if err != nil {
			t.Fatal(err)
		}
		if event.EventType == "fact_created" {
			it := items[i]
			factItem = &it
		}
	}
	if factItem == nil {
		t.Fatalf("expected fact_created outbox item, got %d items", len(items))
	}
	if err := db.AckCairnOutboxItem(context.Background(), factItem.EventID, "project_fact_projection"); err != nil {
		t.Fatal(err)
	}
	var status string
	if err := db.QueryRow(`SELECT status FROM cairn_outbox WHERE event_id = ? AND consumer = 'project_fact_projection'`, factItem.EventID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "delivered" {
		t.Fatalf("status=%s want delivered", status)
	}

	items2, err := db.ClaimCairnOutboxItems(context.Background(), "yaml_export", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items2) == 0 {
		t.Fatal("expected pending yaml_export items")
	}
	if err := db.NackCairnOutboxItem(context.Background(), items2[0].EventID, "yaml_export", "transient failure"); err != nil {
		t.Fatal(err)
	}
	var pendingStatus string
	var attempts int
	if err := db.QueryRow(`SELECT status, attempts FROM cairn_outbox WHERE event_id = ? AND consumer = 'yaml_export'`, items2[0].EventID).Scan(&pendingStatus, &attempts); err != nil {
		t.Fatal(err)
	}
	if pendingStatus != "pending" || attempts != 1 {
		t.Fatalf("status=%s attempts=%d", pendingStatus, attempts)
	}
}

func TestCairnGetActiveRunForConversation(t *testing.T) {
	db := newCairnTestDB(t)
	run := newCairnTestRun(t, db)
	active, err := db.GetActiveCairnRunForConversation(context.Background(), run.ConversationID)
	if err != nil {
		t.Fatal(err)
	}
	if active == nil || active.ID != run.ID {
		t.Fatalf("active=%+v", active)
	}
	if _, err := db.CompleteCairnRun(context.Background(), active.ID, active.Revision, "goal_reached", "", "active-run-complete"); err != nil {
		t.Fatal(err)
	}
	active, err = db.GetActiveCairnRunForConversation(context.Background(), run.ConversationID)
	if err != nil {
		t.Fatal(err)
	}
	if active != nil {
		t.Fatalf("expected no active run after terminal, got %+v", active)
	}
}
