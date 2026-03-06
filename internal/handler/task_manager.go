package handler

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrTaskCancelled is the error for user-cancelled tasks
var ErrTaskCancelled = errors.New("agent task cancelled by user")

// ErrTaskAlreadyRunning indicates a task is already running for the conversation
var ErrTaskAlreadyRunning = errors.New("agent task already running for conversation")

// AgentTask describes a running agent task
type AgentTask struct {
	ConversationID string    `json:"conversationId"`
	Message        string    `json:"message,omitempty"`
	StartedAt      time.Time `json:"startedAt"`
	Status         string    `json:"status"`

	cancel func(error)
}

// CompletedTask is a completed task (used for history)
type CompletedTask struct {
	ConversationID string    `json:"conversationId"`
	Message        string    `json:"message,omitempty"`
	StartedAt      time.Time `json:"startedAt"`
	CompletedAt    time.Time `json:"completedAt"`
	Status         string    `json:"status"`
}

// AgentTaskManager manages running agent tasks
type AgentTaskManager struct {
	mu               sync.RWMutex
	tasks            map[string]*AgentTask
	completedTasks   []*CompletedTask // recently completed task history
	maxHistorySize   int              // maximum history size
	historyRetention time.Duration    // history retention duration
}

// NewAgentTaskManager creates a task manager
func NewAgentTaskManager() *AgentTaskManager {
	return &AgentTaskManager{
		tasks:            make(map[string]*AgentTask),
		completedTasks:   make([]*CompletedTask, 0),
		maxHistorySize:   50,             // keep at most 50 history records
		historyRetention: 24 * time.Hour, // retain for 24 hours
	}
}

// StartTask registers and starts a new task
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

// CancelTask cancels the task for the given conversation
func (m *AgentTaskManager) CancelTask(conversationID string, cause error) (bool, error) {
	m.mu.Lock()
	task, exists := m.tasks[conversationID]
	if !exists {
		m.mu.Unlock()
		return false, nil
	}

	// if already in cancelling state, return directly
	if task.Status == "cancelling" {
		m.mu.Unlock()
		return false, nil
	}

	task.Status = "cancelling"
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

// UpdateTaskStatus updates task status without removing it (used to update status before sending events)
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

// FinishTask completes a task and removes it from the manager
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

	// save to history
	completedTask := &CompletedTask{
		ConversationID: task.ConversationID,
		Message:        task.Message,
		StartedAt:      task.StartedAt,
		CompletedAt:    time.Now(),
		Status:         finalStatus,
	}

	// add to history
	m.completedTasks = append(m.completedTasks, completedTask)

	// clean up expired and excess history
	m.cleanupHistory()

	// remove from running tasks
	delete(m.tasks, conversationID)
}

// cleanupHistory cleans up expired history records
func (m *AgentTaskManager) cleanupHistory() {
	now := time.Now()
	cutoffTime := now.Add(-m.historyRetention)

	// filter out expired records
	validTasks := make([]*CompletedTask, 0, len(m.completedTasks))
	for _, task := range m.completedTasks {
		if task.CompletedAt.After(cutoffTime) {
			validTasks = append(validTasks, task)
		}
	}

	// if still exceeds max size, keep only the most recent
	if len(validTasks) > m.maxHistorySize {
		// since items are appended, most recent are at the end, take the last N
		start := len(validTasks) - m.maxHistorySize
		validTasks = validTasks[start:]
	}

	m.completedTasks = validTasks
}

// GetActiveTasks returns all currently running tasks
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

// IsTaskRunning returns whether a task is currently running for the conversation.
func (m *AgentTaskManager) IsTaskRunning(conversationID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, exists := m.tasks[conversationID]
	return exists
}

// GetCompletedTasks returns recently completed task history
func (m *AgentTaskManager) GetCompletedTasks() []*CompletedTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// filter expired records (read lock, does not block other operations)
	// note: cannot call cleanupHistory here as it requires a write lock
	// so filter expired records on return
	now := time.Now()
	cutoffTime := now.Add(-m.historyRetention)

	result := make([]*CompletedTask, 0, len(m.completedTasks))
	for _, task := range m.completedTasks {
		if task.CompletedAt.After(cutoffTime) {
			result = append(result, task)
		}
	}

	// sort by completion time descending (most recent first)
	// since items are appended, most recent are at the end, need to reverse
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	// limit return count
	if len(result) > m.maxHistorySize {
		result = result[:m.maxHistorySize]
	}

	return result
}
