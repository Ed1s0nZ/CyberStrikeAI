package app

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"cyberstrike-ai/internal/agent"
	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/database"
	"cyberstrike-ai/internal/handler"
	"cyberstrike-ai/internal/knowledge"
	"cyberstrike-ai/internal/logger"
	"cyberstrike-ai/internal/mcp"
	"cyberstrike-ai/internal/mcp/builtin"
	"cyberstrike-ai/internal/openai"
	"cyberstrike-ai/internal/robot"
	"cyberstrike-ai/internal/security"
	"cyberstrike-ai/internal/skills"
	"cyberstrike-ai/internal/storage"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// App 应用
type App struct {
	config             *config.Config
	logger             *logger.Logger
	router             *gin.Engine
	mcpServer          *mcp.Server
	externalMCPMgr     *mcp.ExternalMCPManager
	agent              *agent.Agent
	executor           *security.Executor
	db                 *database.DB
	knowledgeDB        *database.DB // 知识库数据库连接（如果使用独立数据库）
	auth               *security.AuthManager
	knowledgeManager   *knowledge.Manager        // 知识库管理器（用于动态初始化）
	knowledgeRetriever *knowledge.Retriever      // 知识库检索器（用于动态初始化）
	knowledgeIndexer   *knowledge.Indexer        // 知识库索引器（用于动态初始化）
	knowledgeHandler   *handler.KnowledgeHandler // 知识库处理器（用于动态初始化）
	agentHandler       *handler.AgentHandler     // Agent处理器（用于更新知识库管理器）
	robotHandler       *handler.RobotHandler     // 机器人处理器（钉钉/飞书/企业微信）
	robotMu            sync.Mutex                // 保护钉钉/飞书长连接的 cancel
	dingCancel         context.CancelFunc        // 钉钉 Stream 取消函数，用于配置变更时重启
	larkCancel         context.CancelFunc        // 飞书长连接取消函数，用于配置变更时重启
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

	if err := bootstrapWebRBAC(context.Background(), db, cfg.Auth.Password); err != nil {
		return nil, fmt.Errorf("初始化引导管理员失败: %w", err)
	}

	// 认证管理器
	authManager := security.NewAuthManager(db, cfg.Auth.SessionDurationHours)

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
		knowledgeIndexer = knowledge.NewIndexer(knowledgeDB, embedder, log.Logger, &cfg.Knowledge.Indexing)

		// 注册知识检索工具到MCP服务器
		knowledge.RegisterKnowledgeTool(mcpServer, knowledgeRetriever, knowledgeManager, log.Logger)

		// 创建知识库API处理器
		knowledgeHandler = handler.NewKnowledgeHandler(knowledgeManager, knowledgeRetriever, knowledgeIndexer, db, log.Logger)
		log.Logger.Info("知识库模块初始化完成", zap.Bool("handler_created", knowledgeHandler != nil))

		// 扫描知识库并建立索引（异步）
		go func() {
			itemsToIndex, err := knowledgeManager.ScanKnowledgeBase()
			if err != nil {
				log.Logger.Warn("扫描知识库失败", zap.Error(err))
				return
			}

			// 检查是否已有索引
			hasIndex, err := knowledgeIndexer.HasIndex()
			if err != nil {
				log.Logger.Warn("检查索引状态失败", zap.Error(err))
				return
			}

			if hasIndex {
				// 如果已有索引，只索引新添加或更新的项
				if len(itemsToIndex) > 0 {
					log.Logger.Info("检测到已有知识库索引，开始增量索引", zap.Int("count", len(itemsToIndex)))
					ctx := context.Background()
					consecutiveFailures := 0
					var firstFailureItemID string
					var firstFailureError error
					failedCount := 0

					for _, itemID := range itemsToIndex {
						if err := knowledgeIndexer.IndexItem(ctx, itemID); err != nil {
							failedCount++
							consecutiveFailures++

							if consecutiveFailures == 1 {
								firstFailureItemID = itemID
								firstFailureError = err
								log.Logger.Warn("索引知识项失败", zap.String("itemId", itemID), zap.Error(err))
							}

							// 如果连续失败2次，立即停止增量索引
							if consecutiveFailures >= 2 {
								log.Logger.Error("连续索引失败次数过多，立即停止增量索引",
									zap.Int("consecutiveFailures", consecutiveFailures),
									zap.Int("totalItems", len(itemsToIndex)),
									zap.String("firstFailureItemId", firstFailureItemID),
									zap.Error(firstFailureError),
								)
								break
							}
							continue
						}

						// 成功时重置连续失败计数
						if consecutiveFailures > 0 {
							consecutiveFailures = 0
							firstFailureItemID = ""
							firstFailureError = nil
						}
					}
					log.Logger.Info("增量索引完成", zap.Int("totalItems", len(itemsToIndex)), zap.Int("failedCount", failedCount))
				} else {
					log.Logger.Info("检测到已有知识库索引，没有需要索引的新项或更新项")
				}
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

	// 初始化Skills管理器
	skillsDir := cfg.SkillsDir
	if skillsDir == "" {
		skillsDir = "skills" // 默认目录
	}
	// 如果是相对路径，相对于配置文件所在目录
	configDir := filepath.Dir(configPath)
	if !filepath.IsAbs(skillsDir) {
		skillsDir = filepath.Join(configDir, skillsDir)
	}
	skillsManager := skills.NewManager(skillsDir, log.Logger)
	log.Logger.Info("Skills管理器已初始化", zap.String("skillsDir", skillsDir))

	agentsDir := cfg.AgentsDir
	if agentsDir == "" {
		agentsDir = "agents"
	}
	if !filepath.IsAbs(agentsDir) {
		agentsDir = filepath.Join(configDir, agentsDir)
	}
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		log.Logger.Warn("创建 agents 目录失败", zap.String("path", agentsDir), zap.Error(err))
	}
	markdownAgentsHandler := handler.NewMarkdownAgentsHandler(agentsDir)
	log.Logger.Info("多代理 Markdown 子 Agent 目录", zap.String("agentsDir", agentsDir))

	// 注册Skills工具到MCP服务器（让AI可以按需调用，带数据库存储支持统计）
	// 创建一个适配器，将database.DB适配为SkillStatsStorage接口
	var skillStatsStorage skills.SkillStatsStorage
	if db != nil {
		skillStatsStorage = &skillStatsDBAdapter{db: db}
	}
	skills.RegisterSkillsToolWithStorage(mcpServer, skillsManager, skillStatsStorage, log.Logger)

	// 创建处理器
	agentHandler := handler.NewAgentHandler(agent, db, cfg, log.Logger)
	agentHandler.SetSkillsManager(skillsManager) // 设置Skills管理器
	agentHandler.SetAgentsMarkdownDir(agentsDir)
	// 如果知识库已启用，设置知识库管理器到AgentHandler以便记录检索日志
	if knowledgeManager != nil {
		agentHandler.SetKnowledgeManager(knowledgeManager)
	}
	monitorHandler := handler.NewMonitorHandler(mcpServer, executor, db, log.Logger)
	monitorHandler.SetExternalMCPManager(externalMCPMgr) // 设置外部MCP管理器，以便获取外部MCP执行记录
	groupHandler := handler.NewGroupHandler(db, log.Logger)
	authHandler := handler.NewAuthHandler(authManager, cfg, configPath, log.Logger)
	attackChainHandler := handler.NewAttackChainHandler(db, &cfg.OpenAI, log.Logger)
	vulnerabilityHandler := handler.NewVulnerabilityHandler(db, log.Logger)
	webshellHandler := handler.NewWebShellHandler(log.Logger, db)
	chatUploadsHandler := handler.NewChatUploadsHandler(log.Logger)
	registerWebshellTools(mcpServer, db, webshellHandler, log.Logger)
	registerWebshellManagementTools(mcpServer, db, webshellHandler, log.Logger)
	configHandler := handler.NewConfigHandler(configPath, cfg, mcpServer, executor, agent, attackChainHandler, externalMCPMgr, log.Logger)
	externalMCPHandler := handler.NewExternalMCPHandler(externalMCPMgr, cfg, configPath, log.Logger)
	roleHandler := handler.NewRoleHandler(cfg, configPath, log.Logger)
	webUsersHandler := handler.NewWebUsersHandler(db, authManager, log.Logger)
	webAccessRolesHandler := handler.NewWebAccessRolesHandler(db, authManager, log.Logger)
	roleHandler.SetSkillsManager(skillsManager) // 设置Skills管理器到RoleHandler
	skillsHandler := handler.NewSkillsHandler(skillsManager, cfg, configPath, log.Logger)
	fofaHandler := handler.NewFofaHandler(cfg, log.Logger)
	terminalHandler := handler.NewTerminalHandler(log.Logger)
	if db != nil {
		skillsHandler.SetDB(db) // 设置数据库连接以便获取调用统计
	}

	// 创建OpenAPI处理器
	conversationHandler := handler.NewConversationHandler(db, log.Logger)
	robotHandler := handler.NewRobotHandler(cfg, db, agentHandler, log.Logger)
	openAPIHandler := handler.NewOpenAPIHandler(db, log.Logger, resultStorage, conversationHandler, agentHandler)

	// 创建 App 实例（部分字段稍后填充）
	app := &App{
		config:             cfg,
		logger:             log,
		router:             router,
		mcpServer:          mcpServer,
		externalMCPMgr:     externalMCPMgr,
		agent:              agent,
		executor:           executor,
		db:                 db,
		knowledgeDB:        knowledgeDBConn,
		auth:               authManager,
		knowledgeManager:   knowledgeManager,
		knowledgeRetriever: knowledgeRetriever,
		knowledgeIndexer:   knowledgeIndexer,
		knowledgeHandler:   knowledgeHandler,
		agentHandler:       agentHandler,
		robotHandler:       robotHandler,
	}
	// 飞书/钉钉长连接（无需公网），启用时在后台启动；后续前端应用配置时会通过 RestartRobotConnections 重启
	app.startRobotConnections()

	// 设置漏洞工具注册器（内置工具，必须设置）
	vulnerabilityRegistrar := func() error {
		registerVulnerabilityTool(mcpServer, db, log.Logger)
		return nil
	}
	configHandler.SetVulnerabilityToolRegistrar(vulnerabilityRegistrar)

	// 设置 WebShell 工具注册器（ApplyConfig 时重新注册）
	webshellRegistrar := func() error {
		registerWebshellTools(mcpServer, db, webshellHandler, log.Logger)
		registerWebshellManagementTools(mcpServer, db, webshellHandler, log.Logger)
		return nil
	}
	configHandler.SetWebshellToolRegistrar(webshellRegistrar)

	// 设置Skills工具注册器（内置工具，必须设置）
	skillsRegistrar := func() error {
		// 创建一个适配器，将database.DB适配为SkillStatsStorage接口
		var skillStatsStorage skills.SkillStatsStorage
		if db != nil {
			skillStatsStorage = &skillStatsDBAdapter{db: db}
		}
		skills.RegisterSkillsToolWithStorage(mcpServer, skillsManager, skillStatsStorage, log.Logger)
		return nil
	}
	configHandler.SetSkillsToolRegistrar(skillsRegistrar)

	handler.RegisterBatchTaskMCPTools(mcpServer, agentHandler, log.Logger)
	batchTaskToolRegistrar := func() error {
		handler.RegisterBatchTaskMCPTools(mcpServer, agentHandler, log.Logger)
		return nil
	}
	configHandler.SetBatchTaskToolRegistrar(batchTaskToolRegistrar)

	// 设置知识库初始化器（用于动态初始化，需要在 App 创建后设置）
	configHandler.SetKnowledgeInitializer(func() (*handler.KnowledgeHandler, error) {
		knowledgeHandler, err := initializeKnowledge(cfg, db, knowledgeDBConn, mcpServer, agentHandler, app, log.Logger)
		if err != nil {
			return nil, err
		}

		// 动态初始化后，设置知识库工具注册器和检索器更新器
		// 这样后续 ApplyConfig 时就能重新注册工具了
		if app.knowledgeRetriever != nil && app.knowledgeManager != nil {
			// 创建闭包，捕获knowledgeRetriever和knowledgeManager的引用
			registrar := func() error {
				knowledge.RegisterKnowledgeTool(mcpServer, app.knowledgeRetriever, app.knowledgeManager, log.Logger)
				return nil
			}
			configHandler.SetKnowledgeToolRegistrar(registrar)
			// 设置检索器更新器，以便在ApplyConfig时更新检索器配置
			configHandler.SetRetrieverUpdater(app.knowledgeRetriever)
			log.Logger.Info("动态初始化后已设置知识库工具注册器和检索器更新器")
		}

		return knowledgeHandler, nil
	})

	// 如果知识库已启用，设置知识库工具注册器和检索器更新器
	if cfg.Knowledge.Enabled && knowledgeRetriever != nil && knowledgeManager != nil {
		// 创建闭包，捕获knowledgeRetriever和knowledgeManager的引用
		registrar := func() error {
			knowledge.RegisterKnowledgeTool(mcpServer, knowledgeRetriever, knowledgeManager, log.Logger)
			return nil
		}
		configHandler.SetKnowledgeToolRegistrar(registrar)
		// 设置检索器更新器，以便在ApplyConfig时更新检索器配置
		configHandler.SetRetrieverUpdater(knowledgeRetriever)
	}

	// 设置机器人连接重启器，前端应用配置后无需重启服务即可使钉钉/飞书新配置生效
	configHandler.SetRobotRestarter(app)

	// 设置路由（使用 App 实例以便动态获取 handler）
	setupRoutes(
		router,
		authHandler,
		agentHandler,
		monitorHandler,
		conversationHandler,
		robotHandler,
		groupHandler,
		configHandler,
		externalMCPHandler,
		attackChainHandler,
		app, // 传递 App 实例以便动态获取 knowledgeHandler
		vulnerabilityHandler,
		webshellHandler,
		chatUploadsHandler,
		roleHandler,
		webUsersHandler,
		webAccessRolesHandler,
		skillsHandler,
		markdownAgentsHandler,
		fofaHandler,
		terminalHandler,
		mcpServer,
		authManager,
		openAPIHandler,
	)

	return app, nil

}

// mcpHandlerWithAuth 在鉴权通过后转发到 MCP 处理；若配置了 auth_header 则校验请求头，否则直接放行
func (a *App) mcpHandlerWithAuth(w http.ResponseWriter, r *http.Request) {
	cfg := a.config.MCP
	if cfg.AuthHeader != "" {
		if r.Header.Get(cfg.AuthHeader) != cfg.AuthHeaderValue {
			a.logger.Logger.Debug("MCP 鉴权失败：header 缺失或值不匹配", zap.String("header", cfg.AuthHeader))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"unauthorized"}`))
			return
		}
	}
	a.mcpServer.HandleHTTP(w, r)
}

// Run 启动应用
func (a *App) Run() error {
	// 启动MCP服务器（如果启用）
	if a.config.MCP.Enabled {
		go func() {
			mcpAddr := fmt.Sprintf("%s:%d", a.config.MCP.Host, a.config.MCP.Port)
			a.logger.Info("启动MCP服务器", zap.String("address", mcpAddr))

			mux := http.NewServeMux()
			mux.HandleFunc("/mcp", a.mcpHandlerWithAuth)

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
	// 停止钉钉/飞书长连接
	a.robotMu.Lock()
	if a.dingCancel != nil {
		a.dingCancel()
		a.dingCancel = nil
	}
	if a.larkCancel != nil {
		a.larkCancel()
		a.larkCancel = nil
	}
	a.robotMu.Unlock()

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

// startRobotConnections 根据当前配置启动钉钉/飞书长连接（不先关闭已有连接，仅用于首次启动）
func (a *App) startRobotConnections() {
	a.robotMu.Lock()
	defer a.robotMu.Unlock()
	cfg := a.config
	if cfg.Robots.Lark.Enabled && cfg.Robots.Lark.AppID != "" && cfg.Robots.Lark.AppSecret != "" {
		ctx, cancel := context.WithCancel(context.Background())
		a.larkCancel = cancel
		go robot.StartLark(ctx, cfg.Robots.Lark, a.robotHandler, a.logger.Logger)
	}
	if cfg.Robots.Dingtalk.Enabled && cfg.Robots.Dingtalk.ClientID != "" && cfg.Robots.Dingtalk.ClientSecret != "" {
		ctx, cancel := context.WithCancel(context.Background())
		a.dingCancel = cancel
		go robot.StartDing(ctx, cfg.Robots.Dingtalk, a.robotHandler, a.logger.Logger)
	}
}

// RestartRobotConnections 重启钉钉/飞书长连接，使前端应用配置后立即生效（实现 handler.RobotRestarter）
func (a *App) RestartRobotConnections() {
	a.robotMu.Lock()
	if a.dingCancel != nil {
		a.dingCancel()
		a.dingCancel = nil
	}
	if a.larkCancel != nil {
		a.larkCancel()
		a.larkCancel = nil
	}
	a.robotMu.Unlock()
	// 给旧 goroutine 一点时间退出
	time.Sleep(200 * time.Millisecond)
	a.startRobotConnections()
}

// setupRoutes 设置路由
func setupRoutes(
	router *gin.Engine,
	authHandler *handler.AuthHandler,
	agentHandler *handler.AgentHandler,
	monitorHandler *handler.MonitorHandler,
	conversationHandler *handler.ConversationHandler,
	robotHandler *handler.RobotHandler,
	groupHandler *handler.GroupHandler,
	configHandler *handler.ConfigHandler,
	externalMCPHandler *handler.ExternalMCPHandler,
	attackChainHandler *handler.AttackChainHandler,
	app *App, // 传递 App 实例以便动态获取 knowledgeHandler
	vulnerabilityHandler *handler.VulnerabilityHandler,
	webshellHandler *handler.WebShellHandler,
	chatUploadsHandler *handler.ChatUploadsHandler,
	roleHandler *handler.RoleHandler,
	webUsersHandler *handler.WebUsersHandler,
	webAccessRolesHandler *handler.WebAccessRolesHandler,
	skillsHandler *handler.SkillsHandler,
	markdownAgentsHandler *handler.MarkdownAgentsHandler,
	fofaHandler *handler.FofaHandler,
	terminalHandler *handler.TerminalHandler,
	mcpServer *mcp.Server,
	authManager *security.AuthManager,
	openAPIHandler *handler.OpenAPIHandler,
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

	// 机器人回调（无需登录，供企业微信/钉钉/飞书服务器调用）
	api.GET("/robot/wecom", robotHandler.HandleWecomGET)
	api.POST("/robot/wecom", robotHandler.HandleWecomPOST)
	api.POST("/robot/dingtalk", robotHandler.HandleDingtalkPOST)
	api.POST("/robot/lark", robotHandler.HandleLarkPOST)

	protected := api.Group("")
	protected.Use(security.AuthMiddleware(authManager))
	{
		// 机器人测试（需登录）：POST /api/robot/test，body: {"platform":"dingtalk","user_id":"test","text":"帮助"}，用于验证机器人逻辑
		protected.POST("/robot/test", security.RequireRoutePermission(http.MethodPost, "/robot/test"), robotHandler.HandleRobotTest)

		// Agent Loop
		protected.POST("/agent-loop", security.RequireRoutePermission(http.MethodPost, "/agent-loop"), agentHandler.AgentLoop)
		// Agent Loop 流式输出
		protected.POST("/agent-loop/stream", security.RequireRoutePermission(http.MethodPost, "/agent-loop/stream"), agentHandler.AgentLoopStream)
		// Agent Loop 取消与任务列表
		protected.POST("/agent-loop/cancel", security.RequireRoutePermission(http.MethodPost, "/agent-loop/cancel"), agentHandler.CancelAgentLoop)
		protected.GET("/agent-loop/tasks", security.RequireRoutePermission(http.MethodGet, "/agent-loop/tasks"), agentHandler.ListAgentTasks)
		protected.GET("/agent-loop/tasks/completed", security.RequireRoutePermission(http.MethodGet, "/agent-loop/tasks/completed"), agentHandler.ListCompletedTasks)

		// Eino DeepAgent 多代理（与单 Agent 并存，需 config.multi_agent.enabled）
		// 多代理路由常注册；是否可用由运行时 h.config.MultiAgent.Enabled 决定（应用配置后无需重启）
		protected.POST("/multi-agent", security.RequireRoutePermission(http.MethodPost, "/multi-agent"), agentHandler.MultiAgentLoop)
		protected.POST("/multi-agent/stream", security.RequireRoutePermission(http.MethodPost, "/multi-agent/stream"), agentHandler.MultiAgentLoopStream)
		protected.GET("/multi-agent/markdown-agents", security.RequireRoutePermission(http.MethodGet, "/multi-agent/markdown-agents"), markdownAgentsHandler.ListMarkdownAgents)
		protected.GET("/multi-agent/markdown-agents/:filename", security.RequireRoutePermission(http.MethodGet, "/multi-agent/markdown-agents/:filename"), markdownAgentsHandler.GetMarkdownAgent)
		protected.POST("/multi-agent/markdown-agents", security.RequireRoutePermission(http.MethodPost, "/multi-agent/markdown-agents"), markdownAgentsHandler.CreateMarkdownAgent)
		protected.PUT("/multi-agent/markdown-agents/:filename", security.RequireRoutePermission(http.MethodPut, "/multi-agent/markdown-agents/:filename"), markdownAgentsHandler.UpdateMarkdownAgent)
		protected.DELETE("/multi-agent/markdown-agents/:filename", security.RequireRoutePermission(http.MethodDelete, "/multi-agent/markdown-agents/:filename"), markdownAgentsHandler.DeleteMarkdownAgent)

		// 信息收集 - FOFA 查询（后端代理）
		protected.POST("/fofa/search", security.RequireRoutePermission(http.MethodPost, "/fofa/search"), fofaHandler.Search)
		// 信息收集 - 自然语言解析为 FOFA 语法（需人工确认后再查询）
		protected.POST("/fofa/parse", security.RequireRoutePermission(http.MethodPost, "/fofa/parse"), fofaHandler.ParseNaturalLanguage)

		// 批量任务管理
		protected.POST("/batch-tasks", security.RequireRoutePermission(http.MethodPost, "/batch-tasks"), agentHandler.CreateBatchQueue)
		protected.GET("/batch-tasks", security.RequireRoutePermission(http.MethodGet, "/batch-tasks"), agentHandler.ListBatchQueues)
		protected.GET("/batch-tasks/:queueId", security.RequireRoutePermission(http.MethodGet, "/batch-tasks/:queueId"), agentHandler.GetBatchQueue)
		protected.POST("/batch-tasks/:queueId/start", security.RequireRoutePermission(http.MethodPost, "/batch-tasks/:queueId/start"), agentHandler.StartBatchQueue)
		protected.POST("/batch-tasks/:queueId/pause", security.RequireRoutePermission(http.MethodPost, "/batch-tasks/:queueId/pause"), agentHandler.PauseBatchQueue)
		protected.PUT("/batch-tasks/:queueId/schedule-enabled", security.RequireRoutePermission(http.MethodPut, "/batch-tasks/:queueId/schedule-enabled"), agentHandler.SetBatchQueueScheduleEnabled)
		protected.DELETE("/batch-tasks/:queueId", security.RequireRoutePermission(http.MethodDelete, "/batch-tasks/:queueId"), agentHandler.DeleteBatchQueue)
		protected.PUT("/batch-tasks/:queueId/tasks/:taskId", security.RequireRoutePermission(http.MethodPut, "/batch-tasks/:queueId/tasks/:taskId"), agentHandler.UpdateBatchTask)
		protected.POST("/batch-tasks/:queueId/tasks", security.RequireRoutePermission(http.MethodPost, "/batch-tasks/:queueId/tasks"), agentHandler.AddBatchTask)
		protected.DELETE("/batch-tasks/:queueId/tasks/:taskId", security.RequireRoutePermission(http.MethodDelete, "/batch-tasks/:queueId/tasks/:taskId"), agentHandler.DeleteBatchTask)

		// 对话历史
		protected.POST("/conversations", security.RequireRoutePermission(http.MethodPost, "/conversations"), conversationHandler.CreateConversation)
		protected.GET("/conversations", security.RequireRoutePermission(http.MethodGet, "/conversations"), conversationHandler.ListConversations)
		protected.GET("/conversations/:id", security.RequireRoutePermission(http.MethodGet, "/conversations/:id"), conversationHandler.GetConversation)
		protected.GET("/messages/:id/process-details", security.RequireRoutePermission(http.MethodGet, "/messages/:id/process-details"), conversationHandler.GetMessageProcessDetails)
		protected.PUT("/conversations/:id", security.RequireRoutePermission(http.MethodPut, "/conversations/:id"), conversationHandler.UpdateConversation)
		protected.DELETE("/conversations/:id", security.RequireRoutePermission(http.MethodDelete, "/conversations/:id"), conversationHandler.DeleteConversation)
		protected.POST("/conversations/:id/delete-turn", security.RequireRoutePermission(http.MethodPost, "/conversations/:id/delete-turn"), conversationHandler.DeleteConversationTurn)
		protected.PUT("/conversations/:id/pinned", security.RequireRoutePermission(http.MethodPut, "/conversations/:id/pinned"), groupHandler.UpdateConversationPinned)

		// 对话分组
		protected.POST("/groups", security.RequireRoutePermission(http.MethodPost, "/groups"), groupHandler.CreateGroup)
		protected.GET("/groups", security.RequireRoutePermission(http.MethodGet, "/groups"), groupHandler.ListGroups)
		protected.GET("/groups/:id", security.RequireRoutePermission(http.MethodGet, "/groups/:id"), groupHandler.GetGroup)
		protected.PUT("/groups/:id", security.RequireRoutePermission(http.MethodPut, "/groups/:id"), groupHandler.UpdateGroup)
		protected.DELETE("/groups/:id", security.RequireRoutePermission(http.MethodDelete, "/groups/:id"), groupHandler.DeleteGroup)
		protected.PUT("/groups/:id/pinned", security.RequireRoutePermission(http.MethodPut, "/groups/:id/pinned"), groupHandler.UpdateGroupPinned)
		protected.GET("/groups/:id/conversations", security.RequireRoutePermission(http.MethodGet, "/groups/:id/conversations"), groupHandler.GetGroupConversations)
		protected.GET("/groups/mappings", security.RequireRoutePermission(http.MethodGet, "/groups/mappings"), groupHandler.GetAllMappings)
		protected.POST("/groups/conversations", security.RequireRoutePermission(http.MethodPost, "/groups/conversations"), groupHandler.AddConversationToGroup)
		protected.DELETE("/groups/:id/conversations/:conversationId", security.RequireRoutePermission(http.MethodDelete, "/groups/:id/conversations/:conversationId"), groupHandler.RemoveConversationFromGroup)
		protected.PUT("/groups/:id/conversations/:conversationId/pinned", security.RequireRoutePermission(http.MethodPut, "/groups/:id/conversations/:conversationId/pinned"), groupHandler.UpdateConversationPinnedInGroup)

		// 监控
		protected.GET("/monitor", security.RequireRoutePermission(http.MethodGet, "/monitor"), monitorHandler.Monitor)
		protected.GET("/monitor/execution/:id", security.RequireRoutePermission(http.MethodGet, "/monitor/execution/:id"), monitorHandler.GetExecution)
		protected.POST("/monitor/executions/names", security.RequireRoutePermission(http.MethodPost, "/monitor/executions/names"), monitorHandler.BatchGetToolNames)
		protected.DELETE("/monitor/execution/:id", security.RequireRoutePermission(http.MethodDelete, "/monitor/execution/:id"), monitorHandler.DeleteExecution)
		protected.DELETE("/monitor/executions", security.RequireRoutePermission(http.MethodDelete, "/monitor/executions"), monitorHandler.DeleteExecutions)
		protected.GET("/monitor/stats", security.RequireRoutePermission(http.MethodGet, "/monitor/stats"), monitorHandler.GetStats)

		// 配置管理
		protected.GET("/config", security.RequireRoutePermission(http.MethodGet, "/config"), configHandler.GetConfig)
		protected.GET("/config/tools", security.RequireRoutePermission(http.MethodGet, "/config/tools"), configHandler.GetTools)
		protected.PUT("/config", security.RequireRoutePermission(http.MethodPut, "/config"), configHandler.UpdateConfig)
		protected.POST("/config/apply", security.RequireRoutePermission(http.MethodPost, "/config/apply"), configHandler.ApplyConfig)
		protected.POST("/config/test-openai", security.RequireRoutePermission(http.MethodPost, "/config/test-openai"), configHandler.TestOpenAI)

		// Web 用户与 Web 访问角色管理
		protected.GET("/security/web-users", security.RequireRoutePermission(http.MethodGet, "/security/web-users"), webUsersHandler.ListWebUsers)
		protected.POST("/security/web-users", security.RequireRoutePermission(http.MethodPost, "/security/web-users"), webUsersHandler.CreateWebUser)
		protected.PUT("/security/web-users/:id", security.RequireRoutePermission(http.MethodPut, "/security/web-users/:id"), webUsersHandler.UpdateWebUser)
		protected.POST("/security/web-users/:id/reset-password", security.RequireRoutePermission(http.MethodPost, "/security/web-users/:id/reset-password"), webUsersHandler.ResetWebUserPassword)
		protected.DELETE("/security/web-users/:id", security.RequireRoutePermission(http.MethodDelete, "/security/web-users/:id"), webUsersHandler.DeleteWebUser)
		protected.GET("/security/web-access-roles", security.RequireRoutePermission(http.MethodGet, "/security/web-access-roles"), webAccessRolesHandler.ListWebAccessRoles)
		protected.POST("/security/web-access-roles", security.RequireRoutePermission(http.MethodPost, "/security/web-access-roles"), webAccessRolesHandler.CreateWebAccessRole)
		protected.PUT("/security/web-access-roles/:id", security.RequireRoutePermission(http.MethodPut, "/security/web-access-roles/:id"), webAccessRolesHandler.UpdateWebAccessRole)
		protected.DELETE("/security/web-access-roles/:id", security.RequireRoutePermission(http.MethodDelete, "/security/web-access-roles/:id"), webAccessRolesHandler.DeleteWebAccessRole)
		protected.GET("/security/web-access-roles/permission-catalog", security.RequireRoutePermission(http.MethodGet, "/security/web-access-roles/permission-catalog"), webAccessRolesHandler.GetPermissionCatalog)

		// 系统设置 - 终端（执行命令，提高运维效率）
		protected.POST("/terminal/run", security.RequireRoutePermission(http.MethodPost, "/terminal/run"), terminalHandler.RunCommand)
		protected.POST("/terminal/run/stream", security.RequireRoutePermission(http.MethodPost, "/terminal/run/stream"), terminalHandler.RunCommandStream)
		protected.GET("/terminal/ws", security.RequireRoutePermission(http.MethodGet, "/terminal/ws"), terminalHandler.RunCommandWS)

		// 外部MCP管理
		protected.GET("/external-mcp", security.RequireRoutePermission(http.MethodGet, "/external-mcp"), externalMCPHandler.GetExternalMCPs)
		protected.GET("/external-mcp/stats", security.RequireRoutePermission(http.MethodGet, "/external-mcp/stats"), externalMCPHandler.GetExternalMCPStats)
		protected.GET("/external-mcp/:name", security.RequireRoutePermission(http.MethodGet, "/external-mcp/:name"), externalMCPHandler.GetExternalMCP)
		protected.PUT("/external-mcp/:name", security.RequireRoutePermission(http.MethodPut, "/external-mcp/:name"), externalMCPHandler.AddOrUpdateExternalMCP)
		protected.DELETE("/external-mcp/:name", security.RequireRoutePermission(http.MethodDelete, "/external-mcp/:name"), externalMCPHandler.DeleteExternalMCP)
		protected.POST("/external-mcp/:name/start", security.RequireRoutePermission(http.MethodPost, "/external-mcp/:name/start"), externalMCPHandler.StartExternalMCP)
		protected.POST("/external-mcp/:name/stop", security.RequireRoutePermission(http.MethodPost, "/external-mcp/:name/stop"), externalMCPHandler.StopExternalMCP)

		// 攻击链可视化
		protected.GET("/attack-chain/:conversationId", security.RequireRoutePermission(http.MethodGet, "/attack-chain/:conversationId"), attackChainHandler.GetAttackChain)
		protected.POST("/attack-chain/:conversationId/regenerate", security.RequireRoutePermission(http.MethodPost, "/attack-chain/:conversationId/regenerate"), attackChainHandler.RegenerateAttackChain)

		// 知识库管理（始终注册路由，通过 App 实例动态获取 handler）
		knowledgeRoutes := protected.Group("/knowledge")
		{
			knowledgeRoutes.GET("/categories", security.RequireRoutePermission(http.MethodGet, "/knowledge/categories"), func(c *gin.Context) {
				if app.knowledgeHandler == nil {
					c.JSON(http.StatusOK, gin.H{
						"categories": []string{},
						"enabled":    false,
						"message":    "知识库功能未启用，请前往系统设置启用知识检索功能",
					})
					return
				}
				app.knowledgeHandler.GetCategories(c)
			})
			knowledgeRoutes.GET("/items", security.RequireRoutePermission(http.MethodGet, "/knowledge/items"), func(c *gin.Context) {
				if app.knowledgeHandler == nil {
					c.JSON(http.StatusOK, gin.H{
						"items":   []interface{}{},
						"enabled": false,
						"message": "知识库功能未启用，请前往系统设置启用知识检索功能",
					})
					return
				}
				app.knowledgeHandler.GetItems(c)
			})
			knowledgeRoutes.GET("/items/:id", security.RequireRoutePermission(http.MethodGet, "/knowledge/items/:id"), func(c *gin.Context) {
				if app.knowledgeHandler == nil {
					c.JSON(http.StatusOK, gin.H{
						"enabled": false,
						"message": "知识库功能未启用，请前往系统设置启用知识检索功能",
					})
					return
				}
				app.knowledgeHandler.GetItem(c)
			})
			knowledgeRoutes.POST("/items", security.RequireRoutePermission(http.MethodPost, "/knowledge/items"), func(c *gin.Context) {
				if app.knowledgeHandler == nil {
					c.JSON(http.StatusOK, gin.H{
						"enabled": false,
						"error":   "知识库功能未启用，请前往系统设置启用知识检索功能",
					})
					return
				}
				app.knowledgeHandler.CreateItem(c)
			})
			knowledgeRoutes.PUT("/items/:id", security.RequireRoutePermission(http.MethodPut, "/knowledge/items/:id"), func(c *gin.Context) {
				if app.knowledgeHandler == nil {
					c.JSON(http.StatusOK, gin.H{
						"enabled": false,
						"error":   "知识库功能未启用，请前往系统设置启用知识检索功能",
					})
					return
				}
				app.knowledgeHandler.UpdateItem(c)
			})
			knowledgeRoutes.DELETE("/items/:id", security.RequireRoutePermission(http.MethodDelete, "/knowledge/items/:id"), func(c *gin.Context) {
				if app.knowledgeHandler == nil {
					c.JSON(http.StatusOK, gin.H{
						"enabled": false,
						"error":   "知识库功能未启用，请前往系统设置启用知识检索功能",
					})
					return
				}
				app.knowledgeHandler.DeleteItem(c)
			})
			knowledgeRoutes.GET("/index-status", security.RequireRoutePermission(http.MethodGet, "/knowledge/index-status"), func(c *gin.Context) {
				if app.knowledgeHandler == nil {
					c.JSON(http.StatusOK, gin.H{
						"enabled":          false,
						"total_items":      0,
						"indexed_items":    0,
						"progress_percent": 0,
						"is_complete":      false,
						"message":          "知识库功能未启用，请前往系统设置启用知识检索功能",
					})
					return
				}
				app.knowledgeHandler.GetIndexStatus(c)
			})
			knowledgeRoutes.POST("/index", security.RequireRoutePermission(http.MethodPost, "/knowledge/index"), func(c *gin.Context) {
				if app.knowledgeHandler == nil {
					c.JSON(http.StatusOK, gin.H{
						"enabled": false,
						"error":   "知识库功能未启用，请前往系统设置启用知识检索功能",
					})
					return
				}
				app.knowledgeHandler.RebuildIndex(c)
			})
			knowledgeRoutes.POST("/scan", security.RequireRoutePermission(http.MethodPost, "/knowledge/scan"), func(c *gin.Context) {
				if app.knowledgeHandler == nil {
					c.JSON(http.StatusOK, gin.H{
						"enabled": false,
						"error":   "知识库功能未启用，请前往系统设置启用知识检索功能",
					})
					return
				}
				app.knowledgeHandler.ScanKnowledgeBase(c)
			})
			knowledgeRoutes.GET("/retrieval-logs", security.RequireRoutePermission(http.MethodGet, "/knowledge/retrieval-logs"), func(c *gin.Context) {
				if app.knowledgeHandler == nil {
					c.JSON(http.StatusOK, gin.H{
						"logs":    []interface{}{},
						"enabled": false,
						"message": "知识库功能未启用，请前往系统设置启用知识检索功能",
					})
					return
				}
				app.knowledgeHandler.GetRetrievalLogs(c)
			})
			knowledgeRoutes.DELETE("/retrieval-logs/:id", security.RequireRoutePermission(http.MethodDelete, "/knowledge/retrieval-logs/:id"), func(c *gin.Context) {
				if app.knowledgeHandler == nil {
					c.JSON(http.StatusOK, gin.H{
						"enabled": false,
						"error":   "知识库功能未启用，请前往系统设置启用知识检索功能",
					})
					return
				}
				app.knowledgeHandler.DeleteRetrievalLog(c)
			})
			knowledgeRoutes.POST("/search", security.RequireRoutePermission(http.MethodPost, "/knowledge/search"), func(c *gin.Context) {
				if app.knowledgeHandler == nil {
					c.JSON(http.StatusOK, gin.H{
						"results": []interface{}{},
						"enabled": false,
						"message": "知识库功能未启用，请前往系统设置启用知识检索功能",
					})
					return
				}
				app.knowledgeHandler.Search(c)
			})
			knowledgeRoutes.GET("/stats", security.RequireRoutePermission(http.MethodGet, "/knowledge/stats"), func(c *gin.Context) {
				if app.knowledgeHandler == nil {
					c.JSON(http.StatusOK, gin.H{
						"enabled":          false,
						"total_categories": 0,
						"total_items":      0,
						"message":          "知识库功能未启用，请前往系统设置启用知识检索功能",
					})
					return
				}
				app.knowledgeHandler.GetStats(c)
			})
		}

		// 漏洞管理
		protected.GET("/vulnerabilities", security.RequireRoutePermission(http.MethodGet, "/vulnerabilities"), vulnerabilityHandler.ListVulnerabilities)
		protected.GET("/vulnerabilities/stats", security.RequireRoutePermission(http.MethodGet, "/vulnerabilities/stats"), vulnerabilityHandler.GetVulnerabilityStats)
		protected.GET("/vulnerabilities/:id", security.RequireRoutePermission(http.MethodGet, "/vulnerabilities/:id"), vulnerabilityHandler.GetVulnerability)
		protected.POST("/vulnerabilities", security.RequireRoutePermission(http.MethodPost, "/vulnerabilities"), vulnerabilityHandler.CreateVulnerability)
		protected.PUT("/vulnerabilities/:id", security.RequireRoutePermission(http.MethodPut, "/vulnerabilities/:id"), vulnerabilityHandler.UpdateVulnerability)
		protected.DELETE("/vulnerabilities/:id", security.RequireRoutePermission(http.MethodDelete, "/vulnerabilities/:id"), vulnerabilityHandler.DeleteVulnerability)

		// WebShell 管理（代理执行 + 连接配置存 SQLite）
		protected.GET("/webshell/connections", security.RequireRoutePermission(http.MethodGet, "/webshell/connections"), webshellHandler.ListConnections)
		protected.POST("/webshell/connections", security.RequireRoutePermission(http.MethodPost, "/webshell/connections"), webshellHandler.CreateConnection)
		protected.GET("/webshell/connections/:id/ai-history", security.RequireRoutePermission(http.MethodGet, "/webshell/connections/:id/ai-history"), webshellHandler.GetAIHistory)
		protected.GET("/webshell/connections/:id/ai-conversations", security.RequireRoutePermission(http.MethodGet, "/webshell/connections/:id/ai-conversations"), webshellHandler.ListAIConversations)
		protected.GET("/webshell/connections/:id/state", security.RequireRoutePermission(http.MethodGet, "/webshell/connections/:id/state"), webshellHandler.GetConnectionState)
		protected.PUT("/webshell/connections/:id", security.RequireRoutePermission(http.MethodPut, "/webshell/connections/:id"), webshellHandler.UpdateConnection)
		protected.PUT("/webshell/connections/:id/state", security.RequireRoutePermission(http.MethodPut, "/webshell/connections/:id/state"), webshellHandler.SaveConnectionState)
		protected.DELETE("/webshell/connections/:id", security.RequireRoutePermission(http.MethodDelete, "/webshell/connections/:id"), webshellHandler.DeleteConnection)
		protected.POST("/webshell/exec", security.RequireRoutePermission(http.MethodPost, "/webshell/exec"), webshellHandler.Exec)
		protected.POST("/webshell/file", security.RequireRoutePermission(http.MethodPost, "/webshell/file"), webshellHandler.FileOp)

		// 对话附件（chat_uploads）管理
		protected.GET("/chat-uploads", security.RequireRoutePermission(http.MethodGet, "/chat-uploads"), chatUploadsHandler.List)
		protected.GET("/chat-uploads/download", security.RequireRoutePermission(http.MethodGet, "/chat-uploads/download"), chatUploadsHandler.Download)
		protected.GET("/chat-uploads/content", security.RequireRoutePermission(http.MethodGet, "/chat-uploads/content"), chatUploadsHandler.GetContent)
		protected.POST("/chat-uploads", security.RequireRoutePermission(http.MethodPost, "/chat-uploads"), chatUploadsHandler.Upload)
		protected.POST("/chat-uploads/mkdir", security.RequireRoutePermission(http.MethodPost, "/chat-uploads/mkdir"), chatUploadsHandler.Mkdir)
		protected.DELETE("/chat-uploads", security.RequireRoutePermission(http.MethodDelete, "/chat-uploads"), chatUploadsHandler.Delete)
		protected.PUT("/chat-uploads/rename", security.RequireRoutePermission(http.MethodPut, "/chat-uploads/rename"), chatUploadsHandler.Rename)
		protected.PUT("/chat-uploads/content", security.RequireRoutePermission(http.MethodPut, "/chat-uploads/content"), chatUploadsHandler.PutContent)

		// 角色管理
		protected.GET("/roles", security.RequireRoutePermission(http.MethodGet, "/roles"), roleHandler.GetRoles)
		protected.GET("/roles/:name", security.RequireRoutePermission(http.MethodGet, "/roles/:name"), roleHandler.GetRole)
		protected.GET("/roles/skills/list", security.RequireRoutePermission(http.MethodGet, "/roles/skills/list"), roleHandler.GetSkills)
		protected.POST("/roles", security.RequireRoutePermission(http.MethodPost, "/roles"), roleHandler.CreateRole)
		protected.PUT("/roles/:name", security.RequireRoutePermission(http.MethodPut, "/roles/:name"), roleHandler.UpdateRole)
		protected.DELETE("/roles/:name", security.RequireRoutePermission(http.MethodDelete, "/roles/:name"), roleHandler.DeleteRole)

		// Skills管理
		protected.GET("/skills", security.RequireRoutePermission(http.MethodGet, "/skills"), skillsHandler.GetSkills)
		protected.GET("/skills/stats", security.RequireRoutePermission(http.MethodGet, "/skills/stats"), skillsHandler.GetSkillStats)
		protected.DELETE("/skills/stats", security.RequireRoutePermission(http.MethodDelete, "/skills/stats"), skillsHandler.ClearSkillStats)
		protected.GET("/skills/:name", security.RequireRoutePermission(http.MethodGet, "/skills/:name"), skillsHandler.GetSkill)
		protected.GET("/skills/:name/bound-roles", security.RequireRoutePermission(http.MethodGet, "/skills/:name/bound-roles"), skillsHandler.GetSkillBoundRoles)
		protected.POST("/skills", security.RequireRoutePermission(http.MethodPost, "/skills"), skillsHandler.CreateSkill)
		protected.PUT("/skills/:name", security.RequireRoutePermission(http.MethodPut, "/skills/:name"), skillsHandler.UpdateSkill)
		protected.DELETE("/skills/:name", security.RequireRoutePermission(http.MethodDelete, "/skills/:name"), skillsHandler.DeleteSkill)
		protected.DELETE("/skills/:name/stats", security.RequireRoutePermission(http.MethodDelete, "/skills/:name/stats"), skillsHandler.ClearSkillStatsByName)

		// MCP端点
		protected.POST("/mcp", security.RequireRoutePermission(http.MethodPost, "/mcp"), func(c *gin.Context) {
			mcpServer.HandleHTTP(c.Writer, c.Request)
		})

		// OpenAPI结果聚合端点（可选，用于获取对话的完整结果）
		protected.GET("/conversations/:id/results", security.RequireRoutePermission(http.MethodGet, "/conversations/:id/results"), openAPIHandler.GetConversationResults)
	}

	// OpenAPI规范（需要认证，避免暴露API结构信息）
	protected.GET("/openapi/spec", security.RequireRoutePermission(http.MethodGet, "/openapi/spec"), openAPIHandler.GetOpenAPISpec)

	// API文档页面（公开访问，但需要登录后才能使用API）
	router.GET("/api-docs", func(c *gin.Context) {
		c.HTML(http.StatusOK, "api-docs.html", nil)
	})

	// 静态文件
	router.Static("/static", "./web/static")
	router.LoadHTMLGlob("web/templates/*")

	// 前端页面
	router.GET("/", func(c *gin.Context) {
		version := app.config.Version
		if version == "" {
			version = "v1.0.0"
		}
		c.HTML(http.StatusOK, "index.html", gin.H{"Version": version})
	})
}

// registerVulnerabilityTool 注册漏洞记录工具到MCP服务器
func registerVulnerabilityTool(mcpServer *mcp.Server, db *database.DB, logger *zap.Logger) {
	tool := mcp.Tool{
		Name:             builtin.ToolRecordVulnerability,
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
			Status:         "open",
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

// registerWebshellTools 注册 WebShell 相关 MCP 工具，供 AI 助手在指定连接上执行命令与文件操作
func registerWebshellTools(mcpServer *mcp.Server, db *database.DB, webshellHandler *handler.WebShellHandler, logger *zap.Logger) {
	if db == nil || webshellHandler == nil {
		logger.Warn("跳过 WebShell 工具注册：db 或 webshellHandler 为空")
		return
	}

	// webshell_exec
	execTool := mcp.Tool{
		Name:             builtin.ToolWebshellExec,
		Description:      "在指定的 WebShell 连接上执行一条系统命令，返回命令的标准输出。connection_id 由用户在 AI 助手上下文中选定。",
		ShortDescription: "在 WebShell 连接上执行命令",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"connection_id": map[string]interface{}{
					"type":        "string",
					"description": "WebShell 连接 ID（如 ws_xxx）",
				},
				"command": map[string]interface{}{
					"type":        "string",
					"description": "要执行的系统命令",
				},
			},
			"required": []string{"connection_id", "command"},
		},
	}
	execHandler := func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		cid, _ := args["connection_id"].(string)
		cmd, _ := args["command"].(string)
		if cid == "" || cmd == "" {
			return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: "connection_id 和 command 均为必填"}}, IsError: true}, nil
		}
		conn, err := db.GetWebshellConnection(cid)
		if err != nil || conn == nil {
			return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: "未找到该 WebShell 连接或查询失败"}}, IsError: true}, nil
		}
		output, ok, errMsg := webshellHandler.ExecWithConnection(conn, cmd)
		if errMsg != "" {
			return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: errMsg}}, IsError: true}, nil
		}
		if !ok {
			return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: "HTTP 非 200，输出:\n" + output}}, IsError: false}, nil
		}
		return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: output}}, IsError: false}, nil
	}
	mcpServer.RegisterTool(execTool, execHandler)

	// webshell_file_list
	listTool := mcp.Tool{
		Name:             builtin.ToolWebshellFileList,
		Description:      "在指定 WebShell 连接上列出目录内容。path 默认为当前目录（.）。",
		ShortDescription: "在 WebShell 上列出目录",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"connection_id": map[string]interface{}{"type": "string", "description": "WebShell 连接 ID"},
				"path":          map[string]interface{}{"type": "string", "description": "目录路径，默认 ."},
			},
			"required": []string{"connection_id"},
		},
	}
	listHandler := func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		cid, _ := args["connection_id"].(string)
		path, _ := args["path"].(string)
		if cid == "" {
			return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: "connection_id 必填"}}, IsError: true}, nil
		}
		conn, err := db.GetWebshellConnection(cid)
		if err != nil || conn == nil {
			return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: "未找到该 WebShell 连接"}}, IsError: true}, nil
		}
		output, ok, errMsg := webshellHandler.FileOpWithConnection(conn, "list", path, "", "")
		if errMsg != "" {
			return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: errMsg}}, IsError: true}, nil
		}
		return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: output}}, IsError: !ok}, nil
	}
	mcpServer.RegisterTool(listTool, listHandler)

	// webshell_file_read
	readTool := mcp.Tool{
		Name:             builtin.ToolWebshellFileRead,
		Description:      "在指定 WebShell 连接上读取文件内容。",
		ShortDescription: "在 WebShell 上读取文件",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"connection_id": map[string]interface{}{"type": "string", "description": "WebShell 连接 ID"},
				"path":          map[string]interface{}{"type": "string", "description": "文件路径"},
			},
			"required": []string{"connection_id", "path"},
		},
	}
	readHandler := func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		cid, _ := args["connection_id"].(string)
		path, _ := args["path"].(string)
		if cid == "" || path == "" {
			return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: "connection_id 和 path 必填"}}, IsError: true}, nil
		}
		conn, err := db.GetWebshellConnection(cid)
		if err != nil || conn == nil {
			return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: "未找到该 WebShell 连接"}}, IsError: true}, nil
		}
		output, ok, errMsg := webshellHandler.FileOpWithConnection(conn, "read", path, "", "")
		if errMsg != "" {
			return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: errMsg}}, IsError: true}, nil
		}
		return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: output}}, IsError: !ok}, nil
	}
	mcpServer.RegisterTool(readTool, readHandler)

	// webshell_file_write
	writeTool := mcp.Tool{
		Name:             builtin.ToolWebshellFileWrite,
		Description:      "在指定 WebShell 连接上写入文件内容（会覆盖已有文件）。",
		ShortDescription: "在 WebShell 上写入文件",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"connection_id": map[string]interface{}{"type": "string", "description": "WebShell 连接 ID"},
				"path":          map[string]interface{}{"type": "string", "description": "文件路径"},
				"content":       map[string]interface{}{"type": "string", "description": "要写入的内容"},
			},
			"required": []string{"connection_id", "path", "content"},
		},
	}
	writeHandler := func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		cid, _ := args["connection_id"].(string)
		path, _ := args["path"].(string)
		content, _ := args["content"].(string)
		if cid == "" || path == "" {
			return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: "connection_id 和 path 必填"}}, IsError: true}, nil
		}
		conn, err := db.GetWebshellConnection(cid)
		if err != nil || conn == nil {
			return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: "未找到该 WebShell 连接"}}, IsError: true}, nil
		}
		output, ok, errMsg := webshellHandler.FileOpWithConnection(conn, "write", path, content, "")
		if errMsg != "" {
			return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: errMsg}}, IsError: true}, nil
		}
		if !ok {
			return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: "写入可能失败，输出:\n" + output}}, IsError: false}, nil
		}
		return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: "写入成功\n" + output}}, IsError: false}, nil
	}
	mcpServer.RegisterTool(writeTool, writeHandler)

	logger.Info("WebShell 工具注册成功")
}

// registerWebshellManagementTools 注册 WebShell 连接管理 MCP 工具
func registerWebshellManagementTools(mcpServer *mcp.Server, db *database.DB, webshellHandler *handler.WebShellHandler, logger *zap.Logger) {
	if db == nil {
		logger.Warn("跳过 WebShell 管理工具注册：db 为空")
		return
	}

	// manage_webshell_list - 列出所有 webshell 连接
	listTool := mcp.Tool{
		Name:             builtin.ToolManageWebshellList,
		Description:      "列出所有已保存的 WebShell 连接，返回连接ID、URL、类型、备注等信息。",
		ShortDescription: "列出所有 WebShell 连接",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
	}
	listHandler := func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		connections, err := db.ListWebshellConnections()
		if err != nil {
			return &mcp.ToolResult{
				Content: []mcp.Content{{Type: "text", Text: "获取连接列表失败: " + err.Error()}},
				IsError: true,
			}, nil
		}
		if len(connections) == 0 {
			return &mcp.ToolResult{
				Content: []mcp.Content{{Type: "text", Text: "暂无 WebShell 连接"}},
				IsError: false,
			}, nil
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("找到 %d 个 WebShell 连接：\n\n", len(connections)))
		for _, conn := range connections {
			sb.WriteString(fmt.Sprintf("ID: %s\n", conn.ID))
			sb.WriteString(fmt.Sprintf("  URL: %s\n", conn.URL))
			sb.WriteString(fmt.Sprintf("  类型: %s\n", conn.Type))
			sb.WriteString(fmt.Sprintf("  请求方式: %s\n", conn.Method))
			sb.WriteString(fmt.Sprintf("  命令参数: %s\n", conn.CmdParam))
			if conn.Remark != "" {
				sb.WriteString(fmt.Sprintf("  备注: %s\n", conn.Remark))
			}
			sb.WriteString(fmt.Sprintf("  创建时间: %s\n", conn.CreatedAt.Format("2006-01-02 15:04:05")))
			sb.WriteString("\n")
		}
		return &mcp.ToolResult{
			Content: []mcp.Content{{Type: "text", Text: sb.String()}},
			IsError: false,
		}, nil
	}
	mcpServer.RegisterTool(listTool, listHandler)

	// manage_webshell_add - 添加新的 webshell 连接
	addTool := mcp.Tool{
		Name:             builtin.ToolManageWebshellAdd,
		Description:      "添加新的 WebShell 连接到管理系统。支持 PHP、ASP、ASPX、JSP 等类型的一句话木马。",
		ShortDescription: "添加 WebShell 连接",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"url": map[string]interface{}{
					"type":        "string",
					"description": "Shell 地址，如 http://target.com/shell.php（必填）",
				},
				"password": map[string]interface{}{
					"type":        "string",
					"description": "连接密码/密钥，如冰蝎/蚁剑的连接密码",
				},
				"type": map[string]interface{}{
					"type":        "string",
					"description": "Shell 类型：php、asp、aspx、jsp，默认为 php",
					"enum":        []string{"php", "asp", "aspx", "jsp"},
				},
				"method": map[string]interface{}{
					"type":        "string",
					"description": "请求方式：GET 或 POST，默认为 POST",
					"enum":        []string{"GET", "POST"},
				},
				"cmd_param": map[string]interface{}{
					"type":        "string",
					"description": "命令参数名，不填默认为 cmd",
				},
				"remark": map[string]interface{}{
					"type":        "string",
					"description": "备注，便于识别的备注名",
				},
			},
			"required": []string{"url"},
		},
	}
	addHandler := func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		urlStr, _ := args["url"].(string)
		if urlStr == "" {
			return &mcp.ToolResult{
				Content: []mcp.Content{{Type: "text", Text: "错误: url 参数必填"}},
				IsError: true,
			}, nil
		}

		password, _ := args["password"].(string)
		shellType, _ := args["type"].(string)
		if shellType == "" {
			shellType = "php"
		}
		method, _ := args["method"].(string)
		if method == "" {
			method = "post"
		}
		cmdParam, _ := args["cmd_param"].(string)
		if cmdParam == "" {
			cmdParam = "cmd"
		}
		remark, _ := args["remark"].(string)

		// 生成连接ID
		connID := "ws_" + strings.ReplaceAll(uuid.New().String(), "-", "")[:12]
		conn := &database.WebShellConnection{
			ID:        connID,
			URL:       urlStr,
			Password:  password,
			Type:      strings.ToLower(shellType),
			Method:    strings.ToLower(method),
			CmdParam:  cmdParam,
			Remark:    remark,
			CreatedAt: time.Now(),
		}

		if err := db.CreateWebshellConnection(conn); err != nil {
			return &mcp.ToolResult{
				Content: []mcp.Content{{Type: "text", Text: "添加 WebShell 连接失败: " + err.Error()}},
				IsError: true,
			}, nil
		}

		return &mcp.ToolResult{
			Content: []mcp.Content{{
				Type: "text",
				Text: fmt.Sprintf("WebShell 连接添加成功！\n\n连接ID: %s\nURL: %s\n类型: %s\n请求方式: %s\n命令参数: %s", conn.ID, conn.URL, conn.Type, conn.Method, conn.CmdParam),
			}},
			IsError: false,
		}, nil
	}
	mcpServer.RegisterTool(addTool, addHandler)

	// manage_webshell_update - 更新 webshell 连接
	updateTool := mcp.Tool{
		Name:             builtin.ToolManageWebshellUpdate,
		Description:      "更新已存在的 WebShell 连接信息。",
		ShortDescription: "更新 WebShell 连接",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"connection_id": map[string]interface{}{
					"type":        "string",
					"description": "要更新的 WebShell 连接 ID（必填）",
				},
				"url": map[string]interface{}{
					"type":        "string",
					"description": "新的 Shell 地址",
				},
				"password": map[string]interface{}{
					"type":        "string",
					"description": "新的连接密码/密钥",
				},
				"type": map[string]interface{}{
					"type":        "string",
					"description": "新的 Shell 类型：php、asp、aspx、jsp",
					"enum":        []string{"php", "asp", "aspx", "jsp"},
				},
				"method": map[string]interface{}{
					"type":        "string",
					"description": "新的请求方式：GET 或 POST",
					"enum":        []string{"GET", "POST"},
				},
				"cmd_param": map[string]interface{}{
					"type":        "string",
					"description": "新的命令参数名",
				},
				"remark": map[string]interface{}{
					"type":        "string",
					"description": "新的备注",
				},
			},
			"required": []string{"connection_id"},
		},
	}
	updateHandler := func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		connID, _ := args["connection_id"].(string)
		if connID == "" {
			return &mcp.ToolResult{
				Content: []mcp.Content{{Type: "text", Text: "错误: connection_id 参数必填"}},
				IsError: true,
			}, nil
		}

		// 获取现有连接
		existing, err := db.GetWebshellConnection(connID)
		if err != nil || existing == nil {
			return &mcp.ToolResult{
				Content: []mcp.Content{{Type: "text", Text: "未找到指定的 WebShell 连接: " + connID}},
				IsError: true,
			}, nil
		}

		// 更新字段（如果提供了新值）
		if urlStr, ok := args["url"].(string); ok && urlStr != "" {
			existing.URL = urlStr
		}
		if password, ok := args["password"].(string); ok {
			existing.Password = password
		}
		if shellType, ok := args["type"].(string); ok && shellType != "" {
			existing.Type = strings.ToLower(shellType)
		}
		if method, ok := args["method"].(string); ok && method != "" {
			existing.Method = strings.ToLower(method)
		}
		if cmdParam, ok := args["cmd_param"].(string); ok && cmdParam != "" {
			existing.CmdParam = cmdParam
		}
		if remark, ok := args["remark"].(string); ok {
			existing.Remark = remark
		}

		if err := db.UpdateWebshellConnection(existing); err != nil {
			return &mcp.ToolResult{
				Content: []mcp.Content{{Type: "text", Text: "更新 WebShell 连接失败: " + err.Error()}},
				IsError: true,
			}, nil
		}

		return &mcp.ToolResult{
			Content: []mcp.Content{{
				Type: "text",
				Text: fmt.Sprintf("WebShell 连接更新成功！\n\n连接ID: %s\nURL: %s\n类型: %s\n请求方式: %s\n命令参数: %s\n备注: %s", existing.ID, existing.URL, existing.Type, existing.Method, existing.CmdParam, existing.Remark),
			}},
			IsError: false,
		}, nil
	}
	mcpServer.RegisterTool(updateTool, updateHandler)

	// manage_webshell_delete - 删除 webshell 连接
	deleteTool := mcp.Tool{
		Name:             builtin.ToolManageWebshellDelete,
		Description:      "删除指定的 WebShell 连接。",
		ShortDescription: "删除 WebShell 连接",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"connection_id": map[string]interface{}{
					"type":        "string",
					"description": "要删除的 WebShell 连接 ID（必填）",
				},
			},
			"required": []string{"connection_id"},
		},
	}
	deleteHandler := func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		connID, _ := args["connection_id"].(string)
		if connID == "" {
			return &mcp.ToolResult{
				Content: []mcp.Content{{Type: "text", Text: "错误: connection_id 参数必填"}},
				IsError: true,
			}, nil
		}

		if err := db.DeleteWebshellConnection(connID); err != nil {
			return &mcp.ToolResult{
				Content: []mcp.Content{{Type: "text", Text: "删除 WebShell 连接失败: " + err.Error()}},
				IsError: true,
			}, nil
		}

		return &mcp.ToolResult{
			Content: []mcp.Content{{
				Type: "text",
				Text: fmt.Sprintf("WebShell 连接 %s 已成功删除", connID),
			}},
			IsError: false,
		}, nil
	}
	mcpServer.RegisterTool(deleteTool, deleteHandler)

	// manage_webshell_test - 测试 webshell 连接
	testTool := mcp.Tool{
		Name:             builtin.ToolManageWebshellTest,
		Description:      "测试指定的 WebShell 连接是否可用，会尝试执行一个简单的命令（如 whoami 或 dir）。",
		ShortDescription: "测试 WebShell 连接",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"connection_id": map[string]interface{}{
					"type":        "string",
					"description": "要测试的 WebShell 连接 ID（必填）",
				},
				"command": map[string]interface{}{
					"type":        "string",
					"description": "测试命令，默认为 whoami（Linux）或 dir（Windows）",
				},
			},
			"required": []string{"connection_id"},
		},
	}
	testHandler := func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		connID, _ := args["connection_id"].(string)
		if connID == "" {
			return &mcp.ToolResult{
				Content: []mcp.Content{{Type: "text", Text: "错误: connection_id 参数必填"}},
				IsError: true,
			}, nil
		}

		// 获取连接
		conn, err := db.GetWebshellConnection(connID)
		if err != nil || conn == nil {
			return &mcp.ToolResult{
				Content: []mcp.Content{{Type: "text", Text: "未找到指定的 WebShell 连接: " + connID}},
				IsError: true,
			}, nil
		}

		// 确定测试命令
		testCmd, _ := args["command"].(string)
		if testCmd == "" {
			// 根据 shell 类型选择默认命令
			if conn.Type == "asp" || conn.Type == "aspx" {
				testCmd = "dir"
			} else {
				testCmd = "whoami"
			}
		}

		// 执行测试命令
		output, ok, errMsg := webshellHandler.ExecWithConnection(conn, testCmd)
		if errMsg != "" {
			return &mcp.ToolResult{
				Content: []mcp.Content{{Type: "text", Text: fmt.Sprintf("连接测试失败！\n\n连接ID: %s\nURL: %s\n错误: %s", connID, conn.URL, errMsg)}},
				IsError: true,
			}, nil
		}

		if !ok {
			return &mcp.ToolResult{
				Content: []mcp.Content{{Type: "text", Text: fmt.Sprintf("连接测试失败！HTTP 非 200\n\n连接ID: %s\nURL: %s\n输出: %s", connID, conn.URL, output)}},
				IsError: true,
			}, nil
		}

		return &mcp.ToolResult{
			Content: []mcp.Content{{
				Type: "text",
				Text: fmt.Sprintf("连接测试成功！\n\n连接ID: %s\nURL: %s\n类型: %s\n\n测试命令: %s\n输出结果:\n%s", connID, conn.URL, conn.Type, testCmd, output),
			}},
			IsError: false,
		}, nil
	}
	mcpServer.RegisterTool(testTool, testHandler)

	logger.Info("WebShell 管理工具注册成功")
}

// initializeKnowledge 初始化知识库组件（用于动态初始化）
func initializeKnowledge(
	cfg *config.Config,
	db *database.DB,
	knowledgeDBConn *database.DB,
	mcpServer *mcp.Server,
	agentHandler *handler.AgentHandler,
	app *App, // 传递 App 引用以便更新知识库组件
	logger *zap.Logger,
) (*handler.KnowledgeHandler, error) {
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
		knowledgeDBConn, err = database.NewKnowledgeDB(knowledgeDBPath, logger)
		if err != nil {
			return nil, fmt.Errorf("初始化知识库数据库失败: %w", err)
		}
		knowledgeDB = knowledgeDBConn.DB
		logger.Info("使用独立的知识库数据库", zap.String("path", knowledgeDBPath))
	} else {
		// 向后兼容：使用会话数据库
		knowledgeDB = db.DB
		logger.Info("使用会话数据库存储知识库数据（建议配置knowledge_db_path以分离数据）")
	}

	// 创建知识库管理器
	knowledgeManager := knowledge.NewManager(knowledgeDB, cfg.Knowledge.BasePath, logger)

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
	openAIClient := openai.NewClient(&cfg.OpenAI, httpClient, logger)
	embedder := knowledge.NewEmbedder(&cfg.Knowledge, &cfg.OpenAI, openAIClient, logger)

	// 创建检索器
	retrievalConfig := &knowledge.RetrievalConfig{
		TopK:                cfg.Knowledge.Retrieval.TopK,
		SimilarityThreshold: cfg.Knowledge.Retrieval.SimilarityThreshold,
		HybridWeight:        cfg.Knowledge.Retrieval.HybridWeight,
	}
	knowledgeRetriever := knowledge.NewRetriever(knowledgeDB, embedder, retrievalConfig, logger)

	// 创建索引器
	knowledgeIndexer := knowledge.NewIndexer(knowledgeDB, embedder, logger, &cfg.Knowledge.Indexing)

	// 注册知识检索工具到MCP服务器
	knowledge.RegisterKnowledgeTool(mcpServer, knowledgeRetriever, knowledgeManager, logger)

	// 创建知识库API处理器
	knowledgeHandler := handler.NewKnowledgeHandler(knowledgeManager, knowledgeRetriever, knowledgeIndexer, db, logger)
	logger.Info("知识库模块初始化完成", zap.Bool("handler_created", knowledgeHandler != nil))

	// 设置知识库管理器到AgentHandler以便记录检索日志
	agentHandler.SetKnowledgeManager(knowledgeManager)

	// 更新 App 中的知识库组件（如果 App 不为 nil，说明是动态初始化）
	if app != nil {
		app.knowledgeManager = knowledgeManager
		app.knowledgeRetriever = knowledgeRetriever
		app.knowledgeIndexer = knowledgeIndexer
		app.knowledgeHandler = knowledgeHandler
		// 如果使用独立数据库，更新 knowledgeDB
		if knowledgeDBPath != "" {
			app.knowledgeDB = knowledgeDBConn
		}
		logger.Info("App 中的知识库组件已更新")
	}

	// 扫描知识库并建立索引（异步）
	go func() {
		itemsToIndex, err := knowledgeManager.ScanKnowledgeBase()
		if err != nil {
			logger.Warn("扫描知识库失败", zap.Error(err))
			return
		}

		// 检查是否已有索引
		hasIndex, err := knowledgeIndexer.HasIndex()
		if err != nil {
			logger.Warn("检查索引状态失败", zap.Error(err))
			return
		}

		if hasIndex {
			// 如果已有索引，只索引新添加或更新的项
			if len(itemsToIndex) > 0 {
				logger.Info("检测到已有知识库索引，开始增量索引", zap.Int("count", len(itemsToIndex)))
				ctx := context.Background()
				consecutiveFailures := 0
				var firstFailureItemID string
				var firstFailureError error
				failedCount := 0

				for _, itemID := range itemsToIndex {
					if err := knowledgeIndexer.IndexItem(ctx, itemID); err != nil {
						failedCount++
						consecutiveFailures++

						if consecutiveFailures == 1 {
							firstFailureItemID = itemID
							firstFailureError = err
							logger.Warn("索引知识项失败", zap.String("itemId", itemID), zap.Error(err))
						}

						// 如果连续失败2次，立即停止增量索引
						if consecutiveFailures >= 2 {
							logger.Error("连续索引失败次数过多，立即停止增量索引",
								zap.Int("consecutiveFailures", consecutiveFailures),
								zap.Int("totalItems", len(itemsToIndex)),
								zap.String("firstFailureItemId", firstFailureItemID),
								zap.Error(firstFailureError),
							)
							break
						}
						continue
					}

					// 成功时重置连续失败计数
					if consecutiveFailures > 0 {
						consecutiveFailures = 0
						firstFailureItemID = ""
						firstFailureError = nil
					}
				}
				logger.Info("增量索引完成", zap.Int("totalItems", len(itemsToIndex)), zap.Int("failedCount", failedCount))
			} else {
				logger.Info("检测到已有知识库索引，没有需要索引的新项或更新项")
			}
			return
		}

		// 只有在没有索引时才自动重建
		logger.Info("未检测到知识库索引，开始自动构建索引")
		ctx := context.Background()
		if err := knowledgeIndexer.RebuildIndex(ctx); err != nil {
			logger.Warn("重建知识库索引失败", zap.Error(err))
		}
	}()

	return knowledgeHandler, nil
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
