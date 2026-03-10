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

func TestExecutor_FileTool_UsesResolvedWorkdir(t *testing.T) {
	logger := zap.NewNop()
	mcpServer := mcp.NewServer(logger)
	cfg := &config.SecurityConfig{
		Tools: []config.ToolConfig{
			{
				Name:    "create-file",
				Command: "python3",
				Args: []string{
					"-c",
					`import sys; from pathlib import Path; p = Path(sys.argv[1]); p.parent.mkdir(parents=True, exist_ok=True); p.write_text(sys.argv[2], encoding="utf-8"); print(p)`,
				},
				Enabled: true,
				Parameters: []config.ParameterConfig{
					{Name: "filename", Type: "string", Required: true, Position: intPtr(0), Format: "positional"},
					{Name: "content", Type: "string", Required: true, Position: intPtr(1), Format: "positional"},
				},
			},
		},
	}

	executor := NewExecutor(cfg, mcpServer, logger)
	baseDir := t.TempDir()
	executor.defaultWorkDir = baseDir
	executor.buildToolIndex()

	ctx := context.Background()
	res, err := executor.ExecuteTool(ctx, "create-file", map[string]interface{}{
		"filename": "a/b.txt",
		"content":  "ok",
	})
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned error: %s", res.Content[0].Text)
	}

	outPath := filepath.Join(baseDir, "a", "b.txt")
	data, readErr := os.ReadFile(outPath)
	if readErr != nil {
		t.Fatalf("expected file at %s, got error: %v", outPath, readErr)
	}
	if string(data) != "ok" {
		t.Fatalf("unexpected content: %q", string(data))
	}
}

func intPtr(v int) *int { return &v }

func TestSanitizeFierceArgs_RemovesOrphanListFlags(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
		changed  bool
	}{
		{
			name:     "orphan subdomains removed",
			input:    []string{"--domain", "example.com", "--subdomains"},
			expected: []string{"--domain", "example.com"},
			changed:  true,
		},
		{
			name:     "orphan dns-servers removed",
			input:    []string{"--domain", "example.com", "--dns-servers", "--tcp"},
			expected: []string{"--domain", "example.com", "--tcp"},
			changed:  true,
		},
		{
			name:     "valid list flags kept",
			input:    []string{"--domain", "example.com", "--subdomains", "www", "--dns-servers", "8.8.8.8"},
			expected: []string{"--domain", "example.com", "--subdomains", "www", "--dns-servers", "8.8.8.8"},
			changed:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, changed := sanitizeFierceArgs(tt.input)
			if changed != tt.changed {
				t.Fatalf("changed mismatch. expected=%v got=%v", tt.changed, changed)
			}
			if len(got) != len(tt.expected) {
				t.Fatalf("length mismatch. expected=%d got=%d args=%v", len(tt.expected), len(got), got)
			}
			for i := range tt.expected {
				if got[i] != tt.expected[i] {
					t.Fatalf("arg[%d] mismatch. expected=%q got=%q", i, tt.expected[i], got[i])
				}
			}
		})
	}
}

func TestBuildCommandArgs_NmapEmptyPortsDoesNotEmitBarePortFlag(t *testing.T) {
	executor, _ := setupTestExecutor(t)

	targetPos := 0
	toolConfig := &config.ToolConfig{
		Name:    "nmap",
		Command: "nmap",
		Args:    []string{"-sT", "-sV", "-sC"},
		Parameters: []config.ParameterConfig{
			{
				Name:     "target",
				Type:     "string",
				Required: true,
				Position: &targetPos,
				Format:   "positional",
			},
			{
				Name:     "ports",
				Type:     "string",
				Required: false,
				Flag:     "-p",
				Format:   "flag",
			},
		},
	}

	args := map[string]interface{}{
		"target": "127.0.0.1",
		"ports":  "",
	}

	cmdArgs := executor.buildCommandArgs("nmap", toolConfig, args)
	for i := 0; i < len(cmdArgs); i++ {
		if cmdArgs[i] == "-p" {
			t.Fatalf("unexpected bare -p flag in command args: %#v", cmdArgs)
		}
	}
	if len(cmdArgs) == 0 {
		t.Fatal("command args should not be empty")
	}
}

