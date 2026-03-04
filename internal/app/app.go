package app

import (
	"context"
	"database/sql"
	"fmt"
	"html"
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
	"cyberstrike-ai/internal/robot"
	"cyberstrike-ai/internal/logger"
	"cyberstrike-ai/internal/mcp"
	"cyberstrike-ai/internal/mcp/builtin"
	"cyberstrike-ai/internal/openai"
	"cyberstrike-ai/internal/security"
	"cyberstrike-ai/internal/skills"
	"cyberstrike-ai/internal/storage"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// App application
type App struct {
	config             *config.Config
	logger             *logger.Logger
	router             *gin.Engine
	mcpServer          *mcp.Server
	externalMCPMgr     *mcp.ExternalMCPManager
	agent              *agent.Agent
	executor           *security.Executor
	db                 *database.DB
	knowledgeDB        *database.DB // knowledge base database connection (if using a separate database)
	auth               *security.AuthManager
	knowledgeManager   *knowledge.Manager        // knowledge base manager (for dynamic initialization)
	knowledgeRetriever *knowledge.Retriever      // knowledge base retriever (for dynamic initialization)
	knowledgeIndexer   *knowledge.Indexer        // knowledge base indexer (for dynamic initialization)
	knowledgeHandler   *handler.KnowledgeHandler // knowledge base handler (for dynamic initialization)
	memoryHandler      *handler.MemoryHandler    // memory handler (nil when persistent memory is disabled)
	agentHandler       *handler.AgentHandler     // Agent handler (for updating knowledge base manager)
	robotHandler       *handler.RobotHandler     // robot handler (DingTalk/Lark/WeCom/Telegram)
	robotMu            sync.Mutex                 // protects DingTalk/Lark/Telegram long connection cancel
	dingCancel         context.CancelFunc        // DingTalk Stream cancel function, used to restart on config change
	larkCancel         context.CancelFunc        // Lark long connection cancel function, used to restart on config change
	telegramCancel     context.CancelFunc        // Telegram polling cancel function, used to restart on config change
	indexHTML          string                    // cached index.html content
}

// New creates a new application
func New(cfg *config.Config, log *logger.Logger) (*App, error) {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	// CORS middleware
	router.Use(corsMiddleware())

	// authentication manager
	authManager, err := security.NewAuthManager(cfg.Auth.Password, cfg.Auth.SessionDurationHours)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize authentication: %w", err)
	}

	// initialize database
	dbPath := cfg.Database.Path
	if dbPath == "" {
		dbPath = "data/conversations.db"
	}

	// ensure directory exists
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	db, err := database.NewDB(dbPath, log.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// create MCP server (with database persistence)
	mcpServer := mcp.NewServerWithStorage(log.Logger, db)

	// create security tool executor
	executor := security.NewExecutor(&cfg.Security, mcpServer, log.Logger)

	// register tools
	executor.RegisterTools(mcpServer)

	// register vulnerability recording tool
	registerVulnerabilityTool(mcpServer, db, log.Logger)

	if cfg.Auth.GeneratedPassword != "" {
		config.PrintGeneratedPasswordWarning(cfg.Auth.GeneratedPassword, cfg.Auth.GeneratedPasswordPersisted, cfg.Auth.GeneratedPasswordPersistErr)
		cfg.Auth.GeneratedPassword = ""
		cfg.Auth.GeneratedPasswordPersisted = false
		cfg.Auth.GeneratedPasswordPersistErr = ""
	}

	// create external MCP manager (using the same storage as the internal MCP server)
	externalMCPMgr := mcp.NewExternalMCPManagerWithStorage(log.Logger, db)
	if cfg.ExternalMCP.Servers != nil {
		externalMCPMgr.LoadConfigs(&cfg.ExternalMCP)
		// start all enabled external MCP clients
		externalMCPMgr.StartAllEnabled()
	}

	// initialize result storage
	resultStorageDir := "tmp"
	if cfg.Agent.ResultStorageDir != "" {
		resultStorageDir = cfg.Agent.ResultStorageDir
	}

	// ensure storage directory exists
	if err := os.MkdirAll(resultStorageDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create result storage directory: %w", err)
	}

	// create result storage instance
	resultStorage, err := storage.NewFileResultStorage(resultStorageDir, log.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize result storage: %w", err)
	}

	// initialize TimeAwareness before creating the agent variable to avoid
	// package name shadowing.
	taEnabled := cfg.Agent.TimeAwareness.Enabled
	// For backward compatibility: treat a fully zero-value config block as
	// "not explicitly disabled" and default to enabled.
	if !taEnabled && cfg.Agent.TimeAwareness.Timezone == "" {
		taEnabled = true
	}
	timeAwareness := agent.NewTimeAwareness(cfg.Agent.TimeAwareness.Timezone, taEnabled)
	log.Logger.Info("time awareness initialized",
		zap.Bool("enabled", taEnabled),
		zap.String("timezone", cfg.Agent.TimeAwareness.Timezone),
	)

	// initialize PersistentMemory before creating the agent variable
	var persistentMem *agent.PersistentMemory
	memEnabled := cfg.Agent.Memory.Enabled || cfg.Agent.Memory.MaxEntries == 0
	if memEnabled {
		pm, pmErr := agent.NewPersistentMemory(db.DB, log.Logger)
		if pmErr != nil {
			log.Logger.Warn("failed to initialize persistent memory, continuing without it", zap.Error(pmErr))
		} else {
			persistentMem = pm
			log.Logger.Info("persistent memory initialized")
		}
	}

	// create Agent
	maxIterations := cfg.Agent.MaxIterations
	if maxIterations <= 0 {
		maxIterations = 30 // default value
	}
	agentInstance := agent.NewAgent(&cfg.OpenAI, &cfg.Agent, mcpServer, externalMCPMgr, log.Logger, maxIterations)

	// set result storage on Agent
	agentInstance.SetResultStorage(resultStorage)

	// set result storage on Executor (for query tools)
	executor.SetResultStorage(resultStorage)

	// attach time awareness and memory to agent
	agentInstance.SetTimeAwareness(timeAwareness)
	registerTimeTools(mcpServer, timeAwareness, log.Logger)
	var memHandler *handler.MemoryHandler
	if persistentMem != nil {
		agentInstance.SetPersistentMemory(persistentMem)
		registerMemoryTools(mcpServer, persistentMem, log.Logger)
		memHandler = handler.NewMemoryHandler(persistentMem, log.Logger)
	}

	// initialize knowledge base module (if enabled)
	var knowledgeManager *knowledge.Manager
	var knowledgeRetriever *knowledge.Retriever
	var knowledgeIndexer *knowledge.Indexer
	var knowledgeHandler *handler.KnowledgeHandler

	var knowledgeDBConn *database.DB
	log.Logger.Info("checking knowledge base configuration", zap.Bool("enabled", cfg.Knowledge.Enabled))
	if cfg.Knowledge.Enabled {
		// determine knowledge base database path
		knowledgeDBPath := cfg.Database.KnowledgeDBPath
		var knowledgeDB *sql.DB

		if knowledgeDBPath != "" {
			// use a separate knowledge base database
			// ensure directory exists
			if err := os.MkdirAll(filepath.Dir(knowledgeDBPath), 0755); err != nil {
				return nil, fmt.Errorf("failed to create knowledge base database directory: %w", err)
			}

			var err error
			knowledgeDBConn, err = database.NewKnowledgeDB(knowledgeDBPath, log.Logger)
			if err != nil {
				return nil, fmt.Errorf("failed to initialize knowledge base database: %w", err)
			}
			knowledgeDB = knowledgeDBConn.DB
			log.Logger.Info("using separate knowledge base database", zap.String("path", knowledgeDBPath))
		} else {
			// backward compatibility: use the conversation database
			knowledgeDB = db.DB
			log.Logger.Info("using conversation database for knowledge base data (recommended to configure knowledge_db_path to separate data)")
		}

		// create knowledge base manager
		knowledgeManager = knowledge.NewManager(knowledgeDB, cfg.Knowledge.BasePath, log.Logger)

		// create embedder
		// use OpenAI config API Key (if not specified in knowledge base config)
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

		// create retriever
		retrievalConfig := &knowledge.RetrievalConfig{
			TopK:                cfg.Knowledge.Retrieval.TopK,
			SimilarityThreshold: cfg.Knowledge.Retrieval.SimilarityThreshold,
			HybridWeight:        cfg.Knowledge.Retrieval.HybridWeight,
		}
		knowledgeRetriever = knowledge.NewRetriever(knowledgeDB, embedder, retrievalConfig, log.Logger)

		// create indexer
		knowledgeIndexer = knowledge.NewIndexer(knowledgeDB, embedder, log.Logger)

		// register knowledge retrieval tool to MCP server
		knowledge.RegisterKnowledgeTool(mcpServer, knowledgeRetriever, knowledgeManager, log.Logger)

		// create knowledge base API handler
		knowledgeHandler = handler.NewKnowledgeHandler(knowledgeManager, knowledgeRetriever, knowledgeIndexer, db, log.Logger)
		log.Logger.Info("knowledge base module initialization complete", zap.Bool("handler_created", knowledgeHandler != nil))

		// attach proactive RAG context injector to the agent so that relevant
		// knowledge is automatically embedded in the system prompt at the start
		// of every agent loop run.
		ragInjector := agent.NewRAGContextInjector(
			knowledgeRetriever,
			log.Logger,
			agent.RAGContextConfig{}, // use library defaults
		)
		agentInstance.SetRAGInjector(ragInjector)
		log.Logger.Info("RAG context injector attached to agent")

		// scan knowledge base and build index (async)
		go func() {
			itemsToIndex, err := knowledgeManager.ScanKnowledgeBase()
			if err != nil {
				log.Logger.Warn("failed to scan knowledge base", zap.Error(err))
				return
			}

			// check if index already exists
			hasIndex, err := knowledgeIndexer.HasIndex()
			if err != nil {
				log.Logger.Warn("failed to check index status", zap.Error(err))
				return
			}

			if hasIndex {
				// if index exists, only index newly added or updated items
				if len(itemsToIndex) > 0 {
					log.Logger.Info("existing knowledge base index detected, starting incremental indexing", zap.Int("count", len(itemsToIndex)))
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
								log.Logger.Warn("failed to index knowledge item", zap.String("itemId", itemID), zap.Error(err))
							}

							// if 2 consecutive failures, immediately stop incremental indexing
							if consecutiveFailures >= 2 {
								log.Logger.Error("too many consecutive index failures, stopping incremental indexing immediately",
									zap.Int("consecutiveFailures", consecutiveFailures),
									zap.Int("totalItems", len(itemsToIndex)),
									zap.String("firstFailureItemId", firstFailureItemID),
									zap.Error(firstFailureError),
								)
								break
							}
							continue
						}

						// reset consecutive failure count on success
						if consecutiveFailures > 0 {
							consecutiveFailures = 0
							firstFailureItemID = ""
							firstFailureError = nil
						}
					}
					log.Logger.Info("incremental indexing complete", zap.Int("totalItems", len(itemsToIndex)), zap.Int("failedCount", failedCount))
				} else {
					log.Logger.Info("existing knowledge base index detected, no new or updated items to index")
				}
				return
			}

			// only auto-rebuild when no index exists
			log.Logger.Info("no knowledge base index detected, starting automatic index build")
			ctx := context.Background()
			if err := knowledgeIndexer.RebuildIndex(ctx); err != nil {
				log.Logger.Warn("failed to rebuild knowledge base index", zap.Error(err))
			}
		}()
	}

	// get config file path
	configPath := "config.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	// initialize Skills manager
	skillsDir := cfg.SkillsDir
	if skillsDir == "" {
		skillsDir = "skills" // default directory
	}
	// if relative path, relative to the config file directory
	configDir := filepath.Dir(configPath)
	if !filepath.IsAbs(skillsDir) {
		skillsDir = filepath.Join(configDir, skillsDir)
	}
	skillsManager := skills.NewManager(skillsDir, log.Logger)
	log.Logger.Info("Skills manager initialized", zap.String("skillsDir", skillsDir))

	// register Skills tool to MCP server (allowing AI to call on demand, with database storage for statistics)
	// create an adapter to adapt database.DB to the SkillStatsStorage interface
	var skillStatsStorage skills.SkillStatsStorage
	if db != nil {
		skillStatsStorage = &skillStatsDBAdapter{db: db}
	}
	skills.RegisterSkillsToolWithStorage(mcpServer, skillsManager, skillStatsStorage, log.Logger)

	// create handlers
	agentHandler := handler.NewAgentHandler(agentInstance, db, cfg, log.Logger)
	agentHandler.SetSkillsManager(skillsManager) // set Skills manager
	// if knowledge base is enabled, set knowledge base manager on AgentHandler for retrieval log recording
	if knowledgeManager != nil {
		agentHandler.SetKnowledgeManager(knowledgeManager)
	}
	monitorHandler := handler.NewMonitorHandler(mcpServer, executor, db, log.Logger)
	monitorHandler.SetExternalMCPManager(externalMCPMgr) // set external MCP manager to get external MCP execution records
	groupHandler := handler.NewGroupHandler(db, log.Logger)
	authHandler := handler.NewAuthHandler(authManager, cfg, configPath, log.Logger)
	attackChainHandler := handler.NewAttackChainHandler(db, &cfg.OpenAI, log.Logger)
	vulnerabilityHandler := handler.NewVulnerabilityHandler(db, log.Logger)
	configHandler := handler.NewConfigHandler(configPath, cfg, mcpServer, executor, agentInstance, attackChainHandler, externalMCPMgr, log.Logger)
	externalMCPHandler := handler.NewExternalMCPHandler(externalMCPMgr, cfg, configPath, log.Logger)
	roleHandler := handler.NewRoleHandler(cfg, configPath, log.Logger)
	roleHandler.SetSkillsManager(skillsManager) // set Skills manager on RoleHandler
	skillsHandler := handler.NewSkillsHandler(skillsManager, cfg, configPath, log.Logger)
	fofaHandler := handler.NewFofaHandler(cfg, log.Logger)
	terminalHandler := handler.NewTerminalHandler(log.Logger)
	if db != nil {
		skillsHandler.SetDB(db) // set database connection for fetching call statistics
	}

	// create OpenAPI handler
	conversationHandler := handler.NewConversationHandler(db, log.Logger)
	robotHandler := handler.NewRobotHandler(cfg, db, agentHandler, log.Logger)
	openAPIHandler := handler.NewOpenAPIHandler(db, log.Logger, resultStorage, conversationHandler, agentHandler)

	// create App instance (some fields filled in later)
	app := &App{
		config:             cfg,
		logger:             log,
		router:             router,
		mcpServer:          mcpServer,
		externalMCPMgr:     externalMCPMgr,
		agent:              agentInstance,
		executor:           executor,
		db:                 db,
		knowledgeDB:        knowledgeDBConn,
		auth:               authManager,
		knowledgeManager:   knowledgeManager,
		knowledgeRetriever: knowledgeRetriever,
		knowledgeIndexer:   knowledgeIndexer,
		knowledgeHandler:   knowledgeHandler,
		memoryHandler:      memHandler,
		agentHandler:       agentHandler,
		robotHandler:       robotHandler,
	}
	// cache index.html at startup to avoid per-request disk reads
	indexHTMLBytes, err := os.ReadFile("web/templates/index.html")
	if err != nil {
		log.Logger.Fatal("failed to load index.html", zap.Error(err))
	}
	app.indexHTML = string(indexHTMLBytes)

	// Lark/DingTalk long connections (no public network needed), start in background when enabled; will be restarted via RestartRobotConnections when frontend applies config
	app.startRobotConnections()

	// set vulnerability tool registrar (built-in tool, must be set)
	vulnerabilityRegistrar := func() error {
		registerVulnerabilityTool(mcpServer, db, log.Logger)
		return nil
	}
	configHandler.SetVulnerabilityToolRegistrar(vulnerabilityRegistrar)

	// set Skills tool registrar (built-in tool, must be set)
	skillsRegistrar := func() error {
		// create an adapter to adapt database.DB to the SkillStatsStorage interface
		var skillStatsStorage skills.SkillStatsStorage
		if db != nil {
			skillStatsStorage = &skillStatsDBAdapter{db: db}
		}
		skills.RegisterSkillsToolWithStorage(mcpServer, skillsManager, skillStatsStorage, log.Logger)
		return nil
	}
	configHandler.SetSkillsToolRegistrar(skillsRegistrar)

	// set knowledge base initializer (for dynamic initialization, must be set after App is created)
	configHandler.SetKnowledgeInitializer(func() (*handler.KnowledgeHandler, error) {
		knowledgeHandler, err := initializeKnowledge(cfg, db, knowledgeDBConn, mcpServer, agentHandler, app, log.Logger)
		if err != nil {
			return nil, err
		}

		// after dynamic initialization, set knowledge base tool registrar and retriever updater
		// so that subsequent ApplyConfig calls can re-register tools
		if app.knowledgeRetriever != nil && app.knowledgeManager != nil {
			// create closure, capturing references to knowledgeRetriever and knowledgeManager
			registrar := func() error {
				knowledge.RegisterKnowledgeTool(mcpServer, app.knowledgeRetriever, app.knowledgeManager, log.Logger)
				return nil
			}
			configHandler.SetKnowledgeToolRegistrar(registrar)
			// set retriever updater so ApplyConfig can update retriever config
			configHandler.SetRetrieverUpdater(app.knowledgeRetriever)
			log.Logger.Info("knowledge base tool registrar and retriever updater set after dynamic initialization")

			// attach RAG context injector to the agent when knowledge is dynamically enabled
			ragInjector := agent.NewRAGContextInjector(
				app.knowledgeRetriever,
				log.Logger,
				agent.RAGContextConfig{},
			)
			agentInstance.SetRAGInjector(ragInjector)
			log.Logger.Info("RAG context injector attached to agent (dynamic init)")
		}

		return knowledgeHandler, nil
	})

	// if knowledge base is enabled, set knowledge base tool registrar and retriever updater
	if cfg.Knowledge.Enabled && knowledgeRetriever != nil && knowledgeManager != nil {
		// create closure, capturing references to knowledgeRetriever and knowledgeManager
		registrar := func() error {
			knowledge.RegisterKnowledgeTool(mcpServer, knowledgeRetriever, knowledgeManager, log.Logger)
			return nil
		}
		configHandler.SetKnowledgeToolRegistrar(registrar)
		// set retriever updater so ApplyConfig can update retriever config
		configHandler.SetRetrieverUpdater(knowledgeRetriever)
	}

	// set robot connection restarter, so new DingTalk/Lark config takes effect without restarting the service
	configHandler.SetRobotRestarter(app)

	// set up routes (using App instance for dynamic handler access)
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
		app, // pass App instance for dynamic knowledgeHandler access
		vulnerabilityHandler,
		roleHandler,
		skillsHandler,
		fofaHandler,
		terminalHandler,
		mcpServer,
		authManager,
		openAPIHandler,
	)

	return app, nil

}

