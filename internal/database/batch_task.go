package database

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
)

// BatchTaskQueueRow 批量任务队列数据库行
type BatchTaskQueueRow struct {
	ID                    string
	Title                 sql.NullString
	Role                  sql.NullString
	AgentMode             sql.NullString
	ScheduleMode          sql.NullString
	CronExpr              sql.NullString
	NextRunAt             sql.NullTime
	ScheduleEnabled       sql.NullInt64
	LastScheduleTriggerAt sql.NullTime
	LastScheduleError     sql.NullString
	LastRunError          sql.NullString
	ProjectID             sql.NullString
	Concurrency           sql.NullInt64
	Status                string
	CreatedAt             time.Time
	StartedAt             sql.NullTime
	CompletedAt           sql.NullTime
	CurrentIndex          int
}

// BatchTaskRow 批量任务数据库行
type BatchTaskRow struct {
	ID             string
	QueueID        string
	Message        string
	ConversationID sql.NullString
	Status         string
	StartedAt      sql.NullTime
	CompletedAt    sql.NullTime
	Error          sql.NullString
	Result         sql.NullString
}

// CreateBatchQueue 创建批量任务队列
func (db *DB) CreateBatchQueue(
	queueID string,
	title string,
	role string,
	agentMode string,
	scheduleMode string,
	cronExpr string,
	nextRunAt *time.Time,
	projectID string,
	concurrency int,
	tasks []map[string]interface{},
) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("开始事务失败: %w", err)
	}
	defer tx.Rollback()

	now := time.Now()
	var nextRunAtValue interface{}
	if nextRunAt != nil {
		nextRunAtValue = *nextRunAt
	}

	var projectIDVal interface{}
	if strings.TrimSpace(projectID) != "" {
		projectIDVal = strings.TrimSpace(projectID)
	}
	if concurrency < 1 {
		concurrency = 1
	}
	_, err = tx.Exec(
		"INSERT INTO batch_task_queues (id, title, role, agent_mode, schedule_mode, cron_expr, next_run_at, schedule_enabled, project_id, concurrency, status, created_at, current_index) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		queueID, title, role, agentMode, scheduleMode, cronExpr, nextRunAtValue, 1, projectIDVal, concurrency, "pending", now, 0,
	)
	if err != nil {
		return fmt.Errorf("创建批量任务队列失败: %w", err)
	}

	// 插入任务
	for _, task := range tasks {
		taskID, ok := task["id"].(string)
		if !ok {
			continue
		}
		message, ok := task["message"].(string)
		if !ok {
			continue
		}

		_, err = tx.Exec(
			"INSERT INTO batch_tasks (id, queue_id, message, status) VALUES (?, ?, ?, ?)",
			taskID, queueID, message, "pending",
		)
		if err != nil {
			return fmt.Errorf("创建批量任务失败: %w", err)
		}
	}

	return tx.Commit()
}

// GetBatchQueue 获取批量任务队列
func (db *DB) GetBatchQueue(queueID string) (*BatchTaskQueueRow, error) {
	var row BatchTaskQueueRow
	var createdAt string
	err := db.QueryRow(
		"SELECT id, title, role, agent_mode, schedule_mode, cron_expr, next_run_at, schedule_enabled, last_schedule_trigger_at, last_schedule_error, last_run_error, project_id, concurrency, status, created_at, started_at, completed_at, current_index FROM batch_task_queues WHERE id = ?",
		queueID,
	).Scan(&row.ID, &row.Title, &row.Role, &row.AgentMode, &row.ScheduleMode, &row.CronExpr, &row.NextRunAt, &row.ScheduleEnabled, &row.LastScheduleTriggerAt, &row.LastScheduleError, &row.LastRunError, &row.ProjectID, &row.Concurrency, &row.Status, &createdAt, &row.StartedAt, &row.CompletedAt, &row.CurrentIndex)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询批量任务队列失败: %w", err)
	}

	parsedTime, parseErr := time.Parse("2006-01-02 15:04:05", createdAt)
	if parseErr != nil {
		// 尝试其他时间格式
		parsedTime, parseErr = time.Parse(time.RFC3339, createdAt)
		if parseErr != nil {
			db.logger.Warn("解析创建时间失败", zap.String("createdAt", createdAt), zap.Error(parseErr))
			parsedTime = time.Now()
		}
	}
	row.CreatedAt = parsedTime
	return &row, nil
}

