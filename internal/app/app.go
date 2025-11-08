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

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// App 应用
type App struct {
	config    *config.Config
	logger    *logger.Logger
	router    *gin.Engine
	mcpServer *mcp.Server
	agent     *agent.Agent
	executor  *security.Executor
	db        *database.DB
}

// New 创建新应用
func New(cfg *config.Config, log *logger.Logger) (*App, error) {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	// CORS中间件
	router.Use(corsMiddleware())

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

	// 创建MCP服务器
	mcpServer := mcp.NewServer(log.Logger)

	// 创建安全工具执行器
	executor := security.NewExecutor(&cfg.Security, mcpServer, log.Logger)

	// 注册工具
	executor.RegisterTools(mcpServer)

	// 创建Agent
	agent := agent.NewAgent(&cfg.OpenAI, mcpServer, log.Logger)

	// 创建处理器
	agentHandler := handler.NewAgentHandler(agent, db, log.Logger)
	monitorHandler := handler.NewMonitorHandler(mcpServer, executor, log.Logger)
	conversationHandler := handler.NewConversationHandler(db, log.Logger)

	// 设置路由
	setupRoutes(router, agentHandler, monitorHandler, conversationHandler, mcpServer)

	return &App{
		config:    cfg,
		logger:    log,
		router:    router,
		mcpServer: mcpServer,
		agent:     agent,
		executor:  executor,
		db:        db,
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

// setupRoutes 设置路由
func setupRoutes(router *gin.Engine, agentHandler *handler.AgentHandler, monitorHandler *handler.MonitorHandler, conversationHandler *handler.ConversationHandler, mcpServer *mcp.Server) {
	// API路由
	api := router.Group("/api")
	{
		// Agent Loop
		api.POST("/agent-loop", agentHandler.AgentLoop)
		
		// 对话历史
		api.POST("/conversations", conversationHandler.CreateConversation)
		api.GET("/conversations", conversationHandler.ListConversations)
		api.GET("/conversations/:id", conversationHandler.GetConversation)
		api.DELETE("/conversations/:id", conversationHandler.DeleteConversation)
		
		// 监控
		api.GET("/monitor", monitorHandler.Monitor)
		api.GET("/monitor/execution/:id", monitorHandler.GetExecution)
		api.GET("/monitor/stats", monitorHandler.GetStats)
		api.GET("/monitor/vulnerabilities", monitorHandler.GetVulnerabilities)
		
		// MCP端点
		api.POST("/mcp", func(c *gin.Context) {
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

