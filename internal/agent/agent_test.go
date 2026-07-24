package agent

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/mcp"
	"cyberstrike-ai/internal/mcp/builtin"

	"go.uber.org/zap"
)

// setupTestAgent 创建测试用的Agent
func setupTestAgent(t *testing.T) *Agent {
	logger := zap.NewNop()
	mcpServer := mcp.NewServer(logger)

	openAICfg := &config.OpenAIConfig{
		APIKey:  "test-key",
		BaseURL: "https://api.test.com/v1",
		Model:   "test-model",
	}

	agentCfg := &config.AgentConfig{
		MaxIterations: 10,
	}

	return NewAgent(openAICfg, agentCfg, mcpServer, nil, logger, 10)
}

func TestAgent_NewAgent_DefaultValues(t *testing.T) {
	logger := zap.NewNop()
	mcpServer := mcp.NewServer(logger)

	openAICfg := &config.OpenAIConfig{
		APIKey:  "test-key",
		BaseURL: "https://api.test.com/v1",
		Model:   "test-model",
	}

	// 测试默认配置
	agent := NewAgent(openAICfg, nil, mcpServer, nil, logger, 0)

	if agent.maxIterations != 30 {
		t.Errorf("默认迭代次数不匹配。期望: 30, 实际: %d", agent.maxIterations)
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
		MaxIterations: 20,
	}

	agent := NewAgent(openAICfg, agentCfg, mcpServer, nil, logger, 15)

	if agent.maxIterations != 15 {
		t.Errorf("迭代次数不匹配。期望: 15, 实际: %d", agent.maxIterations)
	}
}

func TestBuildToolFailureMessageAuthorizationDenied(t *testing.T) {
	msg := buildToolFailureMessage(
		"list_project_facts",
		"tool authorization denied: no access to project",
		errors.New("tool authorization denied: no access to project"),
	)
	for _, want := range []string{
		"工具名称: list_project_facts",
		"错误详情: tool authorization denied: no access to project",
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("message missing %q:\n%s", want, msg)
		}
	}
	for _, notWant := range []string{
		"可能的原因",
		"建议",
		"错误类型",
		"工具 \"list_project_facts\" 不存在或未启用",
		"单次执行超时",
	} {
		if strings.Contains(msg, notWant) {
			t.Fatalf("message should not include generic hint %q:\n%s", notWant, msg)
		}
	}
}

func TestBuildToolFailureMessageCanceled(t *testing.T) {
	msg := buildToolFailureMessage(
		"long_running_tool",
		"工具调用已被手动终止（MCP 监控页）。智能体将携带此结果继续后续步骤，整条任务不会因此被停止。",
		context.Canceled,
	)

	for _, want := range []string{
		"工具名称: long_running_tool",
		"错误详情: 工具调用已被手动终止",
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("message missing %q:\n%s", want, msg)
		}
	}
}

func TestBuildToolFailureMessageDeadlineExceeded(t *testing.T) {
	msg := buildToolFailureMessage(
		"nmap",
		"工具执行超过 15 分钟被自动终止（可在 config.yaml 的 agent.tool_timeout_minutes 中调整）",
		context.DeadlineExceeded,
	)

	for _, want := range []string{
		"工具名称: nmap",
		"错误详情: 工具执行超过 15 分钟被自动终止",
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("message missing %q:\n%s", want, msg)
		}
	}
}

func TestBuildToolFailureMessageUnknownKeepsGenericFallback(t *testing.T) {
	msg := buildToolFailureMessage("custom_tool", "dial tcp: connection reset by peer", errors.New("dial tcp: connection reset by peer"))

	for _, want := range []string{
		"工具名称: custom_tool",
		"错误详情: dial tcp: connection reset by peer",
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("message missing %q:\n%s", want, msg)
		}
	}
}