// Run starts the application
func (a *App) Run() error {
	// start MCP server (if enabled)
	if a.config.MCP.Enabled {
		go func() {
			mcpAddr := fmt.Sprintf("%s:%d", a.config.MCP.Host, a.config.MCP.Port)
			a.logger.Info("starting MCP server", zap.String("address", mcpAddr))

			mux := http.NewServeMux()
			mux.HandleFunc("/mcp", a.mcpServer.HandleHTTP)

			if err := http.ListenAndServe(mcpAddr, mux); err != nil {
				a.logger.Error("MCP server failed to start", zap.Error(err))
			}
		}()
	}

	// start main server
	addr := fmt.Sprintf("%s:%d", a.config.Server.Host, a.config.Server.Port)
	a.logger.Info("starting HTTP server", zap.String("address", addr))

	return a.router.Run(addr)
}

// Shutdown shuts down the application
func (a *App) Shutdown() {
	// stop DingTalk/Lark/Telegram long connections
	a.robotMu.Lock()
	if a.dingCancel != nil {
		a.dingCancel()
		a.dingCancel = nil
	}
	if a.larkCancel != nil {
		a.larkCancel()
		a.larkCancel = nil
	}
	if a.telegramCancel != nil {
		a.telegramCancel()
		a.telegramCancel = nil
	}
	a.robotMu.Unlock()

	// stop all external MCP clients
	if a.externalMCPMgr != nil {
		a.externalMCPMgr.StopAll()
	}

	// close knowledge base database connection (if using separate database)
	if a.knowledgeDB != nil {
		if err := a.knowledgeDB.Close(); err != nil {
			a.logger.Logger.Warn("failed to close knowledge base database connection", zap.Error(err))
		}
	}
}

