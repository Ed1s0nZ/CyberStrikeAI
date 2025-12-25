package database

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"
)

// DB 数据库连接
type DB struct {
	*sql.DB
	logger *zap.Logger
}

// NewDB 创建数据库连接
func NewDB(dbPath string, logger *zap.Logger) (*DB, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_foreign_keys=1")
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}

	database := &DB{
		DB:     db,
		logger: logger,
	}

	// 初始化表
	if err := database.initTables(); err != nil {
		return nil, fmt.Errorf("初始化表失败: %w", err)
	}

	return database, nil
}

// initTables 初始化数据库表
func (db *DB) initTables() error {
	// 创建对话表
	createConversationsTable := `
	CREATE TABLE IF NOT EXISTS conversations (
		id TEXT PRIMARY KEY,
		title TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		last_react_input TEXT,
		last_react_output TEXT
	);`

	// 创建消息表
	createMessagesTable := `
	CREATE TABLE IF NOT EXISTS messages (
		id TEXT PRIMARY KEY,
		conversation_id TEXT NOT NULL,
		role TEXT NOT NULL,
		content TEXT NOT NULL,
		mcp_execution_ids TEXT,
		created_at DATETIME NOT NULL,
		FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE
	);`

	// 创建过程详情表
	createProcessDetailsTable := `
	CREATE TABLE IF NOT EXISTS process_details (
		id TEXT PRIMARY KEY,
		message_id TEXT NOT NULL,
		conversation_id TEXT NOT NULL,
		event_type TEXT NOT NULL,
		message TEXT,
		data TEXT,
		created_at DATETIME NOT NULL,
		FOREIGN KEY (message_id) REFERENCES messages(id) ON DELETE CASCADE,
		FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE
	);`

	// 创建工具执行记录表
	createToolExecutionsTable := `
	CREATE TABLE IF NOT EXISTS tool_executions (
		id TEXT PRIMARY KEY,
		tool_name TEXT NOT NULL,
		arguments TEXT NOT NULL,
		status TEXT NOT NULL,
		result TEXT,
		error TEXT,
		start_time DATETIME NOT NULL,
		end_time DATETIME,
		duration_ms INTEGER,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);`

	// 创建工具统计表
	createToolStatsTable := `
	CREATE TABLE IF NOT EXISTS tool_stats (
		tool_name TEXT PRIMARY KEY,
		total_calls INTEGER NOT NULL DEFAULT 0,
		success_calls INTEGER NOT NULL DEFAULT 0,
		failed_calls INTEGER NOT NULL DEFAULT 0,
		last_call_time DATETIME,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);`

	// 创建攻击链节点表
	createAttackChainNodesTable := `
	CREATE TABLE IF NOT EXISTS attack_chain_nodes (
		id TEXT PRIMARY KEY,
		conversation_id TEXT NOT NULL,
		node_type TEXT NOT NULL,
		node_name TEXT NOT NULL,
		tool_execution_id TEXT,
		metadata TEXT,
		risk_score INTEGER DEFAULT 0,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE,
		FOREIGN KEY (tool_execution_id) REFERENCES tool_executions(id) ON DELETE SET NULL
	);`

	// 创建攻击链边表
	createAttackChainEdgesTable := `
	CREATE TABLE IF NOT EXISTS attack_chain_edges (
		id TEXT PRIMARY KEY,
		conversation_id TEXT NOT NULL,
		source_node_id TEXT NOT NULL,
		target_node_id TEXT NOT NULL,
		edge_type TEXT NOT NULL,
		weight INTEGER DEFAULT 1,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE,
		FOREIGN KEY (source_node_id) REFERENCES attack_chain_nodes(id) ON DELETE CASCADE,
		FOREIGN KEY (target_node_id) REFERENCES attack_chain_nodes(id) ON DELETE CASCADE
	);`

	// 创建知识检索日志表（保留在会话数据库中，因为有外键关联）
	createKnowledgeRetrievalLogsTable := `
	CREATE TABLE IF NOT EXISTS knowledge_retrieval_logs (
		id TEXT PRIMARY KEY,
		conversation_id TEXT,
		message_id TEXT,
		query TEXT NOT NULL,
		risk_type TEXT,
		retrieved_items TEXT,
		created_at DATETIME NOT NULL,
		FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE SET NULL,
		FOREIGN KEY (message_id) REFERENCES messages(id) ON DELETE SET NULL
	);`

	// 创建对话分组表
	createConversationGroupsTable := `
	CREATE TABLE IF NOT EXISTS conversation_groups (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		icon TEXT,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	);`

	// 创建对话分组映射表
	createConversationGroupMappingsTable := `
	CREATE TABLE IF NOT EXISTS conversation_group_mappings (
		id TEXT PRIMARY KEY,
		conversation_id TEXT NOT NULL,
		group_id TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE,
		FOREIGN KEY (group_id) REFERENCES conversation_groups(id) ON DELETE CASCADE,
		UNIQUE(conversation_id, group_id)
	);`

	// 创建漏洞表
	createVulnerabilitiesTable := `
	CREATE TABLE IF NOT EXISTS vulnerabilities (
		id TEXT PRIMARY KEY,
		conversation_id TEXT NOT NULL,
		title TEXT NOT NULL,
		description TEXT,
		severity TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'open',
		vulnerability_type TEXT,
		target TEXT,
		proof TEXT,
		impact TEXT,
		recommendation TEXT,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE
	);`

	// 创建索引
	createIndexes := `
	CREATE INDEX IF NOT EXISTS idx_messages_conversation_id ON messages(conversation_id);
	CREATE INDEX IF NOT EXISTS idx_conversations_updated_at ON conversations(updated_at);
	CREATE INDEX IF NOT EXISTS idx_process_details_message_id ON process_details(message_id);
	CREATE INDEX IF NOT EXISTS idx_process_details_conversation_id ON process_details(conversation_id);
	CREATE INDEX IF NOT EXISTS idx_tool_executions_tool_name ON tool_executions(tool_name);
	CREATE INDEX IF NOT EXISTS idx_tool_executions_start_time ON tool_executions(start_time);
	CREATE INDEX IF NOT EXISTS idx_tool_executions_status ON tool_executions(status);
	CREATE INDEX IF NOT EXISTS idx_chain_nodes_conversation ON attack_chain_nodes(conversation_id);
	CREATE INDEX IF NOT EXISTS idx_chain_edges_conversation ON attack_chain_edges(conversation_id);
	CREATE INDEX IF NOT EXISTS idx_chain_edges_source ON attack_chain_edges(source_node_id);
	CREATE INDEX IF NOT EXISTS idx_chain_edges_target ON attack_chain_edges(target_node_id);
	CREATE INDEX IF NOT EXISTS idx_knowledge_retrieval_logs_conversation ON knowledge_retrieval_logs(conversation_id);
	CREATE INDEX IF NOT EXISTS idx_knowledge_retrieval_logs_message ON knowledge_retrieval_logs(message_id);
	CREATE INDEX IF NOT EXISTS idx_knowledge_retrieval_logs_created_at ON knowledge_retrieval_logs(created_at);
	CREATE INDEX IF NOT EXISTS idx_conversation_group_mappings_conversation ON conversation_group_mappings(conversation_id);
	CREATE INDEX IF NOT EXISTS idx_conversation_group_mappings_group ON conversation_group_mappings(group_id);
	CREATE INDEX IF NOT EXISTS idx_conversations_pinned ON conversations(pinned);
	CREATE INDEX IF NOT EXISTS idx_vulnerabilities_conversation_id ON vulnerabilities(conversation_id);
	CREATE INDEX IF NOT EXISTS idx_vulnerabilities_severity ON vulnerabilities(severity);
	CREATE INDEX IF NOT EXISTS idx_vulnerabilities_status ON vulnerabilities(status);
	CREATE INDEX IF NOT EXISTS idx_vulnerabilities_created_at ON vulnerabilities(created_at);
	`

	if _, err := db.Exec(createConversationsTable); err != nil {
		return fmt.Errorf("创建conversations表失败: %w", err)
	}

	if _, err := db.Exec(createMessagesTable); err != nil {
		return fmt.Errorf("创建messages表失败: %w", err)
	}

	if _, err := db.Exec(createProcessDetailsTable); err != nil {
		return fmt.Errorf("创建process_details表失败: %w", err)
	}

	if _, err := db.Exec(createToolExecutionsTable); err != nil {
		return fmt.Errorf("创建tool_executions表失败: %w", err)
	}

	if _, err := db.Exec(createToolStatsTable); err != nil {
		return fmt.Errorf("创建tool_stats表失败: %w", err)
	}

	if _, err := db.Exec(createAttackChainNodesTable); err != nil {
		return fmt.Errorf("创建attack_chain_nodes表失败: %w", err)
	}

	if _, err := db.Exec(createAttackChainEdgesTable); err != nil {
		return fmt.Errorf("创建attack_chain_edges表失败: %w", err)
	}

	if _, err := db.Exec(createKnowledgeRetrievalLogsTable); err != nil {
		return fmt.Errorf("创建knowledge_retrieval_logs表失败: %w", err)
	}

	if _, err := db.Exec(createConversationGroupsTable); err != nil {
		return fmt.Errorf("创建conversation_groups表失败: %w", err)
	}

	if _, err := db.Exec(createConversationGroupMappingsTable); err != nil {
		return fmt.Errorf("创建conversation_group_mappings表失败: %w", err)
	}

	if _, err := db.Exec(createVulnerabilitiesTable); err != nil {
		return fmt.Errorf("创建vulnerabilities表失败: %w", err)
	}

	// 为已有表添加新字段（如果不存在）- 必须在创建索引之前
	if err := db.migrateConversationsTable(); err != nil {
		db.logger.Warn("迁移conversations表失败", zap.Error(err))
		// 不返回错误，允许继续运行
	}

	if err := db.migrateConversationGroupsTable(); err != nil {
		db.logger.Warn("迁移conversation_groups表失败", zap.Error(err))
		// 不返回错误，允许继续运行
	}

	if err := db.migrateConversationGroupMappingsTable(); err != nil {
		db.logger.Warn("迁移conversation_group_mappings表失败", zap.Error(err))
		// 不返回错误，允许继续运行
	}

	if _, err := db.Exec(createIndexes); err != nil {
		return fmt.Errorf("创建索引失败: %w", err)
	}

	db.logger.Info("数据库表初始化完成")
	return nil
}

