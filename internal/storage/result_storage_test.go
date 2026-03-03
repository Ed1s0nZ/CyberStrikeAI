package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
)

// setupTestStorage creates a storage instance for testing
func setupTestStorage(t *testing.T) (*FileResultStorage, string) {
	tmpDir := filepath.Join(os.TempDir(), "test_result_storage_"+time.Now().Format("20060102_150405"))
	logger := zap.NewNop()

	storage, err := NewFileResultStorage(tmpDir, logger)
	if err != nil {
		t.Fatalf("failed to create test storage: %v", err)
	}

	return storage, tmpDir
}

// cleanupTestStorage cleans up test data
func cleanupTestStorage(t *testing.T, tmpDir string) {
	if err := os.RemoveAll(tmpDir); err != nil {
		t.Logf("failed to clean up test directory: %v", err)
	}
}

func TestNewFileResultStorage(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "test_new_storage_"+time.Now().Format("20060102_150405"))
	defer cleanupTestStorage(t, tmpDir)

	logger := zap.NewNop()
	storage, err := NewFileResultStorage(tmpDir, logger)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	if storage == nil {
		t.Fatal("storage instance is nil")
	}

	// verify directory was created
	if _, err := os.Stat(tmpDir); os.IsNotExist(err) {
		t.Fatal("storage directory was not created")
	}
}

func TestFileResultStorage_SaveResult(t *testing.T) {
	storage, tmpDir := setupTestStorage(t)
	defer cleanupTestStorage(t, tmpDir)

	executionID := "test_exec_001"
	toolName := "nmap_scan"
	result := "Line 1\nLine 2\nLine 3\nLine 4\nLine 5"

	err := storage.SaveResult(executionID, toolName, result)
	if err != nil {
		t.Fatalf("failed to save result: %v", err)
	}

	// verify result file exists
	resultPath := filepath.Join(tmpDir, executionID+".txt")
	if _, err := os.Stat(resultPath); os.IsNotExist(err) {
		t.Fatal("result file was not created")
	}

	// verify metadata file exists
	metadataPath := filepath.Join(tmpDir, executionID+".meta.json")
	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		t.Fatal("metadata file was not created")
	}
}

func TestFileResultStorage_GetResult(t *testing.T) {
	storage, tmpDir := setupTestStorage(t)
	defer cleanupTestStorage(t, tmpDir)

	executionID := "test_exec_002"
	toolName := "test_tool"
	expectedResult := "Test result content\nLine 2\nLine 3"

	// save result first
	err := storage.SaveResult(executionID, toolName, expectedResult)
	if err != nil {
		t.Fatalf("failed to save result: %v", err)
	}

	// get result
	result, err := storage.GetResult(executionID)
	if err != nil {
		t.Fatalf("failed to get result: %v", err)
	}

	if result != expectedResult {
		t.Errorf("result mismatch. expected: %q, actual: %q", expectedResult, result)
	}

	// test non-existent execution ID
	_, err = storage.GetResult("nonexistent_id")
	if err == nil {
		t.Fatal("should have returned an error")
	}
}

func TestFileResultStorage_GetResultMetadata(t *testing.T) {
	storage, tmpDir := setupTestStorage(t)
	defer cleanupTestStorage(t, tmpDir)

	executionID := "test_exec_003"
	toolName := "test_tool"
	result := "Line 1\nLine 2\nLine 3"

	// save result
	err := storage.SaveResult(executionID, toolName, result)
	if err != nil {
		t.Fatalf("failed to save result: %v", err)
	}

	// get metadata
	metadata, err := storage.GetResultMetadata(executionID)
	if err != nil {
		t.Fatalf("failed to get metadata: %v", err)
	}

	if metadata.ExecutionID != executionID {
		t.Errorf("execution ID mismatch. expected: %s, actual: %s", executionID, metadata.ExecutionID)
	}

	if metadata.ToolName != toolName {
		t.Errorf("tool name mismatch. expected: %s, actual: %s", toolName, metadata.ToolName)
	}

	if metadata.TotalSize != len(result) {
		t.Errorf("total size mismatch. expected: %d, actual: %d", len(result), metadata.TotalSize)
	}

	expectedLines := len(strings.Split(result, "\n"))
	if metadata.TotalLines != expectedLines {
		t.Errorf("total lines mismatch. expected: %d, actual: %d", expectedLines, metadata.TotalLines)
	}

	// verify created time is within a reasonable range
	now := time.Now()
	if metadata.CreatedAt.After(now) || metadata.CreatedAt.Before(now.Add(-time.Second)) {
		t.Errorf("created time is not within a reasonable range: %v", metadata.CreatedAt)
	}
}