// startRobotConnections starts DingTalk/Lark/Telegram long connections based on current config (does not close existing connections, for initial startup only)
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
	if cfg.Robots.Telegram.Enabled && cfg.Robots.Telegram.BotToken != "" {
		ctx, cancel := context.WithCancel(context.Background())
		a.telegramCancel = cancel
		go robot.StartTelegram(ctx, cfg.Robots.Telegram, a.robotHandler, a.logger.Logger)
	}
}

// RestartRobotConnections restarts DingTalk/Lark/Telegram long connections so frontend config changes take effect immediately (implements handler.RobotRestarter)
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
	if a.telegramCancel != nil {
		a.telegramCancel()
		a.telegramCancel = nil
	}
	a.robotMu.Unlock()
	// give old goroutines a moment to exit
	time.Sleep(200 * time.Millisecond)
	a.startRobotConnections()
}

// setupRoutes sets up routes
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
	app *App, // pass App instance for dynamic knowledgeHandler access
	vulnerabilityHandler *handler.VulnerabilityHandler,
	roleHandler *handler.RoleHandler,
	skillsHandler *handler.SkillsHandler,
	fofaHandler *handler.FofaHandler,
	terminalHandler *handler.TerminalHandler,
	mcpServer *mcp.Server,
	authManager *security.AuthManager,
	openAPIHandler *handler.OpenAPIHandler,
) {
	// API routes
	api := router.Group("/api")

	// authentication routes
	authRoutes := api.Group("/auth")
	{
		authRoutes.POST("/login", authHandler.Login)
		authRoutes.POST("/logout", security.AuthMiddleware(authManager), authHandler.Logout)
		authRoutes.POST("/change-password", security.AuthMiddleware(authManager), authHandler.ChangePassword)
		authRoutes.GET("/validate", security.AuthMiddleware(authManager), authHandler.Validate)
	}

	// robot callbacks (no login required, called by WeCom/DingTalk/Lark servers)
	api.GET("/robot/wecom", robotHandler.HandleWecomGET)
	api.POST("/robot/wecom", robotHandler.HandleWecomPOST)
	api.POST("/robot/dingtalk", robotHandler.HandleDingtalkPOST)
	api.POST("/robot/lark", robotHandler.HandleLarkPOST)

	protected := api.Group("")
	protected.Use(security.AuthMiddleware(authManager))
	{
		// robot test (login required): POST /api/robot/test, body: {"platform":"dingtalk","user_id":"test","text":"help"}, used to verify robot logic
		protected.POST("/robot/test", robotHandler.HandleRobotTest)

		// Agent Loop
		protected.POST("/agent-loop", agentHandler.AgentLoop)
		// Agent Loop streaming output
		protected.POST("/agent-loop/stream", agentHandler.AgentLoopStream)
		// Agent Loop cancel and task list
		protected.POST("/agent-loop/cancel", agentHandler.CancelAgentLoop)
		protected.GET("/agent-loop/tasks", agentHandler.ListAgentTasks)
		protected.GET("/agent-loop/tasks/completed", agentHandler.ListCompletedTasks)

		// information gathering - FOFA query (backend proxy)
		protected.POST("/fofa/search", fofaHandler.Search)
		// information gathering - parse natural language to FOFA syntax (requires manual confirmation before querying)
		protected.POST("/fofa/parse", fofaHandler.ParseNaturalLanguage)

		// batch task management
		protected.POST("/batch-tasks", agentHandler.CreateBatchQueue)
		protected.GET("/batch-tasks", agentHandler.ListBatchQueues)
		protected.GET("/batch-tasks/:queueId", agentHandler.GetBatchQueue)
		protected.POST("/batch-tasks/:queueId/start", agentHandler.StartBatchQueue)
		protected.POST("/batch-tasks/:queueId/pause", agentHandler.PauseBatchQueue)
		protected.DELETE("/batch-tasks/:queueId", agentHandler.DeleteBatchQueue)
		protected.PUT("/batch-tasks/:queueId/tasks/:taskId", agentHandler.UpdateBatchTask)
		protected.POST("/batch-tasks/:queueId/tasks", agentHandler.AddBatchTask)
		protected.DELETE("/batch-tasks/:queueId/tasks/:taskId", agentHandler.DeleteBatchTask)

		// conversation history
		protected.POST("/conversations", conversationHandler.CreateConversation)
		protected.GET("/conversations", conversationHandler.ListConversations)
		protected.GET("/conversations/:id", conversationHandler.GetConversation)
		protected.PUT("/conversations/:id", conversationHandler.UpdateConversation)
		protected.DELETE("/conversations/:id", conversationHandler.DeleteConversation)
		protected.PUT("/conversations/:id/pinned", groupHandler.UpdateConversationPinned)

		// conversation groups
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

		// monitoring
		protected.GET("/monitor", monitorHandler.Monitor)
		protected.GET("/monitor/execution/:id", monitorHandler.GetExecution)
		protected.DELETE("/monitor/execution/:id", monitorHandler.DeleteExecution)
		protected.DELETE("/monitor/executions", monitorHandler.DeleteExecutions)
		protected.GET("/monitor/stats", monitorHandler.GetStats)

		// configuration management
		protected.GET("/config", configHandler.GetConfig)
		protected.GET("/config/tools", configHandler.GetTools)
		protected.PUT("/config", configHandler.UpdateConfig)
		protected.POST("/config/apply", configHandler.ApplyConfig)

		// system settings - terminal (execute commands to improve operations efficiency)
		protected.POST("/terminal/run", terminalHandler.RunCommand)
		protected.POST("/terminal/run/stream", terminalHandler.RunCommandStream)
		protected.GET("/terminal/ws", terminalHandler.RunCommandWS)

		// external MCP management
		protected.GET("/external-mcp", externalMCPHandler.GetExternalMCPs)
		protected.GET("/external-mcp/stats", externalMCPHandler.GetExternalMCPStats)
		protected.GET("/external-mcp/:name", externalMCPHandler.GetExternalMCP)
		protected.PUT("/external-mcp/:name", externalMCPHandler.AddOrUpdateExternalMCP)
		protected.DELETE("/external-mcp/:name", externalMCPHandler.DeleteExternalMCP)
		protected.POST("/external-mcp/:name/start", externalMCPHandler.StartExternalMCP)
		protected.POST("/external-mcp/:name/stop", externalMCPHandler.StopExternalMCP)

		// attack chain visualization
		protected.GET("/attack-chain/:conversationId", attackChainHandler.GetAttackChain)
		protected.POST("/attack-chain/:conversationId/regenerate", attackChainHandler.RegenerateAttackChain)

		// knowledge base management (always register routes, dynamically get handler via App instance)
		knowledgeRoutes := protected.Group("/knowledge")
		{
			knowledgeRoutes.GET("/categories", func(c *gin.Context) {
				if app.knowledgeHandler == nil {
					c.JSON(http.StatusOK, gin.H{
						"categories": []string{},
						"enabled":    false,
						"message":    "Knowledge base feature is not enabled. Please go to system settings to enable knowledge retrieval.",
					})
					return
				}
				app.knowledgeHandler.GetCategories(c)
			})
			knowledgeRoutes.GET("/items", func(c *gin.Context) {
				if app.knowledgeHandler == nil {
					c.JSON(http.StatusOK, gin.H{
						"items":   []interface{}{},
						"enabled": false,
						"message": "Knowledge base feature is not enabled. Please go to system settings to enable knowledge retrieval.",
					})
					return
				}
				app.knowledgeHandler.GetItems(c)
			})
			knowledgeRoutes.GET("/items/:id", func(c *gin.Context) {
				if app.knowledgeHandler == nil {
					c.JSON(http.StatusOK, gin.H{
						"enabled": false,
						"message": "Knowledge base feature is not enabled. Please go to system settings to enable knowledge retrieval.",
					})
					return
				}
				app.knowledgeHandler.GetItem(c)
			})
			knowledgeRoutes.POST("/items", func(c *gin.Context) {
				if app.knowledgeHandler == nil {
					c.JSON(http.StatusOK, gin.H{
						"enabled": false,
						"error":   "Knowledge base feature is not enabled. Please go to system settings to enable knowledge retrieval.",
					})
					return
				}
				app.knowledgeHandler.CreateItem(c)
			})
			knowledgeRoutes.PUT("/items/:id", func(c *gin.Context) {
				if app.knowledgeHandler == nil {
					c.JSON(http.StatusOK, gin.H{
						"enabled": false,
						"error":   "Knowledge base feature is not enabled. Please go to system settings to enable knowledge retrieval.",
					})
					return
				}
				app.knowledgeHandler.UpdateItem(c)
			})
			knowledgeRoutes.DELETE("/items/:id", func(c *gin.Context) {
				if app.knowledgeHandler == nil {
					c.JSON(http.StatusOK, gin.H{
						"enabled": false,
						"error":   "Knowledge base feature is not enabled. Please go to system settings to enable knowledge retrieval.",
					})
					return
				}
				app.knowledgeHandler.DeleteItem(c)
			})
			knowledgeRoutes.GET("/index-status", func(c *gin.Context) {
				if app.knowledgeHandler == nil {
					c.JSON(http.StatusOK, gin.H{
						"enabled":          false,
						"total_items":      0,
						"indexed_items":    0,
						"progress_percent": 0,
						"is_complete":      false,
						"message":          "Knowledge base feature is not enabled. Please go to system settings to enable knowledge retrieval.",
					})
					return
				}
				app.knowledgeHandler.GetIndexStatus(c)
			})
			knowledgeRoutes.POST("/index", func(c *gin.Context) {
				if app.knowledgeHandler == nil {
					c.JSON(http.StatusOK, gin.H{
						"enabled": false,
						"error":   "Knowledge base feature is not enabled. Please go to system settings to enable knowledge retrieval.",
					})
					return
				}
				app.knowledgeHandler.RebuildIndex(c)
			})
			knowledgeRoutes.POST("/scan", func(c *gin.Context) {
				if app.knowledgeHandler == nil {
					c.JSON(http.StatusOK, gin.H{
						"enabled": false,
						"error":   "Knowledge base feature is not enabled. Please go to system settings to enable knowledge retrieval.",
					})
					return
				}
				app.knowledgeHandler.ScanKnowledgeBase(c)
			})
			knowledgeRoutes.GET("/retrieval-logs", func(c *gin.Context) {
				if app.knowledgeHandler == nil {
					c.JSON(http.StatusOK, gin.H{
						"logs":    []interface{}{},
						"enabled": false,
						"message": "Knowledge base feature is not enabled. Please go to system settings to enable knowledge retrieval.",
					})
					return
				}
				app.knowledgeHandler.GetRetrievalLogs(c)
			})
			knowledgeRoutes.DELETE("/retrieval-logs/:id", func(c *gin.Context) {
				if app.knowledgeHandler == nil {
					c.JSON(http.StatusOK, gin.H{
						"enabled": false,
						"error":   "Knowledge base feature is not enabled. Please go to system settings to enable knowledge retrieval.",
					})
					return
				}
				app.knowledgeHandler.DeleteRetrievalLog(c)
			})
			knowledgeRoutes.POST("/search", func(c *gin.Context) {
				if app.knowledgeHandler == nil {
					c.JSON(http.StatusOK, gin.H{
						"results": []interface{}{},
						"enabled": false,
						"message": "Knowledge base feature is not enabled. Please go to system settings to enable knowledge retrieval.",
					})
					return
				}
				app.knowledgeHandler.Search(c)
			})
			knowledgeRoutes.GET("/stats", func(c *gin.Context) {
				if app.knowledgeHandler == nil {
					c.JSON(http.StatusOK, gin.H{
						"enabled":          false,
						"total_categories": 0,
						"total_items":      0,
						"message":          "Knowledge base feature is not enabled. Please go to system settings to enable knowledge retrieval.",
					})
					return
				}
				app.knowledgeHandler.GetStats(c)
			})
		}

		// persistent memory management
		memoryRoutes := protected.Group("/memories")
		{
			memoryRoutes.GET("", func(c *gin.Context) {
				if app.memoryHandler == nil {
					c.JSON(http.StatusOK, gin.H{
						"entries": []interface{}{},
						"total":   0,
						"enabled": false,
						"message": "Persistent memory is not enabled. Set agent.memory.enabled: true in config.yaml.",
					})
					return
				}
				app.memoryHandler.ListMemories(c)
			})
			memoryRoutes.GET("/stats", func(c *gin.Context) {
				if app.memoryHandler == nil {
					c.JSON(http.StatusOK, gin.H{
						"total":   0,
						"enabled": false,
						"message": "Persistent memory is not enabled.",
					})
					return
				}
				app.memoryHandler.GetMemoryStats(c)
			})
			memoryRoutes.POST("", func(c *gin.Context) {
				if app.memoryHandler == nil {
					c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Persistent memory is not enabled."})
					return
				}
				app.memoryHandler.CreateMemory(c)
			})
			memoryRoutes.PUT("/:id", func(c *gin.Context) {
				if app.memoryHandler == nil {
					c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Persistent memory is not enabled."})
					return
				}
				app.memoryHandler.UpdateMemory(c)
			})
			memoryRoutes.DELETE("", func(c *gin.Context) {
				if app.memoryHandler == nil {
					c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Persistent memory is not enabled."})
					return
				}
				app.memoryHandler.DeleteAllMemories(c)
			})
			memoryRoutes.DELETE("/:id", func(c *gin.Context) {
				if app.memoryHandler == nil {
					c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Persistent memory is not enabled."})
					return
				}
				app.memoryHandler.DeleteMemory(c)
			})
		}

		// vulnerability management
		protected.GET("/vulnerabilities", vulnerabilityHandler.ListVulnerabilities)
		protected.GET("/vulnerabilities/stats", vulnerabilityHandler.GetVulnerabilityStats)
		protected.GET("/vulnerabilities/:id", vulnerabilityHandler.GetVulnerability)
		protected.POST("/vulnerabilities", vulnerabilityHandler.CreateVulnerability)
		protected.PUT("/vulnerabilities/:id", vulnerabilityHandler.UpdateVulnerability)
		protected.DELETE("/vulnerabilities/:id", vulnerabilityHandler.DeleteVulnerability)

		// role management
		protected.GET("/roles", roleHandler.GetRoles)
		protected.GET("/roles/:name", roleHandler.GetRole)
		protected.GET("/roles/skills/list", roleHandler.GetSkills)
		protected.POST("/roles", roleHandler.CreateRole)
		protected.PUT("/roles/:name", roleHandler.UpdateRole)
		protected.DELETE("/roles/:name", roleHandler.DeleteRole)

		// Skills management
		protected.GET("/skills", skillsHandler.GetSkills)
		protected.GET("/skills/stats", skillsHandler.GetSkillStats)
		protected.DELETE("/skills/stats", skillsHandler.ClearSkillStats)
		protected.GET("/skills/:name", skillsHandler.GetSkill)
		protected.GET("/skills/:name/bound-roles", skillsHandler.GetSkillBoundRoles)
		protected.POST("/skills", skillsHandler.CreateSkill)
		protected.PUT("/skills/:name", skillsHandler.UpdateSkill)
		protected.DELETE("/skills/:name", skillsHandler.DeleteSkill)
		protected.DELETE("/skills/:name/stats", skillsHandler.ClearSkillStatsByName)

		// MCP endpoint
		protected.POST("/mcp", func(c *gin.Context) {
			mcpServer.HandleHTTP(c.Writer, c.Request)
		})

		// OpenAPI result aggregation endpoint (optional, for fetching complete conversation results)
		protected.GET("/conversations/:id/results", openAPIHandler.GetConversationResults)
	}

	// OpenAPI spec (requires authentication to avoid exposing API structure)
	protected.GET("/openapi/spec", openAPIHandler.GetOpenAPISpec)

	// API documentation page (publicly accessible, but login required to use the API)
	router.GET("/api-docs", func(c *gin.Context) {
		c.HTML(http.StatusOK, "api-docs.html", nil)
	})

	// static files
	router.Static("/static", "./web/static")
	router.LoadHTMLGlob("web/templates/*")

	// frontend page
	router.GET("/", func(c *gin.Context) {
		version := app.config.Version
		if version == "" {
			version = "v1.0.0"
		}
		safeVersion := html.EscapeString(version)
		body := strings.Replace(app.indexHTML, "{{.Version}}", safeVersion, 1)
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(body))
	})
}

