package database

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"
)

// DB database connection
type DB struct {
	*sql.DB
	logger *zap.Logger
}

// NewDB database connection
func NewDB(dbPath string, logger *zap.Logger) (*DB, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_foreign_keys=1")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	database := &DB{
		DB:     db,
		logger: logger,
	}

	// 
	if err := database.initTables(); err != nil {
		return nil, fmt.Errorf("failed to initialize tables: %w", err)
	}

	return database, nil
}

// initTables initialize database
func (db *DB) initTables() error {
	// create conversations table
	createConversationsTable := `
	CREATE TABLE IF NOT EXISTS conversations (
		id TEXT PRIMARY KEY,
		title TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		last_react_input TEXT,
		last_react_output TEXT
	);`

	// create messages table
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

	// create process details table
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

	// create tool execution records table
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

	// create tool stats table
	createToolStatsTable := `
	CREATE TABLE IF NOT EXISTS tool_stats (
		tool_name TEXT PRIMARY KEY,
		total_calls INTEGER NOT NULL DEFAULT 0,
		success_calls INTEGER NOT NULL DEFAULT 0,
		failed_calls INTEGER NOT NULL DEFAULT 0,
		last_call_time DATETIME,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);`

	// create Skills stats table
	createSkillStatsTable := `
	CREATE TABLE IF NOT EXISTS skill_stats (
		skill_name TEXT PRIMARY KEY,
		total_calls INTEGER NOT NULL DEFAULT 0,
		success_calls INTEGER NOT NULL DEFAULT 0,
		failed_calls INTEGER NOT NULL DEFAULT 0,
		last_call_time DATETIME,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);`

	// create attack chain nodes table
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

	// create attack chain edges table
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

	// create knowledge retrieval logs table（，）
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

	// create conversation groups table
	createConversationGroupsTable := `
	CREATE TABLE IF NOT EXISTS conversation_groups (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		icon TEXT,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	);`

	// create conversation group mappings table
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

	// create vulnerabilities table
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

	// create batch task queues table
	createBatchTaskQueuesTable := `
	CREATE TABLE IF NOT EXISTS batch_task_queues (
		id TEXT PRIMARY KEY,
		title TEXT,
		status TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		started_at DATETIME,
		completed_at DATETIME,
		current_index INTEGER NOT NULL DEFAULT 0
	);`

	// create batch tasks table
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

	// create WebShell connections table
	createWebshellConnectionsTable := `
	CREATE TABLE IF NOT EXISTS webshell_connections (
		id TEXT PRIMARY KEY,
		url TEXT NOT NULL,
		password TEXT NOT NULL DEFAULT '',
		type TEXT NOT NULL DEFAULT 'php',
		method TEXT NOT NULL DEFAULT 'post',
		cmd_param TEXT NOT NULL DEFAULT '',
		remark TEXT NOT NULL DEFAULT '',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);`

	// create WebShell connection states table（/status）
	createWebshellConnectionStatesTable := `
	CREATE TABLE IF NOT EXISTS webshell_connection_states (
		connection_id TEXT PRIMARY KEY,
		state_json TEXT NOT NULL DEFAULT '{}',
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (connection_id) REFERENCES webshell_connections(id) ON DELETE CASCADE
	);`

	// create indexes
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
	CREATE INDEX IF NOT EXISTS idx_batch_tasks_queue_id ON batch_tasks(queue_id);
	CREATE INDEX IF NOT EXISTS idx_batch_task_queues_created_at ON batch_task_queues(created_at);
	CREATE INDEX IF NOT EXISTS idx_batch_task_queues_title ON batch_task_queues(title);
	CREATE INDEX IF NOT EXISTS idx_webshell_connections_created_at ON webshell_connections(created_at);
	CREATE INDEX IF NOT EXISTS idx_webshell_connection_states_updated_at ON webshell_connection_states(updated_at);
	`

	if _, err := db.Exec(createConversationsTable); err != nil {
		return fmt.Errorf("conversationstable failed: %w", err)
	}

	if _, err := db.Exec(createMessagesTable); err != nil {
		return fmt.Errorf("messagestable failed: %w", err)
	}

	if _, err := db.Exec(createProcessDetailsTable); err != nil {
		return fmt.Errorf("process_detailstable failed: %w", err)
	}

	if _, err := db.Exec(createToolExecutionsTable); err != nil {
		return fmt.Errorf("tool_executionstable failed: %w", err)
	}

	if _, err := db.Exec(createToolStatsTable); err != nil {
		return fmt.Errorf("tool_statstable failed: %w", err)
	}

	if _, err := db.Exec(createSkillStatsTable); err != nil {
		return fmt.Errorf("skill_statstable failed: %w", err)
	}

	if _, err := db.Exec(createAttackChainNodesTable); err != nil {
		return fmt.Errorf("attack_chain_nodestable failed: %w", err)
	}

	if _, err := db.Exec(createAttackChainEdgesTable); err != nil {
		return fmt.Errorf("attack_chain_edgestable failed: %w", err)
	}

	if _, err := db.Exec(createKnowledgeRetrievalLogsTable); err != nil {
		return fmt.Errorf("knowledge_retrieval_logstable failed: %w", err)
	}

	if _, err := db.Exec(createConversationGroupsTable); err != nil {
		return fmt.Errorf("conversation_groupstable failed: %w", err)
	}

	if _, err := db.Exec(createConversationGroupMappingsTable); err != nil {
		return fmt.Errorf("conversation_group_mappingstable failed: %w", err)
	}

	if _, err := db.Exec(createVulnerabilitiesTable); err != nil {
		return fmt.Errorf("vulnerabilitiestable failed: %w", err)
	}

	if _, err := db.Exec(createBatchTaskQueuesTable); err != nil {
		return fmt.Errorf("batch_task_queuestable failed: %w", err)
	}

	if _, err := db.Exec(createBatchTasksTable); err != nil {
		return fmt.Errorf("batch_taskstable failed: %w", err)
	}

	if _, err := db.Exec(createWebshellConnectionsTable); err != nil {
		return fmt.Errorf("webshell_connectionstable failed: %w", err)
	}

	if _, err := db.Exec(createWebshellConnectionStatesTable); err != nil {
		return fmt.Errorf("webshell_connection_statestable failed: %w", err)
	}

	// add new fields to existing tables（）- create indexes
	if err := db.migrateConversationsTable(); err != nil {
		db.logger.Warn("migrateconversationstable failed", zap.Error(err))
		// do not return error, allow to continue
	}

	if err := db.migrateConversationGroupsTable(); err != nil {
		db.logger.Warn("migrateconversation_groupstable failed", zap.Error(err))
		// do not return error, allow to continue
	}

	if err := db.migrateConversationGroupMappingsTable(); err != nil {
		db.logger.Warn("migrateconversation_group_mappingstable failed", zap.Error(err))
		// do not return error, allow to continue
	}

	if err := db.migrateBatchTaskQueuesTable(); err != nil {
		db.logger.Warn("migratebatch_task_queuestable failed", zap.Error(err))
		// do not return error, allow to continue
	}

	if _, err := db.Exec(createIndexes); err != nil {
		return fmt.Errorf("create indexes: %w", err)
	}

	db.logger.Info("database table initialization complete")
	return nil
}

// migrateConversationsTable migrateconversations，add
func (db *DB) migrateConversationsTable() error {
	// last_react_inputfield exists
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('conversations') WHERE name='last_react_input'").Scan(&count)
	if err != nil {
		// if query fails, try to add field
		if _, addErr := db.Exec("ALTER TABLE conversations ADD COLUMN last_react_input TEXT"); addErr != nil {
			// if field already exists, ignore error（SQLiteerror）
			errMsg := strings.ToLower(addErr.Error())
			if !strings.Contains(errMsg, "duplicate column") && !strings.Contains(errMsg, "already exists") {
				db.logger.Warn("addlast_react_inputfield failed", zap.Error(addErr))
			}
		}
	} else if count == 0 {
		// field does not exist, add it
		if _, err := db.Exec("ALTER TABLE conversations ADD COLUMN last_react_input TEXT"); err != nil {
			db.logger.Warn("addlast_react_inputfield failed", zap.Error(err))
		}
	}

	// last_react_outputfield exists
	err = db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('conversations') WHERE name='last_react_output'").Scan(&count)
	if err != nil {
		// if query fails, try to add field
		if _, addErr := db.Exec("ALTER TABLE conversations ADD COLUMN last_react_output TEXT"); addErr != nil {
			// if field already exists, ignore error
			errMsg := strings.ToLower(addErr.Error())
			if !strings.Contains(errMsg, "duplicate column") && !strings.Contains(errMsg, "already exists") {
				db.logger.Warn("addlast_react_outputfield failed", zap.Error(addErr))
			}
		}
	} else if count == 0 {
		// field does not exist, add it
		if _, err := db.Exec("ALTER TABLE conversations ADD COLUMN last_react_output TEXT"); err != nil {
			db.logger.Warn("addlast_react_outputfield failed", zap.Error(err))
		}
	}

	// pinnedfield exists
	err = db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('conversations') WHERE name='pinned'").Scan(&count)
	if err != nil {
		// if query fails, try to add field
		if _, addErr := db.Exec("ALTER TABLE conversations ADD COLUMN pinned INTEGER DEFAULT 0"); addErr != nil {
			// if field already exists, ignore error
			errMsg := strings.ToLower(addErr.Error())
			if !strings.Contains(errMsg, "duplicate column") && !strings.Contains(errMsg, "already exists") {
				db.logger.Warn("addpinnedfield failed", zap.Error(addErr))
			}
		}
	} else if count == 0 {
		// field does not exist, add it
		if _, err := db.Exec("ALTER TABLE conversations ADD COLUMN pinned INTEGER DEFAULT 0"); err != nil {
			db.logger.Warn("addpinnedfield failed", zap.Error(err))
		}
	}

	// webshell_connection_id field exists（WebShell AI conversation）
	err = db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('conversations') WHERE name='webshell_connection_id'").Scan(&count)
	if err != nil {
		if _, addErr := db.Exec("ALTER TABLE conversations ADD COLUMN webshell_connection_id TEXT"); addErr != nil {
			errMsg := strings.ToLower(addErr.Error())
			if !strings.Contains(errMsg, "duplicate column") && !strings.Contains(errMsg, "already exists") {
				db.logger.Warn("addwebshell_connection_idfield failed", zap.Error(addErr))
			}
		}
	} else if count == 0 {
		if _, err := db.Exec("ALTER TABLE conversations ADD COLUMN webshell_connection_id TEXT"); err != nil {
			db.logger.Warn("addwebshell_connection_idfield failed", zap.Error(err))
		}
	}

	return nil
}

// migrateConversationGroupsTable migrateconversation_groups，add
func (db *DB) migrateConversationGroupsTable() error {
	// pinnedfield exists
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('conversation_groups') WHERE name='pinned'").Scan(&count)
	if err != nil {
		// if query fails, try to add field
		if _, addErr := db.Exec("ALTER TABLE conversation_groups ADD COLUMN pinned INTEGER DEFAULT 0"); addErr != nil {
			// if field already exists, ignore error
			errMsg := strings.ToLower(addErr.Error())
			if !strings.Contains(errMsg, "duplicate column") && !strings.Contains(errMsg, "already exists") {
				db.logger.Warn("addpinnedfield failed", zap.Error(addErr))
			}
		}
	} else if count == 0 {
		// field does not exist, add it
		if _, err := db.Exec("ALTER TABLE conversation_groups ADD COLUMN pinned INTEGER DEFAULT 0"); err != nil {
			db.logger.Warn("addpinnedfield failed", zap.Error(err))
		}
	}

	return nil
}

// migrateConversationGroupMappingsTable migrateconversation_group_mappings，add
func (db *DB) migrateConversationGroupMappingsTable() error {
	// pinnedfield exists
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('conversation_group_mappings') WHERE name='pinned'").Scan(&count)
	if err != nil {
		// if query fails, try to add field
		if _, addErr := db.Exec("ALTER TABLE conversation_group_mappings ADD COLUMN pinned INTEGER DEFAULT 0"); addErr != nil {
			// if field already exists, ignore error
			errMsg := strings.ToLower(addErr.Error())
			if !strings.Contains(errMsg, "duplicate column") && !strings.Contains(errMsg, "already exists") {
				db.logger.Warn("addpinnedfield failed", zap.Error(addErr))
			}
		}
	} else if count == 0 {
		// field does not exist, add it
		if _, err := db.Exec("ALTER TABLE conversation_group_mappings ADD COLUMN pinned INTEGER DEFAULT 0"); err != nil {
			db.logger.Warn("addpinnedfield failed", zap.Error(err))
		}
	}

	return nil
}

// migrateBatchTaskQueuesTable migratebatch_task_queues，addtitlerole
func (db *DB) migrateBatchTaskQueuesTable() error {
	// titlefield exists
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('batch_task_queues') WHERE name='title'").Scan(&count)
	if err != nil {
		// if query fails, try to add field
		if _, addErr := db.Exec("ALTER TABLE batch_task_queues ADD COLUMN title TEXT"); addErr != nil {
			// if field already exists, ignore error
			errMsg := strings.ToLower(addErr.Error())
			if !strings.Contains(errMsg, "duplicate column") && !strings.Contains(errMsg, "already exists") {
				db.logger.Warn("addtitlefield failed", zap.Error(addErr))
			}
		}
	} else if count == 0 {
		// field does not exist, add it
		if _, err := db.Exec("ALTER TABLE batch_task_queues ADD COLUMN title TEXT"); err != nil {
			db.logger.Warn("addtitlefield failed", zap.Error(err))
		}
	}

	// rolefield exists
	var roleCount int
	err = db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('batch_task_queues') WHERE name='role'").Scan(&roleCount)
	if err != nil {
		// if query fails, try to add field
		if _, addErr := db.Exec("ALTER TABLE batch_task_queues ADD COLUMN role TEXT"); addErr != nil {
			// if field already exists, ignore error
			errMsg := strings.ToLower(addErr.Error())
			if !strings.Contains(errMsg, "duplicate column") && !strings.Contains(errMsg, "already exists") {
				db.logger.Warn("addrolefield failed", zap.Error(addErr))
			}
		}
	} else if roleCount == 0 {
		// field does not exist, add it
		if _, err := db.Exec("ALTER TABLE batch_task_queues ADD COLUMN role TEXT"); err != nil {
			db.logger.Warn("addrolefield failed", zap.Error(err))
		}
	}

	return nil
}

// NewKnowledgeDB knowledge basedatabase connection（knowledge base）
func NewKnowledgeDB(dbPath string, logger *zap.Logger) (*DB, error) {
	sqlDB, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_foreign_keys=1")
	if err != nil {
		return nil, fmt.Errorf("knowledge base: %w", err)
	}

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("knowledge base: %w", err)
	}

	database := &DB{
		DB:     sqlDB,
		logger: logger,
	}

	// knowledge base
	if err := database.initKnowledgeTables(); err != nil {
		return nil, fmt.Errorf("knowledge basetable failed: %w", err)
	}

	return database, nil
}

// initKnowledgeTables knowledge base（knowledge base）
func (db *DB) initKnowledgeTables() error {
	// knowledge base
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

	// knowledge base
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

	// create knowledge retrieval logs table（knowledge base，，conversationsmessages）
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

	// create indexes
	createIndexes := `
	CREATE INDEX IF NOT EXISTS idx_knowledge_items_category ON knowledge_base_items(category);
	CREATE INDEX IF NOT EXISTS idx_knowledge_embeddings_item_id ON knowledge_embeddings(item_id);
	CREATE INDEX IF NOT EXISTS idx_knowledge_retrieval_logs_conversation ON knowledge_retrieval_logs(conversation_id);
	CREATE INDEX IF NOT EXISTS idx_knowledge_retrieval_logs_message ON knowledge_retrieval_logs(message_id);
	CREATE INDEX IF NOT EXISTS idx_knowledge_retrieval_logs_created_at ON knowledge_retrieval_logs(created_at);
	`

	if _, err := db.Exec(createKnowledgeBaseItemsTable); err != nil {
		return fmt.Errorf("knowledge_base_itemstable failed: %w", err)
	}

	if _, err := db.Exec(createKnowledgeEmbeddingsTable); err != nil {
		return fmt.Errorf("knowledge_embeddingstable failed: %w", err)
	}

	if _, err := db.Exec(createKnowledgeRetrievalLogsTable); err != nil {
		return fmt.Errorf("knowledge_retrieval_logstable failed: %w", err)
	}

	if _, err := db.Exec(createIndexes); err != nil {
		return fmt.Errorf("create indexes: %w", err)
	}

	db.logger.Info("knowledge basedatabase table initialization complete")
	return nil
}

// Close database connection
func (db *DB) Close() error {
	return db.DB.Close()
}
