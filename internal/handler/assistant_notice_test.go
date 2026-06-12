package handler

import (
	"path/filepath"
	"testing"

	"cyberstrike-ai/internal/database"

	"go.uber.org/zap"
)

func TestAppendAssistantMessageNoticeReplacesPlaceholder(t *testing.T) {
	db, err := database.NewDB(filepath.Join(t.TempDir(), "test.db"), zap.NewNop())
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	defer db.Close()

	h := &AgentHandler{db: db}
	conv, err := db.CreateConversation("notice test", database.ConversationCreateMeta{})
	if err != nil {
		t.Fatalf("CreateConversation failed: %v", err)
	}
	msg, err := db.AddMessage(conv.ID, "assistant", "处理中...", nil)
	if err != nil {
		t.Fatalf("AddMessage failed: %v", err)
	}

	const cancelMsg = "任务已被用户取消，后续操作已停止。"
	if err := h.appendAssistantMessageNotice(msg.ID, cancelMsg); err != nil {
		t.Fatalf("appendAssistantMessageNotice failed: %v", err)
	}

	messages, err := db.GetMessages(conv.ID)
	if err != nil {
		t.Fatalf("GetMessages failed: %v", err)
	}
	if got := messages[0].Content; got != cancelMsg {
		t.Fatalf("message content = %q, want %q", got, cancelMsg)
	}
}

func TestAppendAssistantMessageNoticeAppendsToExistingContentOnce(t *testing.T) {
	db, err := database.NewDB(filepath.Join(t.TempDir(), "test.db"), zap.NewNop())
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	defer db.Close()

	h := &AgentHandler{db: db}
	conv, err := db.CreateConversation("notice test", database.ConversationCreateMeta{})
	if err != nil {
		t.Fatalf("CreateConversation failed: %v", err)
	}
	msg, err := db.AddMessage(conv.ID, "assistant", "已生成的部分内容", nil)
	if err != nil {
		t.Fatalf("AddMessage failed: %v", err)
	}

	const cancelMsg = "任务已被用户取消，后续操作已停止。"
	if err := h.appendAssistantMessageNotice(msg.ID, cancelMsg); err != nil {
		t.Fatalf("appendAssistantMessageNotice failed: %v", err)
	}
	if err := h.appendAssistantMessageNotice(msg.ID, cancelMsg); err != nil {
		t.Fatalf("second appendAssistantMessageNotice failed: %v", err)
	}

	messages, err := db.GetMessages(conv.ID)
	if err != nil {
		t.Fatalf("GetMessages failed: %v", err)
	}
	want := "已生成的部分内容\n\n" + cancelMsg
	if got := messages[0].Content; got != want {
		t.Fatalf("message content = %q, want %q", got, want)
	}
}
