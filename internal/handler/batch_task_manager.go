package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"cyberstrike-ai/internal/database"
)

// BatchTask 批量任务项
type BatchTask struct {
	ID             string     `json:"id"`
	Message        string     `json:"message"`
	ConversationID string     `json:"conversationId,omitempty"`
	Status         string     `json:"status"` // pending, running, completed, failed, cancelled
	StartedAt      *time.Time `json:"startedAt,omitempty"`
	CompletedAt    *time.Time `json:"completedAt,omitempty"`
	Error          string     `json:"error,omitempty"`
	Result         string     `json:"result,omitempty"`
}

// BatchTaskQueue 批量任务队列
type BatchTaskQueue struct {
	ID           string       `json:"id"`
	Tasks        []*BatchTask `json:"tasks"`
	Status       string       `json:"status"` // pending, running, paused, completed, cancelled
	CreatedAt    time.Time    `json:"createdAt"`
	StartedAt    *time.Time   `json:"startedAt,omitempty"`
	CompletedAt  *time.Time   `json:"completedAt,omitempty"`
	CurrentIndex int          `json:"currentIndex"`
	mu           sync.RWMutex
}

// BatchTaskManager 批量任务管理器
type BatchTaskManager struct {
	db            *database.DB
	queues        map[string]*BatchTaskQueue
	taskCancels   map[string]context.CancelFunc // 存储每个队列当前任务的取消函数
	mu            sync.RWMutex
}

// NewBatchTaskManager 创建批量任务管理器
func NewBatchTaskManager() *BatchTaskManager {
	return &BatchTaskManager{
		queues:      make(map[string]*BatchTaskQueue),
		taskCancels: make(map[string]context.CancelFunc),
	}
}

// SetDB 设置数据库连接
func (m *BatchTaskManager) SetDB(db *database.DB) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.db = db
}

// CreateBatchQueue 创建批量任务队列
func (m *BatchTaskManager) CreateBatchQueue(tasks []string) *BatchTaskQueue {
	m.mu.Lock()
	defer m.mu.Unlock()

	queueID := time.Now().Format("20060102150405") + "-" + generateShortID()
	queue := &BatchTaskQueue{
		ID:           queueID,
		Tasks:        make([]*BatchTask, 0, len(tasks)),
		Status:       "pending",
		CreatedAt:    time.Now(),
		CurrentIndex: 0,
	}

	// 准备数据库保存的任务数据
	dbTasks := make([]map[string]interface{}, 0, len(tasks))

	for _, message := range tasks {
		if message == "" {
			continue // 跳过空行
		}
		taskID := generateShortID()
		task := &BatchTask{
			ID:      taskID,
			Message: message,
			Status:  "pending",
		}
		queue.Tasks = append(queue.Tasks, task)
		dbTasks = append(dbTasks, map[string]interface{}{
			"id":      taskID,
			"message": message,
		})
	}

	// 保存到数据库
	if m.db != nil {
		if err := m.db.CreateBatchQueue(queueID, dbTasks); err != nil {
			// 如果数据库保存失败，记录错误但继续（使用内存缓存）
			// 这里可以添加日志记录
		}
	}

	m.queues[queueID] = queue
	return queue
}

// GetBatchQueue 获取批量任务队列
func (m *BatchTaskManager) GetBatchQueue(queueID string) (*BatchTaskQueue, bool) {
	m.mu.RLock()
	queue, exists := m.queues[queueID]
	m.mu.RUnlock()

	if exists {
		return queue, true
	}

	// 如果内存中不存在，尝试从数据库加载
	if m.db != nil {
		if queue := m.loadQueueFromDB(queueID); queue != nil {
			m.mu.Lock()
			m.queues[queueID] = queue
			m.mu.Unlock()
			return queue, true
		}
	}

	return nil, false
}

