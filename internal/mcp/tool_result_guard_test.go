package mcp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
)

type inMemoryMonitorStorage struct {
	executions map[string]*ToolExecution
}

func newInMemoryMonitorStorage() *inMemoryMonitorStorage {
	return &inMemoryMonitorStorage{executions: map[string]*ToolExecution{}}
}

func (s *inMemoryMonitorStorage) SaveToolExecution(exec *ToolExecution) error {
	if exec != nil {
		s.executions[exec.ID] = cloneToolExecution(exec)
	}
	return nil
}

func (s *inMemoryMonitorStorage) UpdateToolExecutionResult(id string, result *ToolResult) error {
	exec := s.executions[id]
	if exec == nil {
		exec = &ToolExecution{ID: id}
		s.executions[id] = exec
	}
	exec.Result = cloneToolResult(result)
	return nil
}

func (s *inMemoryMonitorStorage) LoadToolExecutions() ([]*ToolExecution, error) {
	out := make([]*ToolExecution, 0, len(s.executions))
	for _, exec := range s.executions {
		out = append(out, cloneToolExecution(exec))
	}
	return out, nil
}

func (s *inMemoryMonitorStorage) GetToolExecution(id string) (*ToolExecution, error) {
	if exec := s.executions[id]; exec != nil {
		return cloneToolExecution(exec), nil
	}
	return nil, nil
}

func (s *inMemoryMonitorStorage) SaveToolStats(string, *ToolStats) error { return nil }

func (s *inMemoryMonitorStorage) LoadToolStats() (map[string]*ToolStats, error) {
	return map[string]*ToolStats{}, nil
}

func (s *inMemoryMonitorStorage) UpdateToolStats(string, int, int, int, *time.Time) error {
	return nil
}

func TestServerCallToolStoresAndReturnsSameGuardedResult(t *testing.T) {
	storage := newInMemoryMonitorStorage()
	server := NewServerWithStorage(zap.NewNop(), storage)
	server.ConfigureToolWaitTimeoutSeconds(0)
	server.ConfigureToolResultMaxBytes(400)
	spillRoot := t.TempDir()
	server.ConfigureToolResultSpillRoot(spillRoot)
	server.RegisterTool(Tool{Name: "big", InputSchema: map[string]interface{}{"type": "object"}}, func(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
		return &ToolResult{Content: []Content{{Type: "text", Text: strings.Repeat("x", 800)}}}, nil
	})

	ctx := WithMCPConversationID(context.Background(), "conv-spill")
	result, executionID, err := server.CallTool(ctx, "big", nil)
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if executionID == "" {
		t.Fatal("missing execution id")
	}
	returned := ToolResultPlainText(result)
	if !strings.Contains(returned, "<persisted-output>") || !strings.Contains(returned, "Full output saved to:") {
		t.Fatalf("returned result was not spilled: %q", returned)
	}
	if len(returned) > 400 {
		t.Fatalf("returned result exceeded hard limit: len=%d text=%q", len(returned), returned)
	}

	spillPath := filepath.Join(spillRoot, "conversations", "conv-spill", "trunc", executionID)
	abs, err := filepath.Abs(spillPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(returned, abs) {
		t.Fatalf("missing spill path %q in %q", abs, returned)
	}
	body, err := os.ReadFile(abs)
	if err != nil {
		t.Fatalf("read spill file: %v", err)
	}
	if string(body) != strings.Repeat("x", 800) {
		t.Fatalf("spill body mismatch: len=%d", len(body))
	}

	inMem, ok := server.GetExecution(executionID)
	if !ok || inMem == nil || inMem.Result == nil {
		t.Fatalf("missing in-memory execution: %#v", inMem)
	}
	stored := storage.executions[executionID]
	if stored == nil || stored.Result == nil {
		t.Fatalf("missing stored execution: %#v", stored)
	}
	if ToolResultPlainText(inMem.Result) != returned {
		t.Fatalf("in-memory result != returned\nmem=%q\nret=%q", ToolResultPlainText(inMem.Result), returned)
	}
	if ToolResultPlainText(stored.Result) != returned {
		t.Fatalf("stored result != returned\nstored=%q\nret=%q", ToolResultPlainText(stored.Result), returned)
	}
}

func TestExecutionServiceStoresGuardedResult(t *testing.T) {
	service := NewExecutionService(nil, zap.NewNop())
	service.ConfigureToolResultMaxBytes(400)
	spillRoot := t.TempDir()
	service.ConfigureToolResultSpillRoot(spillRoot)
	handle, err := service.Submit(context.Background(), ExecutionRequest{
		ToolName:       "big",
		ConversationID: "svc-conv",
		Run: func(context.Context) (*ToolResult, error) {
			return &ToolResult{Content: []Content{{Type: "text", Text: strings.Repeat("a", 800)}}}, nil
		},
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	snap, err := service.Wait(context.Background(), handle.ID, time.Second)
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	got := ToolResultPlainText(snap.Execution.Result)
	if !strings.Contains(got, "<persisted-output>") {
		t.Fatalf("service result was not spilled: %q", got)
	}
	if len(got) > 400 {
		t.Fatalf("service result exceeded hard limit: len=%d text=%q", len(got), got)
	}
	path := filepath.Join(spillRoot, "conversations", "svc-conv", "trunc", handle.ID)
	abs, _ := filepath.Abs(path)
	body, err := os.ReadFile(abs)
	if err != nil {
		t.Fatalf("read spill: %v", err)
	}
	if string(body) != strings.Repeat("a", 800) {
		t.Fatalf("unexpected spill body len=%d", len(body))
	}
}
