package app

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"cyberstrike-ai/internal/agent"
	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/database"
	"cyberstrike-ai/internal/handler"
	"cyberstrike-ai/internal/knowledge"
	"cyberstrike-ai/internal/logger"
	"cyberstrike-ai/internal/mcp"
	"cyberstrike-ai/internal/openai"
	"cyberstrike-ai/internal/security"
	"cyberstrike-ai/internal/storage"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// App 应用
type App struct {
	config         *config.Config
	logger         *logger.Logger
	router         *gin.Engine
	mcpServer      *mcp.Server
	externalMCPMgr *mcp.ExternalMCPManager
	agent          *agent.Agent
	executor       *security.Executor
	db             *database.DB
	knowledgeDB    *database.DB // 知识库数据库连接（如果使用独立数据库）
	auth           *security.AuthManager
}

// New 创建新应用
func New(cfg *config.Config, log *logger.Logger) (*App, error) {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	// CORS中间件
	router.Use(corsMiddleware())

	// 认证管理器
	authManager, err := security.NewAuthManager(cfg.Auth.Password, cfg.Auth.SessionDurationHours)
	if err != nil {
		return nil, fmt.Errorf("初始化认证失败: %w", err)
	}

	// 初始化数据库
	dbPath := cfg.Database.Path
	if dbPath == "" {
		dbPath = "data/conversations.db"
	}

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("创建数据库目录失败: %w", err)
	}

	db, err := database.NewDB(dbPath, log.Logger)
	if err != nil {
		return nil, fmt.Errorf("初始化数据库失败: %w", err)
	}

	// 创建MCP服务器（带数据库持久化）
	mcpServer := mcp.NewServerWithStorage(log.Logger, db)

	// 创建安全工具执行器
	executor := security.NewExecutor(&cfg.Security, mcpServer, log.Logger)

	// 注册工具
	executor.RegisterTools(mcpServer)

	// 注册漏洞记录工具
	registerVulnerabilityTool(mcpServer, db, log.Logger)

	if cfg.Auth.GeneratedPassword != "" {
		config.PrintGeneratedPasswordWarning(cfg.Auth.GeneratedPassword, cfg.Auth.GeneratedPasswordPersisted, cfg.Auth.GeneratedPasswordPersistErr)
		cfg.Auth.GeneratedPassword = ""
		cfg.Auth.GeneratedPasswordPersisted = false
		cfg.Auth.GeneratedPasswordPersistErr = ""
	}

	// 创建外部MCP管理器（使用与内部MCP服务器相同的存储）
	externalMCPMgr := mcp.NewExternalMCPManagerWithStorage(log.Logger, db)
	if cfg.ExternalMCP.Servers != nil {
		externalMCPMgr.LoadConfigs(&cfg.ExternalMCP)
		// 启动所有启用的外部MCP客户端
		externalMCPMgr.StartAllEnabled()
	}

	// 初始化结果存储
	resultStorageDir := "tmp"
	if cfg.Agent.ResultStorageDir != "" {
		resultStorageDir = cfg.Agent.ResultStorageDir
	}

	// 确保存储目录存在
	if err := os.MkdirAll(resultStorageDir, 0755); err != nil {
		return nil, fmt.Errorf("创建结果存储目录失败: %w", err)
	}

	// 创建结果存储实例
	resultStorage, err := storage.NewFileResultStorage(resultStorageDir, log.Logger)
	if err != nil {
		return nil, fmt.Errorf("初始化结果存储失败: %w", err)
	}

	// 创建Agent
	maxIterations := cfg.Agent.MaxIterations
	if maxIterations <= 0 {
		maxIterations = 30 // 默认值
	}
	agent := agent.NewAgent(&cfg.OpenAI, &cfg.Agent, mcpServer, externalMCPMgr, log.Logger, maxIterations)

	// 设置结果存储到Agent
	agent.SetResultStorage(resultStorage)

	// 设置结果存储到Executor（用于查询工具）
	executor.SetResultStorage(resultStorage)

	// 初始化知识库模块（如果启用）
	var knowledgeManager *knowledge.Manager
	var knowledgeRetriever *knowledge.Retriever
	var knowledgeIndexer *knowledge.Indexer
	var knowledgeHandler *handler.KnowledgeHandler

	var knowledgeDBConn *database.DB
	log.Logger.Info("检查知识库配置", zap.Bool("enabled", cfg.Knowledge.Enabled))
	if cfg.Knowledge.Enabled {
		// 确定知识库数据库路径
		knowledgeDBPath := cfg.Database.KnowledgeDBPath
		var knowledgeDB *sql.DB

		if knowledgeDBPath != "" {
			// 使用独立的知识库数据库
			// 确保目录存在
			if err := os.MkdirAll(filepath.Dir(knowledgeDBPath), 0755); err != nil {
				return nil, fmt.Errorf("创建知识库数据库目录失败: %w", err)
			}

			var err error
			knowledgeDBConn, err = database.NewKnowledgeDB(knowledgeDBPath, log.Logger)
			if err != nil {
				return nil, fmt.Errorf("初始化知识库数据库失败: %w", err)
			}
			knowledgeDB = knowledgeDBConn.DB
			log.Logger.Info("使用独立的知识库数据库", zap.String("path", knowledgeDBPath))
		} else {
			// 向后兼容：使用会话数据库
			knowledgeDB = db.DB
			log.Logger.Info("使用会话数据库存储知识库数据（建议配置knowledge_db_path以分离数据）")
		}

		// 创建知识库管理器
		knowledgeManager = knowledge.NewManager(knowledgeDB, cfg.Knowledge.BasePath, log.Logger)

		// 创建嵌入器
		// 使用OpenAI配置的API Key（如果知识库配置中没有指定）
		if cfg.Knowledge.Embedding.APIKey == "" {
			cfg.Knowledge.Embedding.APIKey = cfg.OpenAI.APIKey
		}
		if cfg.Knowledge.Embedding.BaseURL == "" {
			cfg.Knowledge.Embedding.BaseURL = cfg.OpenAI.BaseURL
		}

		httpClient := &http.Client{
			Timeout: 30 * time.Minute,
		}
		openAIClient := openai.NewClient(&cfg.OpenAI, httpClient, log.Logger)
		embedder := knowledge.NewEmbedder(&cfg.Knowledge, &cfg.OpenAI, openAIClient, log.Logger)

		// 创建检索器
		retrievalConfig := &knowledge.RetrievalConfig{
			TopK:                cfg.Knowledge.Retrieval.TopK,
			SimilarityThreshold: cfg.Knowledge.Retrieval.SimilarityThreshold,
			HybridWeight:        cfg.Knowledge.Retrieval.HybridWeight,
		}
		knowledgeRetriever = knowledge.NewRetriever(knowledgeDB, embedder, retrievalConfig, log.Logger)

		// 创建索引器
		knowledgeIndexer = knowledge.NewIndexer(knowledgeDB, embedder, log.Logger)

		// 注册知识检索工具到MCP服务器
		knowledge.RegisterKnowledgeTool(mcpServer, knowledgeRetriever, knowledgeManager, log.Logger)

		// 创建知识库API处理器
		knowledgeHandler = handler.NewKnowledgeHandler(knowledgeManager, knowledgeRetriever, knowledgeIndexer, db, log.Logger)
		log.Logger.Info("知识库模块初始化完成", zap.Bool("handler_created", knowledgeHandler != nil))

		// 扫描知识库并建立索引（异步）
		go func() {
			if err := knowledgeManager.ScanKnowledgeBase(); err != nil {
				log.Logger.Warn("扫描知识库失败", zap.Error(err))
				return
			}

			// 检查是否已有索引，如果有则跳过自动重建
			hasIndex, err := knowledgeIndexer.HasIndex()
			if err != nil {
				log.Logger.Warn("检查索引状态失败", zap.Error(err))
				return
			}

			if hasIndex {
				log.Logger.Info("检测到已有知识库索引，跳过自动重建。如需重建，请手动点击重建索引按钮")
				return
			}

			// 只有在没有索引时才自动重建
			log.Logger.Info("未检测到知识库索引，开始自动构建索引")
			ctx := context.Background()
			if err := knowledgeIndexer.RebuildIndex(ctx); err != nil {
				log.Logger.Warn("重建知识库索引失败", zap.Error(err))
			}
		}()
	}

	// 获取配置文件路径
	configPath := "config.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	// 创建处理器
	agentHandler := handler.NewAgentHandler(agent, db, log.Logger)
	// 如果知识库已启用，设置知识库管理器到AgentHandler以便记录检索日志
	if knowledgeManager != nil {
		agentHandler.SetKnowledgeManager(knowledgeManager)
	}
	monitorHandler := handler.NewMonitorHandler(mcpServer, executor, db, log.Logger)
	monitorHandler.SetExternalMCPManager(externalMCPMgr) // 设置外部MCP管理器，以便获取外部MCP执行记录
	conversationHandler := handler.NewConversationHandler(db, log.Logger)
	groupHandler := handler.NewGroupHandler(db, log.Logger)
	authHandler := handler.NewAuthHandler(authManager, cfg, configPath, log.Logger)
	attackChainHandler := handler.NewAttackChainHandler(db, &cfg.OpenAI, log.Logger)
	vulnerabilityHandler := handler.NewVulnerabilityHandler(db, log.Logger)
	configHandler := handler.NewConfigHandler(configPath, cfg, mcpServer, executor, agent, attackChainHandler, externalMCPMgr, log.Logger)
	// 如果知识库已启用，设置知识库工具注册器，以便在ApplyConfig时重新注册知识库工具
	if cfg.Knowledge.Enabled && knowledgeRetriever != nil && knowledgeManager != nil {
		// 创建闭包，捕获knowledgeRetriever和knowledgeManager的引用
		registrar := func() error {
			knowledge.RegisterKnowledgeTool(mcpServer, knowledgeRetriever, knowledgeManager, log.Logger)
			return nil
		}
		configHandler.SetKnowledgeToolRegistrar(registrar)
	}
	externalMCPHandler := handler.NewExternalMCPHandler(externalMCPMgr, cfg, configPath, log.Logger)

	// 设置路由
	setupRoutes(
		router,
		authHandler,
		agentHandler,
		monitorHandler,
		conversationHandler,
		groupHandler,
		configHandler,
		externalMCPHandler,
		attackChainHandler,
		knowledgeHandler,
		vulnerabilityHandler,
		mcpServer,
		authManager,
	)

	return &App{
		config:         cfg,
		logger:         log,
		router:         router,
		mcpServer:      mcpServer,
		externalMCPMgr: externalMCPMgr,
		agent:          agent,
		executor:       executor,
		db:             db,
		knowledgeDB:    knowledgeDBConn,
		auth:           authManager,
	}, nil
}

