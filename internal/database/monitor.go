package database

import (
	"database/sql"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"cyberstrike-ai/internal/mcp"

	"go.uber.org/zap"
)

// SaveToolExecution 保存工具执行记录
func (db *DB) SaveToolExecution(exec *mcp.ToolExecution) error {
	argsJSON, err := json.Marshal(exec.Arguments)
	if err != nil {
		db.logger.Warn("序列化执行参数失败", zap.Error(err))
		argsJSON = []byte("{}")
	}

	var resultJSON sql.NullString
	if exec.Result != nil {
		resultBytes, err := json.Marshal(exec.Result)
		if err != nil {
			db.logger.Warn("序列化执行结果失败", zap.Error(err))
		} else {
			resultJSON = sql.NullString{String: string(resultBytes), Valid: true}
		}
	}

	var errorText sql.NullString
	if exec.Error != "" {
		errorText = sql.NullString{String: exec.Error, Valid: true}
	}

	var endTime sql.NullTime
	if exec.EndTime != nil {
		endTime = sql.NullTime{Time: *exec.EndTime, Valid: true}
	}

	var durationMs sql.NullInt64
	if exec.Duration > 0 {
		durationMs = sql.NullInt64{Int64: exec.Duration.Milliseconds(), Valid: true}
	}

	query := `
		INSERT OR REPLACE INTO tool_executions 
		(id, tool_name, arguments, status, result, error, start_time, end_time, duration_ms, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err = db.Exec(query,
		exec.ID,
		exec.ToolName,
		string(argsJSON),
		exec.Status,
		resultJSON,
		errorText,
		exec.StartTime,
		endTime,
		durationMs,
		time.Now(),
	)

	if err != nil {
		db.logger.Error("保存工具执行记录失败", zap.Error(err), zap.String("executionId", exec.ID))
		return err
	}

	return nil
}

// RecoverInterruptedToolExecutions marks persisted running tool executions from
// a previous process as failed so the monitor view does not stay stuck.
func (db *DB) RecoverInterruptedToolExecutions(errorMsg string) (int64, error) {
	now := time.Now()
	res, err := db.Exec(
		`UPDATE tool_executions
		 SET status = ?,
		     error = CASE WHEN TRIM(COALESCE(error, '')) = '' THEN ? ELSE error END,
		     end_time = COALESCE(end_time, ?),
		     duration_ms = CASE
		         WHEN duration_ms IS NULL THEN CAST((julianday(?) - julianday(start_time)) * 86400000 AS INTEGER)
		         ELSE duration_ms
		     END
		 WHERE status = ?`,
		"failed", errorMsg, now, now, "running",
	)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return n, nil
}

// CountToolExecutions 统计工具执行记录总数
func (db *DB) CountToolExecutions(status, toolName string) (int, error) {
	query := `SELECT COUNT(*) FROM tool_executions`
	args := []interface{}{}
	conditions := []string{}
	if status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, status)
	}
	if toolName != "" {
		// 支持部分匹配（模糊搜索），不区分大小写
		conditions = append(conditions, "LOWER(tool_name) LIKE ?")
		args = append(args, "%"+strings.ToLower(toolName)+"%")
	}
	if len(conditions) > 0 {
		query += ` WHERE ` + conditions[0]
		for i := 1; i < len(conditions); i++ {
			query += ` AND ` + conditions[i]
		}
	}
	var count int
	err := db.QueryRow(query, args...).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// LoadToolExecutions 加载所有工具执行记录（支持分页）
func (db *DB) LoadToolExecutions() ([]*mcp.ToolExecution, error) {
	return db.LoadToolExecutionsWithPagination(0, 1000, "", "")
}

// LoadToolExecutionsWithPagination 分页加载工具执行记录
// limit: 最大返回记录数，0 表示使用默认值 1000
// offset: 跳过的记录数，用于分页
// status: 状态筛选，空字符串表示不过滤
// toolName: 工具名称筛选，空字符串表示不过滤
func (db *DB) LoadToolExecutionsWithPagination(offset, limit int, status, toolName string) ([]*mcp.ToolExecution, error) {
	if limit <= 0 {
		limit = 1000 // 默认限制
	}
	if limit > 10000 {
		limit = 10000 // 最大限制，防止一次性加载过多数据
	}

	query := `
		SELECT id, tool_name, arguments, status, result, error, start_time, end_time, duration_ms
		FROM tool_executions
	`
	args := []interface{}{}
	conditions := []string{}
	if status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, status)
	}
	if toolName != "" {
		// 支持部分匹配（模糊搜索），不区分大小写
		conditions = append(conditions, "LOWER(tool_name) LIKE ?")
		args = append(args, "%"+strings.ToLower(toolName)+"%")
	}
	if len(conditions) > 0 {
		query += ` WHERE ` + conditions[0]
		for i := 1; i < len(conditions); i++ {
			query += ` AND ` + conditions[i]
		}
	}
	query += ` ORDER BY start_time DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var executions []*mcp.ToolExecution
	for rows.Next() {
		var exec mcp.ToolExecution
		var argsJSON string
		var resultJSON sql.NullString
		var errorText sql.NullString
		var endTime sql.NullTime
		var durationMs sql.NullInt64

		err := rows.Scan(
			&exec.ID,
			&exec.ToolName,
			&argsJSON,
			&exec.Status,
			&resultJSON,
			&errorText,
			&exec.StartTime,
			&endTime,
			&durationMs,
		)
		if err != nil {
			db.logger.Warn("加载执行记录失败", zap.Error(err))
			continue
		}

		// 解析参数
		if err := json.Unmarshal([]byte(argsJSON), &exec.Arguments); err != nil {
			db.logger.Warn("解析执行参数失败", zap.Error(err))
			exec.Arguments = make(map[string]interface{})
		}

		// 解析结果
		if resultJSON.Valid && resultJSON.String != "" {
			var result mcp.ToolResult
			if err := json.Unmarshal([]byte(resultJSON.String), &result); err != nil {
				db.logger.Warn("解析执行结果失败", zap.Error(err))
			} else {
				exec.Result = &result
			}
		}

		// 设置错误
		if errorText.Valid {
			exec.Error = errorText.String
		}

		// 设置结束时间
		if endTime.Valid {
			exec.EndTime = &endTime.Time
		}

		// 设置持续时间
		if durationMs.Valid {
			exec.Duration = time.Duration(durationMs.Int64) * time.Millisecond
		}

		executions = append(executions, &exec)
	}

	return executions, nil
}

