package app

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"cyberstrike-ai/internal/agent"
	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/database"
	"cyberstrike-ai/internal/handler"
	"cyberstrike-ai/internal/logger"
	"cyberstrike-ai/internal/mcp"
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

	// 获取配置文件路径
	configPath := "config.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	// 创建处理器
	agentHandler := handler.NewAgentHandler(agent, db, log.Logger)
	monitorHandler := handler.NewMonitorHandler(mcpServer, executor, db, log.Logger)
	monitorHandler.SetExternalMCPManager(externalMCPMgr) // 设置外部MCP管理器，以便获取外部MCP执行记录
	conversationHandler := handler.NewConversationHandler(db, log.Logger)
	authHandler := handler.NewAuthHandler(authManager, cfg, configPath, log.Logger)
	configHandler := handler.NewConfigHandler(configPath, cfg, mcpServer, executor, agent, externalMCPMgr, log.Logger)
	externalMCPHandler := handler.NewExternalMCPHandler(externalMCPMgr, cfg, configPath, log.Logger)
	attackChainHandler := handler.NewAttackChainHandler(db, &cfg.OpenAI, log.Logger)

	// 设置路由
	setupRoutes(
		router,
		authHandler,
		agentHandler,
		monitorHandler,
		conversationHandler,
		configHandler,
		externalMCPHandler,
		attackChainHandler,
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
}

// setupRoutes 设置路由
func setupRoutes(
	router *gin.Engine,
	authHandler *handler.AuthHandler,
	agentHandler *handler.AgentHandler,
	monitorHandler *handler.MonitorHandler,
	conversationHandler *handler.ConversationHandler,
	configHandler *handler.ConfigHandler,
	externalMCPHandler *handler.ExternalMCPHandler,
	attackChainHandler *handler.AttackChainHandler,
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
		protected.DELETE("/conversations/:id", conversationHandler.DeleteConversation)

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