// migrateConversationsTable 迁移conversations表，添加新字段
func (db *DB) migrateConversationsTable() error {
	// 检查last_react_input字段是否存在
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('conversations') WHERE name='last_react_input'").Scan(&count)
	if err != nil {
		// 如果查询失败，尝试添加字段
		if _, addErr := db.Exec("ALTER TABLE conversations ADD COLUMN last_react_input TEXT"); addErr != nil {
			// 如果字段已存在，忽略错误（SQLite错误信息可能不同）
			errMsg := strings.ToLower(addErr.Error())
			if !strings.Contains(errMsg, "duplicate column") && !strings.Contains(errMsg, "already exists") {
				db.logger.Warn("添加last_react_input字段失败", zap.Error(addErr))
			}
		}
	} else if count == 0 {
		// 字段不存在，添加它
		if _, err := db.Exec("ALTER TABLE conversations ADD COLUMN last_react_input TEXT"); err != nil {
			db.logger.Warn("添加last_react_input字段失败", zap.Error(err))
		}
	}

	// 检查last_react_output字段是否存在
	err = db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('conversations') WHERE name='last_react_output'").Scan(&count)
	if err != nil {
		// 如果查询失败，尝试添加字段
		if _, addErr := db.Exec("ALTER TABLE conversations ADD COLUMN last_react_output TEXT"); addErr != nil {
			// 如果字段已存在，忽略错误
			errMsg := strings.ToLower(addErr.Error())
			if !strings.Contains(errMsg, "duplicate column") && !strings.Contains(errMsg, "already exists") {
				db.logger.Warn("添加last_react_output字段失败", zap.Error(addErr))
			}
		}
	} else if count == 0 {
		// 字段不存在，添加它
		if _, err := db.Exec("ALTER TABLE conversations ADD COLUMN last_react_output TEXT"); err != nil {
			db.logger.Warn("添加last_react_output字段失败", zap.Error(err))
		}
	}

	// 检查pinned字段是否存在
	err = db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('conversations') WHERE name='pinned'").Scan(&count)
	if err != nil {
		// 如果查询失败，尝试添加字段
		if _, addErr := db.Exec("ALTER TABLE conversations ADD COLUMN pinned INTEGER DEFAULT 0"); addErr != nil {
			// 如果字段已存在，忽略错误
			errMsg := strings.ToLower(addErr.Error())
			if !strings.Contains(errMsg, "duplicate column") && !strings.Contains(errMsg, "already exists") {
				db.logger.Warn("添加pinned字段失败", zap.Error(addErr))
			}
		}
	} else if count == 0 {
		// 字段不存在，添加它
		if _, err := db.Exec("ALTER TABLE conversations ADD COLUMN pinned INTEGER DEFAULT 0"); err != nil {
			db.logger.Warn("添加pinned字段失败", zap.Error(err))
		}
	}

	return nil
}