// loadQueueFromDB 从数据库加载单个队列
func (m *BatchTaskManager) loadQueueFromDB(queueID string) *BatchTaskQueue {
	if m.db == nil {
		return nil
	}

	queueRow, err := m.db.GetBatchQueue(queueID)
	if err != nil || queueRow == nil {
		return nil
	}

	taskRows, err := m.db.GetBatchTasks(queueID)
	if err != nil {
		return nil
	}

	queue := &BatchTaskQueue{
		ID:           queueRow.ID,
		Status:       queueRow.Status,
		CreatedAt:    queueRow.CreatedAt,
		CurrentIndex: queueRow.CurrentIndex,
		Tasks:        make([]*BatchTask, 0, len(taskRows)),
	}

	if queueRow.StartedAt.Valid {
		queue.StartedAt = &queueRow.StartedAt.Time
	}
	if queueRow.CompletedAt.Valid {
		queue.CompletedAt = &queueRow.CompletedAt.Time
	}

	for _, taskRow := range taskRows {
		task := &BatchTask{
			ID:      taskRow.ID,
			Message: taskRow.Message,
			Status:  taskRow.Status,
		}
		if taskRow.ConversationID.Valid {
			task.ConversationID = taskRow.ConversationID.String
		}
		if taskRow.StartedAt.Valid {
			task.StartedAt = &taskRow.StartedAt.Time
		}
		if taskRow.CompletedAt.Valid {
			task.CompletedAt = &taskRow.CompletedAt.Time
		}
		if taskRow.Error.Valid {
			task.Error = taskRow.Error.String
		}
		if taskRow.Result.Valid {
			task.Result = taskRow.Result.String
		}
		queue.Tasks = append(queue.Tasks, task)
	}

	return queue
}

// GetAllQueues 获取所有队列
func (m *BatchTaskManager) GetAllQueues() []*BatchTaskQueue {
	m.mu.RLock()
	result := make([]*BatchTaskQueue, 0, len(m.queues))
	for _, queue := range m.queues {
		result = append(result, queue)
	}
	m.mu.RUnlock()

	// 如果数据库可用，确保所有数据库中的队列都已加载到内存
	if m.db != nil {
		dbQueues, err := m.db.GetAllBatchQueues()
		if err == nil {
			m.mu.Lock()
			for _, queueRow := range dbQueues {
				if _, exists := m.queues[queueRow.ID]; !exists {
					if queue := m.loadQueueFromDB(queueRow.ID); queue != nil {
						m.queues[queueRow.ID] = queue
						result = append(result, queue)
					}
				}
			}
			m.mu.Unlock()
		}
	}

	return result
}

// ListQueues 列出队列（支持筛选和分页）
func (m *BatchTaskManager) ListQueues(limit, offset int, status, keyword string) ([]*BatchTaskQueue, int, error) {
	var queues []*BatchTaskQueue
	var total int

	// 如果数据库可用，从数据库查询
	if m.db != nil {
		// 获取总数
		count, err := m.db.CountBatchQueues(status, keyword)
		if err != nil {
			return nil, 0, fmt.Errorf("统计队列总数失败: %w", err)
		}
		total = count

		// 获取队列列表（只获取ID）
		queueRows, err := m.db.ListBatchQueues(limit, offset, status, keyword)
		if err != nil {
			return nil, 0, fmt.Errorf("查询队列列表失败: %w", err)
		}

		// 加载完整的队列信息（从内存或数据库）
		m.mu.Lock()
		for _, queueRow := range queueRows {
			var queue *BatchTaskQueue
			// 先从内存查找
			if cached, exists := m.queues[queueRow.ID]; exists {
				queue = cached
			} else {
				// 从数据库加载
				queue = m.loadQueueFromDB(queueRow.ID)
				if queue != nil {
					m.queues[queueRow.ID] = queue
				}
			}
			if queue != nil {
				queues = append(queues, queue)
			}
		}
		m.mu.Unlock()
	} else {
		// 没有数据库，从内存中筛选和分页
		m.mu.RLock()
		allQueues := make([]*BatchTaskQueue, 0, len(m.queues))
		for _, queue := range m.queues {
			allQueues = append(allQueues, queue)
		}
		m.mu.RUnlock()

		// 筛选
		filtered := make([]*BatchTaskQueue, 0)
		for _, queue := range allQueues {
			// 状态筛选
			if status != "" && status != "all" && queue.Status != status {
				continue
			}
			// 关键字搜索
			if keyword != "" {
				keywordLower := strings.ToLower(keyword)
				queueIDLower := strings.ToLower(queue.ID)
				if !strings.Contains(queueIDLower, keywordLower) {
					// 也可以搜索创建时间
					createdAtStr := queue.CreatedAt.Format("2006-01-02 15:04:05")
					if !strings.Contains(createdAtStr, keyword) {
						continue
					}
				}
			}
			filtered = append(filtered, queue)
		}

		// 按创建时间倒序排序
		sort.Slice(filtered, func(i, j int) bool {
			return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
		})

		total = len(filtered)

		// 分页
		start := offset
		if start > len(filtered) {
			start = len(filtered)
		}
		end := start + limit
		if end > len(filtered) {
			end = len(filtered)
		}
		if start < len(filtered) {
			queues = filtered[start:end]
		}
	}

	return queues, total, nil
}