func TestFileResultStorage_GetResultPage(t *testing.T) {
	storage, tmpDir := setupTestStorage(t)
	defer cleanupTestStorage(t, tmpDir)

	executionID := "test_exec_004"
	toolName := "test_tool"
	// create a result with 10 lines
	lines := make([]string, 10)
	for i := 0; i < 10; i++ {
		lines[i] = fmt.Sprintf("Line %d", i+1)
	}
	result := strings.Join(lines, "\n")

	// save result
	err := storage.SaveResult(executionID, toolName, result)
	if err != nil {
		t.Fatalf("failed to save result: %v", err)
	}

	// test first page (3 lines per page)
	page, err := storage.GetResultPage(executionID, 1, 3)
	if err != nil {
		t.Fatalf("failed to get first page: %v", err)
	}

	if page.Page != 1 {
		t.Errorf("page number mismatch. expected: 1, actual: %d", page.Page)
	}

	if page.Limit != 3 {
		t.Errorf("lines per page mismatch. expected: 3, actual: %d", page.Limit)
	}

	if page.TotalLines != 10 {
		t.Errorf("total lines mismatch. expected: 10, actual: %d", page.TotalLines)
	}

	if page.TotalPages != 4 {
		t.Errorf("total pages mismatch. expected: 4, actual: %d", page.TotalPages)
	}

	if len(page.Lines) != 3 {
		t.Errorf("first page line count mismatch. expected: 3, actual: %d", len(page.Lines))
	}

	if page.Lines[0] != "Line 1" {
		t.Errorf("first line content mismatch. expected: Line 1, actual: %s", page.Lines[0])
	}

	// test second page
	page2, err := storage.GetResultPage(executionID, 2, 3)
	if err != nil {
		t.Fatalf("failed to get second page: %v", err)
	}

	if len(page2.Lines) != 3 {
		t.Errorf("second page line count mismatch. expected: 3, actual: %d", len(page2.Lines))
	}

	if page2.Lines[0] != "Line 4" {
		t.Errorf("first line of second page mismatch. expected: Line 4, actual: %s", page2.Lines[0])
	}

	// test last page (may not be full)
	page4, err := storage.GetResultPage(executionID, 4, 3)
	if err != nil {
		t.Fatalf("failed to get fourth page: %v", err)
	}

	if len(page4.Lines) != 1 {
		t.Errorf("fourth page line count mismatch. expected: 1, actual: %d", len(page4.Lines))
	}

	// test out-of-range page number (should return last page)
	page5, err := storage.GetResultPage(executionID, 5, 3)
	if err != nil {
		t.Fatalf("failed to get fifth page: %v", err)
	}

	// out-of-range page number should be corrected to last page, so should return last page content
	if page5.Page != 4 {
		t.Errorf("out-of-range page number should be corrected to last page. expected: 4, actual: %d", page5.Page)
	}

	// last page should have only 1 line
	if len(page5.Lines) != 1 {
		t.Errorf("last page should have only 1 line. actual: %d lines", len(page5.Lines))
	}
}

func TestFileResultStorage_SearchResult(t *testing.T) {
	storage, tmpDir := setupTestStorage(t)
	defer cleanupTestStorage(t, tmpDir)

	executionID := "test_exec_005"
	toolName := "test_tool"
	result := "Line 1: error occurred\nLine 2: success\nLine 3: error again\nLine 4: ok"

	// save result
	err := storage.SaveResult(executionID, toolName, result)
	if err != nil {
		t.Fatalf("failed to save result: %v", err)
	}

	// search for lines containing "error" (simple string matching)
	matchedLines, err := storage.SearchResult(executionID, "error", false)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if len(matchedLines) != 2 {
		t.Errorf("search result count mismatch. expected: 2, actual: %d", len(matchedLines))
	}

	// verify search result content
	for i, line := range matchedLines {
		if !strings.Contains(line, "error") {
			t.Errorf("search result line %d does not contain keyword: %s", i+1, line)
		}
	}

	// test searching for a non-existent keyword
	noMatch, err := storage.SearchResult(executionID, "nonexistent", false)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if len(noMatch) != 0 {
		t.Errorf("searching for non-existent keyword should return empty result. actual: %d lines", len(noMatch))
	}

	// test regex search
	regexMatched, err := storage.SearchResult(executionID, "error.*again", true)
	if err != nil {
		t.Fatalf("regex search failed: %v", err)
	}

	if len(regexMatched) != 1 {
		t.Errorf("regex search result count mismatch. expected: 1, actual: %d", len(regexMatched))
	}
}

