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
	if err := db.CreateBatchQueue(queueID, "recover", "", "eino_single", "manual", "", nil, "", 1, tasks); err != nil {
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
	queueIDs, conversationIDs, failedTasks, err := db.RecoverInterruptedBatchQueues(restartMsg)
	if err != nil {
		t.Fatalf("RecoverInterruptedBatchQueues: %v", err)
	}
	if failedTasks != 1 {
		t.Fatalf("failedTasks = %d, want 1", failedTasks)
	}
	if len(queueIDs) != 1 || queueIDs[0] != queueID {
		t.Fatalf("queueIDs = %#v, want [%q]", queueIDs, queueID)
	}
	if len(conversationIDs) != 0 {
		t.Fatalf("conversationIDs = %#v, want empty", conversationIDs)
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

func TestRecoverInterruptedBatchQueuesDoesNotResumePausedQueue(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "batch-recovery-paused.db")
	db, err := NewDB(dbPath, zap.NewNop())
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer db.Close()

	queueID := "queue-paused-recover"
	tasks := []map[string]interface{}{
		{"id": "task-running", "message": "interrupted"},
		{"id": "task-pending", "message": "next"},
	}
	if err := db.CreateBatchQueue(queueID, "recover paused", "", "eino_single", "manual", "", nil, "", 1, tasks); err != nil {
		t.Fatalf("CreateBatchQueue: %v", err)
	}
	if err := db.UpdateBatchQueueStatus(queueID, "paused"); err != nil {
		t.Fatalf("UpdateBatchQueueStatus paused: %v", err)
	}
	if err := db.UpdateBatchTaskStatus(queueID, "task-running", "running", "", "", ""); err != nil {
		t.Fatalf("UpdateBatchTaskStatus task-running: %v", err)
	}

	const restartMsg = "服务重启，任务已中断，请重新发起或继续"
	queueIDs, _, failedTasks, err := db.RecoverInterruptedBatchQueues(restartMsg)
	if err != nil {
		t.Fatalf("RecoverInterruptedBatchQueues: %v", err)
	}
	if len(queueIDs) != 0 {
		t.Fatalf("queueIDs = %#v, want empty for paused queue", queueIDs)
	}
	if failedTasks != 1 {
		t.Fatalf("failedTasks = %d, want 1", failedTasks)
	}

	queue, err := db.GetBatchQueue(queueID)
	if err != nil {
		t.Fatalf("GetBatchQueue: %v", err)
	}
	if queue.Status != "paused" {
		t.Fatalf("queue status = %q, want paused", queue.Status)
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
	if statuses["task-running"] != "failed" {
		t.Fatalf("task-running status = %q, want failed", statuses["task-running"])
	}
	if errors["task-running"] != restartMsg {
		t.Fatalf("task-running error = %q, want %q", errors["task-running"], restartMsg)
	}
	if statuses["task-pending"] != "pending" {
		t.Fatalf("task-pending status = %q, want pending", statuses["task-pending"])
	}
}

func TestRecoverInterruptedBatchQueuesBeforeSkipsTasksClaimedAfterSnapshot(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "batch-recovery-snapshot.db")
	db, err := NewDB(dbPath, zap.NewNop())
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer db.Close()

	queueID := "queue-recover-snapshot"
	tasks := []map[string]interface{}{
		{"id": "task-running-before", "message": "before restart"},
	}
	if err := db.CreateBatchQueue(queueID, "recover snapshot", "", "eino_single", "manual", "", nil, "", 1, tasks); err != nil {
		t.Fatalf("CreateBatchQueue: %v", err)
	}
	if err := db.UpdateBatchQueueStatus(queueID, "running"); err != nil {
		t.Fatalf("UpdateBatchQueueStatus running: %v", err)
	}
	if err := db.UpdateBatchTaskStatus(queueID, "task-running-before", "running", "", "", ""); err != nil {
		t.Fatalf("UpdateBatchTaskStatus before: %v", err)
	}
	maxTaskRowID, err := db.MaxBatchTaskRowID()
	if err != nil {
		t.Fatalf("MaxBatchTaskRowID: %v", err)
	}

	if _, err := db.Exec(
		`INSERT INTO batch_tasks (id, queue_id, message, status, started_at)
		 VALUES (?, ?, ?, ?, ?)`,
		"task-running-after", queueID, "claimed after startup", "running", time.Now(),
	); err != nil {
		t.Fatalf("insert after snapshot task: %v", err)
	}

	const restartMsg = "服务重启，任务已中断，请重新发起或继续"
	queueIDs, _, failedTasks, err := db.RecoverInterruptedBatchQueuesBefore(restartMsg, maxTaskRowID)
	if err != nil {
		t.Fatalf("RecoverInterruptedBatchQueuesBefore: %v", err)
	}
	if failedTasks != 1 {
		t.Fatalf("failedTasks = %d, want 1", failedTasks)
	}
	if len(queueIDs) != 1 || queueIDs[0] != queueID {
		t.Fatalf("queueIDs = %#v, want [%q]", queueIDs, queueID)
	}

	gotTasks, err := db.GetBatchTasks(queueID)
	if err != nil {
		t.Fatalf("GetBatchTasks: %v", err)
	}
	statuses := map[string]string{}
	for _, task := range gotTasks {
		statuses[task.ID] = task.Status
	}
	if statuses["task-running-before"] != "failed" {
		t.Fatalf("task-running-before status = %q, want failed", statuses["task-running-before"])
	}
	if statuses["task-running-after"] != "running" {
		t.Fatalf("task-running-after status = %q, want running", statuses["task-running-after"])
	}
}

func TestClaimBatchTaskForRunClearsStalePersistentConversationData(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "batch-claim.db")
	db, err := NewDB(dbPath, zap.NewNop())
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer db.Close()

	oldConv, err := db.CreateConversation("old", ConversationCreateMeta{})
	if err != nil {
		t.Fatalf("CreateConversation old: %v", err)
	}

	queueID := "queue-claim"
	tasks := []map[string]interface{}{
		{"id": "task-1", "message": "retry this task"},
	}
	if err := db.CreateBatchQueue(queueID, "claim", "", "eino_single", "manual", "", nil, "", 1, tasks); err != nil {
		t.Fatalf("CreateBatchQueue: %v", err)
	}
	if err := db.UpdateBatchTaskStatus(queueID, "task-1", "failed", oldConv.ID, "old result", "old error"); err != nil {
		t.Fatalf("UpdateBatchTaskStatus failed: %v", err)
	}
	if err := db.UpdateBatchTaskStatus(queueID, "task-1", "pending", "", "", ""); err != nil {
		t.Fatalf("UpdateBatchTaskStatus pending: %v", err)
	}

	if err := db.ClaimBatchTaskForRun(queueID, "task-1"); err != nil {
		t.Fatalf("ClaimBatchTaskForRun: %v", err)
	}

	gotTasks, err := db.GetBatchTasks(queueID)
	if err != nil {
		t.Fatalf("GetBatchTasks: %v", err)
	}
	if len(gotTasks) != 1 {
		t.Fatalf("task count = %d, want 1", len(gotTasks))
	}
	task := gotTasks[0]
	if task.Status != "running" {
		t.Fatalf("task status = %q, want running", task.Status)
	}
	if !task.ConversationID.Valid || task.ConversationID.String != oldConv.ID {
		t.Fatalf("conversation_id = %#v, want old conversation preserved until replacement", task.ConversationID)
	}
	if task.Error.Valid {
		t.Fatalf("error = %q, want NULL", task.Error.String)
	}
	if task.Result.Valid {
		t.Fatalf("result = %q, want NULL", task.Result.String)
	}
	if task.CompletedAt.Valid {
		t.Fatalf("completed_at = %#v, want NULL", task.CompletedAt.Time)
	}
	if !task.StartedAt.Valid {
		t.Fatalf("started_at is NULL, want set")
	}
}

