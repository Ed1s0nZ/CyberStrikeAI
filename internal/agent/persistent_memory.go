package agent

import (
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// MemoryCategory defines the type of memory entry.
type MemoryCategory string

const (
	// MemoryCategoryCredential stores discovered credentials, tokens, and secrets.
	MemoryCategoryCredential MemoryCategory = "credential"
	// MemoryCategoryTarget stores information about targets (IPs, domains, services).
	MemoryCategoryTarget MemoryCategory = "target"
	// MemoryCategoryVulnerability stores vulnerability notes and exploit details.
	MemoryCategoryVulnerability MemoryCategory = "vulnerability"
	// MemoryCategoryFact stores general facts and observations.
	MemoryCategoryFact MemoryCategory = "fact"
	// MemoryCategoryNote stores operational notes and planning reminders.
	MemoryCategoryNote MemoryCategory = "note"
)

// MemoryEntry represents a single persistent memory item.
type MemoryEntry struct {
	ID             string         `json:"id"`
	Key            string         `json:"key"`
	Value          string         `json:"value"`
	Category       MemoryCategory `json:"category"`
	ConversationID string         `json:"conversation_id,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

// PersistentMemory manages long-lived memory entries that survive conversation
// compression and persist across sessions. Entries are stored in SQLite and
// injected as a context block in every agent system prompt.
type PersistentMemory struct {
	db     *sql.DB
	mu     sync.RWMutex
	logger *zap.Logger
}

// NewPersistentMemory creates a PersistentMemory backed by the given SQLite DB.
// It runs the table migration on first call so existing databases are safe.
func NewPersistentMemory(db *sql.DB, logger *zap.Logger) (*PersistentMemory, error) {
	if logger == nil {
		logger = zap.NewNop()
	}
	pm := &PersistentMemory{db: db, logger: logger}
	if err := pm.migrate(); err != nil {
		return nil, fmt.Errorf("persistent memory migration failed: %w", err)
	}
	return pm, nil
}

// migrate creates the agent_memories table if it does not exist.
func (pm *PersistentMemory) migrate() error {
	createTable := `
	CREATE TABLE IF NOT EXISTS agent_memories (
		id TEXT PRIMARY KEY,
		key TEXT NOT NULL,
		value TEXT NOT NULL,
		category TEXT NOT NULL DEFAULT 'fact',
		conversation_id TEXT,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_agent_memories_category ON agent_memories(category);
	CREATE INDEX IF NOT EXISTS idx_agent_memories_key ON agent_memories(key);
	CREATE INDEX IF NOT EXISTS idx_agent_memories_conversation ON agent_memories(conversation_id);
	`
	if _, err := pm.db.Exec(createTable); err != nil {
		return err
	}
	return nil
}

// Store upserts a memory entry by key. If a record with the same key already
// exists it is updated in-place; otherwise a new entry is inserted.
func (pm *PersistentMemory) Store(key, value string, category MemoryCategory, conversationID string) (*MemoryEntry, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	now := time.Now().UTC()

	// Check for existing entry with same key.
	var existingID string
	err := pm.db.QueryRow("SELECT id FROM agent_memories WHERE key = ?", key).Scan(&existingID)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("query existing memory: %w", err)
	}

	if existingID != "" {
		// Update existing entry.
		_, err = pm.db.Exec(
			"UPDATE agent_memories SET value = ?, category = ?, conversation_id = ?, updated_at = ? WHERE id = ?",
			value, string(category), conversationID, now, existingID,
		)
		if err != nil {
			return nil, fmt.Errorf("update memory: %w", err)
		}
		pm.logger.Debug("updated persistent memory", zap.String("key", key), zap.String("category", string(category)))
		return &MemoryEntry{
			ID:             existingID,
			Key:            key,
			Value:          value,
			Category:       category,
			ConversationID: conversationID,
			UpdatedAt:      now,
		}, nil
	}

	// Insert new entry.
	id := uuid.New().String()
	_, err = pm.db.Exec(
		"INSERT INTO agent_memories (id, key, value, category, conversation_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		id, key, value, string(category), conversationID, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert memory: %w", err)
	}
	pm.logger.Debug("stored new persistent memory", zap.String("key", key), zap.String("category", string(category)))
	return &MemoryEntry{
		ID:             id,
		Key:            key,
		Value:          value,
		Category:       category,
		ConversationID: conversationID,
		CreatedAt:      now,
		UpdatedAt:      now,
	}, nil
}

// Retrieve fetches memory entries matching the query. The query is matched
// case-insensitively against both the key and value fields. If category is
// non-empty only entries of that category are returned.
func (pm *PersistentMemory) Retrieve(query string, category MemoryCategory, limit int) ([]*MemoryEntry, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if limit <= 0 {
		limit = 20
	}

	var (
		rows *sql.Rows
		err  error
	)

	likeQ := "%" + strings.ToLower(query) + "%"

	if category != "" {
		rows, err = pm.db.Query(
			`SELECT id, key, value, category, conversation_id, created_at, updated_at
			 FROM agent_memories
			 WHERE category = ? AND (LOWER(key) LIKE ? OR LOWER(value) LIKE ?)
			 ORDER BY updated_at DESC LIMIT ?`,
			string(category), likeQ, likeQ, limit,
		)
	} else if query != "" {
		rows, err = pm.db.Query(
			`SELECT id, key, value, category, conversation_id, created_at, updated_at
			 FROM agent_memories
			 WHERE LOWER(key) LIKE ? OR LOWER(value) LIKE ?
			 ORDER BY updated_at DESC LIMIT ?`,
			likeQ, likeQ, limit,
		)
	} else {
		rows, err = pm.db.Query(
			`SELECT id, key, value, category, conversation_id, created_at, updated_at
			 FROM agent_memories
			 ORDER BY updated_at DESC LIMIT ?`,
			limit,
		)
	}

	if err != nil {
		return nil, fmt.Errorf("query memories: %w", err)
	}
	defer rows.Close()

	return pm.scanRows(rows)
}

// List returns all memory entries optionally filtered by category, ordered by
// most recently updated first.
func (pm *PersistentMemory) List(category MemoryCategory, limit int) ([]*MemoryEntry, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}

	var (
		rows *sql.Rows
		err  error
	)

	if category != "" {
		rows, err = pm.db.Query(
			`SELECT id, key, value, category, conversation_id, created_at, updated_at
			 FROM agent_memories WHERE category = ? ORDER BY updated_at DESC LIMIT ?`,
			string(category), limit,
		)
	} else {
		rows, err = pm.db.Query(
			`SELECT id, key, value, category, conversation_id, created_at, updated_at
			 FROM agent_memories ORDER BY updated_at DESC LIMIT ?`,
			limit,
		)
	}

	if err != nil {
		return nil, fmt.Errorf("list memories: %w", err)
	}
	defer rows.Close()

	return pm.scanRows(rows)
}

// UpdateByID updates the key, value, and category of an existing memory entry
// identified by its UUID. Returns an error if the entry is not found.
func (pm *PersistentMemory) UpdateByID(id, key, value string, category MemoryCategory) (*MemoryEntry, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	now := time.Now().UTC()

	res, err := pm.db.Exec(
		"UPDATE agent_memories SET key = ?, value = ?, category = ?, updated_at = ? WHERE id = ?",
		key, value, string(category), now, id,
	)
	if err != nil {
		return nil, fmt.Errorf("update memory by id: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, fmt.Errorf("memory entry not found: %s", id)
	}

	pm.logger.Debug("updated persistent memory by id", zap.String("id", id), zap.String("key", key))

	var e MemoryEntry
	var convID sql.NullString
	var createdAtStr, updatedAtStr string
	err = pm.db.QueryRow(
		"SELECT id, key, value, category, conversation_id, created_at, updated_at FROM agent_memories WHERE id = ?", id,
	).Scan(&e.ID, &e.Key, &e.Value, &e.Category, &convID, &createdAtStr, &updatedAtStr)
	if err != nil {
		return nil, fmt.Errorf("fetch updated memory: %w", err)
	}
	if convID.Valid {
		e.ConversationID = convID.String
	}
	if t, parseErr := time.Parse(time.RFC3339Nano, createdAtStr); parseErr == nil {
		e.CreatedAt = t
	}
	if t, parseErr := time.Parse(time.RFC3339Nano, updatedAtStr); parseErr == nil {
		e.UpdatedAt = t
	}
	return &e, nil
}

// Delete removes a memory entry by ID. Returns nil if the entry did not exist.
func (pm *PersistentMemory) Delete(id string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	res, err := pm.db.Exec("DELETE FROM agent_memories WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete memory: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		pm.logger.Debug("delete memory: entry not found", zap.String("id", id))
	}
	return nil
}

// BuildContextBlock returns a compact text summary of all stored memories
// suitable for injection into a system prompt. Returns an empty string when
// there are no memories.
func (pm *PersistentMemory) BuildContextBlock() string {
	entries, err := pm.List("", 100)
	if err != nil || len(entries) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("<persistent_memory>\n")
	sb.WriteString("The following facts were remembered from previous sessions or earlier in this session:\n")

	// Group by category for readability.
	categories := []MemoryCategory{
		MemoryCategoryCredential,
		MemoryCategoryTarget,
		MemoryCategoryVulnerability,
		MemoryCategoryFact,
		MemoryCategoryNote,
	}

	byCategory := make(map[MemoryCategory][]*MemoryEntry)
	for _, e := range entries {
		byCategory[e.Category] = append(byCategory[e.Category], e)
	}

	for _, cat := range categories {
		items := byCategory[cat]
		if len(items) == 0 {
			continue
		}
		sb.WriteString(fmt.Sprintf("\n[%s]\n", strings.ToUpper(string(cat))))
		for _, item := range items {
			sb.WriteString(fmt.Sprintf("  • %s: %s\n", item.Key, item.Value))
		}
	}

	sb.WriteString("</persistent_memory>\n")
	return sb.String()
}

func (pm *PersistentMemory) scanRows(rows *sql.Rows) ([]*MemoryEntry, error) {
	var entries []*MemoryEntry
	for rows.Next() {
		var e MemoryEntry
		var convID sql.NullString
		var createdAt, updatedAt string
		if err := rows.Scan(&e.ID, &e.Key, &e.Value, &e.Category, &convID, &createdAt, &updatedAt); err != nil {
			pm.logger.Warn("scan memory row", zap.Error(err))
			continue
		}
		if convID.Valid {
			e.ConversationID = convID.String
		}
		if t, err := time.Parse(time.RFC3339Nano, createdAt); err == nil {
			e.CreatedAt = t
		}
		if t, err := time.Parse(time.RFC3339Nano, updatedAt); err == nil {
			e.UpdatedAt = t
		}
		entries = append(entries, &e)
	}
	return entries, rows.Err()
}