// Run 启动应用
func (a *App) Run() error {
	// 启动MCP服务器（如果启用）
	if a.config.MCP.Enabled {
		go func() {
			mcpAddr := fmt.Sprintf("%s:%d", a.config.MCP.Host, a.config.MCP.Port)
			a.logger.Info("启动MCP服务器", zap.String("address", mcpAddr))

			mux := http.NewServeMux()
			mux.HandleFunc("/mcp", a.mcpServer.HandleHTTP)

			if err := http.ListenAndServe(mcpAddr, mux); err != nil {
				a.logger.Error("MCP服务器启动失败", zap.Error(err))
			}
		}()
	}

	// 启动主服务器
	addr := fmt.Sprintf("%s:%d", a.config.Server.Host, a.config.Server.Port)
	a.logger.Info("启动HTTP服务器", zap.String("address", addr))

	return a.router.Run(addr)
}

// Shutdown 关闭应用
func (a *App) Shutdown() {
	// 停止所有外部MCP客户端
	if a.externalMCPMgr != nil {
		a.externalMCPMgr.StopAll()
	}

	// 关闭知识库数据库连接（如果使用独立数据库）
	if a.knowledgeDB != nil {
		if err := a.knowledgeDB.Close(); err != nil {
			a.logger.Logger.Warn("关闭知识库数据库连接失败", zap.Error(err))
		}
	}
}

