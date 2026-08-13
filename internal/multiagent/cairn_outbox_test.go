package multiagent

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"

	"cyberstrike-ai/internal/database"
)

// newCairnOutboxFixture 构建 canonical run + 一条 fact_created 事件。
func newCairnOutboxFixture(t *testing.T, db *database.DB, stateDir string) (*database.CairnRun, *database.CairnIntent, string) {
	t.Helper()
	project, err := db.CreateProject(&database.Project{Name: "cairn-outbox-proj"})
	if err != nil {
		t.Fatal(err)
	}
	conversation, err := db.CreateConversation("cairn outbox", database.ConversationCreateMeta{ProjectID: project.ID})
	if err != nil {
		t.Fatal(err)
	}
	run, err := db.CreateCairnRunWithScopeSnapshot(context.Background(), &database.CairnRun{
		ProjectID:         project.ID,
		ConversationID:    conversation.ID,
		GoalText:          "verify outbox projection",
		ScopeSnapshotJSON: `{"targets":["http://127.0.0.1"]}`,
		ScopeSHA256:       "outbox-scope",
	}, "outbox-run-create")
	if err != nil {
		t.Fatal(err)
	}
	intents, revision, err := db.CommitCairnReasonDecision(context.Background(), run.ID, run.Revision, []string{"inspect target"}, "outbox-reason")
	if err != nil {
		t.Fatal(err)
	}
	intent, revision, err := db.ClaimNextCairnIntent(context.Background(), run.ID, revision, "outbox-claim")
	if err != nil || intent == nil {
		t.Fatalf("intent=%+v err=%v", intent, err)
	}
	if _, _, err := db.CompleteCairnIntentWithFact(context.Background(), run.ID, revision, &database.CairnFact{
		SourceIntentID:  intents[0].ID,
		SourceSegmentID: "outbox-segment",
		SourceEventID:   "outbox-observation",
		Statement:       "target answered HTTP 200",
		ObservationType: "positive",
		Confidence:      "confirmed",
		EvidenceJSON:    `{}`,
		ProvenanceJSON:  `{}`,
	}, "outbox-fact"); err != nil {
		t.Fatal(err)
	}
	return run, intent, conversation.ID
}

func TestCairnOutboxWorkerProjectsFactAndYAML(t *testing.T) {
	stateDir := t.TempDir()
	db, err := database.NewDB(filepath.Join(t.TempDir(), "cairn-outbox.db"), zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	run, _, conversationID := newCairnOutboxFixture(t, db, stateDir)
	_ = conversationID

	worker := NewCairnOutboxWorker(db, stateDir, zap.NewNop(), 0)
	worker.DrainOnce(context.Background())

	// project_facts 投影
	var factCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM project_facts WHERE project_id = ? AND category = 'cairn_fact'`, run.ProjectID).Scan(&factCount); err != nil {
		t.Fatal(err)
	}
	if factCount != 1 {
		t.Fatalf("projected facts=%d want 1", factCount)
	}

	// YAML 导出
	yamlPath := StateFilePath(stateDir, run.ProjectID, run.ConversationID)
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		t.Fatalf("yaml export missing: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("yaml export is empty")
	}

	// outbox 全部投递
	var pending int
	if err := db.QueryRow(`SELECT COUNT(*) FROM cairn_outbox WHERE event_id IN (SELECT event_id FROM cairn_events WHERE run_id = ?) AND status != 'delivered'`, run.ID).Scan(&pending); err != nil {
		t.Fatal(err)
	}
	if pending != 0 {
		t.Fatalf("undelivered outbox items=%d", pending)
	}
}
