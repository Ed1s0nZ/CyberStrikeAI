package database

import (
	"path/filepath"
	"testing"

	"go.uber.org/zap"
)

func TestRepairInterruptedBatchRunsPausesQueueAndFailsRunningTask(t *testing.T) {
	db, err := NewDB(filepath.Join(t.TempDir(), "batch-repair.db"), zap.NewNop())
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer db.Close()

	queueID := "queue-with-pending"
	runningTaskID := "task-running"
	pendingTaskID := "task-pending"
	if err := db.CreateBatchQueue(queueID, "repair test", "", "eino_single", "manual", "", nil, "", []map[string]interface{}{
		{"id": runningTaskID, "message": "running task"},
		{"id": pendingTaskID, "message": "pending task"},
	}); err != nil {
		t.Fatalf("CreateBatchQueue: %v", err)
	}

	conv, err := db.CreateConversation("interrupted batch", ConversationCreateMeta{})
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	assistantMsg, err := db.AddMessage(conv.ID, "assistant", "处理中...", nil)
	if err != nil {
		t.Fatalf("AddMessage: %v", err)
	}
	if err := db.UpdateBatchTaskStatus(queueID, runningTaskID, "running", conv.ID, "", ""); err != nil {
		t.Fatalf("UpdateBatchTaskStatus running: %v", err)
	}
	if err := db.UpdateBatchQueueStatus(queueID, "running"); err != nil {
		t.Fatalf("UpdateBatchQueueStatus running: %v", err)
	}

	const reason = "服务重启导致批量任务中断，已自动标记当前子任务失败"
	summary, err := db.RepairInterruptedBatchRuns(reason)
	if err != nil {
		t.Fatalf("RepairInterruptedBatchRuns: %v", err)
	}
	if summary.TasksFailed != 1 || summary.QueuesRepaired != 1 {
		t.Fatalf("summary = %+v, want 1 failed task and 1 repaired queue", summary)
	}

	queue, err := db.GetBatchQueue(queueID)
	if err != nil {
		t.Fatalf("GetBatchQueue: %v", err)
	}
	if queue.Status != "paused" {
		t.Fatalf("queue status = %q, want paused", queue.Status)
	}
	if !queue.LastRunError.Valid || queue.LastRunError.String != reason {
		t.Fatalf("last_run_error = %#v, want %q", queue.LastRunError, reason)
	}
	if queue.CompletedAt.Valid {
		t.Fatalf("paused queue completed_at should be empty, got %v", queue.CompletedAt.Time)
	}

	tasks, err := db.GetBatchTasks(queueID)
	if err != nil {
		t.Fatalf("GetBatchTasks: %v", err)
	}
	byID := map[string]*BatchTaskRow{}
	for _, task := range tasks {
		byID[task.ID] = task
	}
	if byID[runningTaskID].Status != "failed" {
		t.Fatalf("running task status = %q, want failed", byID[runningTaskID].Status)
	}
	if !byID[runningTaskID].CompletedAt.Valid {
		t.Fatal("failed running task should have completed_at")
	}
	if !byID[runningTaskID].Error.Valid || byID[runningTaskID].Error.String != reason {
		t.Fatalf("running task error = %#v, want %q", byID[runningTaskID].Error, reason)
	}
	if byID[pendingTaskID].Status != "pending" {
		t.Fatalf("pending task status = %q, want pending", byID[pendingTaskID].Status)
	}

	messages, err := db.GetMessages(conv.ID)
	if err != nil {
		t.Fatalf("GetMessages: %v", err)
	}
	var gotAssistant *Message
	for i := range messages {
		if messages[i].ID == assistantMsg.ID {
			gotAssistant = &messages[i]
			break
		}
	}
	if gotAssistant == nil {
		t.Fatal("assistant message not found")
	}
	if gotAssistant.Content != reason {
		t.Fatalf("assistant content = %q, want %q", gotAssistant.Content, reason)
	}
	details, err := db.GetProcessDetails(assistantMsg.ID)
	if err != nil {
		t.Fatalf("GetProcessDetails: %v", err)
	}
	if len(details) != 1 || details[0].EventType != "error" || details[0].Message != reason {
		t.Fatalf("process details = %+v, want one error detail with reason", details)
	}
}

func TestRepairInterruptedBatchRunsCompletesQueueWithoutPendingTasks(t *testing.T) {
	db, err := NewDB(filepath.Join(t.TempDir(), "batch-repair-complete.db"), zap.NewNop())
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer db.Close()

	queueID := "queue-without-pending"
	taskID := "task-only"
	if err := db.CreateBatchQueue(queueID, "repair complete test", "", "eino_single", "manual", "", nil, "", []map[string]interface{}{
		{"id": taskID, "message": "running task"},
	}); err != nil {
		t.Fatalf("CreateBatchQueue: %v", err)
	}
	if err := db.UpdateBatchTaskStatus(queueID, taskID, "running", "", "", ""); err != nil {
		t.Fatalf("UpdateBatchTaskStatus running: %v", err)
	}
	if err := db.UpdateBatchQueueStatus(queueID, "running"); err != nil {
		t.Fatalf("UpdateBatchQueueStatus running: %v", err)
	}

	const reason = "restart interrupted batch task"
	summary, err := db.RepairInterruptedBatchRuns(reason)
	if err != nil {
		t.Fatalf("RepairInterruptedBatchRuns: %v", err)
	}
	if summary.TasksFailed != 1 || summary.QueuesRepaired != 1 {
		t.Fatalf("summary = %+v, want 1 failed task and 1 repaired queue", summary)
	}

	queue, err := db.GetBatchQueue(queueID)
	if err != nil {
		t.Fatalf("GetBatchQueue: %v", err)
	}
	if queue.Status != "completed" {
		t.Fatalf("queue status = %q, want completed", queue.Status)
	}
	if !queue.CompletedAt.Valid {
		t.Fatal("completed repaired queue should have completed_at")
	}
	if !queue.LastRunError.Valid || queue.LastRunError.String != reason {
		t.Fatalf("last_run_error = %#v, want %q", queue.LastRunError, reason)
	}
}