func TestBuildCommandArgs_NmapScanTypeDoesNotBreakMinRatePair(t *testing.T) {
	executor, _ := setupTestExecutor(t)

	targetPos := 0
	toolConfig := &config.ToolConfig{
		Name:    "nmap",
		Command: "nmap",
		Args:    []string{"-sT", "-sV", "-sC"},
		Parameters: []config.ParameterConfig{
			{
				Name:     "target",
				Type:     "string",
				Required: true,
				Position: &targetPos,
				Format:   "positional",
			},
			{
				Name:     "ports",
				Type:     "string",
				Required: false,
				Flag:     "-p",
				Format:   "flag",
			},
			{
				Name:     "scan_type",
				Type:     "string",
				Required: false,
				Format:   "template",
				Template: "{value}",
			},
			{
				Name:     "additional_args",
				Type:     "string",
				Required: false,
				Format:   "positional",
			},
		},
	}

	args := map[string]interface{}{
		"target":          "example.com",
		"ports":           "1-1000",
		"scan_type":       "-sV -sC",
		"additional_args": "-T4 --min-rate 500",
	}

	cmdArgs := executor.buildCommandArgs("nmap", toolConfig, args)

	for i := 0; i < len(cmdArgs)-1; i++ {
		if cmdArgs[i] == "--min-rate" && cmdArgs[i+1] != "500" {
			t.Fatalf("--min-rate value pairing broken: %#v", cmdArgs)
		}
	}
}

func TestSanitizeNmapArgs_NonRoot(t *testing.T) {
	input := []string{"-sS", "-O", "example.com", "-p", "80,443", "-A"}
	got, changed := sanitizeNmapArgs(input, false)

	if !changed {
		t.Fatal("expected nmap args to be sanitized for non-root")
	}

	joined := strings.Join(got, " ")
	if strings.Contains(joined, "-sS") || strings.Contains(joined, " -O") || strings.Contains(joined, " -A") {
		t.Fatalf("sanitized args still contain privileged flags: %#v", got)
	}
	if !strings.Contains(joined, "-sT") {
		t.Fatalf("expected -sT fallback in sanitized args: %#v", got)
	}
}

func TestBuildCommandArgs_NmapPortsAndScanTypeOrder(t *testing.T) {
	executor, _ := setupTestExecutor(t)

	targetPos := 0
	toolConfig := &config.ToolConfig{
		Name:    "nmap",
		Command: "nmap",
		Args:    []string{"-sT", "-sV", "-sC"},
		Parameters: []config.ParameterConfig{
			{
				Name:     "target",
				Type:     "string",
				Required: true,
				Position: &targetPos,
				Format:   "positional",
			},
			{
				Name:   "ports",
				Type:   "string",
				Flag:   "-p",
				Format: "flag",
			},
			{
				Name:     "scan_type",
				Type:     "string",
				Format:   "template",
				Template: "{value}",
			},
			{
				Name:   "additional_args",
				Type:   "string",
				Format: "positional",
			},
		},
	}

	args := map[string]interface{}{
		"target":          "104.21.64.164",
		"ports":           "1-10000",
		"scan_type":       "-sV -sC",
		"additional_args": "-T4 -A",
	}

	cmdArgs := executor.buildCommandArgs("nmap", toolConfig, args)
	for i := 0; i < len(cmdArgs)-1; i++ {
		if cmdArgs[i] == "-p" && strings.HasPrefix(cmdArgs[i+1], "-") {
			t.Fatalf("invalid -p argument ordering: %#v", cmdArgs)
		}
	}
}

func TestSanitizeFeroxbusterArgs_DeduplicateThreads(t *testing.T) {
	input := []string{
		"-u", "https://example.com",
		"-t", "10",
		"--threads", "50",
		"-t", "30",
		"--auto-bail",
	}

	got, changed := sanitizeFeroxbusterArgs(input)
	if !changed {
		t.Fatal("expected feroxbuster args to be sanitized")
	}

	threadsFlags := 0
	for _, arg := range got {
		if arg == "-t" || arg == "--threads" || strings.HasPrefix(arg, "--threads=") {
			threadsFlags++
		}
	}
	if threadsFlags != 1 {
		t.Fatalf("expected exactly one thread flag, got %d: %#v", threadsFlags, got)
	}
	if !strings.Contains(strings.Join(got, " "), "-t 30") {
		t.Fatalf("expected to keep last thread setting (-t 30), got: %#v", got)
	}
}

func TestSanitizeFeroxbusterArgs_NoChangeOnSingleThreadFlag(t *testing.T) {
	input := []string{"-u", "https://example.com", "-t", "20", "--auto-bail"}
	got, changed := sanitizeFeroxbusterArgs(input)

	if changed {
		t.Fatalf("did not expect sanitization for single thread flag, got: %#v", got)
	}
	if strings.Join(input, " ") != strings.Join(got, " ") {
		t.Fatalf("expected args unchanged, input=%#v got=%#v", input, got)
	}
}

