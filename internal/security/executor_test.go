package security

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/mcp"
	"cyberstrike-ai/internal/storage"

	"go.uber.org/zap"
)

// setupTestExecutor creates an executor for testing
func setupTestExecutor(t *testing.T) (*Executor, *mcp.Server) {
	logger := zap.NewNop()
	mcpServer := mcp.NewServer(logger)

	cfg := &config.SecurityConfig{
		Tools: []config.ToolConfig{},
	}

	executor := NewExecutor(cfg, mcpServer, logger)
	return executor, mcpServer
}

// setupTestStorage creates a storage instance for testing
func setupTestStorage(t *testing.T) *storage.FileResultStorage {
	tmpDir := filepath.Join(os.TempDir(), "test_executor_storage_"+time.Now().Format("20060102_150405"))
	logger := zap.NewNop()

	storage, err := storage.NewFileResultStorage(tmpDir, logger)
	if err != nil {
		t.Fatalf("failed to create test storage: %v", err)
	}

	return storage
}

func TestExecutor_ExecuteInternalTool_QueryExecutionResult(t *testing.T) {
	executor, _ := setupTestExecutor(t)
	testStorage := setupTestStorage(t)
	executor.SetResultStorage(testStorage)

	// prepare test data
	executionID := "test_exec_001"
	toolName := "nmap_scan"
	result := "Line 1: Port 22 open\nLine 2: Port 80 open\nLine 3: Port 443 open\nLine 4: error occurred"

	// save test result
	err := testStorage.SaveResult(executionID, toolName, result)
	if err != nil {
		t.Fatalf("failed to save test result: %v", err)
	}

	ctx := context.Background()

	// test 1: basic query (first page)
	args := map[string]interface{}{
		"execution_id": executionID,
		"page":         float64(1),
		"limit":        float64(2),
	}

	toolResult, err := executor.executeQueryExecutionResult(ctx, args)
	if err != nil {
		t.Fatalf("failed to execute query: %v", err)
	}

	if toolResult.IsError {
		t.Fatalf("query should succeed but returned an error: %s", toolResult.Content[0].Text)
	}

	// verify result contains expected content
	resultText := toolResult.Content[0].Text
	if !strings.Contains(resultText, executionID) {
		t.Errorf("result should contain execution ID: %s", executionID)
	}

	if !strings.Contains(resultText, "Page 1/") {
		t.Errorf("result should contain pagination info")
	}

	// test 2: search functionality
	args2 := map[string]interface{}{
		"execution_id": executionID,
		"search":       "error",
		"page":         float64(1),
		"limit":        float64(10),
	}

	toolResult2, err := executor.executeQueryExecutionResult(ctx, args2)
	if err != nil {
		t.Fatalf("failed to execute search: %v", err)
	}

	if toolResult2.IsError {
		t.Fatalf("search should succeed but returned an error: %s", toolResult2.Content[0].Text)
	}

	resultText2 := toolResult2.Content[0].Text
	if !strings.Contains(resultText2, "error") {
		t.Errorf("search result should contain keyword: error")
	}

	// test 3: filter functionality
	args3 := map[string]interface{}{
		"execution_id": executionID,
		"filter":       "Port",
		"page":         float64(1),
		"limit":        float64(10),
	}

	toolResult3, err := executor.executeQueryExecutionResult(ctx, args3)
	if err != nil {
		t.Fatalf("failed to execute filter: %v", err)
	}

	if toolResult3.IsError {
		t.Fatalf("filter should succeed but returned an error: %s", toolResult3.Content[0].Text)
	}

	resultText3 := toolResult3.Content[0].Text
	if !strings.Contains(resultText3, "Port") {
		t.Errorf("filter result should contain keyword: Port")
	}

	// test 4: missing required parameter
	args4 := map[string]interface{}{
		"page": float64(1),
	}

	toolResult4, err := executor.executeQueryExecutionResult(ctx, args4)
	if err != nil {
		t.Fatalf("failed to execute query: %v", err)
	}

	if !toolResult4.IsError {
		t.Fatal("missing execution_id should return an error")
	}

	// test 5: non-existent execution ID
	args5 := map[string]interface{}{
		"execution_id": "nonexistent_id",
		"page":         float64(1),
	}

	toolResult5, err := executor.executeQueryExecutionResult(ctx, args5)
	if err != nil {
		t.Fatalf("failed to execute query: %v", err)
	}

	if !toolResult5.IsError {
		t.Fatal("non-existent execution ID should return an error")
	}
}