// registerVulnerabilityTool registers the vulnerability recording tool to the MCP server
func registerVulnerabilityTool(mcpServer *mcp.Server, db *database.DB, logger *zap.Logger) {
	tool := mcp.Tool{
		Name:             builtin.ToolRecordVulnerability,
		Description:      "Record details of discovered vulnerabilities to the vulnerability management system. When a valid vulnerability is found, use this tool to record vulnerability information including title, description, severity, type, target, proof, impact, and recommendations.",
		ShortDescription: "Record details of discovered vulnerabilities to the vulnerability management system",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"title": map[string]interface{}{
					"type":        "string",
					"description": "Vulnerability title (required)",
				},
				"description": map[string]interface{}{
					"type":        "string",
					"description": "Detailed vulnerability description",
				},
				"severity": map[string]interface{}{
					"type":        "string",
					"description": "Vulnerability severity: critical, high, medium, low, info",
					"enum":        []string{"critical", "high", "medium", "low", "info"},
				},
				"vulnerability_type": map[string]interface{}{
					"type":        "string",
					"description": "Vulnerability type, e.g.: SQL Injection, XSS, CSRF, Command Injection, etc.",
				},
				"target": map[string]interface{}{
					"type":        "string",
					"description": "Affected target (URL, IP address, service, etc.)",
				},
				"proof": map[string]interface{}{
					"type":        "string",
					"description": "Vulnerability proof (POC, screenshots, request/response, etc.)",
				},
				"impact": map[string]interface{}{
					"type":        "string",
					"description": "Vulnerability impact description",
				},
				"recommendation": map[string]interface{}{
					"type":        "string",
					"description": "Remediation recommendations",
				},
			},
			"required": []string{"title", "severity"},
		},
	}

	handler := func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		// get conversation_id from args (automatically added by Agent)
		conversationID, _ := args["conversation_id"].(string)
		if conversationID == "" {
			return &mcp.ToolResult{
				Content: []mcp.Content{
					{
						Type: "text",
						Text: "Error: conversation_id is not set. This is a system error, please retry.",
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
						Text: "Error: title parameter is required and cannot be empty",
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
						Text: "Error: severity parameter is required and cannot be empty",
					},
				},
				IsError: true,
			}, nil
		}

		// validate severity
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
						Text: fmt.Sprintf("Error: severity must be one of critical, high, medium, low, or info. Current value: %s", severity),
					},
				},
				IsError: true,
			}, nil
		}

		// get optional parameters
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

		// create vulnerability record
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
			logger.Error("failed to record vulnerability", zap.Error(err))
			return &mcp.ToolResult{
				Content: []mcp.Content{
					{
						Type: "text",
						Text: fmt.Sprintf("Failed to record vulnerability: %v", err),
					},
				},
				IsError: true,
			}, nil
		}

		logger.Info("vulnerability recorded successfully",
			zap.String("id", created.ID),
			zap.String("title", created.Title),
			zap.String("severity", created.Severity),
			zap.String("conversation_id", conversationID),
		)

		return &mcp.ToolResult{
			Content: []mcp.Content{
				{
					Type: "text",
					Text: fmt.Sprintf("Vulnerability recorded successfully!\n\nVulnerability ID: %s\nTitle: %s\nSeverity: %s\nStatus: %s\n\nYou can view and manage this vulnerability on the vulnerability management page.", created.ID, created.Title, created.Severity, created.Status),
				},
			},
			IsError: false,
		}, nil
	}

	mcpServer.RegisterTool(tool, handler)
	logger.Info("vulnerability recording tool registered successfully")
}