// LoadFromDB 从数据库加载所有队列
func (m *BatchTaskManager) LoadFromDB() error {
	if m.db == nil {
		return nil
	}

	queueRows, err := m.db.GetAllBatchQueues()
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, queueRow := range queueRows {
		if _, exists := m.queues[queueRow.ID]; exists {
			continue // 已存在，跳过
		}

		taskRows, err := m.db.GetBatchTasks(queueRow.ID)
		if err != nil {
			continue // 跳过加载失败的任务
		}

		queue := &BatchTaskQueue{
			ID:           queueRow.ID,
			Status:       queueRow.Status,
			CreatedAt:    queueRow.CreatedAt,
			CurrentIndex: queueRow.CurrentIndex,
			Tasks:        make([]*BatchTask, 0, len(taskRows)),
		}

		if queueRow.StartedAt.Valid {
			queue.StartedAt = &queueRow.StartedAt.Time
		}
		if queueRow.CompletedAt.Valid {
			queue.CompletedAt = &queueRow.CompletedAt.Time
		}

		for _, taskRow := range taskRows {
			task := &BatchTask{
				ID:      taskRow.ID,
				Message: taskRow.Message,
				Status:  taskRow.Status,
			}
			if taskRow.ConversationID.Valid {
				task.ConversationID = taskRow.ConversationID.String
			}
			if taskRow.StartedAt.Valid {
				task.StartedAt = &taskRow.StartedAt.Time
			}
			if taskRow.CompletedAt.Valid {
				task.CompletedAt = &taskRow.CompletedAt.Time
			}
			if taskRow.Error.Valid {
				task.Error = taskRow.Error.String
			}
			if taskRow.Result.Valid {
				task.Result = taskRow.Result.String
			}
			queue.Tasks = append(queue.Tasks, task)
		}

		m.queues[queueRow.ID] = queue
	}

	return nil
}

// UpdateTaskStatus 更新任务状态
func (m *BatchTaskManager) UpdateTaskStatus(queueID, taskID, status string, result, errorMsg string) {
	m.UpdateTaskStatusWithConversationID(queueID, taskID, status, result, errorMsg, "")
}

// UpdateTaskStatusWithConversationID 更新任务状态（包含conversationId）
func (m *BatchTaskManager) UpdateTaskStatusWithConversationID(queueID, taskID, status string, result, errorMsg, conversationID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	queue, exists := m.queues[queueID]
	if !exists {
		return
	}

	for _, task := range queue.Tasks {
		if task.ID == taskID {
			task.Status = status
			if result != "" {
				task.Result = result
			}
			if errorMsg != "" {
				task.Error = errorMsg
			}
			if conversationID != "" {
				task.ConversationID = conversationID
			}
			now := time.Now()
			if status == "running" && task.StartedAt == nil {
				task.StartedAt = &now
			}
			if status == "completed" || status == "failed" || status == "cancelled" {
				task.CompletedAt = &now
			}
			break
		}
	}

	// 同步到数据库
	if m.db != nil {
		if err := m.db.UpdateBatchTaskStatus(queueID, taskID, status, conversationID, result, errorMsg); err != nil {
			// 记录错误但继续（使用内存缓存）
		}
	}
}

