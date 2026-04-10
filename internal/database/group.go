package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ConversationGroup 对话分组
type ConversationGroup struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Icon      string    `json:"icon"`
	Pinned    bool      `json:"pinned"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// GroupExistsByName 检查分组名称是否已存在
func (db *DB) GroupExistsByName(name string, excludeID string) (bool, error) {
	var count int
	var err error

	if excludeID != "" {
		err = db.QueryRow(
			"SELECT COUNT(*) FROM conversation_groups WHERE name = ? AND id != ?",
			name, excludeID,
		).Scan(&count)
	} else {
		err = db.QueryRow(
			"SELECT COUNT(*) FROM conversation_groups WHERE name = ?",
			name,
		).Scan(&count)
	}

	if err != nil {
		return false, fmt.Errorf("检查分组名称失败: %w", err)
	}

	return count > 0, nil
}

// CreateGroup 创建分组
func (db *DB) CreateGroup(name, icon string) (*ConversationGroup, error) {
	// 检查名称是否已存在
	exists, err := db.GroupExistsByName(name, "")
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, fmt.Errorf("分组名称已存在")
	}

	id := uuid.New().String()
	now := time.Now()

	if icon == "" {
		icon = "📁"
	}

	_, err = db.Exec(
		"INSERT INTO conversation_groups (id, name, icon, pinned, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
		id, name, icon, 0, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("创建分组失败: %w", err)
	}

	return &ConversationGroup{
		ID:        id,
		Name:      name,
		Icon:      icon,
		Pinned:    false,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// ListGroups 列出所有分组
func (db *DB) ListGroups() ([]*ConversationGroup, error) {
	rows, err := db.Query(
		"SELECT id, name, icon, COALESCE(pinned, 0), created_at, updated_at FROM conversation_groups ORDER BY COALESCE(pinned, 0) DESC, created_at ASC",
	)
	if err != nil {
		return nil, fmt.Errorf("查询分组列表失败: %w", err)
	}
	defer rows.Close()

	var groups []*ConversationGroup
	for rows.Next() {
		var group ConversationGroup
		var createdAt, updatedAt string
		var pinned int

		if err := rows.Scan(&group.ID, &group.Name, &group.Icon, &pinned, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("扫描分组失败: %w", err)
		}

		group.Pinned = pinned != 0

		// 尝试多种时间格式解析
		var err1, err2 error
		group.CreatedAt, err1 = time.Parse("2006-01-02 15:04:05.999999999-07:00", createdAt)
		if err1 != nil {
			group.CreatedAt, err1 = time.Parse("2006-01-02 15:04:05", createdAt)
		}
		if err1 != nil {
			group.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		}

		group.UpdatedAt, err2 = time.Parse("2006-01-02 15:04:05.999999999-07:00", updatedAt)
		if err2 != nil {
			group.UpdatedAt, err2 = time.Parse("2006-01-02 15:04:05", updatedAt)
		}
		if err2 != nil {
			group.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		}

		groups = append(groups, &group)
	}

	return groups, nil
}

// GetGroup 获取分组
func (db *DB) GetGroup(id string) (*ConversationGroup, error) {
	var group ConversationGroup
	var createdAt, updatedAt string
	var pinned int

	err := db.QueryRow(
		"SELECT id, name, icon, COALESCE(pinned, 0), created_at, updated_at FROM conversation_groups WHERE id = ?",
		id,
	).Scan(&group.ID, &group.Name, &group.Icon, &pinned, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("分组不存在")
		}
		return nil, fmt.Errorf("查询分组失败: %w", err)
	}

	// 尝试多种时间格式解析
	var err1, err2 error
	group.CreatedAt, err1 = time.Parse("2006-01-02 15:04:05.999999999-07:00", createdAt)
	if err1 != nil {
		group.CreatedAt, err1 = time.Parse("2006-01-02 15:04:05", createdAt)
	}
	if err1 != nil {
		group.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	}

	group.UpdatedAt, err2 = time.Parse("2006-01-02 15:04:05.999999999-07:00", updatedAt)
	if err2 != nil {
		group.UpdatedAt, err2 = time.Parse("2006-01-02 15:04:05", updatedAt)
	}
	if err2 != nil {
		group.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	}

	group.Pinned = pinned != 0

	return &group, nil
}

// UpdateGroup 更新分组
func (db *DB) UpdateGroup(id, name, icon string) error {
	// 检查名称是否已存在（排除当前分组）
	exists, err := db.GroupExistsByName(name, id)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("分组名称已存在")
	}

	_, err = db.Exec(
		"UPDATE conversation_groups SET name = ?, icon = ?, updated_at = ? WHERE id = ?",
		name, icon, time.Now(), id,
	)
	if err != nil {
		return fmt.Errorf("更新分组失败: %w", err)
	}
	return nil
}

// DeleteGroup 删除分组
func (db *DB) DeleteGroup(id string) error {
	_, err := db.Exec("DELETE FROM conversation_groups WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("删除分组失败: %w", err)
	}
	return nil
}

// AddConversationToGroup 将对话添加到分组
// 注意：一个对话只能属于一个分组，所以在添加新分组之前，会先删除该对话的所有旧分组关联
func (db *DB) AddConversationToGroup(conversationID, groupID string) error {
	// 先删除该对话的所有旧分组关联，确保一个对话只属于一个分组
	_, err := db.Exec(
		"DELETE FROM conversation_group_mappings WHERE conversation_id = ?",
		conversationID,
	)
	if err != nil {
		return fmt.Errorf("删除对话旧分组关联失败: %w", err)
	}

	// 然后插入新的分组关联
	id := uuid.New().String()
	_, err = db.Exec(
		"INSERT INTO conversation_group_mappings (id, conversation_id, group_id, created_at) VALUES (?, ?, ?, ?)",
		id, conversationID, groupID, time.Now(),
	)
	if err != nil {
		return fmt.Errorf("添加对话到分组失败: %w", err)
	}
	return nil
}

// RemoveConversationFromGroup 从分组中移除对话
func (db *DB) RemoveConversationFromGroup(conversationID, groupID string) error {
	_, err := db.Exec(
		"DELETE FROM conversation_group_mappings WHERE conversation_id = ? AND group_id = ?",
		conversationID, groupID,
	)
	if err != nil {
		return fmt.Errorf("从分组中移除对话失败: %w", err)
	}
	return nil
}

// GetConversationsByGroup 获取分组中的所有对话
func (db *DB) GetConversationsByGroup(groupID string) ([]*Conversation, error) {
	rows, err := db.Query(
		`SELECT c.id, c.title, COALESCE(c.pinned, 0), c.created_at, c.updated_at, COALESCE(cgm.pinned, 0) as group_pinned
		 FROM conversations c
		 INNER JOIN conversation_group_mappings cgm ON c.id = cgm.conversation_id
		 WHERE cgm.group_id = ?
		 ORDER BY COALESCE(cgm.pinned, 0) DESC, c.updated_at DESC`,
		groupID,
	)
	if err != nil {
		return nil, fmt.Errorf("查询分组对话失败: %w", err)
	}
	defer rows.Close()

	var conversations []*Conversation
	for rows.Next() {
		var conv Conversation
		var createdAt, updatedAt string
		var pinned int
		var groupPinned int

		if err := rows.Scan(&conv.ID, &conv.Title, &pinned, &createdAt, &updatedAt, &groupPinned); err != nil {
			return nil, fmt.Errorf("扫描对话失败: %w", err)
		}

		// 尝试多种时间格式解析
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

// SearchConversationsByGroup 搜索分组中的对话（按标题和消息内容模糊匹配）
func (db *DB) SearchConversationsByGroup(groupID string, searchQuery string) ([]*Conversation, error) {
	// 构建SQL查询，支持按标题和消息内容搜索
	// 使用 DISTINCT 避免因为一个对话有多条匹配消息而重复
	query := `SELECT DISTINCT c.id, c.title, COALESCE(c.pinned, 0), c.created_at, c.updated_at, COALESCE(cgm.pinned, 0) as group_pinned
		 FROM conversations c
		 INNER JOIN conversation_group_mappings cgm ON c.id = cgm.conversation_id
		 WHERE cgm.group_id = ?`

	args := []interface{}{groupID}

	// 如果有搜索关键词，添加标题和消息内容搜索条件
	if searchQuery != "" {
		searchPattern := "%" + searchQuery + "%"
		// 搜索标题或消息内容
		// 使用 LEFT JOIN 连接消息表，这样即使没有消息的对话也能被搜索到（通过标题）
		query += ` AND (
			LOWER(c.title) LIKE LOWER(?)
			OR EXISTS (
				SELECT 1 FROM messages m 
				WHERE m.conversation_id = c.id 
				AND LOWER(m.content) LIKE LOWER(?)
			)
		)`
		args = append(args, searchPattern, searchPattern)
	}

	query += " ORDER BY COALESCE(cgm.pinned, 0) DESC, c.updated_at DESC"

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("搜索分组对话失败: %w", err)
	}
	defer rows.Close()

	var conversations []*Conversation
	for rows.Next() {
		var conv Conversation
		var createdAt, updatedAt string
		var pinned int
		var groupPinned int

		if err := rows.Scan(&conv.ID, &conv.Title, &pinned, &createdAt, &updatedAt, &groupPinned); err != nil {
			return nil, fmt.Errorf("扫描对话失败: %w", err)
		}

		// 尝试多种时间格式解析
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

// GetGroupByConversation 获取对话所属的分组
func (db *DB) GetGroupByConversation(conversationID string) (string, error) {
	var groupID string
	err := db.QueryRow(
		"SELECT group_id FROM conversation_group_mappings WHERE conversation_id = ? LIMIT 1",
		conversationID,
	).Scan(&groupID)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil // 没有分组
		}
		return "", fmt.Errorf("查询对话分组失败: %w", err)
	}
	return groupID, nil
}

// UpdateConversationPinned 更新对话置顶状态
func (db *DB) UpdateConversationPinned(id string, pinned bool) error {
	pinnedValue := 0
	if pinned {
		pinnedValue = 1
	}
	// 注意：不更新 updated_at，因为置顶操作不应该改变对话的更新时间
	_, err := db.Exec(
		"UPDATE conversations SET pinned = ? WHERE id = ?",
		pinnedValue, id,
	)
	if err != nil {
		return fmt.Errorf("更新对话置顶状态失败: %w", err)
	}
	return nil
}

// UpdateGroupPinned 更新分组置顶状态
func (db *DB) UpdateGroupPinned(id string, pinned bool) error {
	pinnedValue := 0
	if pinned {
		pinnedValue = 1
	}
	_, err := db.Exec(
		"UPDATE conversation_groups SET pinned = ?, updated_at = ? WHERE id = ?",
		pinnedValue, time.Now(), id,
	)
	if err != nil {
		return fmt.Errorf("更新分组置顶状态失败: %w", err)
	}
	return nil
}

// GroupMapping 分组映射关系
type GroupMapping struct {
	ConversationID string `json:"conversationId"`
	GroupID        string `json:"groupId"`
}

// GetAllGroupMappings 批量获取所有分组映射（消除 N+1 查询）
func (db *DB) GetAllGroupMappings() ([]GroupMapping, error) {
	rows, err := db.Query("SELECT conversation_id, group_id FROM conversation_group_mappings")
	if err != nil {
		return nil, fmt.Errorf("查询分组映射失败: %w", err)
	}
	defer rows.Close()

	var mappings []GroupMapping
	for rows.Next() {
		var m GroupMapping
		if err := rows.Scan(&m.ConversationID, &m.GroupID); err != nil {
			return nil, fmt.Errorf("扫描分组映射失败: %w", err)
		}
		mappings = append(mappings, m)
	}

	if mappings == nil {
		mappings = []GroupMapping{}
	}
	return mappings, nil
}

// UpdateConversationPinnedInGroup 更新对话在分组中的置顶状态
func (db *DB) UpdateConversationPinnedInGroup(conversationID, groupID string, pinned bool) error {
	pinnedValue := 0
	if pinned {
		pinnedValue = 1
	}
	_, err := db.Exec(
		"UPDATE conversation_group_mappings SET pinned = ? WHERE conversation_id = ? AND group_id = ?",
		pinnedValue, conversationID, groupID,
	)
	if err != nil {
		return fmt.Errorf("更新分组对话置顶状态失败: %w", err)
	}
	return nil
}