func TestAgentCancelRunningMCPToolsForConversation(t *testing.T) {
	ag := setupTestAgent(t)
	ag.mcpServer.ConfigureToolWaitTimeoutSeconds(1)
	ag.mcpServer.RegisterTool(mcp.Tool{Name: "block", InputSchema: map[string]interface{}{"type": "object"}}, func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})

	ctx1 := mcp.WithMCPConversationID(context.Background(), "conv-1")
	result1, execID1, err := ag.mcpServer.CallTool(ctx1, "block", nil)
	if err != nil {
		t.Fatalf("CallTool conv-1: %v", err)
	}
	if result1 == nil || !result1.IsError || execID1 == "" {
		t.Fatalf("expected bounded wait for conv-1, result=%#v id=%q", result1, execID1)
	}

	ctx2 := mcp.WithMCPConversationID(context.Background(), "conv-2")
	result2, execID2, err := ag.mcpServer.CallTool(ctx2, "block", nil)
	if err != nil {
		t.Fatalf("CallTool conv-2: %v", err)
	}
	if result2 == nil || !result2.IsError || execID2 == "" {
		t.Fatalf("expected bounded wait for conv-2, result=%#v id=%q", result2, execID2)
	}

	if got := ag.CancelRunningMCPToolsForConversation("conv-1", "session ended"); got != 1 {
		t.Fatalf("cancelled count = %d, want 1", got)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		exec1, _ := ag.mcpServer.GetExecution(execID1)
		exec2, _ := ag.mcpServer.GetExecution(execID2)
		if exec1 != nil && exec1.Status == mcp.ToolExecutionStatusCancelled {
			if exec2 == nil || exec2.Status != mcp.ToolExecutionStatusRunning {
				t.Fatalf("conv-2 execution should remain running, got %#v", exec2)
			}
			if !strings.Contains(exec1.Error, "session ended") && (exec1.Result == nil || !strings.Contains(mcp.ToolResultPlainText(exec1.Result), "session ended")) {
				t.Fatalf("cancel note missing from conv-1 execution: %#v", exec1)
			}
			_ = ag.CancelRunningMCPToolsForConversation("conv-2", "")
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("conv-1 execution did not become cancelled")
}

func TestExecuteMCPToolForConversationInjectsConversationID(t *testing.T) {
	ag := setupTestAgent(t)
	gotArgs := make(chan map[string]interface{}, 1)
	ag.mcpServer.RegisterTool(mcp.Tool{Name: builtin.ToolRecordVulnerability, InputSchema: map[string]interface{}{"type": "object"}}, func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		gotArgs <- args
		return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: "ok"}}}, nil
	})

	result, err := ag.ExecuteMCPToolForConversation(context.Background(), "conv-record", builtin.ToolRecordVulnerability, map[string]interface{}{})
	if err != nil {
		t.Fatalf("ExecuteMCPToolForConversation: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("expected successful result, got %#v", result)
	}

	select {
	case args := <-gotArgs:
		if got := args["conversation_id"]; got != "conv-record" {
			t.Fatalf("conversation_id = %#v, want conv-record", got)
		}
	case <-time.After(time.Second):
		t.Fatal("tool was not called")
	}
}

func TestExecuteMCPToolForConversationBindsExecutionConversation(t *testing.T) {
	ag := setupTestAgent(t)
	ag.mcpServer.ConfigureToolWaitTimeoutSeconds(1)
	release := make(chan struct{})
	ag.mcpServer.RegisterTool(mcp.Tool{Name: "slow-bind", InputSchema: map[string]interface{}{"type": "object"}}, func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		select {
		case <-release:
			return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: "done"}}}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})

	result, err := ag.ExecuteMCPToolForConversation(context.Background(), "conv-bound", "slow-bind", nil)
	if err != nil {
		t.Fatalf("ExecuteMCPToolForConversation: %v", err)
	}
	if result == nil || !result.IsError || result.ExecutionID == "" {
		t.Fatalf("expected bounded wait result with execution id, result=%#v", result)
	}

	exec, ok := ag.mcpServer.GetExecution(result.ExecutionID)
	if !ok || exec == nil {
		t.Fatalf("missing execution %q", result.ExecutionID)
	}
	if exec.ConversationID != "conv-bound" {
		t.Fatalf("execution conversation = %q, want conv-bound", exec.ConversationID)
	}
	close(release)
}

func TestExecuteMCPToolForConversationConcurrentRecordIsolation(t *testing.T) {
	ag := setupTestAgent(t)
	seen := make(chan string, 2)
	ag.mcpServer.RegisterTool(mcp.Tool{Name: builtin.ToolRecordVulnerability, InputSchema: map[string]interface{}{"type": "object"}}, func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		if conv, _ := args["conversation_id"].(string); conv != "" {
			seen <- conv
		}
		return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: "ok"}}}, nil
	})

	var wg sync.WaitGroup
	for _, conv := range []string{"conv-a", "conv-b"} {
		conv := conv
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := ag.ExecuteMCPToolForConversation(context.Background(), conv, builtin.ToolRecordVulnerability, map[string]interface{}{}); err != nil {
				t.Errorf("ExecuteMCPToolForConversation %s: %v", conv, err)
			}
		}()
	}
	wg.Wait()
	close(seen)

	got := map[string]int{}
	for conv := range seen {
		got[conv]++
	}
	if got["conv-a"] != 1 || got["conv-b"] != 1 {
		t.Fatalf("conversation ids = %#v, want one call for conv-a and conv-b", got)
	}
}
