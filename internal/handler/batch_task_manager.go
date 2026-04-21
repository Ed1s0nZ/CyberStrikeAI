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
	"unicode/utf8"

	"cyberstrike-ai/internal/database"

	"go.uber.org/zap"
)

// 批量任务状态常量
const (
	BatchQueueStatusPending   = "pending"
	BatchQueueStatusRunning   = "running"
	BatchQueueStatusPaused    = "paused"
	BatchQueueStatusCompleted = "completed"
	BatchQueueStatusCancelled = "cancelled"

	BatchTaskStatusPending   = "pending"
	BatchTaskStatusRunning   = "running"
	BatchTaskStatusCompleted = "completed"
	BatchTaskStatusFailed    = "failed"
	BatchTaskStatusCancelled = "cancelled"

	// MaxBatchTasksPerQueue 单个队列最大任务数
	MaxBatchTasksPerQueue = 10000

	// MaxBatchQueueTitleLen 队列标题最大长度
	MaxBatchQueueTitleLen = 200

	// MaxBatchQueueRoleLen 角色名最大长度
	MaxBatchQueueRoleLen = 100
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
	ID                    string       `json:"id"`
	Title                 string       `json:"title,omitempty"`
	Role                  string       `json:"role,omitempty"` // 角色名称（空字符串表示默认角色）
	AgentMode             string       `json:"agentMode"`      // single | eino_single | deep | plan_execute | supervisor
	ScheduleMode          string       `json:"scheduleMode"`   // manual | cron
	CronExpr              string       `json:"cronExpr,omitempty"`
	NextRunAt             *time.Time   `json:"nextRunAt,omitempty"`
	ScheduleEnabled       bool         `json:"scheduleEnabled"`
	LastScheduleTriggerAt *time.Time   `json:"lastScheduleTriggerAt,omitempty"`
	LastScheduleError     string       `json:"lastScheduleError,omitempty"`
	LastRunError          string       `json:"lastRunError,omitempty"`
	Tasks                 []*BatchTask `json:"tasks"`
	Status                string       `json:"status"` // pending, running, paused, completed, cancelled
	CreatedAt             time.Time    `json:"createdAt"`
	StartedAt             *time.Time   `json:"startedAt,omitempty"`
	CompletedAt           *time.Time   `json:"completedAt,omitempty"`
	CurrentIndex          int          `json:"currentIndex"`
}

// BatchTaskManager 批量任务管理器
type BatchTaskManager struct {
	db          *database.DB
	logger      *zap.Logger
	queues      map[string]*BatchTaskQueue
	taskCancels map[string]context.CancelFunc // 存储每个队列当前任务的取消函数
	mu          sync.RWMutex
}

