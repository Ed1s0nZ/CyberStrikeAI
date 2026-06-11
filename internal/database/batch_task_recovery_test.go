package database

import (
	"path/filepath"
	"testing"
	"time"

	"cyberstrike-ai/internal/mcp"

	"go.uber.org/zap"
)

func TestRecoverInterruptedBatchQueuesContinuesAfterFailedRunningTask(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "batch-recovery.db")
	db, err := NewDB(dbPath, zap.NewNop())
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer db.Close()

	queueID := "queue-recover"
	tasks := []map[string]interface{}{
		{"id": "task-1", "message": "done"},
		{"id": "task-2", "message": "interrupted"},
		{"id": "task-3", "message": "next"},
	}
	if err := db.CreateBatchQueue(queueID, "recover", "", "eino_single", "manual", "", nil, "", tasks); err != nil {
		t.Fatalf("CreateBatchQueue: %v", err)
	}
	if err := db.UpdateBatchQueueStatus(queueID, "running"); err != nil {
		t.Fatalf("UpdateBatchQueueStatus running: %v", err)
	}
	if err := db.UpdateBatchTaskStatus(queueID, "task-1", "completed", "", "ok", ""); err != nil {
		t.Fatalf("UpdateBatchTaskStatus task-1: %v", err)
	}
	if err := db.UpdateBatchTaskStatus(queueID, "task-2", "running", "", "", ""); err != nil {
		t.Fatalf("UpdateBatchTaskStatus task-2: %v", err)
	}
	if err := db.UpdateBatchQueueCurrentIndex(queueID, 1); err != nil {
		t.Fatalf("UpdateBatchQueueCurrentIndex: %v", err)
	}

	const restartMsg = "服务重启，任务已中断，请重新发起或继续"
	queueIDs, failedTasks, err := db.RecoverInterruptedBatchQueues(restartMsg)
	if err != nil {
		t.Fatalf("RecoverInterruptedBatchQueues: %v", err)
	}
	if failedTasks != 1 {
		t.Fatalf("failedTasks = %d, want 1", failedTasks)
	}
	if len(queueIDs) != 1 || queueIDs[0] != queueID {
		t.Fatalf("queueIDs = %#v, want [%q]", queueIDs, queueID)
	}

	queue, err := db.GetBatchQueue(queueID)
	if err != nil {
		t.Fatalf("GetBatchQueue: %v", err)
	}
	if queue.Status != "running" {
		t.Fatalf("queue status = %q, want running", queue.Status)
	}
	if queue.CurrentIndex != 1 {
		t.Fatalf("queue current_index = %d, want 1", queue.CurrentIndex)
	}
	if queue.LastRunError.Valid {
		t.Fatalf("last_run_error = %#v, want unset", queue.LastRunError)
	}

	gotTasks, err := db.GetBatchTasks(queueID)
	if err != nil {
		t.Fatalf("GetBatchTasks: %v", err)
	}
	statuses := map[string]string{}
	errors := map[string]string{}
	for _, task := range gotTasks {
		statuses[task.ID] = task.Status
		if task.Error.Valid {
			errors[task.ID] = task.Error.String
		}
	}
	if statuses["task-1"] != "completed" {
		t.Fatalf("task-1 status = %q, want completed", statuses["task-1"])
	}
	if statuses["task-2"] != "failed" {
		t.Fatalf("task-2 status = %q, want failed", statuses["task-2"])
	}
	if errors["task-2"] != restartMsg {
		t.Fatalf("task-2 error = %q, want %q", errors["task-2"], restartMsg)
	}
	if statuses["task-3"] != "pending" {
		t.Fatalf("task-3 status = %q, want pending", statuses["task-3"])
	}
}

func TestRecoverInterruptedAssistantPlaceholders(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "assistant-recovery.db")
	db, err := NewDB(dbPath, zap.NewNop())
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer db.Close()

	conv, err := db.CreateConversation("recover", ConversationCreateMeta{})
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	if _, err := db.AddMessage(conv.ID, "user", "hello", nil); err != nil {
		t.Fatalf("AddMessage user: %v", err)
	}
	assistantMsg, err := db.AddMessage(conv.ID, "assistant", "处理中...", nil)
	if err != nil {
		t.Fatalf("AddMessage assistant: %v", err)
	}

	const restartMsg = "服务重启，任务已中断，请重新发起或继续"
	n, err := db.RecoverInterruptedAssistantPlaceholders(restartMsg)
	if err != nil {
		t.Fatalf("RecoverInterruptedAssistantPlaceholders: %v", err)
	}
	if n != 1 {
		t.Fatalf("recovered placeholders = %d, want 1", n)
	}

	messages, err := db.GetMessages(conv.ID)
	if err != nil {
		t.Fatalf("GetMessages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("message count = %d, want 2", len(messages))
	}
	if messages[1].ID != assistantMsg.ID || messages[1].Content != restartMsg {
		t.Fatalf("assistant message = %#v, want content %q on %q", messages[1], restartMsg, assistantMsg.ID)
	}
	details, err := db.GetProcessDetails(assistantMsg.ID)
	if err != nil {
		t.Fatalf("GetProcessDetails: %v", err)
	}
	if len(details) != 1 || details[0].EventType != "error" || details[0].Message != restartMsg {
		t.Fatalf("process details = %#v, want restart error detail", details)
	}
}

func TestRecoverInterruptedToolExecutions(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "tool-recovery.db")
	db, err := NewDB(dbPath, zap.NewNop())
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer db.Close()

	exec := &mcp.ToolExecution{
		ID:        "tool-recover",
		ToolName:  "shell",
		Arguments: map[string]interface{}{"cmd": "sleep 10"},
		Status:    "running",
		StartTime: time.Now().Add(-time.Minute),
	}
	if err := db.SaveToolExecution(exec); err != nil {
		t.Fatalf("SaveToolExecution: %v", err)
	}

	const restartMsg = "服务重启，任务已中断，请重新发起或继续"
	n, err := db.RecoverInterruptedToolExecutions(restartMsg)
	if err != nil {
		t.Fatalf("RecoverInterruptedToolExecutions: %v", err)
	}
	if n != 1 {
		t.Fatalf("recovered executions = %d, want 1", n)
	}

	got, err := db.GetToolExecution(exec.ID)
	if err != nil {
		t.Fatalf("GetToolExecution: %v", err)
	}
	if got.Status != "failed" {
		t.Fatalf("tool status = %q, want failed", got.Status)
	}
	if got.Error != restartMsg {
		t.Fatalf("tool error = %q, want %q", got.Error, restartMsg)
	}
	if got.EndTime == nil {
		t.Fatal("tool end time is nil, want set")
	}
}