// GetAllBatchQueues 获取所有批量任务队列
func (db *DB) GetAllBatchQueues() ([]*BatchTaskQueueRow, error) {
	rows, err := db.Query(
		"SELECT id, title, role, agent_mode, schedule_mode, cron_expr, next_run_at, schedule_enabled, last_schedule_trigger_at, last_schedule_error, last_run_error, project_id, concurrency, status, created_at, started_at, completed_at, current_index FROM batch_task_queues ORDER BY created_at DESC",
	)
	if err != nil {
		return nil, fmt.Errorf("查询批量任务队列列表失败: %w", err)
	}
	defer rows.Close()

	var queues []*BatchTaskQueueRow
	for rows.Next() {
		var row BatchTaskQueueRow
		var createdAt string
		if err := rows.Scan(&row.ID, &row.Title, &row.Role, &row.AgentMode, &row.ScheduleMode, &row.CronExpr, &row.NextRunAt, &row.ScheduleEnabled, &row.LastScheduleTriggerAt, &row.LastScheduleError, &row.LastRunError, &row.ProjectID, &row.Concurrency, &row.Status, &createdAt, &row.StartedAt, &row.CompletedAt, &row.CurrentIndex); err != nil {
			return nil, fmt.Errorf("扫描批量任务队列失败: %w", err)
		}
		parsedTime, parseErr := time.Parse("2006-01-02 15:04:05", createdAt)
		if parseErr != nil {
			parsedTime, parseErr = time.Parse(time.RFC3339, createdAt)
			if parseErr != nil {
				db.logger.Warn("解析创建时间失败", zap.String("createdAt", createdAt), zap.Error(parseErr))
				parsedTime = time.Now()
			}
		}
		row.CreatedAt = parsedTime
		queues = append(queues, &row)
	}

	return queues, nil
}

// ListBatchQueues 列出批量任务队列（支持筛选和分页）
func (db *DB) ListBatchQueues(limit, offset int, status, keyword string) ([]*BatchTaskQueueRow, error) {
	query := "SELECT id, title, role, agent_mode, schedule_mode, cron_expr, next_run_at, schedule_enabled, last_schedule_trigger_at, last_schedule_error, last_run_error, project_id, concurrency, status, created_at, started_at, completed_at, current_index FROM batch_task_queues WHERE 1=1"
	args := []interface{}{}

	// 状态筛选
	if status != "" && status != "all" {
		query += " AND status = ?"
		args = append(args, status)
	}

	// 关键字搜索（搜索队列ID和标题）
	if keyword != "" {
		query += " AND (id LIKE ? OR title LIKE ?)"
		args = append(args, "%"+keyword+"%", "%"+keyword+"%")
	}

	query += " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("查询批量任务队列列表失败: %w", err)
	}
	defer rows.Close()

	var queues []*BatchTaskQueueRow
	for rows.Next() {
		var row BatchTaskQueueRow
		var createdAt string
		if err := rows.Scan(&row.ID, &row.Title, &row.Role, &row.AgentMode, &row.ScheduleMode, &row.CronExpr, &row.NextRunAt, &row.ScheduleEnabled, &row.LastScheduleTriggerAt, &row.LastScheduleError, &row.LastRunError, &row.ProjectID, &row.Concurrency, &row.Status, &createdAt, &row.StartedAt, &row.CompletedAt, &row.CurrentIndex); err != nil {
			return nil, fmt.Errorf("扫描批量任务队列失败: %w", err)
		}
		parsedTime, parseErr := time.Parse("2006-01-02 15:04:05", createdAt)
		if parseErr != nil {
			parsedTime, parseErr = time.Parse(time.RFC3339, createdAt)
			if parseErr != nil {
				db.logger.Warn("解析创建时间失败", zap.String("createdAt", createdAt), zap.Error(parseErr))
				parsedTime = time.Now()
			}
		}
		row.CreatedAt = parsedTime
		queues = append(queues, &row)
	}

	return queues, nil
}

