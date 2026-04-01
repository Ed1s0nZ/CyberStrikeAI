package handler

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrTaskCancelled cancel taskerror
var ErrTaskCancelled = errors.New("agent task cancelled by user")

// ErrTaskAlreadyRunning session already has a running task
var ErrTaskAlreadyRunning = errors.New("agent task already running for conversation")

// AgentTask describes a running Agent task
type AgentTask struct {
	ConversationID string    `json:"conversationId"`
	Message        string    `json:"message,omitempty"`
	StartedAt      time.Time `json:"startedAt"`
	Status         string    `json:"status"`
	CancellingAt time.Time `json:"-"` // cancelling status，for cleaning up long-stuck tasks

	cancel func(error)
}

// CompletedTask （record）
type CompletedTask struct {
	ConversationID string    `json:"conversationId"`
	Message        string    `json:"message,omitempty"`
	StartedAt      time.Time `json:"startedAt"`
	CompletedAt    time.Time `json:"completedAt"`
	Status         string    `json:"status"`
}

// AgentTaskManager manages running Agent tasks
type AgentTaskManager struct {
	mu             sync.RWMutex
	tasks          map[string]*AgentTask
	completedTasks []*CompletedTask // recently completed task history
	maxHistorySize int // record
	historyRetention time.Duration // record
}

const (
	// cancellingStuckThreshold 「」list。currentreturns，
	// exceed means stuck，release session ASAP。common practice is release within 30-60s。
	cancellingStuckThreshold = 45 * time.Second
	// cancellingStuckThresholdLegacy record CancellingAt StartedAt 
	cancellingStuckThresholdLegacy = 2 * time.Minute
	cleanupInterval = 15 * time.Second // paired with above threshold， 60s 
)

// NewAgentTaskManager create task manager
func NewAgentTaskManager() *AgentTaskManager {
	m := &AgentTaskManager{
		tasks:            make(map[string]*AgentTask),
		completedTasks:   make([]*CompletedTask, 0),
		maxHistorySize: 50, // 50record
		historyRetention: 24 * time.Hour,  // retain for 24 hours
	}
	go m.runStuckCancellingCleanup()
	return m
}

// runStuckCancellingCleanup periodically force-end tasks stuck in cancelling state，message
func (m *AgentTaskManager) runStuckCancellingCleanup() {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()
	for range ticker.C {
		m.cleanupStuckCancelling()
	}
}

func (m *AgentTaskManager) cleanupStuckCancelling() {
	m.mu.Lock()
	var toFinish []string
	now := time.Now()
	for id, task := range m.tasks {
		if task.Status != "cancelling" {
			continue
		}
		var elapsed time.Duration
		if !task.CancellingAt.IsZero() {
			elapsed = now.Sub(task.CancellingAt)
			if elapsed < cancellingStuckThreshold {
				continue
			}
		} else {
			elapsed = now.Sub(task.StartedAt)
			if elapsed < cancellingStuckThresholdLegacy {
				continue
			}
		}
		toFinish = append(toFinish, id)
	}
	m.mu.Unlock()
	for _, id := range toFinish {
		m.FinishTask(id, "cancelled")
	}
}

// StartTask register and start a new task
func (m *AgentTaskManager) StartTask(conversationID, message string, cancel context.CancelCauseFunc) (*AgentTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.tasks[conversationID]; exists {
		return nil, ErrTaskAlreadyRunning
	}

	task := &AgentTask{
		ConversationID: conversationID,
		Message:        message,
		StartedAt:      time.Now(),
		Status:         "running",
		cancel: func(err error) {
			if cancel != nil {
				cancel(err)
			}
		},
	}

	m.tasks[conversationID] = task
	return task, nil
}

// CancelTask cancel task for specified session。，returns (true, nil) for interface idempotency, frontend no error。
func (m *AgentTaskManager) CancelTask(conversationID string, cause error) (bool, error) {
	m.mu.Lock()
	task, exists := m.tasks[conversationID]
	if !exists {
		m.mu.Unlock()
		return false, nil
	}

	// if already in cancellation flow，treat as success (idempotent)，avoid frontend showing task not found on repeat clicks
	if task.Status == "cancelling" {
		m.mu.Unlock()
		return true, nil
	}

	task.Status = "cancelling"
	task.CancellingAt = time.Now()
	cancel := task.cancel
	m.mu.Unlock()

	if cause == nil {
		cause = ErrTaskCancelled
	}
	if cancel != nil {
		cancel(cause)
	}
	return true, nil
}

// UpdateTaskStatus statusdelete（status）
func (m *AgentTaskManager) UpdateTaskStatus(conversationID string, status string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[conversationID]
	if !exists {
		return
	}

	if status != "" {
		task.Status = status
	}
}

// FinishTask complete task and remove from manager
func (m *AgentTaskManager) FinishTask(conversationID string, finalStatus string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[conversationID]
	if !exists {
		return
	}

	if finalStatus != "" {
		task.Status = finalStatus
	}

	// record
	completedTask := &CompletedTask{
		ConversationID: task.ConversationID,
		Message:        task.Message,
		StartedAt:       task.StartedAt,
		CompletedAt:     time.Now(),
		Status:          finalStatus,
	}
	
	// addrecord
	m.completedTasks = append(m.completedTasks, completedTask)
	
	// record
	m.cleanupHistory()

	// remove from running tasks
	delete(m.tasks, conversationID)
}

// cleanupHistory record
func (m *AgentTaskManager) cleanupHistory() {
	now := time.Now()
	cutoffTime := now.Add(-m.historyRetention)
	
	// record
	validTasks := make([]*CompletedTask, 0, len(m.completedTasks))
	for _, task := range m.completedTasks {
		if task.CompletedAt.After(cutoffTime) {
			validTasks = append(validTasks, task)
		}
	}
	
	// if still exceeds max count，keep only newest
	if len(validTasks) > m.maxHistorySize {
		// sort by completion time，
		// since appended, newest at end，take last N directly
		start := len(validTasks) - m.maxHistorySize
		validTasks = validTasks[start:]
	}
	
	m.completedTasks = validTasks
}

// GetActiveTasks returns
func (m *AgentTaskManager) GetActiveTasks() []*AgentTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*AgentTask, 0, len(m.tasks))
	for _, task := range m.tasks {
		result = append(result, &AgentTask{
			ConversationID: task.ConversationID,
			Message:        task.Message,
			StartedAt:      task.StartedAt,
			Status:         task.Status,
		})
	}
	return result
}

// GetCompletedTasks returnsrecently completed task history
func (m *AgentTaskManager) GetCompletedTasks() []*CompletedTask {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	// record（，）
	// ：cannot directly call cleanupHistory here，because write lock needed
	// returnsrecord
	now := time.Now()
	cutoffTime := now.Add(-m.historyRetention)
	
	result := make([]*CompletedTask, 0, len(m.completedTasks))
	for _, task := range m.completedTasks {
		if task.CompletedAt.After(cutoffTime) {
			result = append(result, task)
		}
	}
	
	// sort by completion time descending（newest first）
	// since appended, newest at end，need to reverse
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	
	// returns
	if len(result) > m.maxHistorySize {
		result = result[:m.maxHistorySize]
	}
	
	return result
}
