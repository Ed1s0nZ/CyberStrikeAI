package database

import (
	"path/filepath"
	"testing"

	"go.uber.org/zap"
)

func TestRepairInterruptedManualAgentMessagesMarksPlaceholderFailed(t *testing.T) {
	db, err := NewDB(filepath.Join(t.TempDir(), "manual-repair.db"), zap.NewNop())
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer db.Close()

	conv, err := db.CreateConversation("interrupted manual", ConversationCreateMeta{})
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	assistantMsg, err := db.AddMessage(conv.ID, "assistant", "处理中...", nil)
	if err != nil {
		t.Fatalf("AddMessage assistant placeholder: %v", err)
	}
	normalMsg, err := db.AddMessage(conv.ID, "assistant", "正常历史回复", nil)
	if err != nil {
		t.Fatalf("AddMessage normal assistant: %v", err)
	}

	const reason = "服务重启导致任务中断，已自动标记失败"
	summary, err := db.RepairInterruptedManualAgentMessages(reason)
	if err != nil {
		t.Fatalf("RepairInterruptedManualAgentMessages: %v", err)
	}
	if summary.MessagesRepaired != 1 || summary.DetailsInserted != 1 {
		t.Fatalf("summary = %+v, want one repaired message and one detail", summary)
	}

	messages, err := db.GetMessages(conv.ID)
	if err != nil {
		t.Fatalf("GetMessages: %v", err)
	}
	byID := map[string]Message{}
	for _, msg := range messages {
		byID[msg.ID] = msg
	}
	if byID[assistantMsg.ID].Content != reason {
		t.Fatalf("placeholder content = %q, want %q", byID[assistantMsg.ID].Content, reason)
	}
	if byID[normalMsg.ID].Content != "正常历史回复" {
		t.Fatalf("normal assistant content changed to %q", byID[normalMsg.ID].Content)
	}

	details, err := db.GetProcessDetails(assistantMsg.ID)
	if err != nil {
		t.Fatalf("GetProcessDetails placeholder: %v", err)
	}
	if len(details) != 1 || details[0].EventType != "error" || details[0].Message != reason {
		t.Fatalf("process details = %+v, want one error detail with reason", details)
	}
}

func TestRepairInterruptedManualAgentMessagesIsIdempotent(t *testing.T) {
	db, err := NewDB(filepath.Join(t.TempDir(), "manual-repair-idempotent.db"), zap.NewNop())
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer db.Close()

	conv, err := db.CreateConversation("interrupted manual idempotent", ConversationCreateMeta{})
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	assistantMsg, err := db.AddMessage(conv.ID, "assistant", "处理中...", nil)
	if err != nil {
		t.Fatalf("AddMessage assistant placeholder: %v", err)
	}

	const reason = "服务重启导致任务中断，已自动标记失败"
	first, err := db.RepairInterruptedManualAgentMessages(reason)
	if err != nil {
		t.Fatalf("first RepairInterruptedManualAgentMessages: %v", err)
	}
	if first.MessagesRepaired != 1 || first.DetailsInserted != 1 {
		t.Fatalf("first summary = %+v, want one repaired message and one detail", first)
	}

	second, err := db.RepairInterruptedManualAgentMessages(reason)
	if err != nil {
		t.Fatalf("second RepairInterruptedManualAgentMessages: %v", err)
	}
	if second.MessagesRepaired != 0 || second.DetailsInserted != 0 {
		t.Fatalf("second summary = %+v, want no changes", second)
	}

	details, err := db.GetProcessDetails(assistantMsg.ID)
	if err != nil {
		t.Fatalf("GetProcessDetails placeholder: %v", err)
	}
	if len(details) != 1 {
		t.Fatalf("process detail count = %d, want 1: %+v", len(details), details)
	}
}
