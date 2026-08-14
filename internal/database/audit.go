package database

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// AuditLog platform operation audit record.
type AuditLog struct {
	ID                string                 `json:"id"`
	CreatedAt         time.Time              `json:"createdAt"`
	Level             string                 `json:"level"`
	Category          string                 `json:"category"`
	Action            string                 `json:"action"`
	Result            string                 `json:"result"`
	Actor             string                 `json:"actor"`
	SessionHint       string                 `json:"sessionHint,omitempty"`
	ClientIP          string                 `json:"clientIp,omitempty"`
	UserAgent         string                 `json:"userAgent,omitempty"`
	ResourceType      string                 `json:"resourceType,omitempty"`
	ResourceID        string                 `json:"resourceId,omitempty"`
	ResourceAvailable *bool                  `json:"resourceAvailable,omitempty"` // API-only: whether linked resource still exists
	Message           string                 `json:"message"`
	Detail            map[string]interface{} `json:"detail,omitempty"`
}

// ListAuditLogsFilter query parameters.
type ListAuditLogsFilter struct {
	Actor         string
	Level         string
	Category      string
	Action        string
	Result        string
	Query         string
	ResourceType  string
	ResourceID    string
	RelatedUserID string
	Since         *time.Time
	Until         *time.Time
	Limit         int
	Offset        int
}

func buildAuditLogsWhere(filter ListAuditLogsFilter) (string, []interface{}) {
	conditions := []string{"1=1"}
	args := []interface{}{}
	if filter.Actor != "" {
		conditions = append(conditions, "actor = ?")
		args = append(args, filter.Actor)
	}
	if filter.Level != "" {
		conditions = append(conditions, "level = ?")
		args = append(args, filter.Level)
	}
	if filter.Category != "" {
		conditions = append(conditions, "category = ?")
		args = append(args, filter.Category)
	}
	if filter.Action != "" {
		conditions = append(conditions, "action = ?")
		args = append(args, filter.Action)
	}
	if filter.Result != "" {
		conditions = append(conditions, "result = ?")
		args = append(args, filter.Result)
	}
	if filter.ResourceType != "" {
		conditions = append(conditions, "resource_type = ?")
		args = append(args, filter.ResourceType)
	}
	if filter.ResourceID != "" {
		conditions = append(conditions, "resource_id = ?")
		args = append(args, filter.ResourceID)
	}
	if relatedUserID := strings.TrimSpace(filter.RelatedUserID); relatedUserID != "" {
		conditions = append(conditions, `(resource_id = ? OR detail_json LIKE ? OR detail_json LIKE ?)`)
		args = append(args, relatedUserID, `%"user_id":"`+relatedUserID+`"%`, `%"userId":"`+relatedUserID+`"%`)
	}
	if filter.Since != nil {
		conditions = append(conditions, sqliteEpochGE("created_at", ">="))
		args = append(args, formatSQLiteUTC(*filter.Since))
	}
	if filter.Until != nil {
		conditions = append(conditions, sqliteEpochGE("created_at", "<="))
		args = append(args, formatSQLiteUTC(*filter.Until))
	}
	if q := strings.TrimSpace(filter.Query); q != "" {
		like := "%" + q + "%"
		conditions = append(conditions, "(message LIKE ? OR resource_id LIKE ? OR action LIKE ? OR category LIKE ? OR detail_json LIKE ?)")
		args = append(args, like, like, like, like, like)
	}
	return strings.Join(conditions, " AND "), args
}

// AppendAuditLog inserts one audit row.
func (db *DB) AppendAuditLog(row *AuditLog) error {
	if row == nil {
		return errors.New("audit log is nil")
	}
	if strings.TrimSpace(row.ID) == "" {
		return errors.New("audit id is required")
	}
	if row.CreatedAt.IsZero() {
		row.CreatedAt = time.Now().UTC()
	} else {
		row.CreatedAt = row.CreatedAt.UTC()
	}
	if strings.TrimSpace(row.Level) == "" {
		row.Level = "info"
	}
	detailJSON := ""
	if len(row.Detail) > 0 {
		if b, err := json.Marshal(row.Detail); err == nil {
			detailJSON = string(b)
		}
	}
	query := `
		INSERT INTO audit_logs (
			id, created_at, level, category, action, result, actor, session_hint,
			client_ip, user_agent, resource_type, resource_id, message, detail_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := db.Exec(query,
		row.ID, formatSQLiteUTC(row.CreatedAt), row.Level, row.Category, row.Action, row.Result,
		row.Actor, row.SessionHint, row.ClientIP, row.UserAgent,
		row.ResourceType, row.ResourceID, row.Message, detailJSON,
	)
	return err
}

// GetAuditLogByID returns one row.
func (db *DB) GetAuditLogByID(id string) (*AuditLog, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errors.New("id is required")
	}
	query := `
		SELECT id, created_at, level, category, action, result, actor,
			COALESCE(session_hint, ''), COALESCE(client_ip, ''), COALESCE(user_agent, ''),
			COALESCE(resource_type, ''), COALESCE(resource_id, ''), message, COALESCE(detail_json, '')
		FROM audit_logs WHERE id = ?
	`
	var row AuditLog
	var detailJSON string
	err := db.QueryRow(query, id).Scan(
		&row.ID, &row.CreatedAt, &row.Level, &row.Category, &row.Action, &row.Result, &row.Actor,
		&row.SessionHint, &row.ClientIP, &row.UserAgent,
		&row.ResourceType, &row.ResourceID, &row.Message, &detailJSON,
	)
	if err != nil {
		return nil, err
	}
	if detailJSON != "" {
		_ = json.Unmarshal([]byte(detailJSON), &row.Detail)
	}
	return &row, nil
}

// CountAuditLogs counts rows matching filter.
func (db *DB) CountAuditLogs(filter ListAuditLogsFilter) (int64, error) {
	where, args := buildAuditLogsWhere(filter)
	query := `SELECT COUNT(*) FROM audit_logs WHERE ` + where
	var n int64
	err := db.QueryRow(query, args...).Scan(&n)
	return n, err
}

// ListAuditLogs lists audit rows newest first.
func (db *DB) ListAuditLogs(filter ListAuditLogsFilter) ([]*AuditLog, error) {
	where, args := buildAuditLogsWhere(filter)
	limit := filter.Limit
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	query := `
		SELECT id, created_at, level, category, action, result, actor,
			COALESCE(session_hint, ''), COALESCE(client_ip, ''), COALESCE(user_agent, ''),
			COALESCE(resource_type, ''), COALESCE(resource_id, ''), message, COALESCE(detail_json, '')
		FROM audit_logs
		WHERE ` + where + `
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`
	args = append(args, limit, offset)
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*AuditLog
	for rows.Next() {
		var row AuditLog
		var detailJSON string
		if err := rows.Scan(
			&row.ID, &row.CreatedAt, &row.Level, &row.Category, &row.Action, &row.Result, &row.Actor,
			&row.SessionHint, &row.ClientIP, &row.UserAgent,
			&row.ResourceType, &row.ResourceID, &row.Message, &detailJSON,
		); err != nil {
			continue
		}
		if detailJSON != "" {
			_ = json.Unmarshal([]byte(detailJSON), &row.Detail)
		}
		list = append(list, &row)
	}
	return list, rows.Err()
}

// DeleteAuditLogsBefore removes rows older than cutoff.
func (db *DB) DeleteAuditLogsBefore(cutoff time.Time) (int64, error) {
	res, err := db.Exec(`DELETE FROM audit_logs WHERE `+sqliteEpochGE("created_at", "<"), formatSQLiteUTC(cutoff))
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
