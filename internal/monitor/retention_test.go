package monitor

import (
	"path/filepath"
	"testing"
	"time"

	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/database"
	"cyberstrike-ai/internal/mcp"

	"go.uber.org/zap"
)

func TestServicePurgeExpired_respectsZeroRetention(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "monitor.db")
	db, err := database.NewDB(dbPath, zap.NewNop())
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer db.Close()

	exec := &mcp.ToolExecution{
		ID:        "ancient",
		ToolName:  "curl::get",
		Arguments: map[string]interface{}{},
		Status:    "completed",
		StartTime: mustParseTime(t, "2020-01-01T00:00:00Z"),
	}
	if err := db.SaveToolExecution(exec); err != nil {
		t.Fatalf("SaveToolExecution: %v", err)
	}

	zero := 0
	svc := NewService(db, &config.Config{
		Monitor: config.MonitorConfig{RetentionDays: &zero},
	}, zap.NewNop())
	svc.PurgeExpired()

	if _, err := db.GetToolExecution("ancient"); err != nil {
		t.Fatalf("record should remain when retention_days=0: %v", err)
	}
}

func TestServicePurgeExpired_deletesOldRows(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "monitor.db")
	db, err := database.NewDB(dbPath, zap.NewNop())
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer db.Close()

	exec := &mcp.ToolExecution{
		ID:        "ancient",
		ToolName:  "curl::get",
		Arguments: map[string]interface{}{},
		Status:    "completed",
		StartTime: mustParseTime(t, "2020-01-01T00:00:00Z"),
	}
	if err := db.SaveToolExecution(exec); err != nil {
		t.Fatalf("SaveToolExecution: %v", err)
	}

	days := 90
	svc := NewService(db, &config.Config{
		Monitor: config.MonitorConfig{RetentionDays: &days},
	}, zap.NewNop())
	svc.PurgeExpired()

	if _, err := db.GetToolExecution("ancient"); err == nil {
		t.Fatal("record should be purged when older than retention_days")
	}
}

func TestRetentionDaysEffective_defaults(t *testing.T) {
	got := config.MonitorConfig{}.RetentionDaysEffective()
	if got != 90 {
		t.Fatalf("default = %d, want 90", got)
	}
	zero := 0
	cfg := config.MonitorConfig{RetentionDays: &zero}
	if cfg.RetentionDaysEffective() != 0 {
		t.Fatalf("zero = %d, want 0", cfg.RetentionDaysEffective())
	}
}

func mustParseTime(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatalf("parse time: %v", err)
	}
	return parsed
}