func TestExecutor_ExecuteInternalTool_UnknownTool(t *testing.T) {
	executor, _ := setupTestExecutor(t)

	ctx := context.Background()
	args := map[string]interface{}{
		"test": "value",
	}

	// test unknown internal tool type
	toolResult, err := executor.executeInternalTool(ctx, "unknown_tool", "internal:unknown_tool", args)
	if err != nil {
		t.Fatalf("failed to execute internal tool: %v", err)
	}

	if !toolResult.IsError {
		t.Fatal("unknown tool type should return an error")
	}

	if !strings.Contains(toolResult.Content[0].Text, "unknown internal tool type") {
		t.Errorf("error message should contain 'unknown internal tool type'")
	}
}

func TestExecutor_ExecuteInternalTool_NoStorage(t *testing.T) {
	executor, _ := setupTestExecutor(t)
	// do not set storage, test uninitialized case

	ctx := context.Background()
	args := map[string]interface{}{
		"execution_id": "test_id",
	}

	toolResult, err := executor.executeQueryExecutionResult(ctx, args)
	if err != nil {
		t.Fatalf("failed to execute query: %v", err)
	}

	if !toolResult.IsError {
		t.Fatal("uninitialized storage should return an error")
	}

	if !strings.Contains(toolResult.Content[0].Text, "result storage not initialized") {
		t.Errorf("error message should contain 'result storage not initialized'")
	}
}

func TestPaginateLines(t *testing.T) {
	lines := []string{"Line 1", "Line 2", "Line 3", "Line 4", "Line 5"}

	// test first page
	page := paginateLines(lines, 1, 2)
	if page.Page != 1 {
		t.Errorf("page number mismatch. expected: 1, actual: %d", page.Page)
	}
	if page.Limit != 2 {
		t.Errorf("lines per page mismatch. expected: 2, actual: %d", page.Limit)
	}
	if page.TotalLines != 5 {
		t.Errorf("total lines mismatch. expected: 5, actual: %d", page.TotalLines)
	}
	if page.TotalPages != 3 {
		t.Errorf("total pages mismatch. expected: 3, actual: %d", page.TotalPages)
	}
	if len(page.Lines) != 2 {
		t.Errorf("first page line count mismatch. expected: 2, actual: %d", len(page.Lines))
	}

	// test second page
	page2 := paginateLines(lines, 2, 2)
	if len(page2.Lines) != 2 {
		t.Errorf("second page line count mismatch. expected: 2, actual: %d", len(page2.Lines))
	}
	if page2.Lines[0] != "Line 3" {
		t.Errorf("second page first line mismatch. expected: Line 3, actual: %s", page2.Lines[0])
	}

	// test last page
	page3 := paginateLines(lines, 3, 2)
	if len(page3.Lines) != 1 {
		t.Errorf("third page line count mismatch. expected: 1, actual: %d", len(page3.Lines))
	}

	// test out-of-range page number (should return last page)
	page4 := paginateLines(lines, 4, 2)
	if page4.Page != 3 {
		t.Errorf("out-of-range page number should be corrected to last page. expected: 3, actual: %d", page4.Page)
	}
	if len(page4.Lines) != 1 {
		t.Errorf("last page should have only 1 line. actual: %d lines", len(page4.Lines))
	}

	// test invalid page number (less than 1)
	page0 := paginateLines(lines, 0, 2)
	if page0.Page != 1 {
		t.Errorf("invalid page number should be corrected to 1. actual: %d", page0.Page)
	}

	// test empty list
	emptyPage := paginateLines([]string{}, 1, 10)
	if emptyPage.TotalLines != 0 {
		t.Errorf("total lines for empty list should be 0. actual: %d", emptyPage.TotalLines)
	}
	if len(emptyPage.Lines) != 0 {
		t.Errorf("empty list should return empty result. actual: %d lines", len(emptyPage.Lines))
	}
}
