package handler

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrTaskCancelled 用户取消任务的错误
var ErrTaskCancelled = errors.New("agent task cancelled by user")

// ErrTaskAlreadyRunning 会话已有任务正在执行
var ErrTaskAlreadyRunning = errors.New("agent task already running for conversation")

// AgentTask 描述正在运行的Agent任务
type AgentTask struct {
	ConversationID string    `json:"conversationId"`
	Message        string    `json:"message,omitempty"`
	StartedAt      time.Time `json:"startedAt"`
	Status         string    `json:"status"`

	cancel func(error)
}

// AgentTaskManager 管理正在运行的Agent任务
type AgentTaskManager struct {
	mu    sync.RWMutex
	tasks map[string]*AgentTask
}

// NewAgentTaskManager 创建任务管理器
func NewAgentTaskManager() *AgentTaskManager {
	return &AgentTaskManager{
		tasks: make(map[string]*AgentTask),
	}
}

// StartTask 注册并开始一个新的任务
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

// CancelTask 取消指定会话的任务
func (m *AgentTaskManager) CancelTask(conversationID string, cause error) (bool, error) {
	m.mu.Lock()
	task, exists := m.tasks[conversationID]
	if !exists {
		m.mu.Unlock()
		return false, nil
	}

	// 如果已经处于取消流程，直接返回
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

// FinishTask 完成任务并从管理器中移除
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

	delete(m.tasks, conversationID)
}

// GetActiveTasks 返回所有正在运行的任务
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