func TestFileResultStorage_FilterResult(t *testing.T) {
	storage, tmpDir := setupTestStorage(t)
	defer cleanupTestStorage(t, tmpDir)

	executionID := "test_exec_006"
	toolName := "test_tool"
	result := "Line 1: warning message\nLine 2: info message\nLine 3: warning again\nLine 4: debug message"

	// save result
	err := storage.SaveResult(executionID, toolName, result)
	if err != nil {
		t.Fatalf("failed to save result: %v", err)
	}

	// filter lines containing "warning" (simple string matching)
	filteredLines, err := storage.FilterResult(executionID, "warning", false)
	if err != nil {
		t.Fatalf("filter failed: %v", err)
	}

	if len(filteredLines) != 2 {
		t.Errorf("filter result count mismatch. expected: 2, actual: %d", len(filteredLines))
	}

	// verify filter result content
	for i, line := range filteredLines {
		if !strings.Contains(line, "warning") {
			t.Errorf("filter result line %d does not contain keyword: %s", i+1, line)
		}
	}
}

func TestFileResultStorage_DeleteResult(t *testing.T) {
	storage, tmpDir := setupTestStorage(t)
	defer cleanupTestStorage(t, tmpDir)

	executionID := "test_exec_007"
	toolName := "test_tool"
	result := "Test result"

	// save result
	err := storage.SaveResult(executionID, toolName, result)
	if err != nil {
		t.Fatalf("failed to save result: %v", err)
	}

	// verify files exist
	resultPath := filepath.Join(tmpDir, executionID+".txt")
	metadataPath := filepath.Join(tmpDir, executionID+".meta.json")

	if _, err := os.Stat(resultPath); os.IsNotExist(err) {
		t.Fatal("result file does not exist")
	}

	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		t.Fatal("metadata file does not exist")
	}

	// delete result
	err = storage.DeleteResult(executionID)
	if err != nil {
		t.Fatalf("failed to delete result: %v", err)
	}

	// verify files were deleted
	if _, err := os.Stat(resultPath); !os.IsNotExist(err) {
		t.Fatal("result file was not deleted")
	}

	if _, err := os.Stat(metadataPath); !os.IsNotExist(err) {
		t.Fatal("metadata file was not deleted")
	}

	// test deleting a non-existent execution ID (should not return an error)
	err = storage.DeleteResult("nonexistent_id")
	if err != nil {
		t.Errorf("deleting a non-existent execution ID should not return an error: %v", err)
	}
}

func TestFileResultStorage_ConcurrentAccess(t *testing.T) {
	storage, tmpDir := setupTestStorage(t)
	defer cleanupTestStorage(t, tmpDir)

	// concurrently save multiple results
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			executionID := fmt.Sprintf("test_exec_%d", id)
			toolName := "test_tool"
			result := fmt.Sprintf("Result %d\nLine 2\nLine 3", id)

			err := storage.SaveResult(executionID, toolName, result)
			if err != nil {
				t.Errorf("concurrent save failed (ID: %s): %v", executionID, err)
			}

			// concurrent read
			_, err = storage.GetResult(executionID)
			if err != nil {
				t.Errorf("concurrent read failed (ID: %s): %v", executionID, err)
			}

			done <- true
		}(i)
	}

	// wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestFileResultStorage_LargeResult(t *testing.T) {
	storage, tmpDir := setupTestStorage(t)
	defer cleanupTestStorage(t, tmpDir)

	executionID := "test_exec_large"
	toolName := "test_tool"

	// create a large result (1000 lines)
	lines := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		lines[i] = fmt.Sprintf("Line %d: This is a test line with some content", i+1)
	}
	result := strings.Join(lines, "\n")

	// save large result
	err := storage.SaveResult(executionID, toolName, result)
	if err != nil {
		t.Fatalf("failed to save large result: %v", err)
	}

	// verify metadata
	metadata, err := storage.GetResultMetadata(executionID)
	if err != nil {
		t.Fatalf("failed to get metadata: %v", err)
	}

	if metadata.TotalLines != 1000 {
		t.Errorf("total lines mismatch. expected: 1000, actual: %d", metadata.TotalLines)
	}

	// test paginated query of large result
	page, err := storage.GetResultPage(executionID, 1, 100)
	if err != nil {
		t.Fatalf("failed to get first page: %v", err)
	}

	if page.TotalPages != 10 {
		t.Errorf("total pages mismatch. expected: 10, actual: %d", page.TotalPages)
	}

	if len(page.Lines) != 100 {
		t.Errorf("first page line count mismatch. expected: 100, actual: %d", len(page.Lines))
	}
}
