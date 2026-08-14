package database

import (
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
)

// DeleteHitlInterruptLogsByIDs deletes decided HITL audit logs by id (pending rows are skipped).
func (db *DB) DeleteHitlInterruptLogsByIDs(ids []string) (int64, error) {
	if db == nil {
		return 0, fmt.Errorf("database is nil")
	}
	clean := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id != "" {
			clean = append(clean, id)
		}
	}
	if len(clean) == 0 {
		return 0, nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(clean)), ",")
	q := fmt.Sprintf(`DELETE FROM hitl_interrupts WHERE status != 'pending' AND id IN (%s)`, placeholders)
	args := make([]interface{}, len(clean))
	for i, id := range clean {
		args[i] = id
	}
	res, err := db.Exec(q, args...)
	if err != nil {
		db.logger.Error("批量删除人机协同审计日志失败", zap.Error(err), zap.Int("count", len(clean)))
		return 0, fmt.Errorf("批量删除人机协同审计日志失败: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// DeleteHitlInterruptLogsMatching deletes decided logs matching whereSQL (e.g. "WHERE 1=1 AND status != 'pending' ...").
func (db *DB) DeleteHitlInterruptLogsMatching(whereSQL string, args []interface{}) (int64, error) {
	if db == nil {
		return 0, fmt.Errorf("database is nil")
	}
	whereSQL = strings.TrimSpace(whereSQL)
	if whereSQL == "" {
		return 0, fmt.Errorf("where clause is required")
	}
	q := `DELETE FROM hitl_interrupts ` + whereSQL
	res, err := db.Exec(q, args...)
	if err != nil {
		db.logger.Error("清空人机协同审计日志失败", zap.Error(err))
		return 0, fmt.Errorf("清空人机协同审计日志失败: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// PurgeHitlInterruptLogsBefore deletes decided logs with decided/created time before cutoff.
func (db *DB) PurgeHitlInterruptLogsBefore(cutoff time.Time) (int64, error) {
	if db == nil {
		return 0, fmt.Errorf("database is nil")
	}
	res, err := db.Exec(
		`DELETE FROM hitl_interrupts WHERE status != 'pending' AND datetime(COALESCE(decided_at, created_at)) < datetime(?)`,
		cutoff.UTC().Format(time.RFC3339),
	)
	if err != nil {
		db.logger.Error("清理过期人机协同审计日志失败", zap.Error(err))
		return 0, fmt.Errorf("清理过期人机协同审计日志失败: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}