// CountBatchQueues 统计批量任务队列总数（支持筛选条件）
func (db *DB) CountBatchQueues(status, keyword string) (int, error) {
	query := "SELECT COUNT(*) FROM batch_task_queues WHERE 1=1"
	args := []interface{}{}

	// 状态筛选
	if status != "" && status != "all" {
		query += " AND status = ?"
		args = append(args, status)
	}

	// 关键字搜索（搜索队列ID和标题）
	if keyword != "" {
		query += " AND (id LIKE ? OR title LIKE ?)"
		args = append(args, "%"+keyword+"%", "%"+keyword+"%")
	}

	var count int
	err := db.QueryRow(query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("统计批量任务队列总数失败: %w", err)
	}

	return count, nil
}

// GetBatchTasks 获取批量任务队列的所有任务
func (db *DB) GetBatchTasks(queueID string) ([]*BatchTaskRow, error) {
	rows, err := db.Query(
		"SELECT id, queue_id, message, conversation_id, status, started_at, completed_at, error, result FROM batch_tasks WHERE queue_id = ? ORDER BY rowid ASC",
		queueID,
	)
	if err != nil {
		return nil, fmt.Errorf("查询批量任务失败: %w", err)
	}
	defer rows.Close()

	var tasks []*BatchTaskRow
	for rows.Next() {
		var task BatchTaskRow
		if err := rows.Scan(
			&task.ID, &task.QueueID, &task.Message, &task.ConversationID,
			&task.Status, &task.StartedAt, &task.CompletedAt, &task.Error, &task.Result,
		); err != nil {
			return nil, fmt.Errorf("扫描批量任务失败: %w", err)
		}
		tasks = append(tasks, &task)
	}

	return tasks, nil
}

