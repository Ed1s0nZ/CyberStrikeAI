package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/mcp"
	"cyberstrike-ai/internal/storage"

	"go.uber.org/zap"
)

// setupTestAgent creates a test Agent
func setupTestAgent(t *testing.T) (*Agent, *storage.FileResultStorage) {
	logger := zap.NewNop()
	mcpServer := mcp.NewServer(logger)

	openAICfg := &config.OpenAIConfig{
		APIKey:  "test-key",
		BaseURL: "https://api.test.com/v1",
		Model:   "test-model",
	}

	agentCfg := &config.AgentConfig{
		MaxIterations:        10,
		LargeResultThreshold: 100, // set small threshold for testing
		ResultStorageDir:     "",
	}

	agent := NewAgent(openAICfg, agentCfg, mcpServer, nil, logger, 10)

	// create test storage
	tmpDir := filepath.Join(os.TempDir(), "test_agent_storage_"+time.Now().Format("20060102_150405"))
	testStorage, err := storage.NewFileResultStorage(tmpDir, logger)
	if err != nil {
		t.Fatalf("failed to create test storage: %v", err)
	}

	agent.SetResultStorage(testStorage)

	return agent, testStorage
}

func TestAgent_FormatMinimalNotification(t *testing.T) {
	agent, testStorage := setupTestAgent(t)
	_ = testStorage // avoid unused variable warning

	executionID := "test_exec_001"
	toolName := "nmap_scan"
	size := 50000
	lineCount := 1000
	filePath := "tmp/test_exec_001.txt"

	notification := agent.formatMinimalNotification(executionID, toolName, size, lineCount, filePath)

	// verify notification contains required information
	if !strings.Contains(notification, executionID) {
		t.Errorf("notification should contain execution ID: %s", executionID)
	}

	if !strings.Contains(notification, toolName) {
		t.Errorf("notification should contain tool name: %s", toolName)
	}

	if !strings.Contains(notification, "50000") {
		t.Errorf("notification should contain size information")
	}

	if !strings.Contains(notification, "1000") {
		t.Errorf("notification should contain line count information")
	}

	if !strings.Contains(notification, "query_execution_result") {
		t.Errorf("notification should contain query tool usage instructions")
	}
}

func TestAgent_ExecuteToolViaMCP_LargeResult(t *testing.T) {
	agent, _ := setupTestAgent(t)

	// create simulated MCP tool result (large result)
	largeResult := &mcp.ToolResult{
		Content: []mcp.Content{
			{
				Type: "text",
				Text: strings.Repeat("This is a test line with some content.\n", 1000), // ~50KB
			},
		},
		IsError: false,
	}

	// simulate MCP server returning large result
	// since we need to simulate CallTool behavior, we need a mock or real MCP server
	// for simplicity, we directly test the result handling logic

	// set threshold
	agent.mu.Lock()
	agent.largeResultThreshold = 1000 // set small threshold
	agent.mu.Unlock()

	// create execution ID
	executionID := "test_exec_large_001"
	toolName := "test_tool"

	// format result
	var resultText strings.Builder
	for _, content := range largeResult.Content {
		resultText.WriteString(content.Text)
		resultText.WriteString("\n")
	}

	resultStr := resultText.String()
	resultSize := len(resultStr)

	// detect large result and save
	agent.mu.RLock()
	threshold := agent.largeResultThreshold
	storage := agent.resultStorage
	agent.mu.RUnlock()

	if resultSize > threshold && storage != nil {
		// save large result
		err := storage.SaveResult(executionID, toolName, resultStr)
		if err != nil {
			t.Fatalf("failed to save large result: %v", err)
		}

		// generate notification
		lines := strings.Split(resultStr, "\n")
		filePath := storage.GetResultPath(executionID)
		notification := agent.formatMinimalNotification(executionID, toolName, resultSize, len(lines), filePath)

		// verify notification format
		if !strings.Contains(notification, executionID) {
			t.Errorf("notification should contain execution ID")
		}

		// verify result was saved
		savedResult, err := storage.GetResult(executionID)
		if err != nil {
			t.Fatalf("failed to retrieve saved result: %v", err)
		}

		if savedResult != resultStr {
			t.Errorf("saved result does not match original result")
		}
	} else {
		t.Fatal("large result should have been detected and saved")
	}
}

func TestAgent_ExecuteToolViaMCP_SmallResult(t *testing.T) {
	agent, _ := setupTestAgent(t)

	// create small result
	smallResult := &mcp.ToolResult{
		Content: []mcp.Content{
			{
				Type: "text",
				Text: "Small result content",
			},
		},
		IsError: false,
	}

	// set large threshold
	agent.mu.Lock()
	agent.largeResultThreshold = 100000 // 100KB
	agent.mu.Unlock()

	// format result
	var resultText strings.Builder
	for _, content := range smallResult.Content {
		resultText.WriteString(content.Text)
		resultText.WriteString("\n")
	}

	resultStr := resultText.String()
	resultSize := len(resultStr)

	// detect large result
	agent.mu.RLock()
	threshold := agent.largeResultThreshold
	storage := agent.resultStorage
	agent.mu.RUnlock()

	if resultSize > threshold && storage != nil {
		t.Fatal("small result should not be saved")
	}

	// small result should be returned directly
	if resultSize <= threshold {
		// this is the expected behavior
		if resultStr == "" {
			t.Fatal("small result should be returned directly and should not be empty")
		}
	}
}