// migrateConversationGroupsTable 迁移conversation_groups表，添加新字段
func (db *DB) migrateConversationGroupsTable() error {
	// 检查pinned字段是否存在
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('conversation_groups') WHERE name='pinned'").Scan(&count)
	if err != nil {
		// 如果查询失败，尝试添加字段
		if _, addErr := db.Exec("ALTER TABLE conversation_groups ADD COLUMN pinned INTEGER DEFAULT 0"); addErr != nil {
			// 如果字段已存在，忽略错误
			errMsg := strings.ToLower(addErr.Error())
			if !strings.Contains(errMsg, "duplicate column") && !strings.Contains(errMsg, "already exists") {
				db.logger.Warn("添加pinned字段失败", zap.Error(addErr))
			}
		}
	} else if count == 0 {
		// 字段不存在，添加它
		if _, err := db.Exec("ALTER TABLE conversation_groups ADD COLUMN pinned INTEGER DEFAULT 0"); err != nil {
			db.logger.Warn("添加pinned字段失败", zap.Error(err))
		}
	}

	return nil
}

// migrateConversationGroupMappingsTable 迁移conversation_group_mappings表，添加新字段
func (db *DB) migrateConversationGroupMappingsTable() error {
	// 检查pinned字段是否存在
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('conversation_group_mappings') WHERE name='pinned'").Scan(&count)
	if err != nil {
		// 如果查询失败，尝试添加字段
		if _, addErr := db.Exec("ALTER TABLE conversation_group_mappings ADD COLUMN pinned INTEGER DEFAULT 0"); addErr != nil {
			// 如果字段已存在，忽略错误
			errMsg := strings.ToLower(addErr.Error())
			if !strings.Contains(errMsg, "duplicate column") && !strings.Contains(errMsg, "already exists") {
				db.logger.Warn("添加pinned字段失败", zap.Error(addErr))
			}
		}
	} else if count == 0 {
		// 字段不存在，添加它
		if _, err := db.Exec("ALTER TABLE conversation_group_mappings ADD COLUMN pinned INTEGER DEFAULT 0"); err != nil {
			db.logger.Warn("添加pinned字段失败", zap.Error(err))
		}
	}

	return nil
}