func TestExecutor_NormalizeToolArgs_RepairsMalformedHTTPFrameworkArgs(t *testing.T) {
	executor, _ := setupTestExecutor(t)
	toolConfig := &config.ToolConfig{
		Name: "http-framework-test",
		Parameters: []config.ParameterConfig{
			{Name: "url", Type: "string", Required: true},
			{Name: "method", Type: "string"},
			{Name: "action", Type: "string"},
			{Name: "data", Type: "string"},
			{Name: "delay", Type: "string"},
			{Name: "allow_insecure", Type: "bool"},
			{Name: "auto_encode_url", Type: "bool"},
		},
	}

	args := map[string]interface{}{
		"action": "get\n</parameter=\n<parameter=allow_insecure=true\n<parameter=auto_encode_url=true\n</parameter=\n<parameter=data>\n<parameter=data>\nhttps://stormapi.su",
		"delay":  "2",
	}

	normalized := executor.normalizeToolArgs("http-framework-test", toolConfig, args)

	if got := normalized["action"]; got != "get" {
		t.Fatalf("expected action to be repaired to get, got %#v", got)
	}
	if got := normalized["allow_insecure"]; got != true {
		t.Fatalf("expected allow_insecure=true, got %#v", got)
	}
	if got := normalized["auto_encode_url"]; got != true {
		t.Fatalf("expected auto_encode_url=true, got %#v", got)
	}
	if got := normalized["data"]; got != "https://stormapi.su" {
		t.Fatalf("expected data to contain the recovered URL, got %#v", got)
	}
	if got := normalized["url"]; got != "https://stormapi.su" {
		t.Fatalf("expected url heuristic to recover the target URL, got %#v", got)
	}

	if missing := executor.findMissingRequiredParams(toolConfig, normalized); len(missing) != 0 {
		t.Fatalf("expected repaired args to satisfy required params, missing=%v", missing)
	}
}

func TestExecutor_NormalizeToolArgs_RepairsRawJSONObjectArgs(t *testing.T) {
	executor, _ := setupTestExecutor(t)
	toolConfig := &config.ToolConfig{
		Name: "http-framework-test",
		Parameters: []config.ParameterConfig{
			{Name: "url", Type: "string", Required: true},
			{Name: "action", Type: "string"},
			{Name: "cookies", Type: "string"},
			{Name: "delay", Type: "string"},
			{Name: "debug", Type: "bool"},
			{Name: "allow_insecure", Type: "bool"},
			{Name: "auto_encode_url", Type: "bool"},
		},
	}

	args := map[string]interface{}{
		"raw": `{"action": "get", "allow_insecure": true, "auto_encode_url": , "cookies": "", "debug": true, "delay": "2", "url": "https://manage.stormapi.su"}`,
	}

	normalized := executor.normalizeToolArgs("http-framework-test", toolConfig, args)

	if _, ok := normalized["raw"]; ok {
		t.Fatalf("expected raw field to be removed after normalization")
	}
	if got := normalized["url"]; got != "https://manage.stormapi.su" {
		t.Fatalf("expected url to be recovered from raw payload, got %#v", got)
	}
	if got := normalized["allow_insecure"]; got != true {
		t.Fatalf("expected allow_insecure=true, got %#v", got)
	}
	if got := normalized["auto_encode_url"]; got != true {
		t.Fatalf("expected auto_encode_url=true for blank boolean value, got %#v", got)
	}
	if got := normalized["cookies"]; got != "" {
		t.Fatalf("expected cookies to remain an explicit empty string, got %#v", got)
	}

	if missing := executor.findMissingRequiredParams(toolConfig, normalized); len(missing) != 0 {
		t.Fatalf("expected repaired raw args to satisfy required params, missing=%v", missing)
	}
}

func TestExecutor_ExecuteTool_ReportsRequiredExecCommand(t *testing.T) {
	executor, mcpServer := setupTestExecutor(t)
	commandPos := 0
	executor.config.Tools = []config.ToolConfig{
		{
			Name:    "exec",
			Command: "sh",
			Args:    []string{"-c"},
			Enabled: true,
			Parameters: []config.ParameterConfig{
				{Name: "command", Type: "string", Required: true, Position: &commandPos, Format: "positional"},
			},
		},
	}
	executor.buildToolIndex()
	executor.RegisterTools(mcpServer)

	result, err := executor.ExecuteTool(context.Background(), "exec", map[string]interface{}{})
	if err != nil {
		t.Fatalf("ExecuteTool returned unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected missing exec command to return an error result")
	}
	if !strings.Contains(result.Content[0].Text, "missing required parameters: command") {
		t.Fatalf("expected explicit missing command error, got %q", result.Content[0].Text)
	}
}