func TestAgent_SetResultStorage(t *testing.T) {
	agent, _ := setupTestAgent(t)

	// create new storage
	tmpDir := filepath.Join(os.TempDir(), "test_new_storage_"+time.Now().Format("20060102_150405"))
	newStorage, err := storage.NewFileResultStorage(tmpDir, zap.NewNop())
	if err != nil {
		t.Fatalf("failed to create new storage: %v", err)
	}

	// set new storage
	agent.SetResultStorage(newStorage)

	// verify storage was updated
	agent.mu.RLock()
	currentStorage := agent.resultStorage
	agent.mu.RUnlock()

	if currentStorage != newStorage {
		t.Fatal("storage was not updated correctly")
	}

	// cleanup
	os.RemoveAll(tmpDir)
}

func TestAgent_NewAgent_DefaultValues(t *testing.T) {
	logger := zap.NewNop()
	mcpServer := mcp.NewServer(logger)

	openAICfg := &config.OpenAIConfig{
		APIKey:  "test-key",
		BaseURL: "https://api.test.com/v1",
		Model:   "test-model",
	}

	// test default configuration
	agent := NewAgent(openAICfg, nil, mcpServer, nil, logger, 0)

	if agent.maxIterations != 30 {
		t.Errorf("default iteration count mismatch. expected: 30, got: %d", agent.maxIterations)
	}

	agent.mu.RLock()
	threshold := agent.largeResultThreshold
	agent.mu.RUnlock()

	if threshold != 50*1024 {
		t.Errorf("default threshold mismatch. expected: %d, got: %d", 50*1024, threshold)
	}
}

func TestAgent_NewAgent_CustomConfig(t *testing.T) {
	logger := zap.NewNop()
	mcpServer := mcp.NewServer(logger)

	openAICfg := &config.OpenAIConfig{
		APIKey:  "test-key",
		BaseURL: "https://api.test.com/v1",
		Model:   "test-model",
	}

	agentCfg := &config.AgentConfig{
		MaxIterations:        20,
		LargeResultThreshold: 100 * 1024, // 100KB
		ResultStorageDir:     "custom_tmp",
	}

	agent := NewAgent(openAICfg, agentCfg, mcpServer, nil, logger, 15)

	if agent.maxIterations != 15 {
		t.Errorf("iteration count mismatch. expected: 15, got: %d", agent.maxIterations)
	}

	agent.mu.RLock()
	threshold := agent.largeResultThreshold
	agent.mu.RUnlock()

	if threshold != 100*1024 {
		t.Errorf("threshold mismatch. expected: %d, got: %d", 100*1024, threshold)
	}
}

func newOpenAITestServer(t *testing.T, responder func(call int, req OpenAIRequest) OpenAIResponse) (*httptest.Server, *int32, *[]OpenAIRequest) {
	t.Helper()

	var callCount int32
	var (
		mu       sync.Mutex
		requests []OpenAIRequest
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}

		var req OpenAIRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode OpenAI request: %v", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		call := int(atomic.AddInt32(&callCount, 1))

		mu.Lock()
		requests = append(requests, req)
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(responder(call, req)); err != nil {
			t.Errorf("failed to encode OpenAI response: %v", err)
		}
	}))

	t.Cleanup(server.Close)
	return server, &callCount, &requests
}

func testToolCallResponse(toolName string, args map[string]interface{}) OpenAIResponse {
	return OpenAIResponse{
		ID: "test-response",
		Choices: []Choice{
			{
				FinishReason: "tool_calls",
				Message: MessageWithTools{
					Role:    "assistant",
					Content: "Calling tool",
					ToolCalls: []ToolCall{
						{
							ID:   "call_1",
							Type: "function",
							Function: FunctionCall{
								Name:      toolName,
								Arguments: args,
							},
						},
					},
				},
			},
		},
	}
}

func testAssistantResponse(content, finishReason string) OpenAIResponse {
	return OpenAIResponse{
		ID: "test-response",
		Choices: []Choice{
			{
				FinishReason: finishReason,
				Message: MessageWithTools{
					Role:    "assistant",
					Content: content,
				},
			},
		},
	}
}

func messagesContain(messages []ChatMessage, needle string) bool {
	for _, msg := range messages {
		if strings.Contains(msg.Content, needle) {
			return true
		}
	}
	return false
}

func formatMessagesForDebug(messages []ChatMessage) string {
	var b strings.Builder
	for i, msg := range messages {
		b.WriteString(msg.Role)
		b.WriteString(": ")
		b.WriteString(msg.Content)
		if i < len(messages)-1 {
			b.WriteString("\n---\n")
		}
	}
	return b.String()
}