func TestUpdateBatchTaskStatusClearsStaleErrorForNewTerminalState(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "batch-status-clear.db")
	db, err := NewDB(dbPath, zap.NewNop())
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer db.Close()

	queueID := "queue-status-clear"
	tasks := []map[string]interface{}{
		{"id": "task-1", "message": "cancelled task"},
		{"id": "task-2", "message": "completed task"},
	}
	if err := db.CreateBatchQueue(queueID, "status clear", "", "eino_single", "manual", "", nil, "", 1, tasks); err != nil {
		t.Fatalf("CreateBatchQueue: %v", err)
	}
	const restartMsg = "服务重启，任务已中断，请重新发起或继续"
	if err := db.UpdateBatchTaskStatus(queueID, "task-1", "failed", "old-conv-1", "", restartMsg); err != nil {
		t.Fatalf("UpdateBatchTaskStatus task-1 failed: %v", err)
	}
	if err := db.UpdateBatchTaskStatus(queueID, "task-1", "cancelled", "new-conv-1", "任务已被用户取消，后续操作已停止。", ""); err != nil {
		t.Fatalf("UpdateBatchTaskStatus task-1 cancelled: %v", err)
	}
	if err := db.UpdateBatchTaskStatus(queueID, "task-2", "failed", "old-conv-2", "", restartMsg); err != nil {
		t.Fatalf("UpdateBatchTaskStatus task-2 failed: %v", err)
	}
	if err := db.UpdateBatchTaskStatus(queueID, "task-2", "completed", "new-conv-2", "ok", ""); err != nil {
		t.Fatalf("UpdateBatchTaskStatus task-2 completed: %v", err)
	}

	gotTasks, err := db.GetBatchTasks(queueID)
	if err != nil {
		t.Fatalf("GetBatchTasks: %v", err)
	}
	byID := map[string]*BatchTaskRow{}
	for _, task := range gotTasks {
		byID[task.ID] = task
	}
	if byID["task-1"].Error.Valid {
		t.Fatalf("cancelled task error = %q, want NULL", byID["task-1"].Error.String)
	}
	if !byID["task-1"].Result.Valid || byID["task-1"].Result.String != "任务已被用户取消，后续操作已停止。" {
		t.Fatalf("cancelled task result = %#v, want cancel result", byID["task-1"].Result)
	}
	if byID["task-2"].Error.Valid {
		t.Fatalf("completed task error = %q, want NULL", byID["task-2"].Error.String)
	}
	if !byID["task-2"].Result.Valid || byID["task-2"].Result.String != "ok" {
		t.Fatalf("completed task result = %#v, want ok", byID["task-2"].Result)
	}
}

