package database

import (
	"path/filepath"
	"testing"

	"go.uber.org/zap"
)

func TestModelTokenUsagePersistsFromUsageProcessDetail(t *testing.T) {
	db := newModelTokenUsageTestDB(t)
	conv, err := db.CreateConversation("usage", ConversationCreateMeta{})
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	msg, err := db.AddMessage(conv.ID, "assistant", "done", nil)
	if err != nil {
		t.Fatalf("AddMessage: %v", err)
	}
	if err := db.AddProcessDetail(msg.ID, conv.ID, modelTokenUsageEventType, "usage", map[string]interface{}{
		"source":           "eino",
		"orchestration":    "deep",
		"reason":           "final",
		"model":            "gpt-test",
		"modelCalls":       2,
		"promptTokens":     10,
		"completionTokens": 3,
		"totalTokens":      13,
		"cachedTokens":     4,
		"reasoningTokens":  1,
	}); err != nil {
		t.Fatalf("AddProcessDetail: %v", err)
	}

	stats, err := db.GetModelTokenUsageStats(ModelTokenUsageFilter{})
	if err != nil {
		t.Fatalf("GetModelTokenUsageStats: %v", err)
	}
	if stats.Summary.Events != 1 || stats.Summary.ModelCalls != 2 || stats.Summary.TotalTokens != 13 || stats.Summary.CachedTokens != 4 || stats.Summary.ReasoningTokens != 1 {
		t.Fatalf("summary = %#v", stats.Summary)
	}
	if len(stats.ByModel) != 1 || stats.ByModel[0].Key != "gpt-test" || stats.ByModel[0].TotalTokens != 13 {
		t.Fatalf("by model = %#v", stats.ByModel)
	}
}

func TestModelTokenUsageBackfillIsIdempotent(t *testing.T) {
	db := newModelTokenUsageTestDB(t)
	conv, err := db.CreateConversation("usage", ConversationCreateMeta{})
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	msg, err := db.AddMessage(conv.ID, "assistant", "done", nil)
	if err != nil {
		t.Fatalf("AddMessage: %v", err)
	}
	if err := db.AddProcessDetail(msg.ID, conv.ID, modelTokenUsageEventType, "usage", map[string]interface{}{
		"source": "eino", "modelCalls": 1, "promptTokens": 7, "completionTokens": 5, "totalTokens": 12,
	}); err != nil {
		t.Fatalf("AddProcessDetail: %v", err)
	}
	if err := db.BackfillModelTokenUsageFromProcessDetails(); err != nil {
		t.Fatalf("Backfill 1: %v", err)
	}
	if err := db.BackfillModelTokenUsageFromProcessDetails(); err != nil {
		t.Fatalf("Backfill 2: %v", err)
	}
	stats, err := db.GetModelTokenUsageStats(ModelTokenUsageFilter{})
	if err != nil {
		t.Fatalf("GetModelTokenUsageStats: %v", err)
	}
	if stats.Summary.Events != 1 || stats.Summary.TotalTokens != 12 {
		t.Fatalf("summary after backfill = %#v", stats.Summary)
	}
}

func newModelTokenUsageTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := NewDB(filepath.Join(t.TempDir(), "usage.db"), zap.NewNop())
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}