// UpdateQueueStatus 更新队列状态
func (m *BatchTaskManager) UpdateQueueStatus(queueID, status string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	queue, exists := m.queues[queueID]
	if !exists {
		return
	}

	queue.Status = status
	now := time.Now()
	if status == "running" && queue.StartedAt == nil {
		queue.StartedAt = &now
	}
	if status == "completed" || status == "cancelled" {
		queue.CompletedAt = &now
	}

	// 同步到数据库
	if m.db != nil {
		if err := m.db.UpdateBatchQueueStatus(queueID, status); err != nil {
			// 记录错误但继续（使用内存缓存）
		}
	}
}

// UpdateTaskMessage 更新任务消息（仅限待执行状态）
func (m *BatchTaskManager) UpdateTaskMessage(queueID, taskID, message string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	queue, exists := m.queues[queueID]
	if !exists {
		return fmt.Errorf("队列不存在")
	}

	// 检查队列状态，只有待执行状态的队列才能编辑任务
	if queue.Status != "pending" {
		return fmt.Errorf("只有待执行状态的队列才能编辑任务")
	}

	// 查找并更新任务
	for _, task := range queue.Tasks {
		if task.ID == taskID {
			// 只有待执行状态的任务才能编辑
			if task.Status != "pending" {
				return fmt.Errorf("只有待执行状态的任务才能编辑")
			}
			task.Message = message

			// 同步到数据库
			if m.db != nil {
				if err := m.db.UpdateBatchTaskMessage(queueID, taskID, message); err != nil {
					return fmt.Errorf("更新任务消息失败: %w", err)
				}
			}
			return nil
		}
	}

	return fmt.Errorf("任务不存在")
}

// AddTaskToQueue 添加任务到队列（仅限待执行状态）
func (m *BatchTaskManager) AddTaskToQueue(queueID, message string) (*BatchTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	queue, exists := m.queues[queueID]
	if !exists {
		return nil, fmt.Errorf("队列不存在")
	}

	// 检查队列状态，只有待执行状态的队列才能添加任务
	if queue.Status != "pending" {
		return nil, fmt.Errorf("只有待执行状态的队列才能添加任务")
	}

	if message == "" {
		return nil, fmt.Errorf("任务消息不能为空")
	}

	// 生成任务ID
	taskID := generateShortID()
	task := &BatchTask{
		ID:      taskID,
		Message: message,
		Status:  "pending",
	}

	// 添加到内存队列
	queue.Tasks = append(queue.Tasks, task)

	// 同步到数据库
	if m.db != nil {
		if err := m.db.AddBatchTask(queueID, taskID, message); err != nil {
			// 如果数据库保存失败，从内存中移除
			queue.Tasks = queue.Tasks[:len(queue.Tasks)-1]
			return nil, fmt.Errorf("添加任务失败: %w", err)
		}
	}

	return task, nil
}

// DeleteTask 删除任务（仅限待执行状态）
func (m *BatchTaskManager) DeleteTask(queueID, taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	queue, exists := m.queues[queueID]
	if !exists {
		return fmt.Errorf("队列不存在")
	}

	// 检查队列状态，只有待执行状态的队列才能删除任务
	if queue.Status != "pending" {
		return fmt.Errorf("只有待执行状态的队列才能删除任务")
	}

	// 查找并删除任务
	taskIndex := -1
	for i, task := range queue.Tasks {
		if task.ID == taskID {
			// 只有待执行状态的任务才能删除
			if task.Status != "pending" {
				return fmt.Errorf("只有待执行状态的任务才能删除")
			}
			taskIndex = i
			break
		}
	}

	if taskIndex == -1 {
		return fmt.Errorf("任务不存在")
	}

	// 从内存队列中删除
	queue.Tasks = append(queue.Tasks[:taskIndex], queue.Tasks[taskIndex+1:]...)

	// 同步到数据库
	if m.db != nil {
		if err := m.db.DeleteBatchTask(queueID, taskID); err != nil {
			// 如果数据库删除失败，恢复内存中的任务
			// 这里需要重新插入，但为了简化，我们只记录错误
			return fmt.Errorf("删除任务失败: %w", err)
		}
	}

	return nil
}

