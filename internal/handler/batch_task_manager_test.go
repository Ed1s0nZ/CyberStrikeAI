package handler

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestGetNextTaskReturnsEarliestPendingTask(t *testing.T) {
	manager := NewBatchTaskManager(zap.NewNop())
	queueID := "queue-resume-order"
	manager.queues[queueID] = &BatchTaskQueue{
		ID:           queueID,
		Status:       BatchQueueStatusPaused,
		CreatedAt:    time.Now(),
		CurrentIndex: 6,
		Tasks: []*BatchTask{
			{ID: "task-1", Status: BatchTaskStatusCancelled},
			{ID: "task-2", Status: BatchTaskStatusCancelled},
			{ID: "task-3", Status: BatchTaskStatusPending},
			{ID: "task-4", Status: BatchTaskStatusCancelled},
			{ID: "task-5", Status: BatchTaskStatusCancelled},
			{ID: "task-6", Status: BatchTaskStatusCancelled},
			{ID: "task-7", Status: BatchTaskStatusPending},
		},
	}

	task, ok := manager.GetNextTask(queueID)
	if !ok {
		t.Fatal("GetNextTask returned no task")
	}
	if task.ID != "task-3" {
		t.Fatalf("GetNextTask returned %q, want task-3", task.ID)
	}
	if got := manager.queues[queueID].CurrentIndex; got != 2 {
		t.Fatalf("CurrentIndex = %d, want 2", got)
	}

	task.Status = BatchTaskStatusCompleted
	manager.MoveToNextTask(queueID)
	task, ok = manager.GetNextTask(queueID)
	if !ok {
		t.Fatal("GetNextTask after MoveToNextTask returned no task")
	}
	if task.ID != "task-7" {
		t.Fatalf("GetNextTask after MoveToNextTask returned %q, want task-7", task.ID)
	}
}

func TestClaimNextTaskClaimsDistinctEarliestPendingTasks(t *testing.T) {
	manager := NewBatchTaskManager(zap.NewNop())
	queueID := "queue-concurrent-claim"
	manager.queues[queueID] = &BatchTaskQueue{
		ID:           queueID,
		Status:       BatchQueueStatusRunning,
		CreatedAt:    time.Now(),
		CurrentIndex: 6,
		Tasks: []*BatchTask{
			{ID: "task-1", Status: BatchTaskStatusCancelled},
			{ID: "task-2", Status: BatchTaskStatusPending},
			{ID: "task-3", Status: BatchTaskStatusPending},
			{ID: "task-4", Status: BatchTaskStatusCancelled},
		},
	}

	first, ok := manager.ClaimNextTask(queueID)
	if !ok {
		t.Fatal("first ClaimNextTask returned no task")
	}
	if first.ID != "task-2" {
		t.Fatalf("first ClaimNextTask returned %q, want task-2", first.ID)
	}
	if first.Status != BatchTaskStatusRunning {
		t.Fatalf("first task status = %q, want running", first.Status)
	}

	second, ok := manager.ClaimNextTask(queueID)
	if !ok {
		t.Fatal("second ClaimNextTask returned no task")
	}
	if second.ID != "task-3" {
		t.Fatalf("second ClaimNextTask returned %q, want task-3", second.ID)
	}
	if second.Status != BatchTaskStatusRunning {
		t.Fatalf("second task status = %q, want running", second.Status)
	}

	if _, ok := manager.ClaimNextTask(queueID); ok {
		t.Fatal("third ClaimNextTask returned a task, want none")
	}
	pending, running, exists := manager.QueueHasActiveTasks(queueID)
	if !exists {
		t.Fatal("QueueHasActiveTasks returned exists=false")
	}
	if pending {
		t.Fatal("QueueHasActiveTasks pending=true, want false")
	}
	if !running {
		t.Fatal("QueueHasActiveTasks running=false, want true")
	}
}

func TestClaimNextTaskClearsStaleConversationData(t *testing.T) {
	manager := NewBatchTaskManager(zap.NewNop())
	queueID := "queue-stale-conversation"
	completedAt := time.Now()
	manager.queues[queueID] = &BatchTaskQueue{
		ID:        queueID,
		Status:    BatchQueueStatusRunning,
		CreatedAt: time.Now(),
		Tasks: []*BatchTask{
			{
				ID:             "task-1",
				Status:         BatchTaskStatusPending,
				ConversationID: "old-conversation",
				Error:          "服务重启，任务已中断，请重新发起或继续",
				Result:         "old result",
				CompletedAt:    &completedAt,
			},
		},
	}

	task, ok := manager.ClaimNextTask(queueID)
	if !ok {
		t.Fatal("ClaimNextTask returned no task")
	}
	if task.ID != "task-1" {
		t.Fatalf("ClaimNextTask returned %q, want task-1", task.ID)
	}
	if task.ConversationID != "old-conversation" {
		t.Fatalf("ConversationID = %q, want old-conversation preserved until replacement", task.ConversationID)
	}
	if task.Error != "" {
		t.Fatalf("Error = %q, want empty", task.Error)
	}
	if task.Result != "" {
		t.Fatalf("Result = %q, want empty", task.Result)
	}
	if task.CompletedAt != nil {
		t.Fatalf("CompletedAt = %#v, want nil", task.CompletedAt)
	}
}

