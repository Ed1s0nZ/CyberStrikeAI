package handler

import (
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