// GetToolExecution 根据ID获取单条工具执行记录
func (db *DB) GetToolExecution(id string) (*mcp.ToolExecution, error) {
	query := `
		SELECT id, tool_name, arguments, status, result, error, start_time, end_time, duration_ms
		FROM tool_executions
		WHERE id = ?
	`

	row := db.QueryRow(query, id)

	var exec mcp.ToolExecution
	var argsJSON string
	var resultJSON sql.NullString
	var errorText sql.NullString
	var endTime sql.NullTime
	var durationMs sql.NullInt64

	err := row.Scan(
		&exec.ID,
		&exec.ToolName,
		&argsJSON,
		&exec.Status,
		&resultJSON,
		&errorText,
		&exec.StartTime,
		&endTime,
		&durationMs,
	)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal([]byte(argsJSON), &exec.Arguments); err != nil {
		db.logger.Warn("解析执行参数失败", zap.Error(err))
		exec.Arguments = make(map[string]interface{})
	}

	if resultJSON.Valid && resultJSON.String != "" {
		var result mcp.ToolResult
		if err := json.Unmarshal([]byte(resultJSON.String), &result); err != nil {
			db.logger.Warn("解析执行结果失败", zap.Error(err))
		} else {
			exec.Result = &result
		}
	}

	if errorText.Valid {
		exec.Error = errorText.String
	}

	if endTime.Valid {
		exec.EndTime = &endTime.Time
	}

	if durationMs.Valid {
		exec.Duration = time.Duration(durationMs.Int64) * time.Millisecond
	}

	return &exec, nil
}

// DeleteToolExecution 删除工具执行记录
func (db *DB) DeleteToolExecution(id string) error {
	query := `DELETE FROM tool_executions WHERE id = ?`
	_, err := db.Exec(query, id)
	if err != nil {
		db.logger.Error("删除工具执行记录失败", zap.Error(err), zap.String("executionId", id))
		return err
	}
	return nil
}

// DeleteToolExecutions 批量删除工具执行记录
func (db *DB) DeleteToolExecutions(ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	// 构建 IN 查询的占位符
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}

	query := `DELETE FROM tool_executions WHERE id IN (` + strings.Join(placeholders, ",") + `)`
	_, err := db.Exec(query, args...)
	if err != nil {
		db.logger.Error("批量删除工具执行记录失败", zap.Error(err), zap.Int("count", len(ids)))
		return err
	}
	return nil
}