// setupRoutes 设置路由
func setupRoutes(
	router *gin.Engine,
	authHandler *handler.AuthHandler,
	agentHandler *handler.AgentHandler,
	monitorHandler *handler.MonitorHandler,
	conversationHandler *handler.ConversationHandler,
	groupHandler *handler.GroupHandler,
	configHandler *handler.ConfigHandler,
	externalMCPHandler *handler.ExternalMCPHandler,
	attackChainHandler *handler.AttackChainHandler,
	knowledgeHandler *handler.KnowledgeHandler,
	vulnerabilityHandler *handler.VulnerabilityHandler,
	mcpServer *mcp.Server,
	authManager *security.AuthManager,
) {
	// API路由
	api := router.Group("/api")

	// 认证相关路由
	authRoutes := api.Group("/auth")
	{
		authRoutes.POST("/login", authHandler.Login)
		authRoutes.POST("/logout", security.AuthMiddleware(authManager), authHandler.Logout)
		authRoutes.POST("/change-password", security.AuthMiddleware(authManager), authHandler.ChangePassword)
		authRoutes.GET("/validate", security.AuthMiddleware(authManager), authHandler.Validate)
	}

	protected := api.Group("")
	protected.Use(security.AuthMiddleware(authManager))
	{
		// Agent Loop
		protected.POST("/agent-loop", agentHandler.AgentLoop)
		// Agent Loop 流式输出
		protected.POST("/agent-loop/stream", agentHandler.AgentLoopStream)
		// Agent Loop 取消与任务列表
		protected.POST("/agent-loop/cancel", agentHandler.CancelAgentLoop)
		protected.GET("/agent-loop/tasks", agentHandler.ListAgentTasks)

		// 对话历史
		protected.POST("/conversations", conversationHandler.CreateConversation)
		protected.GET("/conversations", conversationHandler.ListConversations)
		protected.GET("/conversations/:id", conversationHandler.GetConversation)
		protected.PUT("/conversations/:id", conversationHandler.UpdateConversation)
		protected.DELETE("/conversations/:id", conversationHandler.DeleteConversation)
		protected.PUT("/conversations/:id/pinned", groupHandler.UpdateConversationPinned)

		// 对话分组
		protected.POST("/groups", groupHandler.CreateGroup)
		protected.GET("/groups", groupHandler.ListGroups)
		protected.GET("/groups/:id", groupHandler.GetGroup)
		protected.PUT("/groups/:id", groupHandler.UpdateGroup)
		protected.DELETE("/groups/:id", groupHandler.DeleteGroup)
		protected.PUT("/groups/:id/pinned", groupHandler.UpdateGroupPinned)
		protected.GET("/groups/:id/conversations", groupHandler.GetGroupConversations)
		protected.POST("/groups/conversations", groupHandler.AddConversationToGroup)
		protected.DELETE("/groups/:id/conversations/:conversationId", groupHandler.RemoveConversationFromGroup)
		protected.PUT("/groups/:id/conversations/:conversationId/pinned", groupHandler.UpdateConversationPinnedInGroup)

		// 监控
		protected.GET("/monitor", monitorHandler.Monitor)
		protected.GET("/monitor/execution/:id", monitorHandler.GetExecution)
		protected.DELETE("/monitor/execution/:id", monitorHandler.DeleteExecution)
		protected.GET("/monitor/stats", monitorHandler.GetStats)

		// 配置管理
		protected.GET("/config", configHandler.GetConfig)
		protected.GET("/config/tools", configHandler.GetTools)
		protected.PUT("/config", configHandler.UpdateConfig)
		protected.POST("/config/apply", configHandler.ApplyConfig)

		// 外部MCP管理
		protected.GET("/external-mcp", externalMCPHandler.GetExternalMCPs)
		protected.GET("/external-mcp/stats", externalMCPHandler.GetExternalMCPStats)
		protected.GET("/external-mcp/:name", externalMCPHandler.GetExternalMCP)
		protected.PUT("/external-mcp/:name", externalMCPHandler.AddOrUpdateExternalMCP)
		protected.DELETE("/external-mcp/:name", externalMCPHandler.DeleteExternalMCP)
		protected.POST("/external-mcp/:name/start", externalMCPHandler.StartExternalMCP)
		protected.POST("/external-mcp/:name/stop", externalMCPHandler.StopExternalMCP)

		// 攻击链可视化
		protected.GET("/attack-chain/:conversationId", attackChainHandler.GetAttackChain)
		protected.POST("/attack-chain/:conversationId/regenerate", attackChainHandler.RegenerateAttackChain)

		// 知识库管理（如果启用）
		if knowledgeHandler != nil {
			protected.GET("/knowledge/categories", knowledgeHandler.GetCategories)
			protected.GET("/knowledge/items", knowledgeHandler.GetItems)
			protected.GET("/knowledge/items/:id", knowledgeHandler.GetItem)
			protected.POST("/knowledge/items", knowledgeHandler.CreateItem)
			protected.PUT("/knowledge/items/:id", knowledgeHandler.UpdateItem)
			protected.DELETE("/knowledge/items/:id", knowledgeHandler.DeleteItem)
			protected.GET("/knowledge/index-status", knowledgeHandler.GetIndexStatus)
			protected.POST("/knowledge/index", knowledgeHandler.RebuildIndex)
			protected.POST("/knowledge/scan", knowledgeHandler.ScanKnowledgeBase)
			protected.GET("/knowledge/retrieval-logs", knowledgeHandler.GetRetrievalLogs)
			protected.DELETE("/knowledge/retrieval-logs/:id", knowledgeHandler.DeleteRetrievalLog)
			protected.POST("/knowledge/search", knowledgeHandler.Search)
		}

		// 漏洞管理
		protected.GET("/vulnerabilities", vulnerabilityHandler.ListVulnerabilities)
		protected.GET("/vulnerabilities/stats", vulnerabilityHandler.GetVulnerabilityStats)
		protected.GET("/vulnerabilities/:id", vulnerabilityHandler.GetVulnerability)
		protected.POST("/vulnerabilities", vulnerabilityHandler.CreateVulnerability)
		protected.PUT("/vulnerabilities/:id", vulnerabilityHandler.UpdateVulnerability)
		protected.DELETE("/vulnerabilities/:id", vulnerabilityHandler.DeleteVulnerability)

		// MCP端点
		protected.POST("/mcp", func(c *gin.Context) {
			mcpServer.HandleHTTP(c.Writer, c.Request)
		})
	}

	// 静态文件
	router.Static("/static", "./web/static")
	router.LoadHTMLGlob("web/templates/*")

	// 前端页面
	router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", nil)
	})
}

