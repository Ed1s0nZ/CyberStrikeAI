package database

import (
	"database/sql"
	"time"

	"go.uber.org/zap"
)

// WebShellConnection WebShell 连接配置
type WebShellConnection struct {
	ID        string    `json:"id"`
	URL       string    `json:"url"`
	Password  string    `json:"password"`
	Type      string    `json:"type"`
	Method    string    `json:"method"`
	CmdParam  string    `json:"cmdParam"`
	Remark    string    `json:"remark"`
	CreatedAt time.Time `json:"createdAt"`
}

// GetWebshellConnectionState 获取连接关联的持久化状态 JSON，不存在时返回 "{}"
func (db *DB) GetWebshellConnectionState(connectionID string) (string, error) {
	var stateJSON string
	err := db.QueryRow(`SELECT state_json FROM webshell_connection_states WHERE connection_id = ?`, connectionID).Scan(&stateJSON)
	if err == sql.ErrNoRows {
		return "{}", nil
	}
	if err != nil {
		db.logger.Error("查询 WebShell 连接状态失败", zap.Error(err), zap.String("connectionID", connectionID))
		return "", err
	}
	if stateJSON == "" {
		stateJSON = "{}"
	}
	return stateJSON, nil
}

// UpsertWebshellConnectionState 保存连接关联的持久化状态 JSON
func (db *DB) UpsertWebshellConnectionState(connectionID, stateJSON string) error {
	if stateJSON == "" {
		stateJSON = "{}"
	}
	query := `
		INSERT INTO webshell_connection_states (connection_id, state_json, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(connection_id) DO UPDATE SET
			state_json = excluded.state_json,
			updated_at = excluded.updated_at
	`
	if _, err := db.Exec(query, connectionID, stateJSON, time.Now()); err != nil {
		db.logger.Error("保存 WebShell 连接状态失败", zap.Error(err), zap.String("connectionID", connectionID))
		return err
	}
	return nil
}

// ListWebshellConnections 列出所有 WebShell 连接，按创建时间倒序
func (db *DB) ListWebshellConnections() ([]WebShellConnection, error) {
	query := `
		SELECT id, url, password, type, method, cmd_param, remark, created_at
		FROM webshell_connections
		ORDER BY created_at DESC
	`
	rows, err := db.Query(query)
	if err != nil {
		db.logger.Error("查询 WebShell 连接列表失败", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	var list []WebShellConnection
	for rows.Next() {
		var c WebShellConnection
		err := rows.Scan(&c.ID, &c.URL, &c.Password, &c.Type, &c.Method, &c.CmdParam, &c.Remark, &c.CreatedAt)
		if err != nil {
			db.logger.Warn("扫描 WebShell 连接行失败", zap.Error(err))
			continue
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

// GetWebshellConnection 根据 ID 获取一条连接
func (db *DB) GetWebshellConnection(id string) (*WebShellConnection, error) {
	query := `
		SELECT id, url, password, type, method, cmd_param, remark, created_at
		FROM webshell_connections WHERE id = ?
	`
	var c WebShellConnection
	err := db.QueryRow(query, id).Scan(&c.ID, &c.URL, &c.Password, &c.Type, &c.Method, &c.CmdParam, &c.Remark, &c.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		db.logger.Error("查询 WebShell 连接失败", zap.Error(err), zap.String("id", id))
		return nil, err
	}
	return &c, nil
}

// CreateWebshellConnection 创建 WebShell 连接
func (db *DB) CreateWebshellConnection(c *WebShellConnection) error {
	query := `
		INSERT INTO webshell_connections (id, url, password, type, method, cmd_param, remark, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := db.Exec(query, c.ID, c.URL, c.Password, c.Type, c.Method, c.CmdParam, c.Remark, c.CreatedAt)
	if err != nil {
		db.logger.Error("创建 WebShell 连接失败", zap.Error(err), zap.String("id", c.ID))
		return err
	}
	return nil
}

// UpdateWebshellConnection 更新 WebShell 连接
func (db *DB) UpdateWebshellConnection(c *WebShellConnection) error {
	query := `
		UPDATE webshell_connections
		SET url = ?, password = ?, type = ?, method = ?, cmd_param = ?, remark = ?
		WHERE id = ?
	`
	result, err := db.Exec(query, c.URL, c.Password, c.Type, c.Method, c.CmdParam, c.Remark, c.ID)
	if err != nil {
		db.logger.Error("更新 WebShell 连接失败", zap.Error(err), zap.String("id", c.ID))
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// DeleteWebshellConnection 删除 WebShell 连接
func (db *DB) DeleteWebshellConnection(id string) error {
	result, err := db.Exec(`DELETE FROM webshell_connections WHERE id = ?`, id)
	if err != nil {
		db.logger.Error("删除 WebShell 连接失败", zap.Error(err), zap.String("id", id))
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}
