package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"
)

// configureDBPool 设置 SQLite 连接池参数，提升并发稳定性
func configureDBPool(db *sql.DB) {
	// SQLite 同一时间只允许一个写入者，限制连接数避免 "database is locked" 错误
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
}

// DB 数据库连接
type DB struct {
	*sql.DB
	logger                   *zap.Logger
	conversationArtifactsDir string
}

// NewDB 创建数据库连接
func NewDB(dbPath string, logger *zap.Logger) (*DB, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_foreign_keys=1&_busy_timeout=5000&_synchronous=NORMAL")
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}

	configureDBPool(db)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}

	database := &DB{
		DB:     db,
		logger: logger,
	}
	// Keep conversation-scoped artifacts near database files, so cleanup can follow conversation lifecycle.
	baseDir := filepath.Join(filepath.Dir(dbPath), "conversation_artifacts")
	if mkErr := os.MkdirAll(baseDir, 0o755); mkErr == nil {
		database.conversationArtifactsDir = baseDir
	} else if logger != nil {
		logger.Warn("创建 conversation artifacts 目录失败", zap.String("dir", baseDir), zap.Error(mkErr))
	}

	// 初始化表
	if err := database.initTables(); err != nil {
		return nil, fmt.Errorf("初始化表失败: %w", err)
	}

	return database, nil
}