// NewKnowledgeDB 创建知识库数据库连接（只包含知识库相关的表）
func NewKnowledgeDB(dbPath string, logger *zap.Logger) (*DB, error) {
	sqlDB, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_foreign_keys=1")
	if err != nil {
		return nil, fmt.Errorf("打开知识库数据库失败: %w", err)
	}

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("连接知识库数据库失败: %w", err)
	}

	database := &DB{
		DB:     sqlDB,
		logger: logger,
	}

	// 初始化知识库表
	if err := database.initKnowledgeTables(); err != nil {
		return nil, fmt.Errorf("初始化知识库表失败: %w", err)
	}

	return database, nil
}

// initKnowledgeTables 初始化知识库数据库表（只包含知识库相关的表）
func (db *DB) initKnowledgeTables() error {
	// 创建知识库项表
	createKnowledgeBaseItemsTable := `
	CREATE TABLE IF NOT EXISTS knowledge_base_items (
		id TEXT PRIMARY KEY,
		category TEXT NOT NULL,
		title TEXT NOT NULL,
		file_path TEXT NOT NULL,
		content TEXT,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	);`

	// 创建知识库向量表
	createKnowledgeEmbeddingsTable := `
	CREATE TABLE IF NOT EXISTS knowledge_embeddings (
		id TEXT PRIMARY KEY,
		item_id TEXT NOT NULL,
		chunk_index INTEGER NOT NULL,
		chunk_text TEXT NOT NULL,
		embedding TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		FOREIGN KEY (item_id) REFERENCES knowledge_base_items(id) ON DELETE CASCADE
	);`

	// 创建知识检索日志表（在独立知识库数据库中，不使用外键约束，因为conversations和messages表可能不在这个数据库中）
	createKnowledgeRetrievalLogsTable := `
	CREATE TABLE IF NOT EXISTS knowledge_retrieval_logs (
		id TEXT PRIMARY KEY,
		conversation_id TEXT,
		message_id TEXT,
		query TEXT NOT NULL,
		risk_type TEXT,
		retrieved_items TEXT,
		created_at DATETIME NOT NULL
	);`

	// 创建索引
	createIndexes := `
	CREATE INDEX IF NOT EXISTS idx_knowledge_items_category ON knowledge_base_items(category);
	CREATE INDEX IF NOT EXISTS idx_knowledge_embeddings_item_id ON knowledge_embeddings(item_id);
	CREATE INDEX IF NOT EXISTS idx_knowledge_retrieval_logs_conversation ON knowledge_retrieval_logs(conversation_id);
	CREATE INDEX IF NOT EXISTS idx_knowledge_retrieval_logs_message ON knowledge_retrieval_logs(message_id);
	CREATE INDEX IF NOT EXISTS idx_knowledge_retrieval_logs_created_at ON knowledge_retrieval_logs(created_at);
	`

	if _, err := db.Exec(createKnowledgeBaseItemsTable); err != nil {
		return fmt.Errorf("创建knowledge_base_items表失败: %w", err)
	}

	if _, err := db.Exec(createKnowledgeEmbeddingsTable); err != nil {
		return fmt.Errorf("创建knowledge_embeddings表失败: %w", err)
	}

	if _, err := db.Exec(createKnowledgeRetrievalLogsTable); err != nil {
		return fmt.Errorf("创建knowledge_retrieval_logs表失败: %w", err)
	}

	if _, err := db.Exec(createIndexes); err != nil {
		return fmt.Errorf("创建索引失败: %w", err)
	}

	db.logger.Info("知识库数据库表初始化完成")
	return nil
}

// Close 关闭数据库连接
func (db *DB) Close() error {
	return db.DB.Close()
}