// registerVulnerabilityTool 注册漏洞记录工具到MCP服务器
func registerVulnerabilityTool(mcpServer *mcp.Server, db *database.DB, logger *zap.Logger) {
	tool := mcp.Tool{
		Name:             "record_vulnerability",
		Description:      "记录发现的漏洞详情到漏洞管理系统。当发现有效漏洞时，使用此工具记录漏洞信息，包括标题、描述、严重程度、类型、目标、证明、影响和建议等。",
		ShortDescription: "记录发现的漏洞详情到漏洞管理系统",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"title": map[string]interface{}{
					"type":        "string",
					"description": "漏洞标题（必需）",
				},
				"description": map[string]interface{}{
					"type":        "string",
					"description": "漏洞详细描述",
				},
				"severity": map[string]interface{}{
					"type":        "string",
					"description": "漏洞严重程度：critical（严重）、high（高）、medium（中）、low（低）、info（信息）",
					"enum":        []string{"critical", "high", "medium", "low", "info"},
				},
				"vulnerability_type": map[string]interface{}{
					"type":        "string",
					"description": "漏洞类型，如：SQL注入、XSS、CSRF、命令注入等",
				},
				"target": map[string]interface{}{
					"type":        "string",
					"description": "受影响的目标（URL、IP地址、服务等）",
				},
				"proof": map[string]interface{}{
					"type":        "string",
					"description": "漏洞证明（POC、截图、请求/响应等）",
				},
				"impact": map[string]interface{}{
					"type":        "string",
					"description": "漏洞影响说明",
				},
				"recommendation": map[string]interface{}{
					"type":        "string",
					"description": "修复建议",
				},
			},
			"required": []string{"title", "severity"},
		},
	}

	handler := func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		// 从参数中获取conversation_id（由Agent自动添加）
		conversationID, _ := args["conversation_id"].(string)
		if conversationID == "" {
			return &mcp.ToolResult{
				Content: []mcp.Content{
					{
						Type: "text",
						Text: "错误: conversation_id 未设置。这是系统错误，请重试。",
					},
				},
				IsError: true,
			}, nil
		}

		title, ok := args["title"].(string)
		if !ok || title == "" {
			return &mcp.ToolResult{
				Content: []mcp.Content{
					{
						Type: "text",
						Text: "错误: title 参数必需且不能为空",
					},
				},
				IsError: true,
			}, nil
		}

		severity, ok := args["severity"].(string)
		if !ok || severity == "" {
			return &mcp.ToolResult{
				Content: []mcp.Content{
					{
						Type: "text",
						Text: "错误: severity 参数必需且不能为空",
					},
				},
				IsError: true,
			}, nil
		}

		// 验证严重程度
		validSeverities := map[string]bool{
			"critical": true,
			"high":     true,
			"medium":   true,
			"low":      true,
			"info":     true,
		}
		if !validSeverities[severity] {
			return &mcp.ToolResult{
				Content: []mcp.Content{
					{
						Type: "text",
						Text: fmt.Sprintf("错误: severity 必须是 critical、high、medium、low 或 info 之一，当前值: %s", severity),
					},
				},
				IsError: true,
			}, nil
		}

		// 获取可选参数
		description := ""
		if d, ok := args["description"].(string); ok {
			description = d
		}

		vulnType := ""
		if t, ok := args["vulnerability_type"].(string); ok {
			vulnType = t
		}

		target := ""
		if t, ok := args["target"].(string); ok {
			target = t
		}

		proof := ""
		if p, ok := args["proof"].(string); ok {
			proof = p
		}

		impact := ""
		if i, ok := args["impact"].(string); ok {
			impact = i
		}

		recommendation := ""
		if r, ok := args["recommendation"].(string); ok {
			recommendation = r
		}

		// 创建漏洞记录
		vuln := &database.Vulnerability{
			ConversationID: conversationID,
			Title:          title,
			Description:    description,
			Severity:       severity,
			Status:          "open",
			Type:           vulnType,
			Target:         target,
			Proof:          proof,
			Impact:         impact,
			Recommendation: recommendation,
		}

		created, err := db.CreateVulnerability(vuln)
		if err != nil {
			logger.Error("记录漏洞失败", zap.Error(err))
			return &mcp.ToolResult{
				Content: []mcp.Content{
					{
						Type: "text",
						Text: fmt.Sprintf("记录漏洞失败: %v", err),
					},
				},
				IsError: true,
			}, nil
		}

		logger.Info("漏洞记录成功",
			zap.String("id", created.ID),
			zap.String("title", created.Title),
			zap.String("severity", created.Severity),
			zap.String("conversation_id", conversationID),
		)

		return &mcp.ToolResult{
			Content: []mcp.Content{
				{
					Type: "text",
					Text: fmt.Sprintf("漏洞已成功记录！\n\n漏洞ID: %s\n标题: %s\n严重程度: %s\n状态: %s\n\n你可以在漏洞管理页面查看和管理此漏洞。", created.ID, created.Title, created.Severity, created.Status),
				},
			},
			IsError: false,
		}, nil
	}

	mcpServer.RegisterTool(tool, handler)
	logger.Info("漏洞记录工具注册成功")
}

// corsMiddleware CORS中间件
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
