package agent

import (
	"context"
	"strings"
	"testing"
	"time"

	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/mcp"

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