// initializeKnowledge initializes knowledge base components (for dynamic initialization)
func initializeKnowledge(
	cfg *config.Config,
	db *database.DB,
	knowledgeDBConn *database.DB,
	mcpServer *mcp.Server,
	agentHandler *handler.AgentHandler,
	app *App, // pass App reference to update knowledge base components
	logger *zap.Logger,
) (*handler.KnowledgeHandler, error) {
	// determine knowledge base database path
	knowledgeDBPath := cfg.Database.KnowledgeDBPath
	var knowledgeDB *sql.DB

	if knowledgeDBPath != "" {
		// use a separate knowledge base database
		// ensure directory exists
		if err := os.MkdirAll(filepath.Dir(knowledgeDBPath), 0755); err != nil {
			return nil, fmt.Errorf("failed to create knowledge base database directory: %w", err)
		}

		var err error
		knowledgeDBConn, err = database.NewKnowledgeDB(knowledgeDBPath, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize knowledge base database: %w", err)
		}
		knowledgeDB = knowledgeDBConn.DB
		logger.Info("using separate knowledge base database", zap.String("path", knowledgeDBPath))
	} else {
		// backward compatibility: use the conversation database
		knowledgeDB = db.DB
		logger.Info("using conversation database for knowledge base data (recommended to configure knowledge_db_path to separate data)")
	}

	// create knowledge base manager
	knowledgeManager := knowledge.NewManager(knowledgeDB, cfg.Knowledge.BasePath, logger)

	// create embedder
	// use OpenAI config API Key (if not specified in knowledge base config)
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

	// create retriever
	retrievalConfig := &knowledge.RetrievalConfig{
		TopK:                cfg.Knowledge.Retrieval.TopK,
		SimilarityThreshold: cfg.Knowledge.Retrieval.SimilarityThreshold,
		HybridWeight:        cfg.Knowledge.Retrieval.HybridWeight,
	}
	knowledgeRetriever := knowledge.NewRetriever(knowledgeDB, embedder, retrievalConfig, logger)

	// create indexer
	knowledgeIndexer := knowledge.NewIndexer(knowledgeDB, embedder, logger)

	// register knowledge retrieval tool to MCP server
	knowledge.RegisterKnowledgeTool(mcpServer, knowledgeRetriever, knowledgeManager, logger)

	// create knowledge base API handler
	knowledgeHandler := handler.NewKnowledgeHandler(knowledgeManager, knowledgeRetriever, knowledgeIndexer, db, logger)
	logger.Info("knowledge base module initialization complete", zap.Bool("handler_created", knowledgeHandler != nil))

	// set knowledge base manager on AgentHandler for retrieval log recording
	agentHandler.SetKnowledgeManager(knowledgeManager)

	// update knowledge base components in App (if App is not nil, this is a dynamic initialization)
	if app != nil {
		app.knowledgeManager = knowledgeManager
		app.knowledgeRetriever = knowledgeRetriever
		app.knowledgeIndexer = knowledgeIndexer
		app.knowledgeHandler = knowledgeHandler
		// if using separate database, update knowledgeDB
		if knowledgeDBPath != "" {
			app.knowledgeDB = knowledgeDBConn
		}
		logger.Info("knowledge base components in App have been updated")
	}

	// scan knowledge base and build index (async)
	go func() {
		itemsToIndex, err := knowledgeManager.ScanKnowledgeBase()
		if err != nil {
			logger.Warn("failed to scan knowledge base", zap.Error(err))
			return
		}

		// check if index already exists
		hasIndex, err := knowledgeIndexer.HasIndex()
		if err != nil {
			logger.Warn("failed to check index status", zap.Error(err))
			return
		}

		if hasIndex {
			// if index exists, only index newly added or updated items
			if len(itemsToIndex) > 0 {
				logger.Info("existing knowledge base index detected, starting incremental indexing", zap.Int("count", len(itemsToIndex)))
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
							logger.Warn("failed to index knowledge item", zap.String("itemId", itemID), zap.Error(err))
						}

						// if 2 consecutive failures, immediately stop incremental indexing
						if consecutiveFailures >= 2 {
							logger.Error("too many consecutive index failures, stopping incremental indexing immediately",
								zap.Int("consecutiveFailures", consecutiveFailures),
								zap.Int("totalItems", len(itemsToIndex)),
								zap.String("firstFailureItemId", firstFailureItemID),
								zap.Error(firstFailureError),
							)
							break
						}
						continue
					}

					// reset consecutive failure count on success
					if consecutiveFailures > 0 {
						consecutiveFailures = 0
						firstFailureItemID = ""
						firstFailureError = nil
					}
				}
				logger.Info("incremental indexing complete", zap.Int("totalItems", len(itemsToIndex)), zap.Int("failedCount", failedCount))
			} else {
				logger.Info("existing knowledge base index detected, no new or updated items to index")
			}
			return
		}

		// only auto-rebuild when no index exists
		logger.Info("no knowledge base index detected, starting automatic index build")
		ctx := context.Background()
		if err := knowledgeIndexer.RebuildIndex(ctx); err != nil {
			logger.Warn("failed to rebuild knowledge base index", zap.Error(err))
		}
	}()

	return knowledgeHandler, nil
}

