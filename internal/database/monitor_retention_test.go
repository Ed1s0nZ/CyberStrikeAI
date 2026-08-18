package database

import (
	"path/filepath"
	"testing"
	"time"

	"cyberstrike-ai/internal/mcp"

	"go.uber.org/zap"
)

func TestPurgeToolExecutionsBefore(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "monitor.db")
	db, err := NewDB(dbPath, zap.NewNop())
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer db.Close()

	oldStart := time.Now().AddDate(0, 0, -100)
	newStart := time.Now().AddDate(0, 0, -1)

	oldExec := &mcp.ToolExecution{
		ID:        "old-completed",
		ToolName:  "nmap::scan",
		Arguments: map[string]interface{}{"target": "127.0.0.1"},
		Status:    "completed",
		StartTime: oldStart,
	}
	oldFailed := &mcp.ToolExecution{
		ID:        "old-failed",
		ToolName:  "nmap::scan",
		Arguments: map[string]interface{}{"target": "127.0.0.1"},
		Status:    "failed",
		Error:     "timeout",
		StartTime: oldStart,
	}
	newExec := &mcp.ToolExecution{
		ID:        "new-completed",
		ToolName:  "nmap::scan",
		Arguments: map[string]interface{}{"target": "127.0.0.1"},
		Status:    "completed",
		StartTime: newStart,
	}
	for _, exec := range []*mcp.ToolExecution{oldExec, oldFailed, newExec} {
		if err := db.SaveToolExecution(exec); err != nil {
			t.Fatalf("SaveToolExecution(%s): %v", exec.ID, err)
		}
	}
	if err := db.UpdateToolStats("nmap::scan", 3, 2, 1, &newStart); err != nil {
		t.Fatalf("UpdateToolStats: %v", err)
	}

	cutoff := time.Now().AddDate(0, 0, -90)
	deleted, err := db.PurgeToolExecutionsBefore(cutoff)
	if err != nil {
		t.Fatalf("PurgeToolExecutionsBefore: %v", err)
	}
	if deleted != 2 {
		t.Fatalf("deleted = %d, want 2", deleted)
	}

	if _, err := db.GetToolExecution("old-completed"); err == nil {
		t.Fatal("old-completed should be deleted")
	}
	if _, err := db.GetToolExecution("old-failed"); err == nil {
		t.Fatal("old-failed should be deleted")
	}
	if _, err := db.GetToolExecution("new-completed"); err != nil {
		t.Fatalf("new-completed should remain: %v", err)
	}

	stats, err := db.LoadToolStats()
	if err != nil {
		t.Fatalf("LoadToolStats: %v", err)
	}
	stat := stats["nmap::scan"]
	if stat == nil {
		t.Fatal("expected stats for nmap::scan")
	}
	if stat.TotalCalls != 1 || stat.SuccessCalls != 1 || stat.FailedCalls != 0 {
		t.Fatalf("stats after purge = %+v, want total=1 success=1 failed=0", stat)
	}

	total, err := db.CountToolExecutions("", "")
	if err != nil {
		t.Fatalf("CountToolExecutions: %v", err)
	}
	if total != 1 {
		t.Fatalf("remaining executions = %d, want 1", total)
	}
}

func TestPurgeToolExecutionsBefore_zeroRetentionSkipsViaService(t *testing.T) {
	// RetentionDaysEffective: 0 means no purge at service layer; DB method still works when called directly.
	dbPath := filepath.Join(t.TempDir(), "monitor.db")
	db, err := NewDB(dbPath, zap.NewNop())
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer db.Close()

	exec := &mcp.ToolExecution{
		ID:        "ancient",
		ToolName:  "curl::get",
		Arguments: map[string]interface{}{},
		Status:    "completed",
		StartTime: time.Now().AddDate(-1, 0, 0),
	}
	if err := db.SaveToolExecution(exec); err != nil {
		t.Fatalf("SaveToolExecution: %v", err)
	}

	deleted, err := db.PurgeToolExecutionsBefore(time.Now())
	if err != nil {
		t.Fatalf("PurgeToolExecutionsBefore: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1", deleted)
	}
}
