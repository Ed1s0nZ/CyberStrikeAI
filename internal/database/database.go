package database

import (
	"database/sql"
	"fmt"

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
		updated_at DATETIME NOT NULL
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

	if _, err := db.Exec(createIndexes); err != nil {
		return fmt.Errorf("创建索引失败: %w", err)
	}

	db.logger.Info("数据库表初始化完成")
	return nil
}

// Close 关闭数据库连接
func (db *DB) Close() error {
	return db.DB.Close()
}

