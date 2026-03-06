package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Conversation represents a conversation
type Conversation struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Pinned    bool      `json:"pinned"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Messages  []Message `json:"messages,omitempty"`
}

// Message represents a message
type Message struct {
	ID              string                   `json:"id"`
	ConversationID  string                   `json:"conversationId"`
	Role            string                   `json:"role"`
	Content         string                   `json:"content"`
	MCPExecutionIDs []string                 `json:"mcpExecutionIds,omitempty"`
	ProcessDetails  []map[string]interface{} `json:"processDetails,omitempty"`
	CreatedAt       time.Time                `json:"createdAt"`
}

// CreateConversation creates a new conversation
func (db *DB) CreateConversation(title string) (*Conversation, error) {
	id := uuid.New().String()
	now := time.Now()

	_, err := db.Exec(
		"INSERT INTO conversations (id, title, created_at, updated_at) VALUES (?, ?, ?, ?)",
		id, title, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create conversation: %w", err)
	}

	return &Conversation{
		ID:        id,
		Title:     title,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// GetConversation gets a conversation
func (db *DB) GetConversation(id string) (*Conversation, error) {
	var conv Conversation
	var createdAt, updatedAt string
	var pinned int

	err := db.QueryRow(
		"SELECT id, title, pinned, created_at, updated_at FROM conversations WHERE id = ?",
		id,
	).Scan(&conv.ID, &conv.Title, &pinned, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("conversation not found")
		}
		return nil, fmt.Errorf("failed to query conversation: %w", err)
	}

	// try multiple time format parsings
	var err1, err2 error
	conv.CreatedAt, err1 = time.Parse("2006-01-02 15:04:05.999999999-07:00", createdAt)
	if err1 != nil {
		conv.CreatedAt, err1 = time.Parse("2006-01-02 15:04:05", createdAt)
	}
	if err1 != nil {
		conv.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	}

	conv.UpdatedAt, err2 = time.Parse("2006-01-02 15:04:05.999999999-07:00", updatedAt)
	if err2 != nil {
		conv.UpdatedAt, err2 = time.Parse("2006-01-02 15:04:05", updatedAt)
	}
	if err2 != nil {
		conv.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	}

	conv.Pinned = pinned != 0

	// load messages
	messages, err := db.GetMessages(id)
	if err != nil {
		return nil, fmt.Errorf("failed to load messages: %w", err)
	}
	conv.Messages = messages

	// load process details (grouped by message ID)
	processDetailsMap, err := db.GetProcessDetailsByConversation(id)
	if err != nil {
		db.logger.Warn("failed to load process details", zap.Error(err))
		processDetailsMap = make(map[string][]ProcessDetail)
	}

	// attach process details to corresponding messages
	for i := range conv.Messages {
		if details, ok := processDetailsMap[conv.Messages[i].ID]; ok {
			// convert ProcessDetail to JSON format for frontend use
			detailsJSON := make([]map[string]interface{}, len(details))
			for j, detail := range details {
				var data interface{}
				if detail.Data != "" {
					if err := json.Unmarshal([]byte(detail.Data), &data); err != nil {
						db.logger.Warn("failed to parse process detail data", zap.Error(err))
					}
				}
				detailsJSON[j] = map[string]interface{}{
					"id":             detail.ID,
					"messageId":      detail.MessageID,
					"conversationId": detail.ConversationID,
					"eventType":      detail.EventType,
					"message":        detail.Message,
					"data":           data,
					"createdAt":      detail.CreatedAt,
				}
			}
			conv.Messages[i].ProcessDetails = detailsJSON
		}
	}

	return &conv, nil
}

// ListConversations lists all conversations
func (db *DB) ListConversations(limit, offset int, search string) ([]*Conversation, error) {
	var rows *sql.Rows
	var err error

	if search != "" {
		// use LIKE for fuzzy search on title and message content
		searchPattern := "%" + search + "%"
		// use DISTINCT to avoid duplicates when a conversation has multiple matching messages
		rows, err = db.Query(
			`SELECT DISTINCT c.id, c.title, COALESCE(c.pinned, 0), c.created_at, c.updated_at
			 FROM conversations c
			 LEFT JOIN messages m ON c.id = m.conversation_id
			 WHERE c.title LIKE ? OR m.content LIKE ?
			 ORDER BY c.updated_at DESC
			 LIMIT ? OFFSET ?`,
			searchPattern, searchPattern, limit, offset,
		)
	} else {
		rows, err = db.Query(
			"SELECT id, title, COALESCE(pinned, 0), created_at, updated_at FROM conversations ORDER BY updated_at DESC LIMIT ? OFFSET ?",
			limit, offset,
		)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to query conversation list: %w", err)
	}
	defer rows.Close()

	var conversations []*Conversation
	for rows.Next() {
		var conv Conversation
		var createdAt, updatedAt string
		var pinned int

		if err := rows.Scan(&conv.ID, &conv.Title, &pinned, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan conversation: %w", err)
		}

		// try multiple time format parsings
		var err1, err2 error
		conv.CreatedAt, err1 = time.Parse("2006-01-02 15:04:05.999999999-07:00", createdAt)
		if err1 != nil {
			conv.CreatedAt, err1 = time.Parse("2006-01-02 15:04:05", createdAt)
		}
		if err1 != nil {
			conv.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		}

		conv.UpdatedAt, err2 = time.Parse("2006-01-02 15:04:05.999999999-07:00", updatedAt)
		if err2 != nil {
			conv.UpdatedAt, err2 = time.Parse("2006-01-02 15:04:05", updatedAt)
		}
		if err2 != nil {
			conv.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		}

		conv.Pinned = pinned != 0

		conversations = append(conversations, &conv)
	}

	return conversations, nil
}

// UpdateConversationTitle updates the conversation title
func (db *DB) UpdateConversationTitle(id, title string) error {
	// note: do not update updated_at, as renaming should not change the conversation update time
	_, err := db.Exec(
		"UPDATE conversations SET title = ? WHERE id = ?",
		title, id,
	)
	if err != nil {
		return fmt.Errorf("failed to update conversation title: %w", err)
	}
	return nil
}

// UpdateConversationTime updates the conversation timestamp
func (db *DB) UpdateConversationTime(id string) error {
	_, err := db.Exec(
		"UPDATE conversations SET updated_at = ? WHERE id = ?",
		time.Now(), id,
	)
	if err != nil {
		return fmt.Errorf("failed to update conversation time: %w", err)
	}
	return nil
}

// DeleteConversation deletes a conversation and all its related data.
// Due to database foreign key constraints with ON DELETE CASCADE, deleting a conversation automatically deletes:
// - messages
// - process_details
// - attack_chain_nodes
// - attack_chain_edges
// - vulnerabilities
// - conversation_group_mappings
// Note: knowledge_retrieval_logs uses ON DELETE SET NULL, records are kept but conversation_id is set to NULL
func (db *DB) DeleteConversation(id string) error {
	// explicitly delete knowledge retrieval logs (although the foreign key is SET NULL, we delete manually for thorough cleanup)
	_, err := db.Exec("DELETE FROM knowledge_retrieval_logs WHERE conversation_id = ?", id)
	if err != nil {
		db.logger.Warn("failed to delete knowledge retrieval logs", zap.String("conversationId", id), zap.Error(err))
		// do not return error, continue to delete conversation
	}

	// delete conversation (foreign key CASCADE will automatically delete other related data)
	_, err = db.Exec("DELETE FROM conversations WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete conversation: %w", err)
	}

	db.logger.Info("conversation and all related data deleted", zap.String("conversationId", id))
	return nil
}

// SaveReActData saves the input and output of the last ReAct round
func (db *DB) SaveReActData(conversationID, reactInput, reactOutput string) error {
	_, err := db.Exec(
		"UPDATE conversations SET last_react_input = ?, last_react_output = ?, updated_at = ? WHERE id = ?",
		reactInput, reactOutput, time.Now(), conversationID,
	)
	if err != nil {
		return fmt.Errorf("failed to save ReAct data: %w", err)
	}
	return nil
}

// GetReActData gets the input and output of the last ReAct round
func (db *DB) GetReActData(conversationID string) (reactInput, reactOutput string, err error) {
	var input, output sql.NullString
	err = db.QueryRow(
		"SELECT last_react_input, last_react_output FROM conversations WHERE id = ?",
		conversationID,
	).Scan(&input, &output)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", "", fmt.Errorf("conversation not found")
		}
		return "", "", fmt.Errorf("failed to get ReAct data: %w", err)
	}

	if input.Valid {
		reactInput = input.String
	}
	if output.Valid {
		reactOutput = output.String
	}

	return reactInput, reactOutput, nil
}

// AddMessage adds a message
func (db *DB) AddMessage(conversationID, role, content string, mcpExecutionIDs []string) (*Message, error) {
	id := uuid.New().String()

	var mcpIDsJSON string
	if len(mcpExecutionIDs) > 0 {
		jsonData, err := json.Marshal(mcpExecutionIDs)
		if err != nil {
			db.logger.Warn("failed to serialize MCP execution IDs", zap.Error(err))
		} else {
			mcpIDsJSON = string(jsonData)
		}
	}

	_, err := db.Exec(
		"INSERT INTO messages (id, conversation_id, role, content, mcp_execution_ids, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		id, conversationID, role, content, mcpIDsJSON, time.Now(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to add message: %w", err)
	}

	// update conversation timestamp
	if err := db.UpdateConversationTime(conversationID); err != nil {
		db.logger.Warn("failed to update conversation time", zap.Error(err))
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

// GetMessages gets all messages for a conversation
func (db *DB) GetMessages(conversationID string) ([]Message, error) {
	rows, err := db.Query(
		"SELECT id, conversation_id, role, content, mcp_execution_ids, created_at FROM messages WHERE conversation_id = ? ORDER BY created_at ASC",
		conversationID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query messages: %w", err)
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var msg Message
		var mcpIDsJSON sql.NullString
		var createdAt string

		if err := rows.Scan(&msg.ID, &msg.ConversationID, &msg.Role, &msg.Content, &mcpIDsJSON, &createdAt); err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}

		// try multiple time format parsings
		var err error
		msg.CreatedAt, err = time.Parse("2006-01-02 15:04:05.999999999-07:00", createdAt)
		if err != nil {
			msg.CreatedAt, err = time.Parse("2006-01-02 15:04:05", createdAt)
		}
		if err != nil {
			msg.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		}

		// parse MCP execution IDs
		if mcpIDsJSON.Valid && mcpIDsJSON.String != "" {
			if err := json.Unmarshal([]byte(mcpIDsJSON.String), &msg.MCPExecutionIDs); err != nil {
				db.logger.Warn("failed to parse MCP execution IDs", zap.Error(err))
			}
		}

		messages = append(messages, msg)
	}

	return messages, nil
}

// ProcessDetail represents a process detail event
type ProcessDetail struct {
	ID             string    `json:"id"`
	MessageID      string    `json:"messageId"`
	ConversationID string    `json:"conversationId"`
	EventType      string    `json:"eventType"` // iteration, thinking, tool_calls_detected, tool_call, tool_result, progress, error
	Message        string    `json:"message"`
	Data           string    `json:"data"` // JSON format data
	CreatedAt      time.Time `json:"createdAt"`
}

// AddProcessDetail adds a process detail event
func (db *DB) AddProcessDetail(messageID, conversationID, eventType, message string, data interface{}) error {
	id := uuid.New().String()

	var dataJSON string
	if data != nil {
		jsonData, err := json.Marshal(data)
		if err != nil {
			db.logger.Warn("failed to serialize process detail data", zap.Error(err))
		} else {
			dataJSON = string(jsonData)
		}
	}

	_, err := db.Exec(
		"INSERT INTO process_details (id, message_id, conversation_id, event_type, message, data, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		id, messageID, conversationID, eventType, message, dataJSON, time.Now(),
	)
	if err != nil {
		return fmt.Errorf("failed to add process detail: %w", err)
	}

	return nil
}

// GetProcessDetails gets process details for a message
func (db *DB) GetProcessDetails(messageID string) ([]ProcessDetail, error) {
	rows, err := db.Query(
		"SELECT id, message_id, conversation_id, event_type, message, data, created_at FROM process_details WHERE message_id = ? ORDER BY created_at ASC",
		messageID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query process details: %w", err)
	}
	defer rows.Close()

	var details []ProcessDetail
	for rows.Next() {
		var detail ProcessDetail
		var createdAt string

		if err := rows.Scan(&detail.ID, &detail.MessageID, &detail.ConversationID, &detail.EventType, &detail.Message, &detail.Data, &createdAt); err != nil {
			return nil, fmt.Errorf("failed to scan process detail: %w", err)
		}

		// try multiple time format parsings
		var err error
		detail.CreatedAt, err = time.Parse("2006-01-02 15:04:05.999999999-07:00", createdAt)
		if err != nil {
			detail.CreatedAt, err = time.Parse("2006-01-02 15:04:05", createdAt)
		}
		if err != nil {
			detail.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		}

		details = append(details, detail)
	}

	return details, nil
}

// GetProcessDetailsByConversation gets all process details for a conversation (grouped by message)
func (db *DB) GetProcessDetailsByConversation(conversationID string) (map[string][]ProcessDetail, error) {
	rows, err := db.Query(
		"SELECT id, message_id, conversation_id, event_type, message, data, created_at FROM process_details WHERE conversation_id = ? ORDER BY created_at ASC",
		conversationID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query process details: %w", err)
	}
	defer rows.Close()

	detailsMap := make(map[string][]ProcessDetail)
	for rows.Next() {
		var detail ProcessDetail
		var createdAt string

		if err := rows.Scan(&detail.ID, &detail.MessageID, &detail.ConversationID, &detail.EventType, &detail.Message, &detail.Data, &createdAt); err != nil {
			return nil, fmt.Errorf("failed to scan process detail: %w", err)
		}

		// try multiple time format parsings
		var err error
		detail.CreatedAt, err = time.Parse("2006-01-02 15:04:05.999999999-07:00", createdAt)
		if err != nil {
			detail.CreatedAt, err = time.Parse("2006-01-02 15:04:05", createdAt)
		}
		if err != nil {
			detail.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		}

		detailsMap[detail.MessageID] = append(detailsMap[detail.MessageID], detail)
	}

	return detailsMap, nil
}

// GetRecentProcessDetailsByConversation gets recent process detail events for a conversation (newest first).
func (db *DB) GetRecentProcessDetailsByConversation(conversationID string, limit int) ([]ProcessDetail, error) {
	if limit <= 0 {
		limit = 50
	}

	rows, err := db.Query(
		"SELECT id, message_id, conversation_id, event_type, message, data, created_at FROM process_details WHERE conversation_id = ? ORDER BY created_at DESC LIMIT ?",
		conversationID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query recent process details: %w", err)
	}
	defer rows.Close()

	details := make([]ProcessDetail, 0, limit)
	for rows.Next() {
		var detail ProcessDetail
		var createdAt string

		if err := rows.Scan(&detail.ID, &detail.MessageID, &detail.ConversationID, &detail.EventType, &detail.Message, &detail.Data, &createdAt); err != nil {
			return nil, fmt.Errorf("failed to scan process detail: %w", err)
		}

		// try multiple time format parsings
		var parseErr error
		detail.CreatedAt, parseErr = time.Parse("2006-01-02 15:04:05.999999999-07:00", createdAt)
		if parseErr != nil {
			detail.CreatedAt, parseErr = time.Parse("2006-01-02 15:04:05", createdAt)
		}
		if parseErr != nil {
			detail.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		}

		details = append(details, detail)
	}

	return details, nil
}