func TestUpdateTaskStatusClearsStaleErrorForCancelledAndCompleted(t *testing.T) {
	manager := NewBatchTaskManager(zap.NewNop())
	queueID := "queue-clear-stale-error"
	manager.queues[queueID] = &BatchTaskQueue{
		ID:        queueID,
		Status:    BatchQueueStatusRunning,
		CreatedAt: time.Now(),
		Tasks: []*BatchTask{
			{
				ID:             "task-cancelled",
				Status:         BatchTaskStatusRunning,
				ConversationID: "old-cancelled",
				Error:          "服务重启，任务已中断，请重新发起或继续",
			},
			{
				ID:             "task-completed",
				Status:         BatchTaskStatusRunning,
				ConversationID: "old-completed",
				Error:          "服务重启，任务已中断，请重新发起或继续",
			},
		},
	}

	manager.UpdateTaskStatusWithConversationID(queueID, "task-cancelled", BatchTaskStatusCancelled, "任务已被用户取消，后续操作已停止。", "", "new-cancelled")
	manager.UpdateTaskStatusWithConversationID(queueID, "task-completed", BatchTaskStatusCompleted, "ok", "", "new-completed")

	cancelled := manager.queues[queueID].Tasks[0]
	if cancelled.Error != "" {
		t.Fatalf("cancelled error = %q, want empty", cancelled.Error)
	}
	if cancelled.Result != "任务已被用户取消，后续操作已停止。" {
		t.Fatalf("cancelled result = %q, want cancel result", cancelled.Result)
	}
	if cancelled.ConversationID != "new-cancelled" {
		t.Fatalf("cancelled conversation = %q, want new-cancelled", cancelled.ConversationID)
	}

	completed := manager.queues[queueID].Tasks[1]
	if completed.Error != "" {
		t.Fatalf("completed error = %q, want empty", completed.Error)
	}
	if completed.Result != "ok" {
		t.Fatalf("completed result = %q, want ok", completed.Result)
	}
	if completed.ConversationID != "new-completed" {
		t.Fatalf("completed conversation = %q, want new-completed", completed.ConversationID)
	}
}

func TestPauseQueueCancelsAllRunningTaskContexts(t *testing.T) {
	manager := NewBatchTaskManager(zap.NewNop())
	queueID := "queue-pause-concurrent"
	manager.queues[queueID] = &BatchTaskQueue{
		ID:        queueID,
		Status:    BatchQueueStatusRunning,
		CreatedAt: time.Now(),
		Tasks: []*BatchTask{
			{ID: "task-1", Status: BatchTaskStatusRunning},
			{ID: "task-2", Status: BatchTaskStatusRunning},
		},
	}

	ctx1, cancel1 := context.WithCancel(context.Background())
	ctx2, cancel2 := context.WithCancel(context.Background())
	manager.SetTaskCancel(queueID, "task-1", cancel1)
	manager.SetTaskCancel(queueID, "task-2", cancel2)

	if !manager.PauseQueue(queueID) {
		t.Fatal("PauseQueue returned false")
	}

	select {
	case <-ctx1.Done():
	default:
		t.Fatal("task-1 context was not cancelled")
	}
	select {
	case <-ctx2.Done():
	default:
		t.Fatal("task-2 context was not cancelled")
	}
	if len(manager.taskCancels) != 0 {
		t.Fatalf("taskCancels still has %d queue entries, want 0", len(manager.taskCancels))
	}
}

func TestUpdateQueueConcurrencyNormalizesValue(t *testing.T) {
	manager := NewBatchTaskManager(zap.NewNop())
	queueID := "queue-concurrency-update"
	manager.queues[queueID] = &BatchTaskQueue{
		ID:          queueID,
		Status:      BatchQueueStatusPending,
		Concurrency: 1,
		CreatedAt:   time.Now(),
	}

	if err := manager.UpdateQueueConcurrency(queueID, 99); err != nil {
		t.Fatalf("UpdateQueueConcurrency: %v", err)
	}
	if got := manager.queues[queueID].Concurrency; got != MaxBatchQueueConcurrency {
		t.Fatalf("Concurrency = %d, want %d", got, MaxBatchQueueConcurrency)
	}

	if err := manager.UpdateQueueConcurrency(queueID, 0); err != nil {
		t.Fatalf("UpdateQueueConcurrency zero: %v", err)
	}
	if got := manager.queues[queueID].Concurrency; got != 1 {
		t.Fatalf("Concurrency after zero = %d, want 1", got)
	}
}

func TestUpdateQueueConcurrencyRejectsRunningQueue(t *testing.T) {
	manager := NewBatchTaskManager(zap.NewNop())
	queueID := "queue-concurrency-running"
	manager.queues[queueID] = &BatchTaskQueue{
		ID:          queueID,
		Status:      BatchQueueStatusRunning,
		Concurrency: 1,
		CreatedAt:   time.Now(),
	}

	if err := manager.UpdateQueueConcurrency(queueID, 2); err == nil {
		t.Fatal("UpdateQueueConcurrency returned nil for running queue")
	}
	if got := manager.queues[queueID].Concurrency; got != 1 {
		t.Fatalf("Concurrency changed to %d, want 1", got)
	}
}
