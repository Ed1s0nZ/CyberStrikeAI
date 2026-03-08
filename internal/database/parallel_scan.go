package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ParallelScanRow represents a parallel scan session in the database
type ParallelScanRow struct {
	ID           string
	Target       string
	Status       string
	MaxRounds    int
	ReconContext sql.NullString
	GroupID      sql.NullString
	CreatedAt    time.Time
	StartedAt    sql.NullTime
	CompletedAt  sql.NullTime
}

// ParallelScanAgentRow represents a parallel scan agent in the database
type ParallelScanAgentRow struct {
	ID              string
	ScanID          string
	Name            string
	ConversationID  sql.NullString
	Status          string
	CurrentRound    int
	TotalIterations int
	TotalToolCalls  int
	TotalVulns      int
	Errors          int
	LastActivity    sql.NullTime
	CreatedAt       time.Time
}

// initParallelScanTables creates tables for parallel scan feature
func (db *DB) initParallelScanTables() error {
	createScansTable := `
	CREATE TABLE IF NOT EXISTS parallel_scans (
		id TEXT PRIMARY KEY,
		target TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'pending',
		max_rounds INTEGER NOT NULL DEFAULT 20,
		recon_context TEXT,
		group_id TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		started_at DATETIME,
		completed_at DATETIME
	);`

	createAgentsTable := `
	CREATE TABLE IF NOT EXISTS parallel_scan_agents (
		id TEXT PRIMARY KEY,
		scan_id TEXT NOT NULL REFERENCES parallel_scans(id) ON DELETE CASCADE,
		name TEXT NOT NULL,
		conversation_id TEXT,
		status TEXT NOT NULL DEFAULT 'pending',
		current_round INTEGER DEFAULT 0,
		total_iterations INTEGER DEFAULT 0,
		total_tool_calls INTEGER DEFAULT 0,
		total_vulns INTEGER DEFAULT 0,
		errors INTEGER DEFAULT 0,
		last_activity DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	if _, err := db.Exec(createScansTable); err != nil {
		return fmt.Errorf("create parallel_scans table: %w", err)
	}
	if _, err := db.Exec(createAgentsTable); err != nil {
		return fmt.Errorf("create parallel_scan_agents table: %w", err)
	}
	return nil
}

// CreateParallelScan creates a new parallel scan session with agents
func (db *DB) CreateParallelScan(target string, maxRounds int, reconContext string, groupID string, agentNames []string) (*ParallelScanRow, []*ParallelScanAgentRow, error) {
	tx, err := db.Begin()
	if err != nil {
		return nil, nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	scanID := uuid.New().String()
	now := time.Now()

	_, err = tx.Exec(
		"INSERT INTO parallel_scans (id, target, status, max_rounds, recon_context, group_id, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		scanID, target, "pending", maxRounds, reconContext, groupID, now,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("insert scan: %w", err)
	}

	agents := make([]*ParallelScanAgentRow, 0, len(agentNames))
	for _, name := range agentNames {
		agentID := uuid.New().String()
		_, err = tx.Exec(
			"INSERT INTO parallel_scan_agents (id, scan_id, name, status, created_at) VALUES (?, ?, ?, ?, ?)",
			agentID, scanID, name, "pending", now,
		)
		if err != nil {
			return nil, nil, fmt.Errorf("insert agent %s: %w", name, err)
		}
		agents = append(agents, &ParallelScanAgentRow{
			ID:        agentID,
			ScanID:    scanID,
			Name:      name,
			Status:    "pending",
			CreatedAt: now,
		})
	}

	if err := tx.Commit(); err != nil {
		return nil, nil, fmt.Errorf("commit: %w", err)
	}

	scan := &ParallelScanRow{
		ID:        scanID,
		Target:    target,
		Status:    "pending",
		MaxRounds: maxRounds,
		CreatedAt: now,
	}
	if reconContext != "" {
		scan.ReconContext = sql.NullString{String: reconContext, Valid: true}
	}
	if groupID != "" {
		scan.GroupID = sql.NullString{String: groupID, Valid: true}
	}

	return scan, agents, nil
}

// GetParallelScan returns a scan by ID
func (db *DB) GetParallelScan(scanID string) (*ParallelScanRow, error) {
	var row ParallelScanRow
	err := db.QueryRow(
		"SELECT id, target, status, max_rounds, recon_context, group_id, created_at, started_at, completed_at FROM parallel_scans WHERE id = ?",
		scanID,
	).Scan(&row.ID, &row.Target, &row.Status, &row.MaxRounds, &row.ReconContext, &row.GroupID, &row.CreatedAt, &row.StartedAt, &row.CompletedAt)
	if err != nil {
		return nil, fmt.Errorf("get scan: %w", err)
	}
	return &row, nil
}

// GetParallelScanAgents returns all agents for a scan
func (db *DB) GetParallelScanAgents(scanID string) ([]*ParallelScanAgentRow, error) {
	rows, err := db.Query(
		"SELECT id, scan_id, name, conversation_id, status, current_round, total_iterations, total_tool_calls, total_vulns, errors, last_activity, created_at FROM parallel_scan_agents WHERE scan_id = ? ORDER BY created_at",
		scanID,
	)
	if err != nil {
		return nil, fmt.Errorf("query agents: %w", err)
	}
	defer rows.Close()

	var agents []*ParallelScanAgentRow
	for rows.Next() {
		var a ParallelScanAgentRow
		if err := rows.Scan(&a.ID, &a.ScanID, &a.Name, &a.ConversationID, &a.Status, &a.CurrentRound, &a.TotalIterations, &a.TotalToolCalls, &a.TotalVulns, &a.Errors, &a.LastActivity, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan agent row: %w", err)
		}
		agents = append(agents, &a)
	}
	return agents, nil
}

// ListParallelScans returns all scans ordered by creation time desc
func (db *DB) ListParallelScans() ([]*ParallelScanRow, error) {
	rows, err := db.Query(
		"SELECT id, target, status, max_rounds, recon_context, group_id, created_at, started_at, completed_at FROM parallel_scans ORDER BY created_at DESC",
	)
	if err != nil {
		return nil, fmt.Errorf("list scans: %w", err)
	}
	defer rows.Close()

	var scans []*ParallelScanRow
	for rows.Next() {
		var s ParallelScanRow
		if err := rows.Scan(&s.ID, &s.Target, &s.Status, &s.MaxRounds, &s.ReconContext, &s.GroupID, &s.CreatedAt, &s.StartedAt, &s.CompletedAt); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		scans = append(scans, &s)
	}
	return scans, nil
}

// UpdateParallelScanStatus updates scan status and timestamps
func (db *DB) UpdateParallelScanStatus(scanID, status string) error {
	now := time.Now()
	switch status {
	case "running":
		_, err := db.Exec("UPDATE parallel_scans SET status = ?, started_at = COALESCE(started_at, ?) WHERE id = ?", status, now, scanID)
		return err
	case "completed", "cancelled":
		_, err := db.Exec("UPDATE parallel_scans SET status = ?, completed_at = ? WHERE id = ?", status, now, scanID)
		return err
	default:
		_, err := db.Exec("UPDATE parallel_scans SET status = ? WHERE id = ?", status, scanID)
		return err
	}
}

// UpdateParallelScanAgent updates an agent's progress
func (db *DB) UpdateParallelScanAgent(agentID string, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}
	setClauses := make([]string, 0, len(updates))
	values := make([]interface{}, 0, len(updates))
	for k, v := range updates {
		setClauses = append(setClauses, k+" = ?")
		values = append(values, v)
	}
	values = append(values, agentID)
	query := fmt.Sprintf("UPDATE parallel_scan_agents SET %s WHERE id = ?", joinStrings(setClauses, ", "))
	_, err := db.Exec(query, values...)
	return err
}

// DeleteParallelScan deletes a scan and its agents
func (db *DB) DeleteParallelScan(scanID string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("DELETE FROM parallel_scan_agents WHERE scan_id = ?", scanID); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM parallel_scans WHERE id = ?", scanID); err != nil {
		return err
	}
	return tx.Commit()
}

func joinStrings(strs []string, sep string) string {
	result := ""
	for i, s := range strs {
		if i > 0 {
			result += sep
		}
		result += s
	}
	return result
}