// registerTimeTools registers the get_current_time tool on the MCP server.
func registerTimeTools(mcpServer *mcp.Server, ta *agent.TimeAwareness, logger *zap.Logger) {
	tool := mcp.Tool{
		Name:             builtin.ToolGetCurrentTime,
		Description:      "Get the current date and time, including timezone, Unix timestamp, and session uptime. Use this tool whenever you need to know the exact current time or when building time-relative plans (e.g. scheduling scans, calculating elapsed time).",
		ShortDescription: "Get current date, time, timezone, and session uptime",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
	}
	handler := func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		return &mcp.ToolResult{
			Content: []mcp.Content{{Type: "text", Text: ta.FormatCurrentTime()}},
		}, nil
	}
	mcpServer.RegisterTool(tool, handler)
	logger.Info("time tool registered successfully")
}

// registerMemoryTools registers the four persistent-memory tools on the MCP server.
func registerMemoryTools(mcpServer *mcp.Server, pm *agent.PersistentMemory, logger *zap.Logger) {
	// ── store_memory ──────────────────────────────────────────────────────────
	storeMemTool := mcp.Tool{
		Name:             builtin.ToolStoreMemory,
		Description:      "Persist an important fact to long-term memory so it is available across conversation compressions and future sessions. Use this for credentials, targets, key findings, and operational notes that must not be forgotten.",
		ShortDescription: "Store a key fact to persistent long-term memory",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"key": map[string]interface{}{
					"type":        "string",
					"description": "A short, unique label for the fact (e.g. 'admin_password', 'target_ip')",
				},
				"value": map[string]interface{}{
					"type":        "string",
					"description": "The fact or value to remember",
				},
				"category": map[string]interface{}{
					"type":        "string",
					"description": "Memory category: credential, target, vulnerability, fact, note",
					"enum":        []string{"credential", "target", "vulnerability", "fact", "note"},
				},
			},
			"required": []string{"key", "value"},
		},
	}
	mcpServer.RegisterTool(storeMemTool, func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		key, _ := args["key"].(string)
		value, _ := args["value"].(string)
		cat := agent.MemoryCategory(args["category"].(string) + "")
		if cat == "" {
			cat = agent.MemoryCategoryFact
		}
		convID, _ := args["conversation_id"].(string)
		entry, err := pm.Store(key, value, cat, convID)
		if err != nil {
			return &mcp.ToolResult{
				Content: []mcp.Content{{Type: "text", Text: "Error storing memory: " + err.Error()}},
				IsError: true,
			}, nil
		}
		return &mcp.ToolResult{
			Content: []mcp.Content{{Type: "text", Text: fmt.Sprintf("Memory stored: [%s] %s = %s (id: %s)", entry.Category, entry.Key, entry.Value, entry.ID)}},
		}, nil
	})

	// ── retrieve_memory ───────────────────────────────────────────────────────
	retrieveMemTool := mcp.Tool{
		Name:             builtin.ToolRetrieveMemory,
		Description:      "Search persistent memory for facts matching a query. Returns matching entries ordered by recency. Use this to recall credentials, targets, or findings from previous sessions.",
		ShortDescription: "Search persistent memory for matching facts",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Search query matched against memory keys and values",
				},
				"category": map[string]interface{}{
					"type":        "string",
					"description": "Filter by category: credential, target, vulnerability, fact, note (optional)",
					"enum":        []string{"credential", "target", "vulnerability", "fact", "note"},
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of results to return (default 20)",
				},
			},
			"required": []string{},
		},
	}
	mcpServer.RegisterTool(retrieveMemTool, func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		query, _ := args["query"].(string)
		cat := agent.MemoryCategory("")
		if cv, ok := args["category"].(string); ok {
			cat = agent.MemoryCategory(cv)
		}
		limit := 20
		if lv, ok := args["limit"].(float64); ok {
			limit = int(lv)
		}
		entries, err := pm.Retrieve(query, cat, limit)
		if err != nil {
			return &mcp.ToolResult{
				Content: []mcp.Content{{Type: "text", Text: "Error retrieving memory: " + err.Error()}},
				IsError: true,
			}, nil
		}
		if len(entries) == 0 {
			return &mcp.ToolResult{
				Content: []mcp.Content{{Type: "text", Text: "No matching memories found."}},
			}, nil
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Found %d memory entries:\n", len(entries)))
		for _, e := range entries {
			sb.WriteString(fmt.Sprintf("  [%s] %s: %s  (id: %s, updated: %s)\n",
				e.Category, e.Key, e.Value, e.ID, e.UpdatedAt.Format("2006-01-02 15:04")))
		}
		return &mcp.ToolResult{
			Content: []mcp.Content{{Type: "text", Text: sb.String()}},
		}, nil
	})

	// ── list_memories ─────────────────────────────────────────────────────────
	listMemTool := mcp.Tool{
		Name:             builtin.ToolListMemories,
		Description:      "List all entries currently stored in persistent memory, optionally filtered by category. Useful for reviewing what has been remembered.",
		ShortDescription: "List all persistent memory entries",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"category": map[string]interface{}{
					"type":        "string",
					"description": "Filter by category: credential, target, vulnerability, fact, note (optional, empty = all)",
					"enum":        []string{"credential", "target", "vulnerability", "fact", "note"},
				},
			},
			"required": []string{},
		},
	}
	mcpServer.RegisterTool(listMemTool, func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		cat := agent.MemoryCategory("")
		if cv, ok := args["category"].(string); ok {
			cat = agent.MemoryCategory(cv)
		}
		entries, err := pm.List(cat, 100)
		if err != nil {
			return &mcp.ToolResult{
				Content: []mcp.Content{{Type: "text", Text: "Error listing memories: " + err.Error()}},
				IsError: true,
			}, nil
		}
		if len(entries) == 0 {
			return &mcp.ToolResult{
				Content: []mcp.Content{{Type: "text", Text: "No memories stored yet."}},
			}, nil
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Persistent memory (%d entries):\n", len(entries)))
		for _, e := range entries {
			sb.WriteString(fmt.Sprintf("  [%s] %s: %s  (id: %s)\n", e.Category, e.Key, e.Value, e.ID))
		}
		return &mcp.ToolResult{
			Content: []mcp.Content{{Type: "text", Text: sb.String()}},
		}, nil
	})

	// ── delete_memory ─────────────────────────────────────────────────────────
	deleteMemTool := mcp.Tool{
		Name:             builtin.ToolDeleteMemory,
		Description:      "Delete a specific memory entry by ID. Use this to remove stale, incorrect, or no-longer-relevant facts.",
		ShortDescription: "Delete a persistent memory entry by ID",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id": map[string]interface{}{
					"type":        "string",
					"description": "The UUID of the memory entry to delete (obtain from list_memories or retrieve_memory)",
				},
			},
			"required": []string{"id"},
		},
	}
	mcpServer.RegisterTool(deleteMemTool, func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		id, _ := args["id"].(string)
		if id == "" {
			return &mcp.ToolResult{
				Content: []mcp.Content{{Type: "text", Text: "Error: id parameter is required"}},
				IsError: true,
			}, nil
		}
		if err := pm.Delete(id); err != nil {
			return &mcp.ToolResult{
				Content: []mcp.Content{{Type: "text", Text: "Error deleting memory: " + err.Error()}},
				IsError: true,
			}, nil
		}
		return &mcp.ToolResult{
			Content: []mcp.Content{{Type: "text", Text: fmt.Sprintf("Memory entry %s deleted.", id)}},
		}, nil
	})

	logger.Info("persistent memory tools registered successfully")
}

// corsMiddleware CORS middleware
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
