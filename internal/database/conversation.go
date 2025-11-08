package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Conversation 对话
type Conversation struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Messages  []Message `json:"messages,omitempty"`
}

// Message 消息
type Message struct {
	ID              string   `json:"id"`
	ConversationID  string   `json:"conversationId"`
	Role            string   `json:"role"`
	Content         string   `json:"content"`
	MCPExecutionIDs []string `json:"mcpExecutionIds,omitempty"`
	CreatedAt       time.Time `json:"createdAt"`
}

// CreateConversation 创建新对话
func (db *DB) CreateConversation(title string) (*Conversation, error) {
	id := uuid.New().String()
	now := time.Now()

	_, err := db.Exec(
		"INSERT INTO conversations (id, title, created_at, updated_at) VALUES (?, ?, ?, ?)",
		id, title, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("创建对话失败: %w", err)
	}

	return &Conversation{
		ID:        id,
		Title:     title,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// GetConversation 获取对话
func (db *DB) GetConversation(id string) (*Conversation, error) {
	var conv Conversation
	var createdAt, updatedAt string

	err := db.QueryRow(
		"SELECT id, title, created_at, updated_at FROM conversations WHERE id = ?",
		id,
	).Scan(&conv.ID, &conv.Title, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("对话不存在")
		}
		return nil, fmt.Errorf("查询对话失败: %w", err)
	}

	// 尝试多种时间格式解析
	var err1, err2 error
	conv.CreatedAt, err1 = time.Parse("2006-01-02 15:04:05.999999999-07:00", createdAt)
	if err1 != nil {
		conv.CreatedAt, err1 = time.Parse("2006-01-02 15:04:05", createdAt)
	}
	if err1 != nil {
		conv.CreatedAt, err1 = time.Parse(time.RFC3339, createdAt)
	}
	
	conv.UpdatedAt, err2 = time.Parse("2006-01-02 15:04:05.999999999-07:00", updatedAt)
	if err2 != nil {
		conv.UpdatedAt, err2 = time.Parse("2006-01-02 15:04:05", updatedAt)
	}
	if err2 != nil {
		conv.UpdatedAt, err2 = time.Parse(time.RFC3339, updatedAt)
	}

	// 加载消息
	messages, err := db.GetMessages(id)
	if err != nil {
		return nil, fmt.Errorf("加载消息失败: %w", err)
	}
	conv.Messages = messages

	return &conv, nil
}

// ListConversations 列出所有对话
func (db *DB) ListConversations(limit, offset int) ([]*Conversation, error) {
	rows, err := db.Query(
		"SELECT id, title, created_at, updated_at FROM conversations ORDER BY updated_at DESC LIMIT ? OFFSET ?",
		limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("查询对话列表失败: %w", err)
	}
	defer rows.Close()

	var conversations []*Conversation
	for rows.Next() {
		var conv Conversation
		var createdAt, updatedAt string

		if err := rows.Scan(&conv.ID, &conv.Title, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("扫描对话失败: %w", err)
		}

		// 尝试多种时间格式解析
		var err1, err2 error
		conv.CreatedAt, err1 = time.Parse("2006-01-02 15:04:05.999999999-07:00", createdAt)
		if err1 != nil {
			conv.CreatedAt, err1 = time.Parse("2006-01-02 15:04:05", createdAt)
		}
		if err1 != nil {
			conv.CreatedAt, err1 = time.Parse(time.RFC3339, createdAt)
		}
		
		conv.UpdatedAt, err2 = time.Parse("2006-01-02 15:04:05.999999999-07:00", updatedAt)
		if err2 != nil {
			conv.UpdatedAt, err2 = time.Parse("2006-01-02 15:04:05", updatedAt)
		}
		if err2 != nil {
			conv.UpdatedAt, err2 = time.Parse(time.RFC3339, updatedAt)
		}
		
		conversations = append(conversations, &conv)
	}

	return conversations, nil
}

// UpdateConversationTitle 更新对话标题
func (db *DB) UpdateConversationTitle(id, title string) error {
	_, err := db.Exec(
		"UPDATE conversations SET title = ?, updated_at = ? WHERE id = ?",
		title, time.Now(), id,
	)
	if err != nil {
		return fmt.Errorf("更新对话标题失败: %w", err)
	}
	return nil
}

// UpdateConversationTime 更新对话时间
func (db *DB) UpdateConversationTime(id string) error {
	_, err := db.Exec(
		"UPDATE conversations SET updated_at = ? WHERE id = ?",
		time.Now(), id,
	)
	if err != nil {
		return fmt.Errorf("更新对话时间失败: %w", err)
	}
	return nil
}

// DeleteConversation 删除对话
func (db *DB) DeleteConversation(id string) error {
	_, err := db.Exec("DELETE FROM conversations WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("删除对话失败: %w", err)
	}
	return nil
}

// AddMessage 添加消息
func (db *DB) AddMessage(conversationID, role, content string, mcpExecutionIDs []string) (*Message, error) {
	id := uuid.New().String()
	
	var mcpIDsJSON string
	if len(mcpExecutionIDs) > 0 {
		jsonData, err := json.Marshal(mcpExecutionIDs)
		if err != nil {
			db.logger.Warn("序列化MCP执行ID失败", zap.Error(err))
		} else {
			mcpIDsJSON = string(jsonData)
		}
	}

	_, err := db.Exec(
		"INSERT INTO messages (id, conversation_id, role, content, mcp_execution_ids, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		id, conversationID, role, content, mcpIDsJSON, time.Now(),
	)
	if err != nil {
		return nil, fmt.Errorf("添加消息失败: %w", err)
	}

	// 更新对话时间
	if err := db.UpdateConversationTime(conversationID); err != nil {
		db.logger.Warn("更新对话时间失败", zap.Error(err))
	}

	message := &Message{
		ID:              id,
		ConversationID:  conversationID,
		Role:            role,
		Content:         content,
		MCPExecutionIDs: mcpExecutionIDs,
		CreatedAt:       time.Now(),
	}

	return message, nil
}

// GetMessages 获取对话的所有消息
func (db *DB) GetMessages(conversationID string) ([]Message, error) {
	rows, err := db.Query(
		"SELECT id, conversation_id, role, content, mcp_execution_ids, created_at FROM messages WHERE conversation_id = ? ORDER BY created_at ASC",
		conversationID,
	)
	if err != nil {
		return nil, fmt.Errorf("查询消息失败: %w", err)
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var msg Message
		var mcpIDsJSON sql.NullString
		var createdAt string

		if err := rows.Scan(&msg.ID, &msg.ConversationID, &msg.Role, &msg.Content, &mcpIDsJSON, &createdAt); err != nil {
			return nil, fmt.Errorf("扫描消息失败: %w", err)
		}

		// 尝试多种时间格式解析
		var err error
		msg.CreatedAt, err = time.Parse("2006-01-02 15:04:05.999999999-07:00", createdAt)
		if err != nil {
			msg.CreatedAt, err = time.Parse("2006-01-02 15:04:05", createdAt)
		}
		if err != nil {
			msg.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
		}

		// 解析MCP执行ID
		if mcpIDsJSON.Valid && mcpIDsJSON.String != "" {
			if err := json.Unmarshal([]byte(mcpIDsJSON.String), &msg.MCPExecutionIDs); err != nil {
				db.logger.Warn("解析MCP执行ID失败", zap.Error(err))
			}
		}

		messages = append(messages, msg)
	}

	return messages, nil
}