func TestRecoverInterruptedAssistantPlaceholdersSkipsBatchPendingConversations(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "assistant-recovery.db")
	db, err := NewDB(dbPath, zap.NewNop())
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer db.Close()

	runningConv, err := db.CreateConversation("running", ConversationCreateMeta{})
	if err != nil {
		t.Fatalf("CreateConversation running: %v", err)
	}
	if _, err := db.AddMessage(runningConv.ID, "user", "running", nil); err != nil {
		t.Fatalf("AddMessage running user: %v", err)
	}
	runningMsg, err := db.AddMessage(runningConv.ID, "assistant", "处理中...", nil)
	if err != nil {
		t.Fatalf("AddMessage running assistant: %v", err)
	}

	pendingConv, err := db.CreateConversation("pending", ConversationCreateMeta{})
	if err != nil {
		t.Fatalf("CreateConversation pending: %v", err)
	}
	if _, err := db.AddMessage(pendingConv.ID, "user", "pending", nil); err != nil {
		t.Fatalf("AddMessage pending user: %v", err)
	}
	pendingMsg, err := db.AddMessage(pendingConv.ID, "assistant", "处理中...", nil)
	if err != nil {
		t.Fatalf("AddMessage pending assistant: %v", err)
	}

	queueID := "queue-placeholders"
	tasks := []map[string]interface{}{
		{"id": "task-running", "message": "running"},
		{"id": "task-pending", "message": "pending"},
	}
	if err := db.CreateBatchQueue(queueID, "recover", "", "eino_single", "manual", "", nil, "", 1, tasks); err != nil {
		t.Fatalf("CreateBatchQueue: %v", err)
	}
	if err := db.UpdateBatchQueueStatus(queueID, "running"); err != nil {
		t.Fatalf("UpdateBatchQueueStatus: %v", err)
	}
	if err := db.UpdateBatchTaskStatus(queueID, "task-running", "running", runningConv.ID, "", ""); err != nil {
		t.Fatalf("UpdateBatchTaskStatus running: %v", err)
	}
	if err := db.UpdateBatchTaskStatus(queueID, "task-pending", "pending", pendingConv.ID, "", ""); err != nil {
		t.Fatalf("UpdateBatchTaskStatus pending: %v", err)
	}

	const restartMsg = "服务重启，任务已中断，请重新发起或继续"
	_, conversationIDs, _, err := db.RecoverInterruptedBatchQueues(restartMsg)
	if err != nil {
		t.Fatalf("RecoverInterruptedBatchQueues: %v", err)
	}
	n, err := db.RecoverInterruptedAssistantPlaceholdersForConversations(restartMsg, conversationIDs)
	if err != nil {
		t.Fatalf("RecoverInterruptedAssistantPlaceholdersForConversations: %v", err)
	}
	if n != 1 {
		t.Fatalf("recovered placeholders = %d, want 1", n)
	}
	n, err = db.RecoverInterruptedAssistantPlaceholders(restartMsg)
	if err != nil {
		t.Fatalf("RecoverInterruptedAssistantPlaceholders: %v", err)
	}
	if n != 0 {
		t.Fatalf("non-batch recovered placeholders = %d, want 0", n)
	}

	messages, err := db.GetMessages(runningConv.ID)
	if err != nil {
		t.Fatalf("GetMessages running: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("running message count = %d, want 2", len(messages))
	}
	if messages[1].ID != runningMsg.ID || messages[1].Content != restartMsg {
		t.Fatalf("running assistant message = %#v, want content %q on %q", messages[1], restartMsg, runningMsg.ID)
	}
	messages, err = db.GetMessages(pendingConv.ID)
	if err != nil {
		t.Fatalf("GetMessages pending: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("pending message count = %d, want 2", len(messages))
	}
	if messages[1].ID != pendingMsg.ID || messages[1].Content != "处理中..." {
		t.Fatalf("pending assistant message = %#v, want placeholder on %q", messages[1], pendingMsg.ID)
	}
	details, err := db.GetProcessDetails(runningMsg.ID)
	if err != nil {
		t.Fatalf("GetProcessDetails: %v", err)
	}
	if len(details) != 1 || details[0].EventType != "error" || details[0].Message != restartMsg {
		t.Fatalf("process details = %#v, want restart error detail", details)
	}
}

func TestRecoverInterruptedAssistantPlaceholdersBeforeUsesStartupSnapshot(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "assistant-snapshot-recovery.db")
	db, err := NewDB(dbPath, zap.NewNop())
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer db.Close()

	batchConv, err := db.CreateConversation("batch pending", ConversationCreateMeta{})
	if err != nil {
		t.Fatalf("CreateConversation batch: %v", err)
	}
	if _, err := db.AddMessage(batchConv.ID, "user", "batch pending", nil); err != nil {
		t.Fatalf("AddMessage batch user: %v", err)
	}
	batchMsg, err := db.AddMessage(batchConv.ID, "assistant", "处理中...", nil)
	if err != nil {
		t.Fatalf("AddMessage batch assistant: %v", err)
	}

	ordinaryConv, err := db.CreateConversation("ordinary", ConversationCreateMeta{})
	if err != nil {
		t.Fatalf("CreateConversation ordinary: %v", err)
	}
	if _, err := db.AddMessage(ordinaryConv.ID, "user", "ordinary", nil); err != nil {
		t.Fatalf("AddMessage ordinary user: %v", err)
	}
	ordinaryMsg, err := db.AddMessage(ordinaryConv.ID, "assistant", "处理中...", nil)
	if err != nil {
		t.Fatalf("AddMessage ordinary assistant: %v", err)
	}

	queueID := "queue-snapshot"
	tasks := []map[string]interface{}{
		{"id": "task-pending", "message": "pending"},
	}
	if err := db.CreateBatchQueue(queueID, "snapshot", "", "eino_single", "manual", "", nil, "", 1, tasks); err != nil {
		t.Fatalf("CreateBatchQueue: %v", err)
	}
	if err := db.UpdateBatchTaskStatus(queueID, "task-pending", "pending", batchConv.ID, "", ""); err != nil {
		t.Fatalf("UpdateBatchTaskStatus pending: %v", err)
	}

	maxRowID, err := db.MaxMessageRowID()
	if err != nil {
		t.Fatalf("MaxMessageRowID: %v", err)
	}
	excluded, err := db.ListBatchTaskConversationIDs()
	if err != nil {
		t.Fatalf("ListBatchTaskConversationIDs: %v", err)
	}

	newConv, err := db.CreateConversation("new batch run", ConversationCreateMeta{})
	if err != nil {
		t.Fatalf("CreateConversation new: %v", err)
	}
	if _, err := db.AddMessage(newConv.ID, "user", "new run", nil); err != nil {
		t.Fatalf("AddMessage new user: %v", err)
	}
	newMsg, err := db.AddMessage(newConv.ID, "assistant", "处理中...", nil)
	if err != nil {
		t.Fatalf("AddMessage new assistant: %v", err)
	}
	if err := db.UpdateBatchTaskStatus(queueID, "task-pending", "running", newConv.ID, "", ""); err != nil {
		t.Fatalf("UpdateBatchTaskStatus running: %v", err)
	}

	const restartMsg = "服务重启，任务已中断，请重新发起或继续"
	n, err := db.RecoverInterruptedAssistantPlaceholdersBefore(restartMsg, maxRowID, excluded)
	if err != nil {
		t.Fatalf("RecoverInterruptedAssistantPlaceholdersBefore: %v", err)
	}
	if n != 1 {
		t.Fatalf("recovered placeholders = %d, want only ordinary placeholder", n)
	}

	messages, err := db.GetMessages(ordinaryConv.ID)
	if err != nil {
		t.Fatalf("GetMessages ordinary: %v", err)
	}
	if messages[1].ID != ordinaryMsg.ID || messages[1].Content != restartMsg {
		t.Fatalf("ordinary assistant = %#v, want restart message", messages[1])
	}
	messages, err = db.GetMessages(batchConv.ID)
	if err != nil {
		t.Fatalf("GetMessages batch: %v", err)
	}
	if messages[1].ID != batchMsg.ID || messages[1].Content != "处理中..." {
		t.Fatalf("batch assistant = %#v, want unchanged placeholder", messages[1])
	}
	messages, err = db.GetMessages(newConv.ID)
	if err != nil {
		t.Fatalf("GetMessages new: %v", err)
	}
	if messages[1].ID != newMsg.ID || messages[1].Content != "处理中..." {
		t.Fatalf("new assistant = %#v, want unchanged placeholder", messages[1])
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