// GetToolExecutionsByIds 根据ID列表获取工具执行记录（用于批量删除前获取统计信息）
func (db *DB) GetToolExecutionsByIds(ids []string) ([]*mcp.ToolExecution, error) {
	if len(ids) == 0 {
		return []*mcp.ToolExecution{}, nil
	}

	// 构建 IN 查询的占位符
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}

	query := `
		SELECT id, tool_name, arguments, status, result, error, start_time, end_time, duration_ms
		FROM tool_executions
		WHERE id IN (` + strings.Join(placeholders, ",") + `)
	`

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var executions []*mcp.ToolExecution
	for rows.Next() {
		var exec mcp.ToolExecution
		var argsJSON string
		var resultJSON sql.NullString
		var errorText sql.NullString
		var endTime sql.NullTime
		var durationMs sql.NullInt64

		err := rows.Scan(
			&exec.ID,
			&exec.ToolName,
			&argsJSON,
			&exec.Status,
			&resultJSON,
			&errorText,
			&exec.StartTime,
			&endTime,
			&durationMs,
		)
		if err != nil {
			db.logger.Warn("加载执行记录失败", zap.Error(err))
			continue
		}

		// 解析参数
		if err := json.Unmarshal([]byte(argsJSON), &exec.Arguments); err != nil {
			db.logger.Warn("解析执行参数失败", zap.Error(err))
			exec.Arguments = make(map[string]interface{})
		}

		// 解析结果
		if resultJSON.Valid && resultJSON.String != "" {
			var result mcp.ToolResult
			if err := json.Unmarshal([]byte(resultJSON.String), &result); err != nil {
				db.logger.Warn("解析执行结果失败", zap.Error(err))
			} else {
				exec.Result = &result
			}
		}

		// 设置错误
		if errorText.Valid {
			exec.Error = errorText.String
		}

		// 设置结束时间
		if endTime.Valid {
			exec.EndTime = &endTime.Time
		}

		// 设置持续时间
		if durationMs.Valid {
			exec.Duration = time.Duration(durationMs.Int64) * time.Millisecond
		}

		executions = append(executions, &exec)
	}

	return executions, nil
}

// SaveToolStats 保存工具统计信息
func (db *DB) SaveToolStats(toolName string, stats *mcp.ToolStats) error {
	var lastCallTime sql.NullTime
	if stats.LastCallTime != nil {
		lastCallTime = sql.NullTime{Time: *stats.LastCallTime, Valid: true}
	}

	query := `
		INSERT OR REPLACE INTO tool_stats 
		(tool_name, total_calls, success_calls, failed_calls, last_call_time, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`

	_, err := db.Exec(query,
		toolName,
		stats.TotalCalls,
		stats.SuccessCalls,
		stats.FailedCalls,
		lastCallTime,
		time.Now(),
	)

	if err != nil {
		db.logger.Error("保存工具统计信息失败", zap.Error(err), zap.String("toolName", toolName))
		return err
	}

	return nil
}

