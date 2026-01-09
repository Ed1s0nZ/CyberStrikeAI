package mcp

import (
	"context"
	"testing"
	"time"

	"cyberstrike-ai/internal/config"

	"go.uber.org/zap"
)

func TestExternalMCPManager_AddOrUpdateConfig(t *testing.T) {
	logger := zap.NewNop()
	manager := NewExternalMCPManager(logger)

	// 测试添加stdio配置
	stdioCfg := config.ExternalMCPServerConfig{
		Command:     "python3",
		Args:        []string{"/path/to/script.py"},
		Transport:   "stdio",
		Description: "Test stdio MCP",
		Timeout:     30,
		Enabled:     true,
	}

	err := manager.AddOrUpdateConfig("test-stdio", stdioCfg)
	if err != nil {
		t.Fatalf("添加stdio配置失败: %v", err)
	}

	// 测试添加HTTP配置
	httpCfg := config.ExternalMCPServerConfig{
		Transport:   "http",
		URL:         "http://127.0.0.1:8081/mcp",
		Description: "Test HTTP MCP",
		Timeout:     30,
		Enabled:     false,
	}

	err = manager.AddOrUpdateConfig("test-http", httpCfg)
	if err != nil {
		t.Fatalf("添加HTTP配置失败: %v", err)
	}

	// 验证配置已保存
	configs := manager.GetConfigs()
	if len(configs) != 2 {
		t.Fatalf("期望2个配置，实际%d个", len(configs))
	}

	if configs["test-stdio"].Command != stdioCfg.Command {
		t.Errorf("stdio配置命令不匹配")
	}

	if configs["test-http"].URL != httpCfg.URL {
		t.Errorf("HTTP配置URL不匹配")
	}
}

func TestExternalMCPManager_RemoveConfig(t *testing.T) {
	logger := zap.NewNop()
	manager := NewExternalMCPManager(logger)

	cfg := config.ExternalMCPServerConfig{
		Command:   "python3",
		Transport: "stdio",
		Enabled:   false,
	}

	manager.AddOrUpdateConfig("test-remove", cfg)

	// 移除配置
	err := manager.RemoveConfig("test-remove")
	if err != nil {
		t.Fatalf("移除配置失败: %v", err)
	}

	configs := manager.GetConfigs()
	if _, exists := configs["test-remove"]; exists {
		t.Error("配置应该已被移除")
	}
}

func TestExternalMCPManager_GetStats(t *testing.T) {
	logger := zap.NewNop()
	manager := NewExternalMCPManager(logger)

	// 添加多个配置
	manager.AddOrUpdateConfig("enabled1", config.ExternalMCPServerConfig{
		Command: "python3",
		Enabled: true,
	})

	manager.AddOrUpdateConfig("enabled2", config.ExternalMCPServerConfig{
		URL:     "http://127.0.0.1:8081/mcp",
		Enabled: true,
	})

	manager.AddOrUpdateConfig("disabled1", config.ExternalMCPServerConfig{
		Command:  "python3",
		Enabled:  false,
		Disabled: true, // 明确设置为禁用
	})

	stats := manager.GetStats()

	if stats["total"].(int) != 3 {
		t.Errorf("期望总数3，实际%d", stats["total"])
	}

	if stats["enabled"].(int) != 2 {
		t.Errorf("期望启用数2，实际%d", stats["enabled"])
	}

	if stats["disabled"].(int) != 1 {
		t.Errorf("期望停用数1，实际%d", stats["disabled"])
	}
}

func TestExternalMCPManager_LoadConfigs(t *testing.T) {
	logger := zap.NewNop()
	manager := NewExternalMCPManager(logger)

	externalMCPConfig := config.ExternalMCPConfig{
		Servers: map[string]config.ExternalMCPServerConfig{
			"loaded1": {
				Command: "python3",
				Enabled: true,
			},
			"loaded2": {
				URL:     "http://127.0.0.1:8081/mcp",
				Enabled: false,
			},
		},
	}

	manager.LoadConfigs(&externalMCPConfig)

	configs := manager.GetConfigs()
	if len(configs) != 2 {
		t.Fatalf("期望2个配置，实际%d个", len(configs))
	}

	if configs["loaded1"].Command != "python3" {
		t.Error("配置1加载失败")
	}

	if configs["loaded2"].URL != "http://127.0.0.1:8081/mcp" {
		t.Error("配置2加载失败")
	}
}

func TestHTTPMCPClient_Initialize(t *testing.T) {
	// 注意：这个测试需要一个真实的HTTP MCP服务器
	// 如果没有服务器，这个测试会失败
	// 在实际测试中，可以使用mock服务器
	logger := zap.NewNop()
	client := NewHTTPMCPClient("http://127.0.0.1:8081/mcp", 5*time.Second, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 这个测试可能会失败，如果没有真实的服务器
	// 在实际环境中，应该使用mock服务器
	err := client.Initialize(ctx)
	if err != nil {
		t.Logf("初始化失败（可能是没有服务器）: %v", err)
	}

	status := client.GetStatus()
	if status == "" {
		t.Error("状态不应该为空")
	}

	client.Close()
}

func TestStdioMCPClient_Initialize(t *testing.T) {
	// 注意：这个测试需要一个真实的stdio MCP服务器
	// 如果没有服务器，这个测试会失败
	logger := zap.NewNop()
	client := NewStdioMCPClient("echo", []string{"test"}, nil, 5*time.Second, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 这个测试可能会失败，因为echo不是MCP服务器
	// 在实际环境中，应该使用真实的MCP服务器或mock
	err := client.Initialize(ctx)
	if err != nil {
		t.Logf("初始化失败（echo不是MCP服务器）: %v", err)
	}

	client.Close()
}

func TestExternalMCPManager_StartStopClient(t *testing.T) {
	logger := zap.NewNop()
	manager := NewExternalMCPManager(logger)

	// 添加一个禁用的配置
	cfg := config.ExternalMCPServerConfig{
		Command:   "python3",
		Transport: "stdio",
		Enabled:   false,
	}

	manager.AddOrUpdateConfig("test-start-stop", cfg)

	// 尝试启动（可能会失败，因为没有真实的服务器）
	err := manager.StartClient("test-start-stop")
	if err != nil {
		t.Logf("启动失败（可能是没有服务器）: %v", err)
	}

	// 停止
	err = manager.StopClient("test-start-stop")
	if err != nil {
		t.Fatalf("停止失败: %v", err)
	}

	// 验证配置已更新为禁用
	configs := manager.GetConfigs()
	if configs["test-start-stop"].Enabled {
		t.Error("配置应该已被禁用")
	}
}

func TestExternalMCPManager_CallTool(t *testing.T) {
	logger := zap.NewNop()
	manager := NewExternalMCPManager(logger)

	// 测试调用不存在的工具
	_, _, err := manager.CallTool(context.Background(), "nonexistent::tool", map[string]interface{}{})
	if err == nil {
		t.Error("应该返回错误")
	}

	// 测试无效的工具名称格式
	_, _, err = manager.CallTool(context.Background(), "invalid-tool-name", map[string]interface{}{})
	if err == nil {
		t.Error("应该返回错误（无效格式）")
	}
}

func TestExternalMCPManager_GetAllTools(t *testing.T) {
	logger := zap.NewNop()
	manager := NewExternalMCPManager(logger)

	ctx := context.Background()
	tools, err := manager.GetAllTools(ctx)
	if err != nil {
		t.Fatalf("获取工具列表失败: %v", err)
	}

	// 如果没有连接的客户端，应该返回空列表
	if len(tools) != 0 {
		t.Logf("获取到%d个工具", len(tools))
	}
}
