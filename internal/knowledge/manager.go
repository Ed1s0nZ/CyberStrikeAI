package knowledge

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Manager 知识库管理器
type Manager struct {
	db       *sql.DB
	basePath string
	logger   *zap.Logger
}

// NewManager 创建新的知识库管理器
func NewManager(db *sql.DB, basePath string, logger *zap.Logger) *Manager {
	return &Manager{
		db:       db,
		basePath: basePath,
		logger:   logger,
	}
}

// ScanKnowledgeBase 扫描知识库目录，更新数据库
// 返回需要索引的知识项ID列表（新添加的或更新的）
func (m *Manager) ScanKnowledgeBase() ([]string, error) {
	if m.basePath == "" {
		return nil, fmt.Errorf("知识库路径未配置")
	}

	// 确保目录存在
	if err := os.MkdirAll(m.basePath, 0755); err != nil {
		return nil, fmt.Errorf("创建知识库目录失败: %w", err)
	}

	var itemsToIndex []string

	// 遍历知识库目录
	err := filepath.WalkDir(m.basePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// 跳过目录和非markdown文件
		if d.IsDir() || !strings.HasSuffix(strings.ToLower(path), ".md") {
			return nil
		}

		// 计算相对路径和分类
		relPath, err := filepath.Rel(m.basePath, path)
		if err != nil {
			return err
		}

		// 第一个目录名作为分类（风险类型）
		parts := strings.Split(relPath, string(filepath.Separator))
		category := "未分类"
		if len(parts) > 1 {
			category = parts[0]
		}

		// 文件名为标题
		title := strings.TrimSuffix(filepath.Base(path), ".md")

		// 读取文件内容
		content, err := os.ReadFile(path)
		if err != nil {
			m.logger.Warn("读取知识库文件失败", zap.String("path", path), zap.Error(err))
			return nil // 继续处理其他文件
		}

		// 检查是否已存在
		var existingID string
		var existingContent string
		var existingUpdatedAt time.Time
		err = m.db.QueryRow(
			"SELECT id, content, updated_at FROM knowledge_base_items WHERE file_path = ?",
			path,
		).Scan(&existingID, &existingContent, &existingUpdatedAt)

		if err == sql.ErrNoRows {
			// 创建新项
			id := uuid.New().String()
			now := time.Now()
			_, err = m.db.Exec(
				"INSERT INTO knowledge_base_items (id, category, title, file_path, content, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
				id, category, title, path, string(content), now, now,
			)
			if err != nil {
				return fmt.Errorf("插入知识项失败: %w", err)
			}
			m.logger.Info("添加知识项", zap.String("id", id), zap.String("title", title), zap.String("category", category))
			// 新添加的项需要索引
			itemsToIndex = append(itemsToIndex, id)
		} else if err == nil {
			// 检查内容是否有变化
			contentChanged := existingContent != string(content)
			if contentChanged {
				// 更新现有项
				_, err = m.db.Exec(
					"UPDATE knowledge_base_items SET category = ?, title = ?, content = ?, updated_at = ? WHERE id = ?",
					category, title, string(content), time.Now(), existingID,
				)
				if err != nil {
					return fmt.Errorf("更新知识项失败: %w", err)
				}
				m.logger.Info("更新知识项", zap.String("id", existingID), zap.String("title", title))
				// 内容已更新的项需要重新索引
				itemsToIndex = append(itemsToIndex, existingID)
			} else {
				m.logger.Debug("知识项未变化，跳过", zap.String("id", existingID), zap.String("title", title))
			}
		} else {
			return fmt.Errorf("查询知识项失败: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return itemsToIndex, nil
}

// GetCategories 获取所有分类（风险类型）
func (m *Manager) GetCategories() ([]string, error) {
	rows, err := m.db.Query("SELECT DISTINCT category FROM knowledge_base_items ORDER BY category")
	if err != nil {
		return nil, fmt.Errorf("查询分类失败: %w", err)
	}
	defer rows.Close()

	var categories []string
	for rows.Next() {
		var category string
		if err := rows.Scan(&category); err != nil {
			return nil, fmt.Errorf("扫描分类失败: %w", err)
		}
		categories = append(categories, category)
	}

	return categories, nil
}

// GetItems 获取知识项列表
func (m *Manager) GetItems(category string) ([]*KnowledgeItem, error) {
	var rows *sql.Rows
	var err error

	if category != "" {
		rows, err = m.db.Query(
			"SELECT id, category, title, file_path, content, created_at, updated_at FROM knowledge_base_items WHERE category = ? ORDER BY title",
			category,
		)
	} else {
		rows, err = m.db.Query(
			"SELECT id, category, title, file_path, content, created_at, updated_at FROM knowledge_base_items ORDER BY category, title",
		)
	}
	if err != nil {
		return nil, fmt.Errorf("查询知识项失败: %w", err)
	}
	defer rows.Close()

	var items []*KnowledgeItem
	for rows.Next() {
		item := &KnowledgeItem{}
		var createdAt, updatedAt string
		if err := rows.Scan(&item.ID, &item.Category, &item.Title, &item.FilePath, &item.Content, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("扫描知识项失败: %w", err)
		}

		// 解析时间 - 支持多种格式
		timeFormats := []string{
			"2006-01-02 15:04:05.999999999-07:00",
			"2006-01-02 15:04:05.999999999",
			"2006-01-02T15:04:05.999999999Z07:00",
			"2006-01-02T15:04:05Z",
			"2006-01-02 15:04:05",
			time.RFC3339,
			time.RFC3339Nano,
		}

		// 解析创建时间
		if createdAt != "" {
			for _, format := range timeFormats {
				parsed, err := time.Parse(format, createdAt)
				if err == nil && !parsed.IsZero() {
					item.CreatedAt = parsed
					break
				}
			}
		}

		// 解析更新时间
		if updatedAt != "" {
			for _, format := range timeFormats {
				parsed, err := time.Parse(format, updatedAt)
				if err == nil && !parsed.IsZero() {
					item.UpdatedAt = parsed
					break
				}
			}
		}

		// 如果更新时间为空，使用创建时间
		if item.UpdatedAt.IsZero() && !item.CreatedAt.IsZero() {
			item.UpdatedAt = item.CreatedAt
		}

		items = append(items, item)
	}

	return items, nil
}

// GetItem 获取单个知识项
func (m *Manager) GetItem(id string) (*KnowledgeItem, error) {
	item := &KnowledgeItem{}
	var createdAt, updatedAt string
	err := m.db.QueryRow(
		"SELECT id, category, title, file_path, content, created_at, updated_at FROM knowledge_base_items WHERE id = ?",
		id,
	).Scan(&item.ID, &item.Category, &item.Title, &item.FilePath, &item.Content, &createdAt, &updatedAt)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("知识项不存在")
	}
	if err != nil {
		return nil, fmt.Errorf("查询知识项失败: %w", err)
	}

	// 解析时间 - 支持多种格式
	timeFormats := []string{
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02T15:04:05.999999999Z07:00",
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
		time.RFC3339,
		time.RFC3339Nano,
	}

	// 解析创建时间
	if createdAt != "" {
		for _, format := range timeFormats {
			parsed, err := time.Parse(format, createdAt)
			if err == nil && !parsed.IsZero() {
				item.CreatedAt = parsed
				break
			}
		}
	}

	// 解析更新时间
	if updatedAt != "" {
		for _, format := range timeFormats {
			parsed, err := time.Parse(format, updatedAt)
			if err == nil && !parsed.IsZero() {
				item.UpdatedAt = parsed
				break
			}
		}
	}

	// 如果更新时间为空，使用创建时间
	if item.UpdatedAt.IsZero() && !item.CreatedAt.IsZero() {
		item.UpdatedAt = item.CreatedAt
	}

	return item, nil
}

// CreateItem 创建知识项
func (m *Manager) CreateItem(category, title, content string) (*KnowledgeItem, error) {
	id := uuid.New().String()
	now := time.Now()

	// 构建文件路径
	filePath := filepath.Join(m.basePath, category, title+".md")

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return nil, fmt.Errorf("创建目录失败: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("写入文件失败: %w", err)
	}

	// 插入数据库
	_, err := m.db.Exec(
		"INSERT INTO knowledge_base_items (id, category, title, file_path, content, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		id, category, title, filePath, content, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("插入知识项失败: %w", err)
	}

	return &KnowledgeItem{
		ID:        id,
		Category:  category,
		Title:     title,
		FilePath:  filePath,
		Content:   content,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// UpdateItem 更新知识项
func (m *Manager) UpdateItem(id, category, title, content string) (*KnowledgeItem, error) {
	// 获取现有项
	item, err := m.GetItem(id)
	if err != nil {
		return nil, err
	}

	// 构建新文件路径
	newFilePath := filepath.Join(m.basePath, category, title+".md")

	// 如果路径改变，需要移动文件
	if item.FilePath != newFilePath {
		// 确保新目录存在
		if err := os.MkdirAll(filepath.Dir(newFilePath), 0755); err != nil {
			return nil, fmt.Errorf("创建目录失败: %w", err)
		}

		// 移动文件
		if err := os.Rename(item.FilePath, newFilePath); err != nil {
			return nil, fmt.Errorf("移动文件失败: %w", err)
		}

		// 删除旧目录（如果为空）
		oldDir := filepath.Dir(item.FilePath)
		if entries, err := os.ReadDir(oldDir); err == nil && len(entries) == 0 {
			os.Remove(oldDir)
		}
	}

	// 写入文件
	if err := os.WriteFile(newFilePath, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("写入文件失败: %w", err)
	}

	// 更新数据库
	_, err = m.db.Exec(
		"UPDATE knowledge_base_items SET category = ?, title = ?, file_path = ?, content = ?, updated_at = ? WHERE id = ?",
		category, title, newFilePath, content, time.Now(), id,
	)
	if err != nil {
		return nil, fmt.Errorf("更新知识项失败: %w", err)
	}

	// 删除旧的向量嵌入（需要重新索引）
	_, err = m.db.Exec("DELETE FROM knowledge_embeddings WHERE item_id = ?", id)
	if err != nil {
		m.logger.Warn("删除旧向量嵌入失败", zap.Error(err))
	}

	return m.GetItem(id)
}

// DeleteItem 删除知识项
func (m *Manager) DeleteItem(id string) error {
	// 获取文件路径
	var filePath string
	err := m.db.QueryRow("SELECT file_path FROM knowledge_base_items WHERE id = ?", id).Scan(&filePath)
	if err != nil {
		return fmt.Errorf("查询知识项失败: %w", err)
	}

	// 删除文件
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		m.logger.Warn("删除文件失败", zap.String("path", filePath), zap.Error(err))
	}

	// 删除数据库记录（级联删除向量）
	_, err = m.db.Exec("DELETE FROM knowledge_base_items WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("删除知识项失败: %w", err)
	}

	return nil
}

// LogRetrieval 记录检索日志
func (m *Manager) LogRetrieval(conversationID, messageID, query, riskType string, retrievedItems []string) error {
	id := uuid.New().String()
	itemsJSON, _ := json.Marshal(retrievedItems)

	_, err := m.db.Exec(
		"INSERT INTO knowledge_retrieval_logs (id, conversation_id, message_id, query, risk_type, retrieved_items, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		id, conversationID, messageID, query, riskType, string(itemsJSON), time.Now(),
	)
	return err
}

// GetIndexStatus 获取索引状态
func (m *Manager) GetIndexStatus() (map[string]interface{}, error) {
	// 获取总知识项数
	var totalItems int
	err := m.db.QueryRow("SELECT COUNT(*) FROM knowledge_base_items").Scan(&totalItems)
	if err != nil {
		return nil, fmt.Errorf("查询总知识项数失败: %w", err)
	}

	// 获取已索引的知识项数（有向量嵌入的）
	var indexedItems int
	err = m.db.QueryRow(`
		SELECT COUNT(DISTINCT item_id) 
		FROM knowledge_embeddings
	`).Scan(&indexedItems)
	if err != nil {
		return nil, fmt.Errorf("查询已索引项数失败: %w", err)
	}

	// 计算进度百分比
	var progressPercent float64
	if totalItems > 0 {
		progressPercent = float64(indexedItems) / float64(totalItems) * 100
	} else {
		progressPercent = 100.0
	}

	// 判断是否完成
	isComplete := indexedItems >= totalItems && totalItems > 0

	return map[string]interface{}{
		"total_items":      totalItems,
		"indexed_items":    indexedItems,
		"progress_percent": progressPercent,
		"is_complete":      isComplete,
	}, nil
}

// GetRetrievalLogs 获取检索日志
func (m *Manager) GetRetrievalLogs(conversationID, messageID string, limit int) ([]*RetrievalLog, error) {
	var rows *sql.Rows
	var err error

	if messageID != "" {
		rows, err = m.db.Query(
			"SELECT id, conversation_id, message_id, query, risk_type, retrieved_items, created_at FROM knowledge_retrieval_logs WHERE message_id = ? ORDER BY created_at DESC LIMIT ?",
			messageID, limit,
		)
	} else if conversationID != "" {
		rows, err = m.db.Query(
			"SELECT id, conversation_id, message_id, query, risk_type, retrieved_items, created_at FROM knowledge_retrieval_logs WHERE conversation_id = ? ORDER BY created_at DESC LIMIT ?",
			conversationID, limit,
		)
	} else {
		rows, err = m.db.Query(
			"SELECT id, conversation_id, message_id, query, risk_type, retrieved_items, created_at FROM knowledge_retrieval_logs ORDER BY created_at DESC LIMIT ?",
			limit,
		)
	}

	if err != nil {
		return nil, fmt.Errorf("查询检索日志失败: %w", err)
	}
	defer rows.Close()

	var logs []*RetrievalLog
	for rows.Next() {
		log := &RetrievalLog{}
		var createdAt string
		var itemsJSON sql.NullString
		if err := rows.Scan(&log.ID, &log.ConversationID, &log.MessageID, &log.Query, &log.RiskType, &itemsJSON, &createdAt); err != nil {
			return nil, fmt.Errorf("扫描检索日志失败: %w", err)
		}

		// 解析时间 - 支持多种格式
		var err error
		timeFormats := []string{
			"2006-01-02 15:04:05.999999999-07:00",
			"2006-01-02 15:04:05.999999999",
			"2006-01-02T15:04:05.999999999Z07:00",
			"2006-01-02T15:04:05Z",
			"2006-01-02 15:04:05",
			time.RFC3339,
			time.RFC3339Nano,
		}

		for _, format := range timeFormats {
			log.CreatedAt, err = time.Parse(format, createdAt)
			if err == nil && !log.CreatedAt.IsZero() {
				break
			}
		}

		// 如果所有格式都失败，记录警告但继续处理
		if log.CreatedAt.IsZero() {
			m.logger.Warn("解析检索日志时间失败",
				zap.String("timeStr", createdAt),
				zap.Error(err),
			)
			// 使用当前时间作为fallback
			log.CreatedAt = time.Now()
		}

		// 解析检索项
		if itemsJSON.Valid {
			json.Unmarshal([]byte(itemsJSON.String), &log.RetrievedItems)
		}

		logs = append(logs, log)
	}

	return logs, nil
}

// DeleteRetrievalLog 删除检索日志
func (m *Manager) DeleteRetrievalLog(id string) error {
	result, err := m.db.Exec("DELETE FROM knowledge_retrieval_logs WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("删除检索日志失败: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("获取删除行数失败: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("检索日志不存在")
	}

	return nil
}
