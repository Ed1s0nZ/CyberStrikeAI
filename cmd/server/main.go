package main

import (
	"cyberstrike-ai/internal/app"
	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/logger"
	"flag"
	"fmt"
)

func main() {
	var configPath = flag.String("config", "config.yaml", "配置文件路径")
	flag.Parse()

	// 加载配置
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Printf("加载配置失败: %v\n", err)
		return
	}

	// 初始化日志
	log := logger.New(cfg.Log.Level, cfg.Log.Output)

	// 创建应用
	application, err := app.New(cfg, log)
	if err != nil {
		log.Fatal("应用初始化失败", "error", err)
	}

	// 启动服务器
	if err := application.Run(); err != nil {
		log.Fatal("服务器启动失败", "error", err)
	}
}