// ListBatchTaskConversationIDs returns every conversation currently associated
// with a batch task. Startup recovery uses this snapshot to avoid treating
// pending batch placeholders as ordinary interrupted chats.
func (db *DB) ListBatchTaskConversationIDs() ([]string, error) {
	rows, err := db.Query(
		`SELECT DISTINCT conversation_id
		 FROM batch_tasks
		 WHERE TRIM(COALESCE(conversation_id, '')) <> ''`,
	)
	if err != nil {
		return nil, fmt.Errorf("查询批量任务对话失败: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("扫描批量任务对话失败: %w", err)
		}
		id = strings.TrimSpace(id)
		if id != "" {
			ids = append(ids, id)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历批量任务对话失败: %w", err)
	}
	return ids, nil
}

func (db *DB) MaxBatchTaskRowID() (int64, error) {
	var rowID sql.NullInt64
	if err := db.QueryRow("SELECT MAX(rowid) FROM batch_tasks").Scan(&rowID); err != nil {
		return 0, fmt.Errorf("查询批量任务快照失败: %w", err)
	}
	if !rowID.Valid {
		return 0, nil
	}
	return rowID.Int64, nil
}

// CountBatchTasksByStatus returns the number of tasks in a queue with the
// provided status.
func (db *DB) CountBatchTasksByStatus(queueID, status string) (int, error) {
	var count int
	if err := db.QueryRow(
		"SELECT COUNT(*) FROM batch_tasks WHERE queue_id = ? AND status = ?",
		queueID, status,
	).Scan(&count); err != nil {
		return 0, fmt.Errorf("统计批量任务状态失败: %w", err)
	}
	return count, nil
}

// UpdateBatchQueueStatus 更新批量任务队列状态
func (db *DB) UpdateBatchQueueStatus(queueID, status string) error {
	var err error
	now := time.Now()

	if status == "running" {
		_, err = db.Exec(
			"UPDATE batch_task_queues SET status = ?, started_at = COALESCE(started_at, ?) WHERE id = ?",
			status, now, queueID,
		)
	} else if status == "completed" || status == "cancelled" {
		_, err = db.Exec(
			"UPDATE batch_task_queues SET status = ?, completed_at = COALESCE(completed_at, ?) WHERE id = ?",
			status, now, queueID,
		)
	} else {
		_, err = db.Exec(
			"UPDATE batch_task_queues SET status = ? WHERE id = ?",
			status, queueID,
		)
	}

	if err != nil {
		return fmt.Errorf("更新批量任务队列状态失败: %w", err)
	}
	return nil
}

// UpdateBatchTaskStatus 更新批量任务状态
func (db *DB) UpdateBatchTaskStatus(queueID, taskID, status string, conversationID, result, errorMsg string) error {
	var err error
	now := time.Now()

	// 构建更新语句
	var updates []string
	var args []interface{}

	updates = append(updates, "status = ?")
	args = append(args, status)

	if conversationID != "" {
		updates = append(updates, "conversation_id = ?")
		args = append(args, conversationID)
	}

	if result != "" {
		updates = append(updates, "result = ?")
		args = append(args, result)
	}

	if errorMsg != "" {
		updates = append(updates, "error = ?")
		args = append(args, errorMsg)
	}

	if status == "running" {
		updates = append(updates, "started_at = COALESCE(started_at, ?)")
		args = append(args, now)
		updates = append(updates, "completed_at = NULL")
		updates = append(updates, "error = NULL")
		updates = append(updates, "result = NULL")
	}

	if status == "completed" || status == "failed" || status == "cancelled" {
		updates = append(updates, "completed_at = COALESCE(completed_at, ?)")
		args = append(args, now)
		switch status {
		case "completed":
			updates = append(updates, "error = NULL")
		case "failed":
			updates = append(updates, "result = NULL")
		case "cancelled":
			updates = append(updates, "error = NULL")
		}
	}

	args = append(args, queueID, taskID)

	// 构建SQL语句
	sql := "UPDATE batch_tasks SET "
	for i, update := range updates {
		if i > 0 {
			sql += ", "
		}
		sql += update
	}
	sql += " WHERE queue_id = ? AND id = ?"

	_, err = db.Exec(sql, args...)
	if err != nil {
		return fmt.Errorf("更新批量任务状态失败: %w", err)
	}
	return nil
}

// ClaimBatchTaskForRun marks a pending task as running and clears stale output
// fields from any previous interrupted/queued association.
func (db *DB) ClaimBatchTaskForRun(queueID, taskID string) error {
	now := time.Now()
	res, err := db.Exec(
		`UPDATE batch_tasks
		 SET status = ?, started_at = COALESCE(started_at, ?),
		     completed_at = NULL, error = NULL, result = NULL
		 WHERE queue_id = ? AND id = ? AND status = ?`,
		"running", now, queueID, taskID, "pending",
	)
	if err != nil {
		return fmt.Errorf("领取批量任务失败: %w", err)
	}
	if n, nerr := res.RowsAffected(); nerr == nil && n == 0 {
		return fmt.Errorf("任务不存在或已被领取")
	}
	return nil
}

// RecoverInterruptedBatchQueues marks tasks that were running before a process
// restart as failed without changing their queue status. The caller can then
// decide whether to continue recovered running queues. It also returns the
// conversation IDs that belonged to the interrupted running subtasks, so only
// those assistant placeholders are finalized.
func (db *DB) RecoverInterruptedBatchQueues(errorMsg string) ([]string, []string, int64, error) {
	return db.RecoverInterruptedBatchQueuesBefore(errorMsg, 0)
}

func (db *DB) RecoverInterruptedBatchQueuesBefore(errorMsg string, maxTaskRowID int64) ([]string, []string, int64, error) {
	tx, err := db.Begin()
	if err != nil {
		return nil, nil, 0, fmt.Errorf("开始事务失败: %w", err)
	}
	defer tx.Rollback()

	taskSnapshotFilter := ""
	args := []interface{}{"running"}
	if maxTaskRowID > 0 {
		taskSnapshotFilter = " AND t.rowid <= ?"
		args = append(args, maxTaskRowID)
	}

	rows, err := tx.Query(
		`SELECT DISTINCT q.id, q.status
		 FROM batch_task_queues q
		 JOIN batch_tasks t ON t.queue_id = q.id
		 WHERE t.status = ?
		 `+taskSnapshotFilter+`
		 ORDER BY q.created_at ASC`,
		args...,
	)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("查询重启前运行中的批量任务队列失败: %w", err)
	}
	defer rows.Close()

	var interruptedQueueIDs []string
	var resumeQueueIDs []string
	for rows.Next() {
		var queueID string
		var queueStatus string
		if err := rows.Scan(&queueID, &queueStatus); err != nil {
			return nil, nil, 0, fmt.Errorf("扫描重启前运行中的批量任务队列失败: %w", err)
		}
		interruptedQueueIDs = append(interruptedQueueIDs, queueID)
		if queueStatus == "running" {
			resumeQueueIDs = append(resumeQueueIDs, queueID)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, 0, fmt.Errorf("遍历重启前运行中的批量任务队列失败: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, nil, 0, fmt.Errorf("关闭重启前运行中的批量任务队列查询失败: %w", err)
	}
	if len(interruptedQueueIDs) == 0 {
		if err := tx.Commit(); err != nil {
			return nil, nil, 0, fmt.Errorf("提交事务失败: %w", err)
		}
		return nil, nil, 0, nil
	}

	now := time.Now()
	var failedTasks int64
	conversationIDSeen := make(map[string]struct{})
	var interruptedConversationIDs []string
	for _, queueID := range interruptedQueueIDs {
		taskQuery := `SELECT DISTINCT conversation_id
			 FROM batch_tasks
			 WHERE queue_id = ? AND status = ? AND TRIM(COALESCE(conversation_id, '')) <> ''`
		taskArgs := []interface{}{queueID, "running"}
		if maxTaskRowID > 0 {
			taskQuery += " AND rowid <= ?"
			taskArgs = append(taskArgs, maxTaskRowID)
		}
		taskRows, err := tx.Query(
			taskQuery,
			taskArgs...,
		)
		if err != nil {
			return nil, nil, 0, fmt.Errorf("查询重启中断的批量子任务对话失败: %w", err)
		}
		for taskRows.Next() {
			var conversationID string
			if err := taskRows.Scan(&conversationID); err != nil {
				_ = taskRows.Close()
				return nil, nil, 0, fmt.Errorf("扫描重启中断的批量子任务对话失败: %w", err)
			}
			conversationID = strings.TrimSpace(conversationID)
			if conversationID == "" {
				continue
			}
			if _, ok := conversationIDSeen[conversationID]; ok {
				continue
			}
			conversationIDSeen[conversationID] = struct{}{}
			interruptedConversationIDs = append(interruptedConversationIDs, conversationID)
		}
		if err := taskRows.Err(); err != nil {
			_ = taskRows.Close()
			return nil, nil, 0, fmt.Errorf("遍历重启中断的批量子任务对话失败: %w", err)
		}
		if err := taskRows.Close(); err != nil {
			return nil, nil, 0, fmt.Errorf("关闭重启中断的批量子任务对话查询失败: %w", err)
		}

		updateQuery := `UPDATE batch_tasks
			 SET status = ?, completed_at = COALESCE(completed_at, ?),
			     error = CASE WHEN TRIM(COALESCE(error, '')) = '' THEN ? ELSE error END
			 WHERE queue_id = ? AND status = ?`
		updateArgs := []interface{}{"failed", now, errorMsg, queueID, "running"}
		if maxTaskRowID > 0 {
			updateQuery += " AND rowid <= ?"
			updateArgs = append(updateArgs, maxTaskRowID)
		}
		res, err := tx.Exec(
			updateQuery,
			updateArgs...,
		)
		if err != nil {
			return nil, nil, 0, fmt.Errorf("标记重启中断的批量子任务失败: %w", err)
		}
		if n, nerr := res.RowsAffected(); nerr == nil {
			failedTasks += n
		}

	}

	if err := tx.Commit(); err != nil {
		return nil, nil, 0, fmt.Errorf("提交事务失败: %w", err)
	}
	return resumeQueueIDs, interruptedConversationIDs, failedTasks, nil
}

// UpdateBatchQueueCurrentIndex 更新批量任务队列的当前索引
func (db *DB) UpdateBatchQueueCurrentIndex(queueID string, currentIndex int) error {
	_, err := db.Exec(
		"UPDATE batch_task_queues SET current_index = ? WHERE id = ?",
		currentIndex, queueID,
	)
	if err != nil {
		return fmt.Errorf("更新批量任务队列当前索引失败: %w", err)
	}
	return nil
}

// UpdateBatchQueueMetadata 更新批量任务队列标题、角色和代理模式
func (db *DB) UpdateBatchQueueMetadata(queueID, title, role, agentMode string) error {
	_, err := db.Exec(
		"UPDATE batch_task_queues SET title = ?, role = ?, agent_mode = ? WHERE id = ?",
		title, role, agentMode, queueID,
	)
	if err != nil {
		return fmt.Errorf("更新批量任务队列元数据失败: %w", err)
	}
	return nil
}

// UpdateBatchQueueConcurrency 更新批量任务队列并发数
func (db *DB) UpdateBatchQueueConcurrency(queueID string, concurrency int) error {
	if concurrency < 1 {
		concurrency = 1
	}
	_, err := db.Exec(
		"UPDATE batch_task_queues SET concurrency = ? WHERE id = ?",
		concurrency, queueID,
	)
	if err != nil {
		return fmt.Errorf("更新批量任务队列并发数失败: %w", err)
	}
	return nil
}

// UpdateBatchQueueSchedule 更新批量任务队列调度相关信息
func (db *DB) UpdateBatchQueueSchedule(queueID, scheduleMode, cronExpr string, nextRunAt *time.Time) error {
	var nextRunAtValue interface{}
	if nextRunAt != nil {
		nextRunAtValue = *nextRunAt
	}
	_, err := db.Exec(
		"UPDATE batch_task_queues SET schedule_mode = ?, cron_expr = ?, next_run_at = ? WHERE id = ?",
		scheduleMode, cronExpr, nextRunAtValue, queueID,
	)
	if err != nil {
		return fmt.Errorf("更新批量任务调度配置失败: %w", err)
	}
	return nil
}

// UpdateBatchQueueScheduleEnabled 是否允许 Cron 自动触发（手工「开始执行」不受影响）
func (db *DB) UpdateBatchQueueScheduleEnabled(queueID string, enabled bool) error {
	v := 0
	if enabled {
		v = 1
	}
	_, err := db.Exec(
		"UPDATE batch_task_queues SET schedule_enabled = ? WHERE id = ?",
		v, queueID,
	)
	if err != nil {
		return fmt.Errorf("更新批量任务调度开关失败: %w", err)
	}
	return nil
}

// RecordBatchQueueScheduledTriggerStart 记录一次由调度触发的开始时间并清空调度层错误
func (db *DB) RecordBatchQueueScheduledTriggerStart(queueID string, at time.Time) error {
	_, err := db.Exec(
		"UPDATE batch_task_queues SET last_schedule_trigger_at = ?, last_schedule_error = NULL WHERE id = ?",
		at, queueID,
	)
	if err != nil {
		return fmt.Errorf("记录调度触发时间失败: %w", err)
	}
	return nil
}

// SetBatchQueueLastScheduleError 调度启动失败等原因（如状态不允许、重置失败）
func (db *DB) SetBatchQueueLastScheduleError(queueID, msg string) error {
	_, err := db.Exec(
		"UPDATE batch_task_queues SET last_schedule_error = ? WHERE id = ?",
		msg, queueID,
	)
	if err != nil {
		return fmt.Errorf("写入调度错误信息失败: %w", err)
	}
	return nil
}

// SetBatchQueueLastRunError 最近一轮执行中出现的子任务失败摘要（空串表示清空）
func (db *DB) SetBatchQueueLastRunError(queueID, msg string) error {
	var v interface{}
	if strings.TrimSpace(msg) == "" {
		v = nil
	} else {
		v = msg
	}
	_, err := db.Exec(
		"UPDATE batch_task_queues SET last_run_error = ? WHERE id = ?",
		v, queueID,
	)
	if err != nil {
		return fmt.Errorf("写入最近运行错误失败: %w", err)
	}
	return nil
}

// ResetBatchQueueForRerun 重置队列和任务状态用于下一轮调度执行
func (db *DB) ResetBatchQueueForRerun(queueID string) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("开始事务失败: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(
		"UPDATE batch_task_queues SET status = ?, current_index = 0, started_at = NULL, completed_at = NULL, last_run_error = NULL, last_schedule_error = NULL WHERE id = ?",
		"pending", queueID,
	)
	if err != nil {
		return fmt.Errorf("重置批量任务队列状态失败: %w", err)
	}

	_, err = tx.Exec(
		"UPDATE batch_tasks SET status = ?, conversation_id = NULL, started_at = NULL, completed_at = NULL, error = NULL, result = NULL WHERE queue_id = ?",
		"pending", queueID,
	)
	if err != nil {
		return fmt.Errorf("重置批量任务状态失败: %w", err)
	}

	return tx.Commit()
}

// UpdateBatchTaskMessage 更新批量任务消息
func (db *DB) UpdateBatchTaskMessage(queueID, taskID, message string) error {
	_, err := db.Exec(
		"UPDATE batch_tasks SET message = ? WHERE queue_id = ? AND id = ?",
		message, queueID, taskID,
	)
	if err != nil {
		return fmt.Errorf("更新批量任务消息失败: %w", err)
	}
	return nil
}

// AddBatchTask 添加任务到批量任务队列
func (db *DB) AddBatchTask(queueID, taskID, message string) error {
	_, err := db.Exec(
		"INSERT INTO batch_tasks (id, queue_id, message, status) VALUES (?, ?, ?, ?)",
		taskID, queueID, message, "pending",
	)
	if err != nil {
		return fmt.Errorf("添加批量任务失败: %w", err)
	}
	return nil
}

// CancelPendingBatchTasks 批量取消队列中所有 pending 状态的任务（单条 SQL）
func (db *DB) CancelPendingBatchTasks(queueID string, completedAt time.Time) error {
	_, err := db.Exec(
		"UPDATE batch_tasks SET status = ?, completed_at = ? WHERE queue_id = ? AND status = ?",
		"cancelled", completedAt, queueID, "pending",
	)
	if err != nil {
		return fmt.Errorf("批量取消 pending 任务失败: %w", err)
	}
	return nil
}

// DeleteBatchTask 删除批量任务
func (db *DB) DeleteBatchTask(queueID, taskID string) error {
	_, err := db.Exec(
		"DELETE FROM batch_tasks WHERE queue_id = ? AND id = ?",
		queueID, taskID,
	)
	if err != nil {
		return fmt.Errorf("删除批量任务失败: %w", err)
	}
	return nil
}

// DeleteBatchQueue 删除批量任务队列
func (db *DB) DeleteBatchQueue(queueID string) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("开始事务失败: %w", err)
	}
	defer tx.Rollback()

	// 删除任务（外键会自动级联删除）
	_, err = tx.Exec("DELETE FROM batch_tasks WHERE queue_id = ?", queueID)
	if err != nil {
		return fmt.Errorf("删除批量任务失败: %w", err)
	}

	// 删除队列
	_, err = tx.Exec("DELETE FROM batch_task_queues WHERE id = ?", queueID)
	if err != nil {
		return fmt.Errorf("删除批量任务队列失败: %w", err)
	}

	return tx.Commit()
}
