package main

import (
	"cyberstrike-ai/internal/app"
	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/logger"
	"flag"
	"fmt"
)

func main() {
	var configPath = flag.String("config", "config.yaml", "config file path")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Printf("failed to load config: %v\n", err)
		return
	}

	// When MCP is enabled and auth_header_value is empty, auto-generate a random key and write it back to config
	if err := config.EnsureMCPAuth(*configPath, cfg); err != nil {
		fmt.Printf("MCP auth configuration failed: %v\n", err)
		return
	}
	if cfg.MCP.Enabled {
		config.PrintMCPConfigJSON(cfg.MCP)
	}

	// Initialize logger
	log := logger.New(cfg.Log.Level, cfg.Log.Output)

	// Create application
	application, err := app.New(cfg, log)
	if err != nil {
		log.Fatal("application initialization failed", "error", err)
	}

	// Start server
	if err := application.Run(); err != nil {
		log.Fatal("server startup failed", "error", err)
	}
}