// GetNextTask 获取下一个待执行的任务
func (m *BatchTaskManager) GetNextTask(queueID string) (*BatchTask, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	queue, exists := m.queues[queueID]
	if !exists {
		return nil, false
	}

	for i := queue.CurrentIndex; i < len(queue.Tasks); i++ {
		task := queue.Tasks[i]
		if task.Status == "pending" {
			queue.CurrentIndex = i
			return task, true
		}
	}

	return nil, false
}

// MoveToNextTask 移动到下一个任务
func (m *BatchTaskManager) MoveToNextTask(queueID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	queue, exists := m.queues[queueID]
	if !exists {
		return
	}

	queue.CurrentIndex++

	// 同步到数据库
	if m.db != nil {
		if err := m.db.UpdateBatchQueueCurrentIndex(queueID, queue.CurrentIndex); err != nil {
			// 记录错误但继续（使用内存缓存）
		}
	}
}

// SetTaskCancel 设置当前任务的取消函数
func (m *BatchTaskManager) SetTaskCancel(queueID string, cancel context.CancelFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cancel != nil {
		m.taskCancels[queueID] = cancel
	} else {
		delete(m.taskCancels, queueID)
	}
}

// PauseQueue 暂停队列
func (m *BatchTaskManager) PauseQueue(queueID string) bool {
	m.mu.Lock()

	queue, exists := m.queues[queueID]
	if !exists {
		m.mu.Unlock()
		return false
	}

	if queue.Status != "running" {
		m.mu.Unlock()
		return false
	}

	queue.Status = "paused"

	// 取消当前正在执行的任务（通过取消context）
	if cancel, exists := m.taskCancels[queueID]; exists {
		cancel()
		delete(m.taskCancels, queueID)
	}

	m.mu.Unlock()

	// 同步队列状态到数据库
	if m.db != nil {
		if err := m.db.UpdateBatchQueueStatus(queueID, "paused"); err != nil {
			// 记录错误但继续（使用内存缓存）
		}
	}

	return true
}

// CancelQueue 取消队列（保留此方法以保持向后兼容，但建议使用PauseQueue）
func (m *BatchTaskManager) CancelQueue(queueID string) bool {
	m.mu.Lock()

	queue, exists := m.queues[queueID]
	if !exists {
		m.mu.Unlock()
		return false
	}

	if queue.Status == "completed" || queue.Status == "cancelled" {
		m.mu.Unlock()
		return false
	}

	queue.Status = "cancelled"
	now := time.Now()
	queue.CompletedAt = &now

	// 取消所有待执行的任务
	for _, task := range queue.Tasks {
		if task.Status == "pending" {
			task.Status = "cancelled"
			task.CompletedAt = &now
			// 同步到数据库
			if m.db != nil {
				m.db.UpdateBatchTaskStatus(queueID, task.ID, "cancelled", "", "", "")
			}
		}
	}

	// 取消当前正在执行的任务
	if cancel, exists := m.taskCancels[queueID]; exists {
		cancel()
		delete(m.taskCancels, queueID)
	}

	m.mu.Unlock()

	// 同步队列状态到数据库
	if m.db != nil {
		if err := m.db.UpdateBatchQueueStatus(queueID, "cancelled"); err != nil {
			// 记录错误但继续（使用内存缓存）
		}
	}

	return true
}

// DeleteQueue 删除队列
func (m *BatchTaskManager) DeleteQueue(queueID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, exists := m.queues[queueID]
	if !exists {
		return false
	}

	// 清理取消函数
	delete(m.taskCancels, queueID)

	// 从数据库删除
	if m.db != nil {
		if err := m.db.DeleteBatchQueue(queueID); err != nil {
			// 记录错误但继续（使用内存缓存）
		}
	}

	delete(m.queues, queueID)
	return true
}

// generateShortID 生成短ID
func generateShortID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return time.Now().Format("150405") + "-" + hex.EncodeToString(b)
}
