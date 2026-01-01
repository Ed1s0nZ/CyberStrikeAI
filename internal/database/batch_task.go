package database

import (
	"database/sql"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// BatchTaskQueueRow 批量任务队列数据库行
type BatchTaskQueueRow struct {
	ID           string
	Status       string
	CreatedAt    time.Time
	StartedAt    sql.NullTime
	CompletedAt  sql.NullTime
	CurrentIndex int
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
func (db *DB) CreateBatchQueue(queueID string, tasks []map[string]interface{}) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("开始事务失败: %w", err)
	}
	defer tx.Rollback()

	now := time.Now()
	_, err = tx.Exec(
		"INSERT INTO batch_task_queues (id, status, created_at, current_index) VALUES (?, ?, ?, ?)",
		queueID, "pending", now, 0,
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
		"SELECT id, status, created_at, started_at, completed_at, current_index FROM batch_task_queues WHERE id = ?",
		queueID,
	).Scan(&row.ID, &row.Status, &createdAt, &row.StartedAt, &row.CompletedAt, &row.CurrentIndex)
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
		"SELECT id, status, created_at, started_at, completed_at, current_index FROM batch_task_queues ORDER BY created_at DESC",
	)
	if err != nil {
		return nil, fmt.Errorf("查询批量任务队列列表失败: %w", err)
	}
	defer rows.Close()

	var queues []*BatchTaskQueueRow
	for rows.Next() {
		var row BatchTaskQueueRow
		var createdAt string
		if err := rows.Scan(&row.ID, &row.Status, &createdAt, &row.StartedAt, &row.CompletedAt, &row.CurrentIndex); err != nil {
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
	query := "SELECT id, status, created_at, started_at, completed_at, current_index FROM batch_task_queues WHERE 1=1"
	args := []interface{}{}

	// 状态筛选
	if status != "" && status != "all" {
		query += " AND status = ?"
		args = append(args, status)
	}

	// 关键字搜索（搜索队列ID）
	if keyword != "" {
		query += " AND id LIKE ?"
		args = append(args, "%"+keyword+"%")
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
		if err := rows.Scan(&row.ID, &row.Status, &createdAt, &row.StartedAt, &row.CompletedAt, &row.CurrentIndex); err != nil {
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

	// 关键字搜索
	if keyword != "" {
		query += " AND id LIKE ?"
		args = append(args, "%"+keyword+"%")
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
		"SELECT id, queue_id, message, conversation_id, status, started_at, completed_at, error, result FROM batch_tasks WHERE queue_id = ? ORDER BY id",
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
	}
	
	if status == "completed" || status == "failed" || status == "cancelled" {
		updates = append(updates, "completed_at = COALESCE(completed_at, ?)")
		args = append(args, now)
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

