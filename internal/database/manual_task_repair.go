package database

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ManualAgentStartupRepairSummary records orphaned manual agent messages fixed during startup.
type ManualAgentStartupRepairSummary struct {
	MessagesRepaired int64
	DetailsInserted  int64
}

// RepairInterruptedManualAgentMessages closes assistant placeholder messages left by manual agent runs.
//
// Manual agent task state is kept in memory. If the process exits while a task is running, the
// in-memory task disappears but the assistant placeholder message remains in the database. On the
// next startup, that task cannot still be running in this process, so mark the placeholder as failed.
func (db *DB) RepairInterruptedManualAgentMessages(reason string) (*ManualAgentStartupRepairSummary, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "服务重启导致任务中断，已自动标记失败"
	}
	now := time.Now()
	summary := &ManualAgentStartupRepairSummary{}

	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("开始事务失败: %w", err)
	}
	defer tx.Rollback()

	type placeholderMessage struct {
		ID             string
		ConversationID string
	}
	var placeholders []placeholderMessage
	rows, err := tx.Query(`
		SELECT id, conversation_id
		FROM messages
		WHERE role = 'assistant'
		  AND TRIM(content) = '处理中...'
		ORDER BY created_at ASC, rowid ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("查询中断手动任务占位消息失败: %w", err)
	}
	for rows.Next() {
		var msg placeholderMessage
		if err := rows.Scan(&msg.ID, &msg.ConversationID); err != nil {
			rows.Close()
			return nil, fmt.Errorf("扫描中断手动任务占位消息失败: %w", err)
		}
		placeholders = append(placeholders, msg)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("关闭中断手动任务占位消息游标失败: %w", err)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历中断手动任务占位消息失败: %w", err)
	}

	for _, msg := range placeholders {
		res, err := tx.Exec(`
			UPDATE messages
			SET content = ?, updated_at = ?
			WHERE id = ?
			  AND role = 'assistant'
			  AND TRIM(content) = '处理中...'
		`, reason, now, msg.ID)
		if err != nil {
			return nil, fmt.Errorf("更新中断手动任务占位消息失败: %w", err)
		}
		affected, _ := res.RowsAffected()
		summary.MessagesRepaired += affected

		var existingDetails int
		if err := tx.QueryRow(`
			SELECT COUNT(1)
			FROM process_details
			WHERE message_id = ?
			  AND conversation_id = ?
			  AND event_type = 'error'
			  AND COALESCE(message, '') = ?
		`, msg.ID, msg.ConversationID, reason).Scan(&existingDetails); err != nil {
			return nil, fmt.Errorf("检查中断手动任务过程详情失败: %w", err)
		}
		if existingDetails == 0 {
			detailID := uuid.New().String()
			if _, err := tx.Exec(`
				INSERT INTO process_details (id, message_id, conversation_id, event_type, message, data, created_at)
				VALUES (?, ?, ?, 'error', ?, '', ?)
			`, detailID, msg.ID, msg.ConversationID, reason, now); err != nil {
				return nil, fmt.Errorf("写入中断手动任务过程详情失败: %w", err)
			}
			summary.DetailsInserted++
		}

		_, _ = tx.Exec(`UPDATE conversations SET updated_at = ? WHERE id = ?`, now, msg.ConversationID)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("提交中断手动任务修复失败: %w", err)
	}
	return summary, nil
}