// initTables 初始化数据库表
func (db *DB) initTables() error {
	// 创建对话表（last_react_input / last_react_output 存「代理消息轨迹」JSON 与助手摘要，列名保留以兼容已有库）
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
		updated_at DATETIME NOT NULL,
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

	// 创建Skills统计表
	createSkillStatsTable := `
	CREATE TABLE IF NOT EXISTS skill_stats (
		skill_name TEXT PRIMARY KEY,
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
		conversation_tag TEXT,
		task_tag TEXT,
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

	// 创建批量任务队列表
	createBatchTaskQueuesTable := `
	CREATE TABLE IF NOT EXISTS batch_task_queues (
		id TEXT PRIMARY KEY,
		title TEXT,
		role TEXT,
		agent_mode TEXT NOT NULL DEFAULT 'single',
		schedule_mode TEXT NOT NULL DEFAULT 'manual',
		cron_expr TEXT,
		next_run_at DATETIME,
		schedule_enabled INTEGER NOT NULL DEFAULT 1,
		last_schedule_trigger_at DATETIME,
		last_schedule_error TEXT,
		last_run_error TEXT,
		status TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		started_at DATETIME,
		completed_at DATETIME,
		current_index INTEGER NOT NULL DEFAULT 0
	);`

	// 创建批量任务表
	createBatchTasksTable := `
	CREATE TABLE IF NOT EXISTS batch_tasks (
		id TEXT PRIMARY KEY,
		queue_id TEXT NOT NULL,
		message TEXT NOT NULL,
		conversation_id TEXT,
		status TEXT NOT NULL,
		started_at DATETIME,
		completed_at DATETIME,
		error TEXT,
		result TEXT,
		FOREIGN KEY (queue_id) REFERENCES batch_task_queues(id) ON DELETE CASCADE
	);`

	// 创建 WebShell 连接表
	createWebshellConnectionsTable := `
	CREATE TABLE IF NOT EXISTS webshell_connections (
		id TEXT PRIMARY KEY,
		url TEXT NOT NULL,
		password TEXT NOT NULL DEFAULT '',
		type TEXT NOT NULL DEFAULT 'php',
		method TEXT NOT NULL DEFAULT 'post',
		cmd_param TEXT NOT NULL DEFAULT '',
		remark TEXT NOT NULL DEFAULT '',
		encoding TEXT NOT NULL DEFAULT '',
		os TEXT NOT NULL DEFAULT '',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);`

	// 创建 WebShell 连接扩展状态表（前端工作区/终端状态持久化）
	createWebshellConnectionStatesTable := `
	CREATE TABLE IF NOT EXISTS webshell_connection_states (
		connection_id TEXT PRIMARY KEY,
		state_json TEXT NOT NULL DEFAULT '{}',
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (connection_id) REFERENCES webshell_connections(id) ON DELETE CASCADE
	);`

	// ========================================================================
	// C2 模块（监听器 / 会话 / 任务 / 文件 / 事件 / Malleable Profile）
	// ========================================================================
	createC2ListenersTable := `
	CREATE TABLE IF NOT EXISTS c2_listeners (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		type TEXT NOT NULL,
		bind_host TEXT NOT NULL DEFAULT '127.0.0.1',
		bind_port INTEGER NOT NULL,
		profile_id TEXT,
		encryption_key TEXT NOT NULL DEFAULT '',
		implant_token TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT 'stopped',
		config_json TEXT NOT NULL DEFAULT '{}',
		remark TEXT NOT NULL DEFAULT '',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		started_at DATETIME,
		last_error TEXT
	);`

	createC2SessionsTable := `
	CREATE TABLE IF NOT EXISTS c2_sessions (
		id TEXT PRIMARY KEY,
		listener_id TEXT NOT NULL,
		implant_uuid TEXT NOT NULL UNIQUE,
		hostname TEXT,
		username TEXT,
		os TEXT,
		arch TEXT,
		pid INTEGER DEFAULT 0,
		process_name TEXT,
		is_admin INTEGER DEFAULT 0,
		internal_ip TEXT,
		external_ip TEXT,
		user_agent TEXT,
		sleep_seconds INTEGER NOT NULL DEFAULT 5,
		jitter_percent INTEGER NOT NULL DEFAULT 0,
		status TEXT NOT NULL DEFAULT 'active',
		first_seen_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		last_check_in DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		metadata_json TEXT DEFAULT '{}',
		note TEXT NOT NULL DEFAULT '',
		FOREIGN KEY (listener_id) REFERENCES c2_listeners(id) ON DELETE CASCADE
	);`

	createC2TasksTable := `
	CREATE TABLE IF NOT EXISTS c2_tasks (
		id TEXT PRIMARY KEY,
		session_id TEXT NOT NULL,
		task_type TEXT NOT NULL,
		payload_json TEXT NOT NULL DEFAULT '{}',
		status TEXT NOT NULL DEFAULT 'queued',
		result_text TEXT,
		result_blob_path TEXT,
		error TEXT,
		source TEXT NOT NULL DEFAULT 'manual',
		conversation_id TEXT,
		approval_status TEXT,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		sent_at DATETIME,
		started_at DATETIME,
		completed_at DATETIME,
		duration_ms INTEGER DEFAULT 0,
		FOREIGN KEY (session_id) REFERENCES c2_sessions(id) ON DELETE CASCADE
	);`

	createC2FilesTable := `
	CREATE TABLE IF NOT EXISTS c2_files (
		id TEXT PRIMARY KEY,
		session_id TEXT NOT NULL,
		task_id TEXT,
		direction TEXT NOT NULL,
		remote_path TEXT NOT NULL,
		local_path TEXT NOT NULL,
		size_bytes INTEGER DEFAULT 0,
		sha256 TEXT,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (session_id) REFERENCES c2_sessions(id) ON DELETE CASCADE
	);`

	createC2EventsTable := `
	CREATE TABLE IF NOT EXISTS c2_events (
		id TEXT PRIMARY KEY,
		level TEXT NOT NULL DEFAULT 'info',
		category TEXT NOT NULL,
		session_id TEXT,
		task_id TEXT,
		message TEXT NOT NULL,
		data_json TEXT,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);`

	createC2ProfilesTable := `
	CREATE TABLE IF NOT EXISTS c2_profiles (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL UNIQUE,
		user_agent TEXT,
		uris_json TEXT NOT NULL DEFAULT '[]',
		request_headers_json TEXT,
		response_headers_json TEXT,
		body_template TEXT,
		jitter_min_ms INTEGER DEFAULT 0,
		jitter_max_ms INTEGER DEFAULT 0,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
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
	CREATE INDEX IF NOT EXISTS idx_vulnerabilities_conversation_tag ON vulnerabilities(conversation_tag);
	CREATE INDEX IF NOT EXISTS idx_vulnerabilities_task_tag ON vulnerabilities(task_tag);
	CREATE INDEX IF NOT EXISTS idx_vulnerabilities_severity ON vulnerabilities(severity);
	CREATE INDEX IF NOT EXISTS idx_vulnerabilities_status ON vulnerabilities(status);
	CREATE INDEX IF NOT EXISTS idx_vulnerabilities_created_at ON vulnerabilities(created_at);
	CREATE INDEX IF NOT EXISTS idx_batch_tasks_queue_id ON batch_tasks(queue_id);
	CREATE INDEX IF NOT EXISTS idx_batch_task_queues_created_at ON batch_task_queues(created_at);
	CREATE INDEX IF NOT EXISTS idx_batch_task_queues_title ON batch_task_queues(title);
	CREATE INDEX IF NOT EXISTS idx_webshell_connections_created_at ON webshell_connections(created_at);
	CREATE INDEX IF NOT EXISTS idx_webshell_connection_states_updated_at ON webshell_connection_states(updated_at);
	CREATE INDEX IF NOT EXISTS idx_c2_listeners_created_at ON c2_listeners(created_at);
	CREATE INDEX IF NOT EXISTS idx_c2_listeners_status ON c2_listeners(status);
	CREATE INDEX IF NOT EXISTS idx_c2_sessions_listener ON c2_sessions(listener_id);
	CREATE INDEX IF NOT EXISTS idx_c2_sessions_status ON c2_sessions(status);
	CREATE INDEX IF NOT EXISTS idx_c2_sessions_last_check_in ON c2_sessions(last_check_in);
	CREATE INDEX IF NOT EXISTS idx_c2_tasks_session ON c2_tasks(session_id);
	CREATE INDEX IF NOT EXISTS idx_c2_tasks_status ON c2_tasks(status);
	CREATE INDEX IF NOT EXISTS idx_c2_tasks_created_at ON c2_tasks(created_at);
	CREATE INDEX IF NOT EXISTS idx_c2_tasks_conversation ON c2_tasks(conversation_id);
	CREATE INDEX IF NOT EXISTS idx_c2_files_session ON c2_files(session_id);
	CREATE INDEX IF NOT EXISTS idx_c2_events_created_at ON c2_events(created_at);
	CREATE INDEX IF NOT EXISTS idx_c2_events_category ON c2_events(category);
	CREATE INDEX IF NOT EXISTS idx_c2_events_session ON c2_events(session_id);
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

	if _, err := db.Exec(createSkillStatsTable); err != nil {
		return fmt.Errorf("创建skill_stats表失败: %w", err)
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

	if _, err := db.Exec(createBatchTaskQueuesTable); err != nil {
		return fmt.Errorf("创建batch_task_queues表失败: %w", err)
	}

	if _, err := db.Exec(createBatchTasksTable); err != nil {
		return fmt.Errorf("创建batch_tasks表失败: %w", err)
	}

	if _, err := db.Exec(createWebshellConnectionsTable); err != nil {
		return fmt.Errorf("创建webshell_connections表失败: %w", err)
	}

	if _, err := db.Exec(createWebshellConnectionStatesTable); err != nil {
		return fmt.Errorf("创建webshell_connection_states表失败: %w", err)
	}

	for tableName, ddl := range map[string]string{
		"c2_listeners": createC2ListenersTable,
		"c2_sessions":  createC2SessionsTable,
		"c2_tasks":     createC2TasksTable,
		"c2_files":     createC2FilesTable,
		"c2_events":    createC2EventsTable,
		"c2_profiles":  createC2ProfilesTable,
	} {
		if _, err := db.Exec(ddl); err != nil {
			return fmt.Errorf("创建%s表失败: %w", tableName, err)
		}
	}

	// 为已有表添加新字段（如果不存在）- 必须在创建索引之前
	if err := db.migrateConversationsTable(); err != nil {
		db.logger.Warn("迁移conversations表失败", zap.Error(err))
		// 不返回错误，允许继续运行
	}

	if err := db.migrateMessagesTable(); err != nil {
		db.logger.Warn("迁移messages表失败", zap.Error(err))
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

	if err := db.migrateBatchTaskQueuesTable(); err != nil {
		db.logger.Warn("迁移batch_task_queues表失败", zap.Error(err))
		// 不返回错误，允许继续运行
	}
	if err := db.migrateVulnerabilitiesTable(); err != nil {
		db.logger.Warn("迁移vulnerabilities表失败", zap.Error(err))
		// 不返回错误，允许继续运行
	}

	if err := db.migrateWebshellConnectionsTable(); err != nil {
		db.logger.Warn("迁移webshell_connections表失败", zap.Error(err))
		// 不返回错误，允许继续运行
	}

	if _, err := db.Exec(createIndexes); err != nil {
		return fmt.Errorf("创建索引失败: %w", err)
	}

	db.logger.Info("数据库表初始化完成")
	return nil
}

// migrateMessagesTable 迁移 messages 表，补充 updated_at 字段。
// 语义：updated_at 表示该条消息最后一次被写入/更新的时间（例如助手占位消息在任务结束时更新正文）。
func (db *DB) migrateMessagesTable() error {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('messages') WHERE name='updated_at'").Scan(&count)
	if err != nil {
		// 如果查询失败，尝试添加字段
		if _, addErr := db.Exec("ALTER TABLE messages ADD COLUMN updated_at DATETIME"); addErr != nil {
			errMsg := strings.ToLower(addErr.Error())
			if !strings.Contains(errMsg, "duplicate column") && !strings.Contains(errMsg, "already exists") {
				return fmt.Errorf("添加 messages.updated_at 字段失败: %w", addErr)
			}
		}
	} else if count == 0 {
		if _, err := db.Exec("ALTER TABLE messages ADD COLUMN updated_at DATETIME"); err != nil {
			errMsg := strings.ToLower(err.Error())
			if !strings.Contains(errMsg, "duplicate column") && !strings.Contains(errMsg, "already exists") {
				return fmt.Errorf("添加 messages.updated_at 字段失败: %w", err)
			}
		}
	}

	// 回填已有数据：让 updated_at 至少等于 created_at，避免前端出现空/当前时间回退。
	_, _ = db.Exec("UPDATE messages SET updated_at = created_at WHERE updated_at IS NULL OR updated_at = ''")
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

	// 检查 webshell_connection_id 字段是否存在（WebShell AI 助手对话关联）
	err = db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('conversations') WHERE name='webshell_connection_id'").Scan(&count)
	if err != nil {
		if _, addErr := db.Exec("ALTER TABLE conversations ADD COLUMN webshell_connection_id TEXT"); addErr != nil {
			errMsg := strings.ToLower(addErr.Error())
			if !strings.Contains(errMsg, "duplicate column") && !strings.Contains(errMsg, "already exists") {
				db.logger.Warn("添加webshell_connection_id字段失败", zap.Error(addErr))
			}
		}
	} else if count == 0 {
		if _, err := db.Exec("ALTER TABLE conversations ADD COLUMN webshell_connection_id TEXT"); err != nil {
			db.logger.Warn("添加webshell_connection_id字段失败", zap.Error(err))
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

// migrateBatchTaskQueuesTable 迁移batch_task_queues表，补充新字段
func (db *DB) migrateBatchTaskQueuesTable() error {
	// 检查title字段是否存在
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('batch_task_queues') WHERE name='title'").Scan(&count)
	if err != nil {
		// 如果查询失败，尝试添加字段
		if _, addErr := db.Exec("ALTER TABLE batch_task_queues ADD COLUMN title TEXT"); addErr != nil {
			// 如果字段已存在，忽略错误
			errMsg := strings.ToLower(addErr.Error())
			if !strings.Contains(errMsg, "duplicate column") && !strings.Contains(errMsg, "already exists") {
				db.logger.Warn("添加title字段失败", zap.Error(addErr))
			}
		}
	} else if count == 0 {
		// 字段不存在，添加它
		if _, err := db.Exec("ALTER TABLE batch_task_queues ADD COLUMN title TEXT"); err != nil {
			db.logger.Warn("添加title字段失败", zap.Error(err))
		}
	}

	// 检查role字段是否存在
	var roleCount int
	err = db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('batch_task_queues') WHERE name='role'").Scan(&roleCount)
	if err != nil {
		// 如果查询失败，尝试添加字段
		if _, addErr := db.Exec("ALTER TABLE batch_task_queues ADD COLUMN role TEXT"); addErr != nil {
			// 如果字段已存在，忽略错误
			errMsg := strings.ToLower(addErr.Error())
			if !strings.Contains(errMsg, "duplicate column") && !strings.Contains(errMsg, "already exists") {
				db.logger.Warn("添加role字段失败", zap.Error(addErr))
			}
		}
	} else if roleCount == 0 {
		// 字段不存在，添加它
		if _, err := db.Exec("ALTER TABLE batch_task_queues ADD COLUMN role TEXT"); err != nil {
			db.logger.Warn("添加role字段失败", zap.Error(err))
		}
	}

	// 检查agent_mode字段是否存在
	var agentModeCount int
	err = db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('batch_task_queues') WHERE name='agent_mode'").Scan(&agentModeCount)
	if err != nil {
		if _, addErr := db.Exec("ALTER TABLE batch_task_queues ADD COLUMN agent_mode TEXT NOT NULL DEFAULT 'single'"); addErr != nil {
			errMsg := strings.ToLower(addErr.Error())
			if !strings.Contains(errMsg, "duplicate column") && !strings.Contains(errMsg, "already exists") {
				db.logger.Warn("添加agent_mode字段失败", zap.Error(addErr))
			}
		}
	} else if agentModeCount == 0 {
		if _, err := db.Exec("ALTER TABLE batch_task_queues ADD COLUMN agent_mode TEXT NOT NULL DEFAULT 'single'"); err != nil {
			db.logger.Warn("添加agent_mode字段失败", zap.Error(err))
		}
	}

	// 检查schedule_mode字段是否存在
	var scheduleModeCount int
	err = db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('batch_task_queues') WHERE name='schedule_mode'").Scan(&scheduleModeCount)
	if err != nil {
		if _, addErr := db.Exec("ALTER TABLE batch_task_queues ADD COLUMN schedule_mode TEXT NOT NULL DEFAULT 'manual'"); addErr != nil {
			errMsg := strings.ToLower(addErr.Error())
			if !strings.Contains(errMsg, "duplicate column") && !strings.Contains(errMsg, "already exists") {
				db.logger.Warn("添加schedule_mode字段失败", zap.Error(addErr))
			}
		}
	} else if scheduleModeCount == 0 {
		if _, err := db.Exec("ALTER TABLE batch_task_queues ADD COLUMN schedule_mode TEXT NOT NULL DEFAULT 'manual'"); err != nil {
			db.logger.Warn("添加schedule_mode字段失败", zap.Error(err))
		}
	}

	// 检查cron_expr字段是否存在
	var cronExprCount int
	err = db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('batch_task_queues') WHERE name='cron_expr'").Scan(&cronExprCount)
	if err != nil {
		if _, addErr := db.Exec("ALTER TABLE batch_task_queues ADD COLUMN cron_expr TEXT"); addErr != nil {
			errMsg := strings.ToLower(addErr.Error())
			if !strings.Contains(errMsg, "duplicate column") && !strings.Contains(errMsg, "already exists") {
				db.logger.Warn("添加cron_expr字段失败", zap.Error(addErr))
			}
		}
	} else if cronExprCount == 0 {
		if _, err := db.Exec("ALTER TABLE batch_task_queues ADD COLUMN cron_expr TEXT"); err != nil {
			db.logger.Warn("添加cron_expr字段失败", zap.Error(err))
		}
	}

	// 检查next_run_at字段是否存在
	var nextRunAtCount int
	err = db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('batch_task_queues') WHERE name='next_run_at'").Scan(&nextRunAtCount)
	if err != nil {
		if _, addErr := db.Exec("ALTER TABLE batch_task_queues ADD COLUMN next_run_at DATETIME"); addErr != nil {
			errMsg := strings.ToLower(addErr.Error())
			if !strings.Contains(errMsg, "duplicate column") && !strings.Contains(errMsg, "already exists") {
				db.logger.Warn("添加next_run_at字段失败", zap.Error(addErr))
			}
		}
	} else if nextRunAtCount == 0 {
		if _, err := db.Exec("ALTER TABLE batch_task_queues ADD COLUMN next_run_at DATETIME"); err != nil {
			db.logger.Warn("添加next_run_at字段失败", zap.Error(err))
		}
	}

	// schedule_enabled：0=暂停 Cron 自动调度，1=允许（手工执行不受影响）
	var scheduleEnCount int
	err = db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('batch_task_queues') WHERE name='schedule_enabled'").Scan(&scheduleEnCount)
	if err != nil {
		if _, addErr := db.Exec("ALTER TABLE batch_task_queues ADD COLUMN schedule_enabled INTEGER NOT NULL DEFAULT 1"); addErr != nil {
			errMsg := strings.ToLower(addErr.Error())
			if !strings.Contains(errMsg, "duplicate column") && !strings.Contains(errMsg, "already exists") {
				db.logger.Warn("添加schedule_enabled字段失败", zap.Error(addErr))
			}
		}
	} else if scheduleEnCount == 0 {
		if _, err := db.Exec("ALTER TABLE batch_task_queues ADD COLUMN schedule_enabled INTEGER NOT NULL DEFAULT 1"); err != nil {
			db.logger.Warn("添加schedule_enabled字段失败", zap.Error(err))
		}
	}

	var lastTrigCount int
	err = db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('batch_task_queues') WHERE name='last_schedule_trigger_at'").Scan(&lastTrigCount)
	if err != nil {
		if _, addErr := db.Exec("ALTER TABLE batch_task_queues ADD COLUMN last_schedule_trigger_at DATETIME"); addErr != nil {
			errMsg := strings.ToLower(addErr.Error())
			if !strings.Contains(errMsg, "duplicate column") && !strings.Contains(errMsg, "already exists") {
				db.logger.Warn("添加last_schedule_trigger_at字段失败", zap.Error(addErr))
			}
		}
	} else if lastTrigCount == 0 {
		if _, err := db.Exec("ALTER TABLE batch_task_queues ADD COLUMN last_schedule_trigger_at DATETIME"); err != nil {
			db.logger.Warn("添加last_schedule_trigger_at字段失败", zap.Error(err))
		}
	}

	var lastSchedErrCount int
	err = db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('batch_task_queues') WHERE name='last_schedule_error'").Scan(&lastSchedErrCount)
	if err != nil {
		if _, addErr := db.Exec("ALTER TABLE batch_task_queues ADD COLUMN last_schedule_error TEXT"); addErr != nil {
			errMsg := strings.ToLower(addErr.Error())
			if !strings.Contains(errMsg, "duplicate column") && !strings.Contains(errMsg, "already exists") {
				db.logger.Warn("添加last_schedule_error字段失败", zap.Error(addErr))
			}
		}
	} else if lastSchedErrCount == 0 {
		if _, err := db.Exec("ALTER TABLE batch_task_queues ADD COLUMN last_schedule_error TEXT"); err != nil {
			db.logger.Warn("添加last_schedule_error字段失败", zap.Error(err))
		}
	}

	var lastRunErrCount int
	err = db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('batch_task_queues') WHERE name='last_run_error'").Scan(&lastRunErrCount)
	if err != nil {
		if _, addErr := db.Exec("ALTER TABLE batch_task_queues ADD COLUMN last_run_error TEXT"); addErr != nil {
			errMsg := strings.ToLower(addErr.Error())
			if !strings.Contains(errMsg, "duplicate column") && !strings.Contains(errMsg, "already exists") {
				db.logger.Warn("添加last_run_error字段失败", zap.Error(addErr))
			}
		}
	} else if lastRunErrCount == 0 {
		if _, err := db.Exec("ALTER TABLE batch_task_queues ADD COLUMN last_run_error TEXT"); err != nil {
			db.logger.Warn("添加last_run_error字段失败", zap.Error(err))
		}
	}

	return nil
}

// migrateVulnerabilitiesTable 迁移 vulnerabilities 表，补充标签字段
func (db *DB) migrateVulnerabilitiesTable() error {
	columns := []struct {
		name string
		stmt string
	}{
		{name: "conversation_tag", stmt: "ALTER TABLE vulnerabilities ADD COLUMN conversation_tag TEXT"},
		{name: "task_tag", stmt: "ALTER TABLE vulnerabilities ADD COLUMN task_tag TEXT"},
	}

	for _, col := range columns {
		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('vulnerabilities') WHERE name=?", col.name).Scan(&count)
		if err != nil {
			if _, addErr := db.Exec(col.stmt); addErr != nil {
				errMsg := strings.ToLower(addErr.Error())
				if !strings.Contains(errMsg, "duplicate column") && !strings.Contains(errMsg, "already exists") {
					db.logger.Warn("添加vulnerabilities字段失败", zap.String("field", col.name), zap.Error(addErr))
				}
			}
			continue
		}
		if count == 0 {
			if _, addErr := db.Exec(col.stmt); addErr != nil {
				db.logger.Warn("添加vulnerabilities字段失败", zap.String("field", col.name), zap.Error(addErr))
			}
		}
	}
	return nil
}

// migrateWebshellConnectionsTable 迁移 webshell_connections 表，补充新字段
func (db *DB) migrateWebshellConnectionsTable() error {
	columns := []struct {
		name string
		stmt string
	}{
		{name: "encoding", stmt: "ALTER TABLE webshell_connections ADD COLUMN encoding TEXT NOT NULL DEFAULT ''"},
		{name: "os", stmt: "ALTER TABLE webshell_connections ADD COLUMN os TEXT NOT NULL DEFAULT ''"},
	}

	for _, col := range columns {
		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('webshell_connections') WHERE name=?", col.name).Scan(&count)
		if err != nil {
			if _, addErr := db.Exec(col.stmt); addErr != nil {
				errMsg := strings.ToLower(addErr.Error())
				if !strings.Contains(errMsg, "duplicate column") && !strings.Contains(errMsg, "already exists") {
					db.logger.Warn("添加webshell_connections字段失败", zap.String("field", col.name), zap.Error(addErr))
				}
			}
			continue
		}
		if count == 0 {
			if _, addErr := db.Exec(col.stmt); addErr != nil {
				db.logger.Warn("添加webshell_connections字段失败", zap.String("field", col.name), zap.Error(addErr))
			}
		}
	}
	return nil
}

// NewKnowledgeDB 创建知识库数据库连接（只包含知识库相关的表）
func NewKnowledgeDB(dbPath string, logger *zap.Logger) (*DB, error) {
	sqlDB, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_foreign_keys=1&_busy_timeout=5000&_synchronous=NORMAL")
	if err != nil {
		return nil, fmt.Errorf("打开知识库数据库失败: %w", err)
	}

	configureDBPool(sqlDB)

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
		sub_indexes TEXT NOT NULL DEFAULT '',
		embedding_model TEXT NOT NULL DEFAULT '',
		embedding_dim INTEGER NOT NULL DEFAULT 0,
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

	if err := db.migrateKnowledgeEmbeddingsColumns(); err != nil {
		return fmt.Errorf("迁移 knowledge_embeddings 列失败: %w", err)
	}

	db.logger.Info("知识库数据库表初始化完成")
	return nil
}

// migrateKnowledgeEmbeddingsColumns 为已有库补充 sub_indexes、embedding_model、embedding_dim。
func (db *DB) migrateKnowledgeEmbeddingsColumns() error {
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='knowledge_embeddings'`).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		return nil
	}
	migrations := []struct {
		col  string
		stmt string
	}{
		{"sub_indexes", `ALTER TABLE knowledge_embeddings ADD COLUMN sub_indexes TEXT NOT NULL DEFAULT ''`},
		{"embedding_model", `ALTER TABLE knowledge_embeddings ADD COLUMN embedding_model TEXT NOT NULL DEFAULT ''`},
		{"embedding_dim", `ALTER TABLE knowledge_embeddings ADD COLUMN embedding_dim INTEGER NOT NULL DEFAULT 0`},
	}
	for _, m := range migrations {
		var colCount int
		q := `SELECT COUNT(*) FROM pragma_table_info('knowledge_embeddings') WHERE name = ?`
		if err := db.QueryRow(q, m.col).Scan(&colCount); err != nil {
			return err
		}
		if colCount > 0 {
			continue
		}
		if _, err := db.Exec(m.stmt); err != nil {
			return err
		}
	}
	return nil
}

// Close 关闭数据库连接
func (db *DB) Close() error {
	return db.DB.Close()
}