// NewBatchTaskManager 创建批量任务管理器
func NewBatchTaskManager(logger *zap.Logger) *BatchTaskManager {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &BatchTaskManager{
		logger:      logger,
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
func (m *BatchTaskManager) CreateBatchQueue(
	title, role, agentMode, scheduleMode, cronExpr string,
	nextRunAt *time.Time,
	tasks []string,
) (*BatchTaskQueue, error) {
	// 输入校验
	if utf8.RuneCountInString(title) > MaxBatchQueueTitleLen {
		return nil, fmt.Errorf("标题不能超过 %d 个字符", MaxBatchQueueTitleLen)
	}
	if utf8.RuneCountInString(role) > MaxBatchQueueRoleLen {
		return nil, fmt.Errorf("角色名不能超过 %d 个字符", MaxBatchQueueRoleLen)
	}
	if len(tasks) > MaxBatchTasksPerQueue {
		return nil, fmt.Errorf("单个队列最多 %d 条任务", MaxBatchTasksPerQueue)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	queueID := time.Now().Format("20060102150405") + "-" + generateShortID()
	queue := &BatchTaskQueue{
		ID:              queueID,
		Title:           title,
		Role:            role,
		AgentMode:       normalizeBatchQueueAgentMode(agentMode),
		ScheduleMode:    normalizeBatchQueueScheduleMode(scheduleMode),
		CronExpr:        strings.TrimSpace(cronExpr),
		NextRunAt:       nextRunAt,
		ScheduleEnabled: true,
		Tasks:           make([]*BatchTask, 0, len(tasks)),
		Status:          BatchQueueStatusPending,
		CreatedAt:       time.Now(),
		CurrentIndex:    0,
	}
	if queue.ScheduleMode != "cron" {
		queue.CronExpr = ""
		queue.NextRunAt = nil
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
			Status:  BatchTaskStatusPending,
		}
		queue.Tasks = append(queue.Tasks, task)
		dbTasks = append(dbTasks, map[string]interface{}{
			"id":      taskID,
			"message": message,
		})
	}

	// 保存到数据库
	if m.db != nil {
		if err := m.db.CreateBatchQueue(
			queueID,
			title,
			role,
			queue.AgentMode,
			queue.ScheduleMode,
			queue.CronExpr,
			queue.NextRunAt,
			dbTasks,
		); err != nil {
			m.logger.Warn("batch queue DB create failed", zap.String("queueId", queueID), zap.Error(err))
		}
	}

	m.queues[queueID] = queue
	return queue, nil
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
		AgentMode:    "single",
		ScheduleMode: "manual",
		Status:       queueRow.Status,
		CreatedAt:    queueRow.CreatedAt,
		CurrentIndex: queueRow.CurrentIndex,
		Tasks:        make([]*BatchTask, 0, len(taskRows)),
	}

	if queueRow.Title.Valid {
		queue.Title = queueRow.Title.String
	}
	if queueRow.Role.Valid {
		queue.Role = queueRow.Role.String
	}
	if queueRow.AgentMode.Valid {
		queue.AgentMode = normalizeBatchQueueAgentMode(queueRow.AgentMode.String)
	}
	if queueRow.ScheduleMode.Valid {
		queue.ScheduleMode = normalizeBatchQueueScheduleMode(queueRow.ScheduleMode.String)
	}
	if queueRow.CronExpr.Valid && queue.ScheduleMode == "cron" {
		queue.CronExpr = strings.TrimSpace(queueRow.CronExpr.String)
	}
	if queueRow.NextRunAt.Valid && queue.ScheduleMode == "cron" {
		t := queueRow.NextRunAt.Time
		queue.NextRunAt = &t
	}
	queue.ScheduleEnabled = true
	if queueRow.ScheduleEnabled.Valid && queueRow.ScheduleEnabled.Int64 == 0 {
		queue.ScheduleEnabled = false
	}
	if queueRow.LastScheduleTriggerAt.Valid {
		t := queueRow.LastScheduleTriggerAt.Time
		queue.LastScheduleTriggerAt = &t
	}
	if queueRow.LastScheduleError.Valid {
		queue.LastScheduleError = strings.TrimSpace(queueRow.LastScheduleError.String)
	}
	if queueRow.LastRunError.Valid {
		queue.LastRunError = strings.TrimSpace(queueRow.LastRunError.String)
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

// GetLoadedQueues 获取内存中已加载的队列（不触发 DB 加载，仅用 RLock）
func (m *BatchTaskManager) GetLoadedQueues() []*BatchTaskQueue {
	m.mu.RLock()
	result := make([]*BatchTaskQueue, 0, len(m.queues))
	for _, queue := range m.queues {
		result = append(result, queue)
	}
	m.mu.RUnlock()
	return result
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
			// 关键字搜索（搜索队列ID和标题）
			if keyword != "" {
				keywordLower := strings.ToLower(keyword)
				queueIDLower := strings.ToLower(queue.ID)
				queueTitleLower := strings.ToLower(queue.Title)
				if !strings.Contains(queueIDLower, keywordLower) && !strings.Contains(queueTitleLower, keywordLower) {
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
			AgentMode:    "single",
			ScheduleMode: "manual",
			Status:       queueRow.Status,
			CreatedAt:    queueRow.CreatedAt,
			CurrentIndex: queueRow.CurrentIndex,
			Tasks:        make([]*BatchTask, 0, len(taskRows)),
		}

		if queueRow.Title.Valid {
			queue.Title = queueRow.Title.String
		}
		if queueRow.Role.Valid {
			queue.Role = queueRow.Role.String
		}
		if queueRow.AgentMode.Valid {
			queue.AgentMode = normalizeBatchQueueAgentMode(queueRow.AgentMode.String)
		}
		if queueRow.ScheduleMode.Valid {
			queue.ScheduleMode = normalizeBatchQueueScheduleMode(queueRow.ScheduleMode.String)
		}
		if queueRow.CronExpr.Valid && queue.ScheduleMode == "cron" {
			queue.CronExpr = strings.TrimSpace(queueRow.CronExpr.String)
		}
		if queueRow.NextRunAt.Valid && queue.ScheduleMode == "cron" {
			t := queueRow.NextRunAt.Time
			queue.NextRunAt = &t
		}
		queue.ScheduleEnabled = true
		if queueRow.ScheduleEnabled.Valid && queueRow.ScheduleEnabled.Int64 == 0 {
			queue.ScheduleEnabled = false
		}
		if queueRow.LastScheduleTriggerAt.Valid {
			t := queueRow.LastScheduleTriggerAt.Time
			queue.LastScheduleTriggerAt = &t
		}
		if queueRow.LastScheduleError.Valid {
			queue.LastScheduleError = strings.TrimSpace(queueRow.LastScheduleError.String)
		}
		if queueRow.LastRunError.Valid {
			queue.LastRunError = strings.TrimSpace(queueRow.LastRunError.String)
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

	// DB 优先：先持久化，成功后再更新内存，避免重启后状态不一致
	if m.db != nil {
		if err := m.db.UpdateBatchTaskStatus(queueID, taskID, status, conversationID, result, errorMsg); err != nil {
			m.logger.Warn("batch task DB status update failed, skipping memory update",
				zap.String("queueId", queueID), zap.String("taskId", taskID), zap.Error(err))
			return
		}
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
			if status == BatchTaskStatusRunning && task.StartedAt == nil {
				task.StartedAt = &now
			}
			if status == BatchTaskStatusCompleted || status == BatchTaskStatusFailed || status == BatchTaskStatusCancelled {
				task.CompletedAt = &now
			}
			break
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

	// DB 优先：先持久化，成功后再更新内存
	if m.db != nil {
		if err := m.db.UpdateBatchQueueStatus(queueID, status); err != nil {
			m.logger.Warn("batch queue DB status update failed, skipping memory update",
				zap.String("queueId", queueID), zap.Error(err))
			return
		}
	}

	queue.Status = status
	now := time.Now()
	if status == BatchQueueStatusRunning && queue.StartedAt == nil {
		queue.StartedAt = &now
	}
	if status == BatchQueueStatusCompleted || status == BatchQueueStatusCancelled {
		queue.CompletedAt = &now
	}
}

// UpdateQueueSchedule 更新队列调度配置
func (m *BatchTaskManager) UpdateQueueSchedule(queueID, scheduleMode, cronExpr string, nextRunAt *time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()

	queue, exists := m.queues[queueID]
	if !exists {
		return
	}

	queue.ScheduleMode = normalizeBatchQueueScheduleMode(scheduleMode)
	if queue.ScheduleMode == "cron" {
		queue.CronExpr = strings.TrimSpace(cronExpr)
		queue.NextRunAt = nextRunAt
	} else {
		queue.CronExpr = ""
		queue.NextRunAt = nil
	}

	if m.db != nil {
		if err := m.db.UpdateBatchQueueSchedule(queueID, queue.ScheduleMode, queue.CronExpr, queue.NextRunAt); err != nil {
			m.logger.Warn("batch queue DB schedule update failed", zap.String("queueId", queueID), zap.Error(err))
		}
	}
}

// UpdateQueueMetadata 更新队列标题、角色和代理模式（非 running 时可用）
func (m *BatchTaskManager) UpdateQueueMetadata(queueID, title, role, agentMode string) error {
	if utf8.RuneCountInString(title) > MaxBatchQueueTitleLen {
		return fmt.Errorf("标题不能超过 %d 个字符", MaxBatchQueueTitleLen)
	}
	if utf8.RuneCountInString(role) > MaxBatchQueueRoleLen {
		return fmt.Errorf("角色名不能超过 %d 个字符", MaxBatchQueueRoleLen)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	queue, exists := m.queues[queueID]
	if !exists {
		return fmt.Errorf("队列不存在")
	}
	if queue.Status == BatchQueueStatusRunning {
		return fmt.Errorf("队列正在运行中，无法修改")
	}

	// 如果未传 agentMode，保留原值
	if strings.TrimSpace(agentMode) != "" {
		agentMode = normalizeBatchQueueAgentMode(agentMode)
	} else {
		agentMode = queue.AgentMode
	}

	queue.Title = title
	queue.Role = role
	queue.AgentMode = agentMode

	if m.db != nil {
		if err := m.db.UpdateBatchQueueMetadata(queueID, title, role, agentMode); err != nil {
			m.logger.Warn("batch queue DB metadata update failed", zap.String("queueId", queueID), zap.Error(err))
		}
	}
	return nil
}

// SetScheduleEnabled 暂停/恢复 Cron 自动调度（不影响手工执行）
func (m *BatchTaskManager) SetScheduleEnabled(queueID string, enabled bool) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	queue, exists := m.queues[queueID]
	if !exists {
		return false
	}
	queue.ScheduleEnabled = enabled
	if m.db != nil {
		_ = m.db.UpdateBatchQueueScheduleEnabled(queueID, enabled)
	}
	return true
}

// RecordScheduledRunStart Cron 触发成功、即将执行子任务时调用
func (m *BatchTaskManager) RecordScheduledRunStart(queueID string) {
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()

	queue, exists := m.queues[queueID]
	if !exists {
		return
	}
	queue.LastScheduleTriggerAt = &now
	queue.LastScheduleError = ""
	if m.db != nil {
		_ = m.db.RecordBatchQueueScheduledTriggerStart(queueID, now)
	}
}

// SetLastScheduleError 调度层失败（未成功开始执行）
func (m *BatchTaskManager) SetLastScheduleError(queueID, msg string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	queue, exists := m.queues[queueID]
	if !exists {
		return
	}
	queue.LastScheduleError = strings.TrimSpace(msg)
	if m.db != nil {
		_ = m.db.SetBatchQueueLastScheduleError(queueID, queue.LastScheduleError)
	}
}

// SetLastRunError 最近一轮批量执行中的失败摘要
func (m *BatchTaskManager) SetLastRunError(queueID, msg string) {
	msg = strings.TrimSpace(msg)
	m.mu.Lock()
	defer m.mu.Unlock()

	queue, exists := m.queues[queueID]
	if !exists {
		return
	}
	queue.LastRunError = msg
	if m.db != nil {
		_ = m.db.SetBatchQueueLastRunError(queueID, msg)
	}
}

// ResetQueueForRerun 重置队列与子任务状态，供 cron 下一轮执行
func (m *BatchTaskManager) ResetQueueForRerun(queueID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	queue, exists := m.queues[queueID]
	if !exists {
		return false
	}

	// DB 优先：先持久化重置，成功后再更新内存，避免 DB 失败导致内存脏状态
	if m.db != nil {
		if err := m.db.ResetBatchQueueForRerun(queueID); err != nil {
			m.logger.Warn("batch queue DB reset for rerun failed, skipping memory update",
				zap.String("queueId", queueID), zap.Error(err))
			return false
		}
	}

	queue.Status = BatchQueueStatusPending
	queue.CurrentIndex = 0
	queue.StartedAt = nil
	queue.CompletedAt = nil
	queue.NextRunAt = nil
	queue.LastRunError = ""
	queue.LastScheduleError = ""
	for _, task := range queue.Tasks {
		task.Status = BatchTaskStatusPending
		task.ConversationID = ""
		task.StartedAt = nil
		task.CompletedAt = nil
		task.Error = ""
		task.Result = ""
	}
	return true
}

// UpdateTaskMessage 更新任务消息（队列空闲时可改；任务需非 running）
func (m *BatchTaskManager) UpdateTaskMessage(queueID, taskID, message string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	queue, exists := m.queues[queueID]
	if !exists {
		return fmt.Errorf("队列不存在")
	}

	if !queueAllowsTaskListMutationLocked(queue) {
		return fmt.Errorf("队列正在执行或未就绪，无法编辑任务")
	}

	// 查找并更新任务
	for _, task := range queue.Tasks {
		if task.ID == taskID {
			if task.Status == BatchTaskStatusRunning {
				return fmt.Errorf("执行中的任务不能编辑")
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

// AddTaskToQueue 添加任务到队列（队列空闲时可添加：含 cron 本轮 completed、手动暂停后等）
func (m *BatchTaskManager) AddTaskToQueue(queueID, message string) (*BatchTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	queue, exists := m.queues[queueID]
	if !exists {
		return nil, fmt.Errorf("队列不存在")
	}

	if !queueAllowsTaskListMutationLocked(queue) {
		return nil, fmt.Errorf("队列正在执行或未就绪，无法添加任务")
	}

	if message == "" {
		return nil, fmt.Errorf("任务消息不能为空")
	}

	// 生成任务ID
	taskID := generateShortID()
	task := &BatchTask{
		ID:      taskID,
		Message: message,
		Status:  BatchTaskStatusPending,
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

// DeleteTask 删除任务（队列空闲时可删；执行中任务不可删）
func (m *BatchTaskManager) DeleteTask(queueID, taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	queue, exists := m.queues[queueID]
	if !exists {
		return fmt.Errorf("队列不存在")
	}

	if !queueAllowsTaskListMutationLocked(queue) {
		return fmt.Errorf("队列正在执行或未就绪，无法删除任务")
	}

	// 查找任务
	taskIndex := -1
	for i, task := range queue.Tasks {
		if task.ID == taskID {
			if task.Status == BatchTaskStatusRunning {
				return fmt.Errorf("执行中的任务不能删除")
			}
			taskIndex = i
			break
		}
	}

	if taskIndex == -1 {
		return fmt.Errorf("任务不存在")
	}

	// DB 优先：先从数据库删除，成功后再从内存移除
	if m.db != nil {
		if err := m.db.DeleteBatchTask(queueID, taskID); err != nil {
			return fmt.Errorf("删除任务失败: %w", err)
		}
	}

	queue.Tasks = append(queue.Tasks[:taskIndex], queue.Tasks[taskIndex+1:]...)
	return nil
}

func queueHasRunningTaskLocked(queue *BatchTaskQueue) bool {
	if queue == nil {
		return false
	}
	for _, t := range queue.Tasks {
		if t != nil && t.Status == BatchTaskStatusRunning {
			return true
		}
	}
	return false
}

// queueAllowsTaskListMutationLocked 是否允许增删改子任务文案/列表（必须在持有 BatchTaskManager.mu 下调用）
func queueAllowsTaskListMutationLocked(queue *BatchTaskQueue) bool {
	if queue == nil {
		return false
	}
	if queue.Status == BatchQueueStatusRunning {
		return false
	}
	if queueHasRunningTaskLocked(queue) {
		return false
	}
	switch queue.Status {
	case BatchQueueStatusPending, BatchQueueStatusPaused, BatchQueueStatusCompleted, BatchQueueStatusCancelled:
		return true
	default:
		return false
	}
}

// GetNextTask 获取下一个待执行的任务
func (m *BatchTaskManager) GetNextTask(queueID string) (*BatchTask, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	queue, exists := m.queues[queueID]
	if !exists {
		return nil, false
	}

	for i := queue.CurrentIndex; i < len(queue.Tasks); i++ {
		task := queue.Tasks[i]
		if task.Status == BatchTaskStatusPending {
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
			m.logger.Warn("batch queue DB index update failed", zap.String("queueId", queueID), zap.Error(err))
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
	var cancelFunc context.CancelFunc

	m.mu.Lock()
	queue, exists := m.queues[queueID]
	if !exists {
		m.mu.Unlock()
		return false
	}

	if queue.Status != BatchQueueStatusRunning {
		m.mu.Unlock()
		return false
	}

	// DB 优先：先持久化，成功后再更新内存
	if m.db != nil {
		if err := m.db.UpdateBatchQueueStatus(queueID, BatchQueueStatusPaused); err != nil {
			m.logger.Warn("batch queue DB pause update failed, skipping memory update",
				zap.String("queueId", queueID), zap.Error(err))
			m.mu.Unlock()
			return false
		}
	}

	queue.Status = BatchQueueStatusPaused

	// 取消当前正在执行的任务（通过取消context）
	if cancel, ok := m.taskCancels[queueID]; ok {
		cancelFunc = cancel
		delete(m.taskCancels, queueID)
	}
	m.mu.Unlock()

	// 释放锁后执行取消回调（cancel 可能阻塞，不应持锁）
	if cancelFunc != nil {
		cancelFunc()
	}

	return true
}

// CancelQueue 取消队列（保留此方法以保持向后兼容，但建议使用PauseQueue）
func (m *BatchTaskManager) CancelQueue(queueID string) bool {
	now := time.Now()
	var cancelFunc context.CancelFunc

	m.mu.Lock()
	queue, exists := m.queues[queueID]
	if !exists {
		m.mu.Unlock()
		return false
	}

	if queue.Status == BatchQueueStatusCompleted || queue.Status == BatchQueueStatusCancelled {
		m.mu.Unlock()
		return false
	}

	// DB 优先：先持久化，成功后再更新内存
	if m.db != nil {
		if err := m.db.CancelPendingBatchTasks(queueID, now); err != nil {
			m.logger.Warn("batch task DB batch cancel failed, skipping memory update",
				zap.String("queueId", queueID), zap.Error(err))
			m.mu.Unlock()
			return false
		}
		if err := m.db.UpdateBatchQueueStatus(queueID, BatchQueueStatusCancelled); err != nil {
			m.logger.Warn("batch queue DB cancel update failed, skipping memory update",
				zap.String("queueId", queueID), zap.Error(err))
			m.mu.Unlock()
			return false
		}
	}

	queue.Status = BatchQueueStatusCancelled
	queue.CompletedAt = &now

	// 内存中批量标记所有 pending 任务为 cancelled
	for _, task := range queue.Tasks {
		if task.Status == BatchTaskStatusPending {
			task.Status = BatchTaskStatusCancelled
			task.CompletedAt = &now
		}
	}

	// 取消当前正在执行的任务
	if cancel, ok := m.taskCancels[queueID]; ok {
		cancelFunc = cancel
		delete(m.taskCancels, queueID)
	}
	m.mu.Unlock()

	// 释放锁后执行取消回调（cancel 可能阻塞，不应持锁）
	if cancelFunc != nil {
		cancelFunc()
	}

	return true
}

// DeleteQueue 删除队列（运行中的队列不允许删除）
func (m *BatchTaskManager) DeleteQueue(queueID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	queue, exists := m.queues[queueID]
	if !exists {
		return false
	}

	// 运行中的队列不允许删除，防止孤儿协程和数据丢失
	if queue.Status == BatchQueueStatusRunning {
		return false
	}

	// 清理取消函数
	delete(m.taskCancels, queueID)

	// 从数据库删除
	if m.db != nil {
		if err := m.db.DeleteBatchQueue(queueID); err != nil {
			m.logger.Warn("batch queue DB delete failed", zap.String("queueId", queueID), zap.Error(err))
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