func TestAgentLoop_LastIterationSummaryWaitsForDeferredToolResults(t *testing.T) {
	releaseTool := make(chan struct{})
	go func() {
		time.Sleep(25 * time.Millisecond)
		close(releaseTool)
	}()

	server, callCount, _ := newOpenAITestServer(t, func(call int, req OpenAIRequest) OpenAIResponse {
		switch call {
		case 1:
			return testToolCallResponse("slow_tool", map[string]interface{}{"target": "example.com"})
		case 2:
			if !messagesContain(req.Messages, "Background tool slow_tool finished.") {
				t.Errorf("summary request did not include late background tool notice\n%s", formatMessagesForDebug(req.Messages))
			}
			if !messagesContain(req.Messages, "late tool result from summary path") {
				t.Errorf("summary request did not include late tool output\n%s", formatMessagesForDebug(req.Messages))
			}
			if !messagesContain(req.Messages, "This is the last iteration.") {
				t.Errorf("summary request did not include last-iteration summary prompt\n%s", formatMessagesForDebug(req.Messages))
			}
			return testAssistantResponse("summary saw late tool result", "stop")
		default:
			t.Fatalf("unexpected OpenAI call %d", call)
			return OpenAIResponse{}
		}
	})

	logger := zap.NewNop()
	mcpServer := mcp.NewServer(logger)
	mcpServer.RegisterTool(mcp.Tool{
		Name:        "slow_tool",
		Description: "slow test tool",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"target": map[string]interface{}{"type": "string"},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		<-releaseTool
		return &mcp.ToolResult{
			Content: []mcp.Content{{Type: "text", Text: "late tool result from summary path"}},
		}, nil
	})

	agent := NewAgent(&config.OpenAIConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
		Model:   "test-model",
	}, &config.AgentConfig{
		ParallelToolExecution: true,
		LargeResultThreshold:  1024,
	}, mcpServer, nil, logger, 1)
	agent.parallelToolWait = 10 * time.Millisecond
	agent.memoryCompressor = nil

	result, err := agent.AgentLoop(context.Background(), "run slow tool", nil)
	if err != nil {
		t.Fatalf("AgentLoop returned error: %v", err)
	}

	if result.Response != "summary saw late tool result" {
		t.Fatalf("unexpected final response: %q", result.Response)
	}

	if got := atomic.LoadInt32(callCount); got != 2 {
		t.Fatalf("expected 2 OpenAI calls, got %d", got)
	}
}

func TestAgentLoop_StopWaitsForDeferredToolResults(t *testing.T) {
	releaseTool := make(chan struct{})

	server, callCount, _ := newOpenAITestServer(t, func(call int, req OpenAIRequest) OpenAIResponse {
		switch call {
		case 1:
			return testToolCallResponse("slow_tool", map[string]interface{}{"target": "example.com"})
		case 2:
			if messagesContain(req.Messages, "late tool result from stop path") {
				t.Errorf("tool result should not be available on the intermediate reasoning pass")
			}
			return testAssistantResponse("working on other tasks", "length")
		case 3:
			close(releaseTool)
			return testAssistantResponse("ready to finish", "stop")
		case 4:
			if !messagesContain(req.Messages, "Background tool slow_tool finished.") {
				t.Errorf("final reasoning request did not include late background tool notice\n%s", formatMessagesForDebug(req.Messages))
			}
			if !messagesContain(req.Messages, "late tool result from stop path") {
				t.Errorf("final reasoning request did not include late tool output\n%s", formatMessagesForDebug(req.Messages))
			}
			return testAssistantResponse("final answer with late tool result", "stop")
		default:
			t.Fatalf("unexpected OpenAI call %d", call)
			return OpenAIResponse{}
		}
	})

	logger := zap.NewNop()
	mcpServer := mcp.NewServer(logger)
	mcpServer.RegisterTool(mcp.Tool{
		Name:        "slow_tool",
		Description: "slow test tool",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"target": map[string]interface{}{"type": "string"},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		<-releaseTool
		return &mcp.ToolResult{
			Content: []mcp.Content{{Type: "text", Text: "late tool result from stop path"}},
		}, nil
	})

	agent := NewAgent(&config.OpenAIConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
		Model:   "test-model",
	}, &config.AgentConfig{
		ParallelToolExecution: true,
		LargeResultThreshold:  1024,
	}, mcpServer, nil, logger, 5)
	agent.parallelToolWait = 10 * time.Millisecond
	agent.memoryCompressor = nil

	result, err := agent.AgentLoop(context.Background(), "run slow tool", nil)
	if err != nil {
		t.Fatalf("AgentLoop returned error: %v", err)
	}

	if result.Response != "final answer with late tool result" {
		t.Fatalf("unexpected final response: %q", result.Response)
	}

	if got := atomic.LoadInt32(callCount); got != 4 {
		t.Fatalf("expected 4 OpenAI calls, got %d", got)
	}
}
