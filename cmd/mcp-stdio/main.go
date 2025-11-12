package main

import (
	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/logger"
	"cyberstrike-ai/internal/mcp"
	"cyberstrike-ai/internal/security"
	"flag"
	"fmt"
	"os"

	"go.uber.org/zap"
)

func main() {
	var configPath = flag.String("config", "config.yaml", "配置文件路径")
	flag.Parse()

	// 加载配置
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		os.Exit(1)
	}

	// 初始化日志（stdio 模式下使用 stderr 输出日志，避免干扰 JSON-RPC 通信）
	log := logger.New(cfg.Log.Level, "stderr")

	// 创建MCP服务器
	mcpServer := mcp.NewServer(log.Logger)

	// 创建安全工具执行器
	executor := security.NewExecutor(&cfg.Security, mcpServer, log.Logger)

	// 注册工具
	executor.RegisterTools(mcpServer)

	log.Logger.Info("MCP服务器（stdio模式）已启动，等待消息...")

	// 运行 stdio 循环
	if err := mcpServer.HandleStdio(); err != nil {
		log.Logger.Error("MCP服务器运行失败", zap.Error(err))
		os.Exit(1)
	}
}

