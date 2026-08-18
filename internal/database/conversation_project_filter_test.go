package database

import (
	"path/filepath"
	"testing"

	"go.uber.org/zap"
)

func TestConversationProjectFilter(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "conversations.db")
	db, err := NewDB(dbPath, zap.NewNop())
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer db.Close()

	p, err := db.CreateProject(&Project{Name: "target-a", Status: "active"})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	convNone, err := db.CreateConversation("unbound", ConversationCreateMeta{})
	if err != nil {
		t.Fatalf("CreateConversation unbound: %v", err)
	}
	convBound, err := db.CreateConversation("bound", ConversationCreateMeta{ProjectID: p.ID})
	if err != nil {
		t.Fatalf("CreateConversation bound: %v", err)
	}

	totalAll, err := db.CountConversations("", "")
	if err != nil || totalAll < 2 {
		t.Fatalf("CountConversations all: total=%d err=%v", totalAll, err)
	}

	totalBound, err := db.CountConversations("", p.ID)
	if err != nil || totalBound != 1 {
		t.Fatalf("CountConversations project: total=%d err=%v", totalBound, err)
	}

	totalUnbound, err := db.CountConversations("", ProjectFilterUnbound)
	if err != nil || totalUnbound != 1 {
		t.Fatalf("CountConversations unbound: total=%d err=%v", totalUnbound, err)
	}

	listBound, err := db.ListConversations(10, 0, "", "", p.ID)
	if err != nil || len(listBound) != 1 || listBound[0].ID != convBound.ID {
		t.Fatalf("ListConversations project: %+v err=%v", listBound, err)
	}

	listUnbound, err := db.ListConversations(10, 0, "", "", ProjectFilterUnbound)
	if err != nil || len(listUnbound) != 1 || listUnbound[0].ID != convNone.ID {
		t.Fatalf("ListConversations unbound: %+v err=%v", listUnbound, err)
	}

	_ = convNone
	_ = convBound
}