// LoadToolStats 加载所有工具统计信息
func (db *DB) LoadToolStats() (map[string]*mcp.ToolStats, error) {
	query := `
		SELECT tool_name, total_calls, success_calls, failed_calls, last_call_time
		FROM tool_stats
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := make(map[string]*mcp.ToolStats)
	for rows.Next() {
		var stat mcp.ToolStats
		var lastCallTime sql.NullTime

		err := rows.Scan(
			&stat.ToolName,
			&stat.TotalCalls,
			&stat.SuccessCalls,
			&stat.FailedCalls,
			&lastCallTime,
		)
		if err != nil {
			db.logger.Warn("加载统计信息失败", zap.Error(err))
			continue
		}

		if lastCallTime.Valid {
			stat.LastCallTime = &lastCallTime.Time
		}

		stats[stat.ToolName] = &stat
	}

	return stats, nil
}

// UpdateToolStats 更新工具统计信息（累加模式）
func (db *DB) UpdateToolStats(toolName string, totalCalls, successCalls, failedCalls int, lastCallTime *time.Time) error {
	var lastCallTimeSQL sql.NullTime
	if lastCallTime != nil {
		lastCallTimeSQL = sql.NullTime{Time: *lastCallTime, Valid: true}
	}

	query := `
		INSERT INTO tool_stats (tool_name, total_calls, success_calls, failed_calls, last_call_time, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(tool_name) DO UPDATE SET
			total_calls = total_calls + ?,
			success_calls = success_calls + ?,
			failed_calls = failed_calls + ?,
			last_call_time = COALESCE(?, last_call_time),
			updated_at = ?
	`

	_, err := db.Exec(query,
		toolName, totalCalls, successCalls, failedCalls, lastCallTimeSQL, time.Now(),
		totalCalls, successCalls, failedCalls, lastCallTimeSQL, time.Now(),
	)

	if err != nil {
		db.logger.Error("更新工具统计信息失败", zap.Error(err), zap.String("toolName", toolName))
		return err
	}

	return nil
}

// CallsTimelineBucket 调用趋势时间桶
type CallsTimelineBucket struct {
	BucketTime time.Time
	Total      int
	Failed     int
}

// truncateCallsTimelineBucket 将时间截断到趋势图桶边界（本地时区，与 handler 侧 truncateToBucket 一致）
func truncateCallsTimelineBucket(t time.Time, dailyBuckets bool) time.Time {
	t = t.In(time.Local)
	if dailyBuckets {
		y, m, d := t.Date()
		return time.Date(y, m, d, 0, 0, 0, 0, time.Local)
	}
	return t.Truncate(time.Hour)
}

// LoadCallsTimeline 按时间范围加载调用趋势（since 起至今，含边界）
func (db *DB) LoadCallsTimeline(since time.Time, dailyBuckets bool) ([]CallsTimelineBucket, error) {
	// 在 Go 侧按本地时区分桶，避免 SQLite strftime 对 UTC 存储时间分桶后再误当本地时间解析（差 8h 等问题）
	query := `
		SELECT start_time,
			CASE WHEN status IN ('failed', 'cancelled') THEN 1 ELSE 0 END AS failed
		FROM tool_executions
		WHERE start_time >= ?
	`

	rows, err := db.Query(query, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	bucketMap := make(map[time.Time]struct{ total, failed int })
	for rows.Next() {
		var startTime time.Time
		var failed int
		if err := rows.Scan(&startTime, &failed); err != nil {
			db.logger.Warn("加载调用趋势失败", zap.Error(err))
			continue
		}
		key := truncateCallsTimelineBucket(startTime, dailyBuckets)
		entry := bucketMap[key]
		entry.total++
		entry.failed += failed
		bucketMap[key] = entry
	}

	buckets := make([]CallsTimelineBucket, 0, len(bucketMap))
	for bucketTime, counts := range bucketMap {
		buckets = append(buckets, CallsTimelineBucket{
			BucketTime: bucketTime,
			Total:      counts.total,
			Failed:     counts.failed,
		})
	}
	sort.Slice(buckets, func(i, j int) bool {
		return buckets[i].BucketTime.Before(buckets[j].BucketTime)
	})
	return buckets, nil
}

// DecreaseToolStats 减少工具统计信息（用于删除执行记录时）
// 如果统计信息变为0，则删除该统计记录
func (db *DB) DecreaseToolStats(toolName string, totalCalls, successCalls, failedCalls int) error {
	// 先更新统计信息
	query := `
		UPDATE tool_stats SET
			total_calls = CASE WHEN total_calls - ? < 0 THEN 0 ELSE total_calls - ? END,
			success_calls = CASE WHEN success_calls - ? < 0 THEN 0 ELSE success_calls - ? END,
			failed_calls = CASE WHEN failed_calls - ? < 0 THEN 0 ELSE failed_calls - ? END,
			updated_at = ?
		WHERE tool_name = ?
	`

	_, err := db.Exec(query, totalCalls, totalCalls, successCalls, successCalls, failedCalls, failedCalls, time.Now(), toolName)
	if err != nil {
		db.logger.Error("减少工具统计信息失败", zap.Error(err), zap.String("toolName", toolName))
		return err
	}

	// 检查更新后的 total_calls 是否为 0，如果是则删除该统计记录
	checkQuery := `SELECT total_calls FROM tool_stats WHERE tool_name = ?`
	var newTotalCalls int
	err = db.QueryRow(checkQuery, toolName).Scan(&newTotalCalls)
	if err != nil {
		// 如果查询失败（记录不存在），直接返回
		return nil
	}

	// 如果 total_calls 为 0，删除该统计记录
	if newTotalCalls == 0 {
		deleteQuery := `DELETE FROM tool_stats WHERE tool_name = ?`
		_, err = db.Exec(deleteQuery, toolName)
		if err != nil {
			db.logger.Warn("删除零统计记录失败", zap.Error(err), zap.String("toolName", toolName))
			// 不返回错误，因为主要操作（更新统计）已成功
		} else {
			db.logger.Info("已删除零统计记录", zap.String("toolName", toolName))
		}
	}

	return nil
}
