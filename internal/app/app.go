package app

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"html"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"cyberstrike-ai/internal/agent"
	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/database"
	"cyberstrike-ai/internal/filemanager"
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
	memoryHandler      *handler.MemoryHandler        // memory handler (nil when persistent memory is disabled)
	fileManagerHandler *handler.FileManagerHandler   // file manager handler
	agentHandler       *handler.AgentHandler         // Agent handler (for updating knowledge base manager)
	robotHandler       *handler.RobotHandler     // robot handler (Lark/WeCom/Telegram)
	robotMu            sync.Mutex                // protects Lark/Telegram long connection cancel
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

	// initialize file manager
	fmEnabled := cfg.Agent.FileManager.Enabled
	// Default to enabled when config block is zero-value (backward compat)
	if !fmEnabled && cfg.Agent.FileManager.StorageDir == "" {
		fmEnabled = true
	}
	var fileMgr *filemanager.Manager
	if fmEnabled {
		fileStorageDir := cfg.Agent.FileManager.StorageDir
		if fileStorageDir == "" {
			fileStorageDir = "managed_files"
		}
		var fmErr error
		fileMgr, fmErr = filemanager.NewManager(db.DB, fileStorageDir, log.Logger)
		if fmErr != nil {
			log.Logger.Warn("failed to initialize file manager, continuing without it", zap.Error(fmErr))
		} else {
			registerFileManagerTools(mcpServer, fileMgr, log.Logger)
			log.Logger.Info("file manager initialized", zap.String("storage_dir", fileStorageDir))
		}
	}

	// ── Cuttlefish (Android VM) tools ──────────────────────────────────────
	cvdHome := os.Getenv("CVD_HOME")
	if cvdHome == "" {
		cvdHome = filepath.Join(os.Getenv("HOME"), "cuttlefish-workspace")
	}
	registerCuttlefishTools(mcpServer, cvdHome, log.Logger)

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

		// create embedder (no implicit fallback to OpenAI endpoint for embeddings)
		httpClient := &http.Client{
			Timeout: 30 * time.Minute,
		}
		openAIClient := openai.NewClient(&cfg.OpenAI, httpClient, log.Logger)
		embedder := knowledge.NewEmbedder(&cfg.Knowledge, openAIClient, log.Logger)
		embeddingEnabled := embedder.Enabled()

		// create retriever
		retrievalConfig := &knowledge.RetrievalConfig{
			TopK:                cfg.Knowledge.Retrieval.TopK,
			SimilarityThreshold: cfg.Knowledge.Retrieval.SimilarityThreshold,
			HybridWeight:        cfg.Knowledge.Retrieval.HybridWeight,
		}
		knowledgeRetriever = knowledge.NewRetriever(knowledgeDB, embedder, retrievalConfig, log.Logger)

		// create indexer
		knowledgeIndexer = knowledge.NewIndexer(knowledgeDB, embedder, log.Logger, cfg.Knowledge.Embedding.MaxTokens)

		// register knowledge retrieval tool to MCP server
		knowledge.RegisterKnowledgeTool(mcpServer, knowledgeRetriever, knowledgeManager, log.Logger)

		// create knowledge base API handler
		knowledgeHandler = handler.NewKnowledgeHandler(knowledgeManager, knowledgeRetriever, knowledgeIndexer, db, log.Logger)
		log.Logger.Info("knowledge base module initialization complete", zap.Bool("handler_created", knowledgeHandler != nil))

		if embeddingEnabled {
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
		} else {
			log.Logger.Warn("knowledge embedding disabled: missing embedding base_url/model/api_key; skipping RAG injector and index build")
		}

		// scan knowledge base and build index (async)
		if embeddingEnabled {
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
	}

	// Resolve the effective config path from CLI args; required for settings/auth persistence.
	configPath := resolveConfigPathFromArgs(os.Args, "config.yaml")

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
	// set file manager on AgentHandler for auto-registering chat uploads
	if fileMgr != nil {
		agentHandler.SetFileManager(fileMgr)
	}
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
	dockerHandler := handler.NewDockerHandler(filepath.Dir(configPath), log.Logger)
	if db != nil {
		skillsHandler.SetDB(db) // set database connection for fetching call statistics
	}

	// create file manager handler
	var fileManagerHandler *handler.FileManagerHandler
	if fileMgr != nil {
		fileManagerHandler = handler.NewFileManagerHandler(fileMgr, log.Logger)
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
		fileManagerHandler: fileManagerHandler,
		agentHandler:       agentHandler,
		robotHandler:       robotHandler,
	}
	// cache index.html at startup to avoid per-request disk reads
	indexHTMLBytes, err := os.ReadFile("web/templates/index.html")
	if err != nil {
		log.Logger.Fatal("failed to load index.html", zap.Error(err))
	}
	app.indexHTML = string(indexHTMLBytes)

	// Lark long connections (no public network needed), start in background when enabled; will be restarted via RestartRobotConnections when frontend applies config
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

	// set memory tool registrar (so memory tools survive config re-apply)
	if persistentMem != nil {
		memoryRegistrar := func() error {
			registerMemoryTools(mcpServer, persistentMem, log.Logger)
			return nil
		}
		configHandler.SetMemoryToolRegistrar(memoryRegistrar)
	}

	// set time tool registrar (so time tools survive config re-apply)
	timeRegistrar := func() error {
		registerTimeTools(mcpServer, timeAwareness, log.Logger)
		return nil
	}
	configHandler.SetTimeToolRegistrar(timeRegistrar)

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

	// set robot connection restarter, so new Lark config takes effect without restarting the service
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
		dockerHandler,
		mcpServer,
		authManager,
		openAPIHandler,
	)

	return app, nil

}

// resolveConfigPathFromArgs extracts the config file path from common flag patterns:
// --config /path/to/file, --config=/path/to/file, -config /path/to/file, -config=/path/to/file.
func resolveConfigPathFromArgs(args []string, defaultPath string) string {
	if len(args) == 0 {
		return defaultPath
	}
	for i := 0; i < len(args); i++ {
		arg := strings.TrimSpace(args[i])
		if arg == "" {
			continue
		}
		switch arg {
		case "--config", "-config", "-c":
			if i+1 < len(args) {
				next := strings.TrimSpace(args[i+1])
				if next != "" && !strings.HasPrefix(next, "-") {
					return next
				}
			}
		}
		if strings.HasPrefix(arg, "--config=") {
			v := strings.TrimSpace(strings.TrimPrefix(arg, "--config="))
			if v != "" {
				return v
			}
		}
		if strings.HasPrefix(arg, "-config=") {
			v := strings.TrimSpace(strings.TrimPrefix(arg, "-config="))
			if v != "" {
				return v
			}
		}
	}
	return defaultPath
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
			// Backward-compatibility alias for legacy SSE endpoint configs.
			mux.HandleFunc("/mcp/sse", a.mcpServer.HandleHTTP)

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
	// stop Lark/Telegram long connections
	a.robotMu.Lock()
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

// startRobotConnections starts Lark/Telegram long connections based on current config (does not close existing connections, for initial startup only)
func (a *App) startRobotConnections() {
	a.robotMu.Lock()
	defer a.robotMu.Unlock()
	cfg := a.config
	if cfg.Robots.Lark.Enabled && cfg.Robots.Lark.AppID != "" && cfg.Robots.Lark.AppSecret != "" {
		ctx, cancel := context.WithCancel(context.Background())
		a.larkCancel = cancel
		go robot.StartLark(ctx, cfg.Robots.Lark, a.robotHandler, a.logger.Logger)
	}
	if cfg.Robots.Telegram.Enabled && cfg.Robots.Telegram.BotToken != "" {
		ctx, cancel := context.WithCancel(context.Background())
		a.telegramCancel = cancel
		go robot.StartTelegram(ctx, cfg.Robots.Telegram, a.robotHandler, a.logger.Logger)
	}
}

// RestartRobotConnections restarts Lark/Telegram long connections so frontend config changes take effect immediately (implements handler.RobotRestarter)
func (a *App) RestartRobotConnections() {
	a.robotMu.Lock()
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
	dockerHandler *handler.DockerHandler,
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

	// robot callbacks (no login required, called by WeCom/Lark servers)
	api.GET("/robot/wecom", robotHandler.HandleWecomGET)
	api.POST("/robot/wecom", robotHandler.HandleWecomPOST)
	api.POST("/robot/lark", robotHandler.HandleLarkPOST)

	protected := api.Group("")
	protected.Use(security.AuthMiddleware(authManager))
	{
		// robot test (login required): POST /api/robot/test, body: {"platform":"lark","user_id":"test","text":"help"}, used to verify robot logic
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
		protected.POST("/config/models", configHandler.DiscoverModels)
		protected.PUT("/config", configHandler.UpdateConfig)
		protected.POST("/config/apply", configHandler.ApplyConfig)

		// system settings - terminal (execute commands to improve operations efficiency)
		protected.POST("/terminal/run", terminalHandler.RunCommand)
		protected.POST("/terminal/run/stream", terminalHandler.RunCommandStream)
		protected.GET("/terminal/ws", terminalHandler.RunCommandWS)
		protected.GET("/docker/status", dockerHandler.GetStatus)
		protected.GET("/docker/logs", dockerHandler.GetLogs)
		protected.POST("/docker/action", dockerHandler.RunAction)

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
			memoryRoutes.PATCH("/:id/status", func(c *gin.Context) {
				if app.memoryHandler == nil {
					c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Persistent memory is not enabled."})
					return
				}
				app.memoryHandler.UpdateMemoryStatus(c)
			})
		}

		// file manager
		fileRoutes := protected.Group("/files")
		{
			fileRoutes.GET("", func(c *gin.Context) {
				if app.fileManagerHandler == nil {
					c.JSON(http.StatusOK, gin.H{"files": []interface{}{}, "total": 0})
					return
				}
				app.fileManagerHandler.ListFiles(c)
			})
			fileRoutes.GET("/stats", func(c *gin.Context) {
				if app.fileManagerHandler == nil {
					c.JSON(http.StatusOK, gin.H{"total": 0, "total_size": 0, "by_type": map[string]int{}, "by_status": map[string]int{}})
					return
				}
				app.fileManagerHandler.GetFileStats(c)
			})
			fileRoutes.GET("/:id", func(c *gin.Context) {
				if app.fileManagerHandler == nil {
					c.JSON(http.StatusNotFound, gin.H{"error": "file manager not available"})
					return
				}
				app.fileManagerHandler.GetFile(c)
			})
			fileRoutes.GET("/:id/content", func(c *gin.Context) {
				if app.fileManagerHandler == nil {
					c.JSON(http.StatusNotFound, gin.H{"error": "file manager not available"})
					return
				}
				app.fileManagerHandler.ReadFileContent(c)
			})
			fileRoutes.POST("/upload", func(c *gin.Context) {
				if app.fileManagerHandler == nil {
					c.JSON(http.StatusServiceUnavailable, gin.H{"error": "file manager not available"})
					return
				}
				app.fileManagerHandler.UploadFile(c)
			})
			fileRoutes.POST("/register", func(c *gin.Context) {
				if app.fileManagerHandler == nil {
					c.JSON(http.StatusServiceUnavailable, gin.H{"error": "file manager not available"})
					return
				}
				app.fileManagerHandler.RegisterFile(c)
			})
			fileRoutes.PUT("/:id", func(c *gin.Context) {
				if app.fileManagerHandler == nil {
					c.JSON(http.StatusServiceUnavailable, gin.H{"error": "file manager not available"})
					return
				}
				app.fileManagerHandler.UpdateFile(c)
			})
			fileRoutes.POST("/:id/log", func(c *gin.Context) {
				if app.fileManagerHandler == nil {
					c.JSON(http.StatusServiceUnavailable, gin.H{"error": "file manager not available"})
					return
				}
				app.fileManagerHandler.AppendLog(c)
			})
			fileRoutes.POST("/:id/findings", func(c *gin.Context) {
				if app.fileManagerHandler == nil {
					c.JSON(http.StatusServiceUnavailable, gin.H{"error": "file manager not available"})
					return
				}
				app.fileManagerHandler.AppendFindings(c)
			})
			fileRoutes.DELETE("/:id", func(c *gin.Context) {
				if app.fileManagerHandler == nil {
					c.JSON(http.StatusServiceUnavailable, gin.H{"error": "file manager not available"})
					return
				}
				app.fileManagerHandler.DeleteFile(c)
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
					"description": "Vulnerability title (optional; auto-generated from description when omitted)",
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
			"required": []string{"severity"},
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

		title, _ := args["title"].(string)
		title = strings.TrimSpace(title)

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

		if title == "" {
			title = autoGenerateVulnerabilityTitle(description, vulnType, target, severity)
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

func autoGenerateVulnerabilityTitle(description, vulnType, target, severity string) string {
	clean := func(s string) string {
		return strings.TrimSpace(strings.Join(strings.Fields(s), " "))
	}
	clip := func(s string, n int) string {
		if n <= 0 {
			return ""
		}
		r := []rune(s)
		if len(r) <= n {
			return s
		}
		return string(r[:n]) + "..."
	}

	description = clean(description)
	vulnType = clean(vulnType)
	target = clean(target)
	severity = strings.ToUpper(clean(severity))

	if description != "" {
		return clip(description, 96)
	}

	parts := make([]string, 0, 3)
	if severity != "" {
		parts = append(parts, severity)
	}
	if vulnType != "" {
		parts = append(parts, vulnType)
	} else {
		parts = append(parts, "Vulnerability")
	}

	title := strings.Join(parts, " ")
	if target != "" {
		title += " on " + target
	}
	return clip(title, 96)
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

	// create embedder (no implicit fallback to OpenAI endpoint for embeddings)
	httpClient := &http.Client{
		Timeout: 30 * time.Minute,
	}
	openAIClient := openai.NewClient(&cfg.OpenAI, httpClient, logger)
	embedder := knowledge.NewEmbedder(&cfg.Knowledge, openAIClient, logger)
	embeddingEnabled := embedder.Enabled()

	// create retriever
	retrievalConfig := &knowledge.RetrievalConfig{
		TopK:                cfg.Knowledge.Retrieval.TopK,
		SimilarityThreshold: cfg.Knowledge.Retrieval.SimilarityThreshold,
		HybridWeight:        cfg.Knowledge.Retrieval.HybridWeight,
	}
	knowledgeRetriever := knowledge.NewRetriever(knowledgeDB, embedder, retrievalConfig, logger)

	// create indexer
	knowledgeIndexer := knowledge.NewIndexer(knowledgeDB, embedder, logger, cfg.Knowledge.Embedding.MaxTokens)

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
	if embeddingEnabled {
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
	} else {
		logger.Warn("knowledge embedding disabled: missing embedding base_url/model/api_key; skipping index build")
	}

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

// allMemoryCategories is the complete list of memory category values used in tool schemas.
var allMemoryCategories = []string{
	"credential", "target", "vulnerability", "fact", "note",
	"tool_run", "discovery", "plan",
}

var allowedMemoryCategories = map[agent.MemoryCategory]struct{}{
	agent.MemoryCategoryCredential:    {},
	agent.MemoryCategoryTarget:        {},
	agent.MemoryCategoryVulnerability: {},
	agent.MemoryCategoryFact:          {},
	agent.MemoryCategoryNote:          {},
	agent.MemoryCategoryToolRun:       {},
	agent.MemoryCategoryDiscovery:     {},
	agent.MemoryCategoryPlan:          {},
}

var allowedMemoryConfidence = map[agent.MemoryConfidence]struct{}{
	agent.MemoryConfidenceHigh:   {},
	agent.MemoryConfidenceMedium: {},
	agent.MemoryConfidenceLow:    {},
}

func normalizeMemoryCategory(raw string) (agent.MemoryCategory, bool) {
	cat := agent.MemoryCategory(strings.TrimSpace(raw))
	if cat == "" {
		return agent.MemoryCategoryFact, true
	}
	_, ok := allowedMemoryCategories[cat]
	return cat, ok
}

func normalizeMemoryConfidence(raw string) (agent.MemoryConfidence, bool) {
	conf := agent.MemoryConfidence(strings.TrimSpace(raw))
	if conf == "" {
		return agent.MemoryConfidenceMedium, true
	}
	_, ok := allowedMemoryConfidence[conf]
	return conf, ok
}

// registerMemoryTools registers all persistent-memory tools on the MCP server.
func registerMemoryTools(mcpServer *mcp.Server, pm *agent.PersistentMemory, logger *zap.Logger) {
	// ── store_memory ──────────────────────────────────────────────────────────
	storeMemTool := mcp.Tool{
		Name: builtin.ToolStoreMemory,
		Description: `Persist an important fact to long-term memory so it is available across conversation compressions and future sessions.

Categories:
  credential  - Discovered passwords, tokens, API keys, SSH keys
  target      - IP addresses, hostnames, domains, open ports, services
  vulnerability - Confirmed or suspected vulnerabilities (use record_vulnerability for formal tracking)
  fact        - General observations, version numbers, technology stack details
  note        - Operational notes, testing strategy, reminders
  tool_run    - Record of a completed tool execution (prevents duplicate scans)
  discovery   - New findings that need further investigation or classification
  plan        - Testing plan with steps; prefix completed steps with [DONE]

Optional fields:
  entity      - The target this memory belongs to (e.g. "192.168.1.1", "api.example.com")
  confidence  - high | medium | low (default: medium)

Always set entity when the memory is specific to a particular target or host.`,
		ShortDescription: "Store a key fact to persistent long-term memory",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"key": map[string]interface{}{
					"type":        "string",
					"description": "A short, unique label for the fact (e.g. 'admin_password', 'target_ip', 'nmap_scan_192.168.1.1')",
				},
				"value": map[string]interface{}{
					"type":        "string",
					"description": "The fact or value to remember",
				},
				"category": map[string]interface{}{
					"type":        "string",
					"description": "Memory category: credential, target, vulnerability, fact, note, tool_run, discovery, plan",
					"enum":        allMemoryCategories,
				},
				"entity": map[string]interface{}{
					"type":        "string",
					"description": "The target entity this memory belongs to (IP, hostname, URL). Enables entity-based grouping and lookup.",
				},
				"confidence": map[string]interface{}{
					"type":        "string",
					"description": "Confidence level: high, medium, low (default: medium)",
					"enum":        []string{"high", "medium", "low"},
				},
			},
			"required": []string{"key", "value"},
		},
	}
	mcpServer.RegisterTool(storeMemTool, func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		key, _ := args["key"].(string)
		value, _ := args["value"].(string)
		if key == "" || value == "" {
			return &mcp.ToolResult{
				Content: []mcp.Content{{Type: "text", Text: "Error: key and value are required"}},
				IsError: true,
			}, nil
		}
		categoryRaw, _ := args["category"].(string)
		cat, ok := normalizeMemoryCategory(categoryRaw)
		if !ok {
			return &mcp.ToolResult{
				Content: []mcp.Content{{Type: "text", Text: "Error: invalid category. Must be one of: credential, target, vulnerability, fact, note, tool_run, discovery, plan"}},
				IsError: true,
			}, nil
		}
		convID, _ := args["conversation_id"].(string)
		entity, _ := args["entity"].(string)
		confidenceRaw, _ := args["confidence"].(string)
		confidence, ok := normalizeMemoryConfidence(confidenceRaw)
		if !ok {
			return &mcp.ToolResult{
				Content: []mcp.Content{{Type: "text", Text: "Error: invalid confidence. Must be: high, medium, or low"}},
				IsError: true,
			}, nil
		}
		entry, err := pm.StoreFull(key, value, cat, convID, entity, confidence, agent.MemoryStatusActive)
		if err != nil {
			return &mcp.ToolResult{
				Content: []mcp.Content{{Type: "text", Text: "Error storing memory: " + err.Error()}},
				IsError: true,
			}, nil
		}
		entityInfo := ""
		if entry.Entity != "" {
			entityInfo = fmt.Sprintf(" entity=%s", entry.Entity)
		}
		return &mcp.ToolResult{
			Content: []mcp.Content{{Type: "text", Text: fmt.Sprintf("Memory stored: [%s] %s = %s%s (id: %s)", entry.Category, entry.Key, entry.Value, entityInfo, entry.ID)}},
		}, nil
	})

	// ── retrieve_memory ───────────────────────────────────────────────────────
	retrieveMemTool := mcp.Tool{
		Name:             builtin.ToolRetrieveMemory,
		Description:      "Search persistent memory for facts matching a query. Returns active entries ordered by recency (disproven/false-positive entries are hidden unless include_dismissed=true). Use this to recall credentials, targets, or findings from previous sessions.",
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
					"description": "Filter by category (optional). Options: credential, target, vulnerability, fact, note, tool_run, discovery, plan",
					"enum":        allMemoryCategories,
				},
				"entity": map[string]interface{}{
					"type":        "string",
					"description": "Filter to a specific entity (IP, hostname, URL) to see all memories about that target",
				},
				"include_dismissed": map[string]interface{}{
					"type":        "boolean",
					"description": "If true, also return false_positive and disproven entries (default: false)",
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
		entity, _ := args["entity"].(string)
		includeDismissed, _ := args["include_dismissed"].(bool)
		limit := 20
		if lv, ok := args["limit"].(float64); ok {
			limit = int(lv)
		}

		var entries []*agent.MemoryEntry
		var err error

		// If entity is specified, apply entity filter while respecting
		// category/include_dismissed semantics (consistent with HTTP API).
		if entity != "" {
			entity = strings.TrimSpace(entity)
			queryLower := strings.ToLower(strings.TrimSpace(query))
			if includeDismissed {
				entries, err = pm.ListAll(cat, 5000)
			} else {
				entries, err = pm.List(cat, 5000)
			}
			if err == nil {
				filtered := make([]*agent.MemoryEntry, 0, len(entries))
				for _, entry := range entries {
					if !strings.EqualFold(strings.TrimSpace(entry.Entity), entity) {
						continue
					}
					if queryLower != "" {
						keyLower := strings.ToLower(entry.Key)
						valueLower := strings.ToLower(entry.Value)
						if !strings.Contains(keyLower, queryLower) && !strings.Contains(valueLower, queryLower) {
							continue
						}
					}
					filtered = append(filtered, entry)
					if len(filtered) >= limit {
						break
					}
				}
				entries = filtered
			}
		} else if includeDismissed {
			entries, err = pm.RetrieveAll(query, cat, limit)
		} else {
			entries, err = pm.Retrieve(query, cat, limit)
		}

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
			statusTag := ""
			if e.Status != agent.MemoryStatusActive {
				statusTag = fmt.Sprintf(" [%s]", string(e.Status))
			}
			entityTag := ""
			if e.Entity != "" {
				entityTag = fmt.Sprintf(" entity=%s", e.Entity)
			}
			sb.WriteString(fmt.Sprintf("  [%s]%s %s: %s%s  (id: %s, updated: %s)\n",
				e.Category, statusTag, e.Key, e.Value, entityTag, e.ID, e.UpdatedAt.Format("2006-01-02 15:04")))
		}
		return &mcp.ToolResult{
			Content: []mcp.Content{{Type: "text", Text: sb.String()}},
		}, nil
	})

	// ── list_memories ─────────────────────────────────────────────────────────
	listMemTool := mcp.Tool{
		Name:             builtin.ToolListMemories,
		Description:      "List all active entries in persistent memory, optionally filtered by category or entity. Disproven and false-positive entries are excluded by default.",
		ShortDescription: "List all persistent memory entries",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"category": map[string]interface{}{
					"type":        "string",
					"description": "Filter by category (optional, empty = all). Options: credential, target, vulnerability, fact, note, tool_run, discovery, plan",
					"enum":        allMemoryCategories,
				},
				"entity": map[string]interface{}{
					"type":        "string",
					"description": "Filter to a specific entity (IP, hostname, URL) to see all memories about that target",
				},
				"include_dismissed": map[string]interface{}{
					"type":        "boolean",
					"description": "If true, also show false_positive and disproven entries (default: false)",
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
		entity, _ := args["entity"].(string)
		includeDismissed, _ := args["include_dismissed"].(bool)

		var entries []*agent.MemoryEntry
		var err error

		if entity != "" {
			entity = strings.TrimSpace(entity)
			if includeDismissed {
				entries, err = pm.ListAll(cat, 5000)
			} else {
				entries, err = pm.List(cat, 5000)
			}
			if err == nil {
				filtered := make([]*agent.MemoryEntry, 0, len(entries))
				for _, entry := range entries {
					if strings.EqualFold(strings.TrimSpace(entry.Entity), entity) {
						filtered = append(filtered, entry)
						if len(filtered) >= 100 {
							break
						}
					}
				}
				entries = filtered
			}
		} else if includeDismissed {
			entries, err = pm.ListAll(cat, 100)
		} else {
			entries, err = pm.List(cat, 100)
		}

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
			statusTag := ""
			if e.Status != agent.MemoryStatusActive {
				statusTag = fmt.Sprintf(" [%s]", string(e.Status))
			}
			entityTag := ""
			if e.Entity != "" {
				entityTag = fmt.Sprintf(" entity=%s", e.Entity)
			}
			sb.WriteString(fmt.Sprintf("  [%s]%s %s: %s%s  (id: %s)\n", e.Category, statusTag, e.Key, e.Value, entityTag, e.ID))
		}
		return &mcp.ToolResult{
			Content: []mcp.Content{{Type: "text", Text: sb.String()}},
		}, nil
	})

	// ── delete_memory ─────────────────────────────────────────────────────────
	deleteMemTool := mcp.Tool{
		Name:             builtin.ToolDeleteMemory,
		Description:      "Delete a specific memory entry by ID. Use this to remove stale, incorrect, or no-longer-relevant facts. Prefer update_memory_status for false positives and disproven findings.",
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

	// ── update_memory_status ──────────────────────────────────────────────────
	updateStatusTool := mcp.Tool{
		Name: builtin.ToolUpdateMemoryStatus,
		Description: `Update the status of a memory entry to reflect its current validity.

Statuses:
  active        - Default; the finding is currently relevant and under investigation
  confirmed     - The finding has been validated and reproduced with evidence
  false_positive - The finding was investigated and ruled out (not a real issue)
  disproven     - The fact was found to be incorrect after further investigation

Use this instead of deleting when:
  - A vulnerability turns out to be a false positive after manual verification
  - A credential or fact was found to be wrong
  - A finding is confirmed with solid proof
  - You want to prevent re-investigation of ruled-out paths

Dismissed entries (false_positive, disproven) are shown in a separate section
in the memory context block so the model knows NOT to re-investigate them.`,
		ShortDescription: "Update the validation status of a memory entry",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id": map[string]interface{}{
					"type":        "string",
					"description": "The UUID of the memory entry to update (obtain from list_memories or retrieve_memory)",
				},
				"status": map[string]interface{}{
					"type":        "string",
					"description": "New status: active, confirmed, false_positive, or disproven",
					"enum":        []string{"active", "confirmed", "false_positive", "disproven"},
				},
			},
			"required": []string{"id", "status"},
		},
	}
	mcpServer.RegisterTool(updateStatusTool, func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		id, _ := args["id"].(string)
		statusStr, _ := args["status"].(string)
		if id == "" || statusStr == "" {
			return &mcp.ToolResult{
				Content: []mcp.Content{{Type: "text", Text: "Error: id and status are required"}},
				IsError: true,
			}, nil
		}
		status := agent.MemoryStatus(statusStr)
		switch status {
		case agent.MemoryStatusActive, agent.MemoryStatusConfirmed, agent.MemoryStatusFalsePositive, agent.MemoryStatusDisproven:
			// valid
		default:
			return &mcp.ToolResult{
				Content: []mcp.Content{{Type: "text", Text: fmt.Sprintf("Error: invalid status '%s'. Must be: active, confirmed, false_positive, or disproven", statusStr)}},
				IsError: true,
			}, nil
		}
		if err := pm.SetStatus(id, status); err != nil {
			return &mcp.ToolResult{
				Content: []mcp.Content{{Type: "text", Text: "Error updating memory status: " + err.Error()}},
				IsError: true,
			}, nil
		}
		return &mcp.ToolResult{
			Content: []mcp.Content{{Type: "text", Text: fmt.Sprintf("Memory entry %s status updated to '%s'.", id, statusStr)}},
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

// registerFileManagerTools registers MCP tools for the file manager so the agent
// can register, update, and query managed files during conversations.
func registerFileManagerTools(mcpServer *mcp.Server, fm *filemanager.Manager, logger *zap.Logger) {
	allFileTypes := []string{"report", "api_docs", "project_file", "target_file", "reversing", "exfiltrated", "other"}
	allFileStatuses := []string{"pending", "processing", "analyzed", "in_progress", "completed", "archived"}

	// ── register_file ─────────────────────────────────────────────────────────
	registerTool := mcp.Tool{
		Name:             builtin.ToolRegisterFile,
		Description:      "Register a file in the file manager. Use this whenever you encounter, create, download, or receive a file that should be tracked. The file manager maintains metadata, processing status, summaries, findings, and logs for each file.",
		ShortDescription: "Register a file for tracking in file manager",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"file_name": map[string]interface{}{
					"type":        "string",
					"description": "Name of the file",
				},
				"file_path": map[string]interface{}{
					"type":        "string",
					"description": "Absolute path to the file on disk",
				},
				"file_size": map[string]interface{}{
					"type":        "integer",
					"description": "File size in bytes",
				},
				"file_type": map[string]interface{}{
					"type":        "string",
					"description": "Type classification of the file",
					"enum":        allFileTypes,
				},
				"summary": map[string]interface{}{
					"type":        "string",
					"description": "Brief summary: what is this file, where it came from, what it contains",
				},
				"handle_plan": map[string]interface{}{
					"type":        "string",
					"description": "How you plan to handle/process this file",
				},
				"conversation_id": map[string]interface{}{
					"type":        "string",
					"description": "ID of the conversation this file is associated with",
				},
			},
			"required": []string{"file_name", "file_path", "file_type"},
		},
	}
	mcpServer.RegisterTool(registerTool, func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		fileName, _ := args["file_name"].(string)
		filePath, _ := args["file_path"].(string)
		fileSize := int64(0)
		if v, ok := args["file_size"].(float64); ok {
			fileSize = int64(v)
		}
		ft, _ := args["file_type"].(string)
		convID, _ := args["conversation_id"].(string)

		f, err := fm.Register(fileName, filePath, fileSize, "", filemanager.FileType(ft), convID)
		if err != nil {
			return &mcp.ToolResult{
				Content: []mcp.Content{{Type: "text", Text: "Error registering file: " + err.Error()}},
				IsError: true,
			}, nil
		}

		// Apply summary and handle_plan if provided
		updates := make(map[string]interface{})
		if summary, ok := args["summary"].(string); ok && summary != "" {
			updates["summary"] = summary
		}
		if plan, ok := args["handle_plan"].(string); ok && plan != "" {
			updates["handle_plan"] = plan
		}
		if len(updates) > 0 {
			updates["status"] = string(filemanager.FileStatusAnalyzed)
			fm.Update(f.ID, updates)
		}

		return &mcp.ToolResult{
			Content: []mcp.Content{{Type: "text", Text: fmt.Sprintf("File registered: %s (id: %s, type: %s, path: %s)", fileName, f.ID, ft, filePath)}},
		}, nil
	})

	// ── update_file ───────────────────────────────────────────────────────────
	updateTool := mcp.Tool{
		Name:             builtin.ToolUpdateFile,
		Description:      "Update a managed file's metadata. Use this to update summary, progress, findings, status, handle_plan, or tags as you work on a file.",
		ShortDescription: "Update file metadata in file manager",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id": map[string]interface{}{
					"type":        "string",
					"description": "The file ID to update",
				},
				"summary": map[string]interface{}{
					"type":        "string",
					"description": "Updated summary of the file",
				},
				"handle_plan": map[string]interface{}{
					"type":        "string",
					"description": "Updated plan for handling the file",
				},
				"progress": map[string]interface{}{
					"type":        "string",
					"description": "Current progress notes",
				},
				"findings": map[string]interface{}{
					"type":        "string",
					"description": "Replace all findings with this text",
				},
				"status": map[string]interface{}{
					"type":        "string",
					"description": "New status",
					"enum":        allFileStatuses,
				},
				"tags": map[string]interface{}{
					"type":        "string",
					"description": "Comma-separated tags",
				},
			},
			"required": []string{"id"},
		},
	}
	mcpServer.RegisterTool(updateTool, func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		id, _ := args["id"].(string)
		if id == "" {
			return &mcp.ToolResult{
				Content: []mcp.Content{{Type: "text", Text: "Error: id is required"}},
				IsError: true,
			}, nil
		}

		updates := make(map[string]interface{})
		for _, field := range []string{"summary", "handle_plan", "progress", "findings", "status", "tags"} {
			if v, ok := args[field].(string); ok && v != "" {
				updates[field] = v
			}
		}

		f, err := fm.Update(id, updates)
		if err != nil {
			return &mcp.ToolResult{
				Content: []mcp.Content{{Type: "text", Text: "Error updating file: " + err.Error()}},
				IsError: true,
			}, nil
		}

		return &mcp.ToolResult{
			Content: []mcp.Content{{Type: "text", Text: fmt.Sprintf("File updated: %s (status: %s)", f.FileName, f.Status)}},
		}, nil
	})

	// ── list_files ────────────────────────────────────────────────────────────
	listTool := mcp.Tool{
		Name:             builtin.ToolListFiles,
		Description:      "List all managed files with optional filtering by type, status, or search query.",
		ShortDescription: "List managed files",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"file_type": map[string]interface{}{
					"type":        "string",
					"description": "Filter by file type",
					"enum":        allFileTypes,
				},
				"status": map[string]interface{}{
					"type":        "string",
					"description": "Filter by status",
					"enum":        allFileStatuses,
				},
				"search": map[string]interface{}{
					"type":        "string",
					"description": "Search in file names, summaries, findings, and tags",
				},
			},
			"required": []string{},
		},
	}
	mcpServer.RegisterTool(listTool, func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		ft := filemanager.FileType("")
		if v, ok := args["file_type"].(string); ok {
			ft = filemanager.FileType(v)
		}
		status := filemanager.FileStatus("")
		if v, ok := args["status"].(string); ok {
			status = filemanager.FileStatus(v)
		}
		search, _ := args["search"].(string)

		files, total, err := fm.List(ft, status, search, 50, 0)
		if err != nil {
			return &mcp.ToolResult{
				Content: []mcp.Content{{Type: "text", Text: "Error listing files: " + err.Error()}},
				IsError: true,
			}, nil
		}
		if total == 0 {
			return &mcp.ToolResult{
				Content: []mcp.Content{{Type: "text", Text: "No managed files found."}},
			}, nil
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Found %d managed files:\n", total))
		for _, f := range files {
			tags := ""
			if f.Tags != "" {
				tags = fmt.Sprintf(" [%s]", f.Tags)
			}
			sb.WriteString(fmt.Sprintf("  - %s (id: %s, type: %s, status: %s, size: %d)%s\n    Summary: %s\n    Path: %s\n",
				f.FileName, f.ID, f.FileType, f.Status, f.FileSize, tags,
				truncate(f.Summary, 200), f.FilePath))
		}
		return &mcp.ToolResult{
			Content: []mcp.Content{{Type: "text", Text: sb.String()}},
		}, nil
	})

	// ── get_file ──────────────────────────────────────────────────────────────
	getTool := mcp.Tool{
		Name:             builtin.ToolGetFile,
		Description:      "Get full details of a managed file including summary, progress, findings, logs, and handle plan.",
		ShortDescription: "Get managed file details",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id": map[string]interface{}{
					"type":        "string",
					"description": "The file ID",
				},
			},
			"required": []string{"id"},
		},
	}
	mcpServer.RegisterTool(getTool, func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		id, _ := args["id"].(string)
		if id == "" {
			return &mcp.ToolResult{
				Content: []mcp.Content{{Type: "text", Text: "Error: id is required"}},
				IsError: true,
			}, nil
		}

		f, err := fm.Get(id)
		if err != nil {
			return &mcp.ToolResult{
				Content: []mcp.Content{{Type: "text", Text: "Error: " + err.Error()}},
				IsError: true,
			}, nil
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("File: %s\n", f.FileName))
		sb.WriteString(fmt.Sprintf("ID: %s\n", f.ID))
		sb.WriteString(fmt.Sprintf("Type: %s | Status: %s | Size: %d bytes\n", f.FileType, f.Status, f.FileSize))
		sb.WriteString(fmt.Sprintf("Path: %s\n", f.FilePath))
		if f.Tags != "" {
			sb.WriteString(fmt.Sprintf("Tags: %s\n", f.Tags))
		}
		sb.WriteString(fmt.Sprintf("Created: %s | Updated: %s\n", f.CreatedAt.Format("2006-01-02 15:04"), f.UpdatedAt.Format("2006-01-02 15:04")))
		if f.Summary != "" {
			sb.WriteString(fmt.Sprintf("\nSummary:\n%s\n", f.Summary))
		}
		if f.HandlePlan != "" {
			sb.WriteString(fmt.Sprintf("\nHandle Plan:\n%s\n", f.HandlePlan))
		}
		if f.Progress != "" {
			sb.WriteString(fmt.Sprintf("\nProgress:\n%s\n", f.Progress))
		}
		if f.Findings != "" {
			sb.WriteString(fmt.Sprintf("\nFindings:\n%s\n", f.Findings))
		}
		if f.Logs != "" {
			sb.WriteString(fmt.Sprintf("\nLogs:\n%s\n", f.Logs))
		}
		return &mcp.ToolResult{
			Content: []mcp.Content{{Type: "text", Text: sb.String()}},
		}, nil
	})

	// ── append_file_log ───────────────────────────────────────────────────────
	logTool := mcp.Tool{
		Name:             builtin.ToolAppendFileLog,
		Description:      "Append a timestamped log entry to a managed file's processing log.",
		ShortDescription: "Append log entry to managed file",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id": map[string]interface{}{
					"type":        "string",
					"description": "The file ID",
				},
				"entry": map[string]interface{}{
					"type":        "string",
					"description": "Log entry text",
				},
			},
			"required": []string{"id", "entry"},
		},
	}
	mcpServer.RegisterTool(logTool, func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		id, _ := args["id"].(string)
		entry, _ := args["entry"].(string)
		if id == "" || entry == "" {
			return &mcp.ToolResult{
				Content: []mcp.Content{{Type: "text", Text: "Error: id and entry are required"}},
				IsError: true,
			}, nil
		}
		if err := fm.AppendLog(id, entry); err != nil {
			return &mcp.ToolResult{
				Content: []mcp.Content{{Type: "text", Text: "Error: " + err.Error()}},
				IsError: true,
			}, nil
		}
		return &mcp.ToolResult{
			Content: []mcp.Content{{Type: "text", Text: "Log entry appended."}},
		}, nil
	})

	// ── append_file_findings ──────────────────────────────────────────────────
	findingsTool := mcp.Tool{
		Name:             builtin.ToolAppendFindings,
		Description:      "Append a finding or discovery to a managed file's findings section.",
		ShortDescription: "Append finding to managed file",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id": map[string]interface{}{
					"type":        "string",
					"description": "The file ID",
				},
				"finding": map[string]interface{}{
					"type":        "string",
					"description": "Finding or discovery text",
				},
			},
			"required": []string{"id", "finding"},
		},
	}
	mcpServer.RegisterTool(findingsTool, func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		id, _ := args["id"].(string)
		finding, _ := args["finding"].(string)
		if id == "" || finding == "" {
			return &mcp.ToolResult{
				Content: []mcp.Content{{Type: "text", Text: "Error: id and finding are required"}},
				IsError: true,
			}, nil
		}
		if err := fm.AppendFindings(id, finding); err != nil {
			return &mcp.ToolResult{
				Content: []mcp.Content{{Type: "text", Text: "Error: " + err.Error()}},
				IsError: true,
			}, nil
		}
		return &mcp.ToolResult{
			Content: []mcp.Content{{Type: "text", Text: "Finding appended."}},
		}, nil
	})

	logger.Info("file manager MCP tools registered")
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// ─────────────────────────────────────────────────────────────────────────────
// Cuttlefish (Android VM) MCP Tools
// ─────────────────────────────────────────────────────────────────────────────

// cvdExec runs a command in the Cuttlefish workspace and returns combined output.
func cvdExec(ctx context.Context, cvdHome string, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = cvdHome
	cmd.Env = append(os.Environ(), "CVD_HOME="+cvdHome)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	out := stdout.String()
	if stderr.Len() > 0 {
		out += "\n" + stderr.String()
	}
	if len(out) > 100000 {
		out = out[:100000] + "\n... (truncated)"
	}
	return out, err
}

// cvdADB returns the path to the Cuttlefish-bundled adb, falling back to system adb.
func cvdADB(cvdHome string) string {
	p := filepath.Join(cvdHome, "bin", "adb")
	if _, err := os.Stat(p); err == nil {
		return p
	}
	return "adb"
}

func registerCuttlefishTools(mcpServer *mcp.Server, cvdHome string, logger *zap.Logger) {
	adb := cvdADB(cvdHome)
	bridgeScript := filepath.Join(cvdHome, "..", "CyberStrikeAI", "scripts", "cuttlefish", "droidrun-bridge.py")
	// Also try the absolute common path
	if _, err := os.Stat(bridgeScript); err != nil {
		bridgeScript = filepath.Join(os.Getenv("HOME"), "CyberStrikeAI", "scripts", "cuttlefish", "droidrun-bridge.py")
	}

	logger.Info("registering Cuttlefish (Android VM) MCP tools", zap.String("cvd_home", cvdHome), zap.String("adb", adb))

	// ── cuttlefish_launch ────────────────────────────────────────────────
	mcpServer.RegisterTool(mcp.Tool{
		Name:             builtin.ToolCuttlefishLaunch,
		Description:      "Launch the Cuttlefish Android virtual device. Starts an AOSP VM with QEMU/KVM preconfigured as a Russian-owned Xiaomi Redmi Note 12 Pro (MTS carrier, Moscow timezone, ru-RU locale). The device becomes available via ADB and WebRTC (https://localhost:8443). Takes 2-4 minutes for first boot.",
		ShortDescription: "Launch Cuttlefish Android VM",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"fresh": map[string]interface{}{
					"type":        "boolean",
					"description": "If true, create a fresh data partition (factory reset). Default false.",
				},
				"memory_mb": map[string]interface{}{
					"type":        "integer",
					"description": "RAM in MB (default 8192)",
				},
				"cpus": map[string]interface{}{
					"type":        "integer",
					"description": "Number of virtual CPUs (default 4)",
				},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		env := os.Environ()
		env = append(env, "CVD_HOME="+cvdHome)
		if fresh, ok := args["fresh"].(bool); ok && fresh {
			env = append(env, "FRESH=1")
		}
		if mem, ok := args["memory_mb"].(float64); ok {
			env = append(env, "CVD_MEMORY="+strconv.Itoa(int(mem)))
		}
		if cpus, ok := args["cpus"].(float64); ok {
			env = append(env, "CVD_CPUS="+strconv.Itoa(int(cpus)))
		}

		launchScript := filepath.Join(cvdHome, "cvd-launch.sh")
		cmd := exec.CommandContext(ctx, launchScript)
		cmd.Dir = cvdHome
		cmd.Env = env
		var buf bytes.Buffer
		cmd.Stdout = &buf
		cmd.Stderr = &buf
		err := cmd.Run()
		out := buf.String()
		if err != nil {
			return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: "Launch failed: " + err.Error() + "\n" + out}}, IsError: true}, nil
		}
		return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: out}}}, nil
	})

	// ── cuttlefish_stop ─────────────────────────────────────────────────
	mcpServer.RegisterTool(mcp.Tool{
		Name:             builtin.ToolCuttlefishStop,
		Description:      "Stop the running Cuttlefish Android virtual device.",
		ShortDescription: "Stop Cuttlefish Android VM",
		InputSchema:      map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
	}, func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		out, err := cvdExec(ctx, cvdHome, filepath.Join(cvdHome, "cvd-stop.sh"))
		if err != nil {
			return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: "Stop failed: " + err.Error() + "\n" + out}}, IsError: true}, nil
		}
		return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: out}}}, nil
	})

	// ── cuttlefish_status ───────────────────────────────────────────────
	mcpServer.RegisterTool(mcp.Tool{
		Name:             builtin.ToolCuttlefishStatus,
		Description:      "Check if Cuttlefish is running, get ADB device serial, and current device properties (locale, carrier, model).",
		ShortDescription: "Check Cuttlefish device status",
		InputSchema:      map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
	}, func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		// Check ADB devices
		devOut, _ := cvdExec(ctx, cvdHome, adb, "devices", "-l")
		// Get key properties if device is connected
		propsOut, _ := cvdExec(ctx, cvdHome, adb, "shell",
			"echo 'boot_completed='$(getprop sys.boot_completed)"+
				" && echo 'locale='$(getprop persist.sys.locale)"+
				" && echo 'timezone='$(getprop persist.sys.timezone)"+
				" && echo 'carrier='$(getprop gsm.sim.operator.alpha)"+
				" && echo 'model='$(getprop ro.product.model)"+
				" && echo 'manufacturer='$(getprop ro.product.manufacturer)"+
				" && echo 'sdk='$(getprop ro.build.version.sdk)"+
				" && echo 'android='$(getprop ro.build.version.release)")
		return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: "ADB Devices:\n" + devOut + "\nProperties:\n" + propsOut}}}, nil
	})

	// ── cuttlefish_install_apk ──────────────────────────────────────────
	mcpServer.RegisterTool(mcp.Tool{
		Name:             builtin.ToolCuttlefishInstall,
		Description:      "Install an APK on the Cuttlefish Android device. Supports debug (-t), downgrade (-d), and replace (-r) flags.",
		ShortDescription: "Install APK on Cuttlefish",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"apk_path": map[string]interface{}{
					"type":        "string",
					"description": "Absolute path to the APK file",
				},
				"replace": map[string]interface{}{
					"type":        "boolean",
					"description": "Replace existing app (default true)",
				},
				"downgrade": map[string]interface{}{
					"type":        "boolean",
					"description": "Allow version downgrade",
				},
				"debug": map[string]interface{}{
					"type":        "boolean",
					"description": "Allow test-only APKs",
				},
			},
			"required": []string{"apk_path"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		apkPath, _ := args["apk_path"].(string)
		installArgs := []string{"install"}
		if r, ok := args["replace"].(bool); !ok || r {
			installArgs = append(installArgs, "-r")
		}
		if d, ok := args["downgrade"].(bool); ok && d {
			installArgs = append(installArgs, "-d")
		}
		if t, ok := args["debug"].(bool); ok && t {
			installArgs = append(installArgs, "-t")
		}
		installArgs = append(installArgs, apkPath)
		out, err := cvdExec(ctx, cvdHome, adb, installArgs...)
		if err != nil {
			return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: "Install failed: " + err.Error() + "\n" + out}}, IsError: true}, nil
		}
		return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: out}}}, nil
	})

	// ── cuttlefish_hotswap ──────────────────────────────────────────────
	mcpServer.RegisterTool(mcp.Tool{
		Name:             builtin.ToolCuttlefishHotswap,
		Description:      "Hot-swap reinstall an APK: force-stops the running app, reinstalls with -r -d -t flags, and relaunches. Use for rapid iteration during testing/reversing.",
		ShortDescription: "Hot-swap APK on Cuttlefish",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"apk_path": map[string]interface{}{
					"type":        "string",
					"description": "Path to the APK file",
				},
				"package_name": map[string]interface{}{
					"type":        "string",
					"description": "Package name (e.g. com.example.app). Auto-detected if omitted.",
				},
			},
			"required": []string{"apk_path"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		apkPath, _ := args["apk_path"].(string)
		cmdArgs := []string{apkPath}
		if pkg, ok := args["package_name"].(string); ok && pkg != "" {
			cmdArgs = append(cmdArgs, pkg)
		}
		out, err := cvdExec(ctx, cvdHome, filepath.Join(cvdHome, "cvd-hotswap.sh"), cmdArgs...)
		if err != nil {
			return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: "Hotswap failed: " + err.Error() + "\n" + out}}, IsError: true}, nil
		}
		return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: out}}}, nil
	})

	// ── cuttlefish_shell ────────────────────────────────────────────────
	mcpServer.RegisterTool(mcp.Tool{
		Name:             builtin.ToolCuttlefishShell,
		Description:      "Execute a shell command on the Cuttlefish Android device via ADB. Returns stdout/stderr. Use for any direct device interaction: file browsing, process listing, property queries, app management, etc.",
		ShortDescription: "Run shell command on Android device",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"command": map[string]interface{}{
					"type":        "string",
					"description": "Shell command to execute on device (e.g. 'ls /data/local/tmp', 'pm list packages -3', 'getprop ro.product.model')",
				},
			},
			"required": []string{"command"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		command, _ := args["command"].(string)
		out, err := cvdExec(ctx, cvdHome, adb, "shell", command)
		if err != nil {
			// ADB shell returns the device command's exit code, which may be non-zero
			// but still have useful output
			return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: out}}}, nil
		}
		return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: out}}}, nil
	})

	// ── cuttlefish_push ─────────────────────────────────────────────────
	mcpServer.RegisterTool(mcp.Tool{
		Name:             builtin.ToolCuttlefishPush,
		Description:      "Push a file from host to the Cuttlefish Android device via ADB.",
		ShortDescription: "Push file to Android device",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"local_path": map[string]interface{}{
					"type":        "string",
					"description": "Path to the file on the host",
				},
				"device_path": map[string]interface{}{
					"type":        "string",
					"description": "Destination path on the device (e.g. /data/local/tmp/file)",
				},
			},
			"required": []string{"local_path", "device_path"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		localPath, _ := args["local_path"].(string)
		devicePath, _ := args["device_path"].(string)
		out, err := cvdExec(ctx, cvdHome, adb, "push", localPath, devicePath)
		if err != nil {
			return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: "Push failed: " + err.Error() + "\n" + out}}, IsError: true}, nil
		}
		return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: out}}}, nil
	})

	// ── cuttlefish_pull ─────────────────────────────────────────────────
	mcpServer.RegisterTool(mcp.Tool{
		Name:             builtin.ToolCuttlefishPull,
		Description:      "Pull a file from the Cuttlefish Android device to the host via ADB.",
		ShortDescription: "Pull file from Android device",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"device_path": map[string]interface{}{
					"type":        "string",
					"description": "Path on the device to pull (e.g. /data/data/com.app/databases/db.sqlite)",
				},
				"local_path": map[string]interface{}{
					"type":        "string",
					"description": "Destination path on the host",
				},
			},
			"required": []string{"device_path", "local_path"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		devicePath, _ := args["device_path"].(string)
		localPath, _ := args["local_path"].(string)
		out, err := cvdExec(ctx, cvdHome, adb, "pull", devicePath, localPath)
		if err != nil {
			return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: "Pull failed: " + err.Error() + "\n" + out}}, IsError: true}, nil
		}
		return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: out}}}, nil
	})

	// ── cuttlefish_screenshot ───────────────────────────────────────────
	mcpServer.RegisterTool(mcp.Tool{
		Name:             builtin.ToolCuttlefishScreenshot,
		Description:      "Take a screenshot of the Cuttlefish Android device. Returns the path to the saved PNG file.",
		ShortDescription: "Screenshot Android device",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"output_path": map[string]interface{}{
					"type":        "string",
					"description": "Where to save the screenshot (default: /tmp/cvd_screenshot.png)",
				},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		outPath := "/tmp/cvd_screenshot.png"
		if p, ok := args["output_path"].(string); ok && p != "" {
			outPath = p
		}
		cmd := exec.CommandContext(ctx, adb, "exec-out", "screencap", "-p")
		cmd.Dir = cvdHome
		var buf bytes.Buffer
		cmd.Stdout = &buf
		if err := cmd.Run(); err != nil {
			return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: "Screenshot failed: " + err.Error()}}, IsError: true}, nil
		}
		if err := os.WriteFile(outPath, buf.Bytes(), 0644); err != nil {
			return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: "Failed to write screenshot: " + err.Error()}}, IsError: true}, nil
		}
		return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: fmt.Sprintf("Screenshot saved to %s (%d bytes)", outPath, buf.Len())}}}, nil
	})

	// ── cuttlefish_logcat ───────────────────────────────────────────────
	mcpServer.RegisterTool(mcp.Tool{
		Name:             builtin.ToolCuttlefishLogcat,
		Description:      "Read Android logcat output from the Cuttlefish device. Supports filtering by tag, priority, and line count.",
		ShortDescription: "Read Android logcat",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"filter": map[string]interface{}{
					"type":        "string",
					"description": "Logcat filter expression (e.g. 'ActivityManager:I *:S' or tag name)",
				},
				"lines": map[string]interface{}{
					"type":        "integer",
					"description": "Number of recent lines to return (default 100)",
				},
				"grep": map[string]interface{}{
					"type":        "string",
					"description": "Grep pattern to filter output",
				},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		lines := 100
		if n, ok := args["lines"].(float64); ok && n > 0 {
			lines = int(n)
		}
		logArgs := []string{"logcat", "-d", "-t", strconv.Itoa(lines)}
		if filter, ok := args["filter"].(string); ok && filter != "" {
			logArgs = append(logArgs, filter)
		}
		out, _ := cvdExec(ctx, cvdHome, adb, logArgs...)
		if grepPat, ok := args["grep"].(string); ok && grepPat != "" {
			var filtered []string
			for _, line := range strings.Split(out, "\n") {
				if strings.Contains(line, grepPat) {
					filtered = append(filtered, line)
				}
			}
			out = strings.Join(filtered, "\n")
		}
		return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: out}}}, nil
	})

	// ── cuttlefish_frida_setup ──────────────────────────────────────────
	mcpServer.RegisterTool(mcp.Tool{
		Name:             builtin.ToolCuttlefishFrida,
		Description:      "Set up Frida server on the Cuttlefish device for dynamic instrumentation. Downloads the matching frida-server binary, pushes it to the device, and starts it. After setup, use frida/frida-tools from the host to attach to processes.",
		ShortDescription: "Setup Frida on Android device",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"version": map[string]interface{}{
					"type":        "string",
					"description": "Frida version (default: 16.6.6)",
				},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		out, err := cvdExec(ctx, cvdHome, filepath.Join(cvdHome, "cvd-api.sh"), "frida-setup")
		if err != nil {
			return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: "Frida setup failed: " + err.Error() + "\n" + out}}, IsError: true}, nil
		}
		return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: out}}}, nil
	})

	// ── cuttlefish_proxy ────────────────────────────────────────────────
	mcpServer.RegisterTool(mcp.Tool{
		Name:             builtin.ToolCuttlefishProxy,
		Description:      "Set or clear HTTP proxy on the Cuttlefish device. Useful for traffic interception with Burp Suite, mitmproxy, etc.",
		ShortDescription: "Set/clear proxy on Android device",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"set", "clear"},
					"description": "set or clear the proxy",
				},
				"host": map[string]interface{}{
					"type":        "string",
					"description": "Proxy host (required for 'set')",
				},
				"port": map[string]interface{}{
					"type":        "string",
					"description": "Proxy port (required for 'set')",
				},
			},
			"required": []string{"action"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		action, _ := args["action"].(string)
		if action == "clear" {
			out, _ := cvdExec(ctx, cvdHome, adb, "shell", "settings put global http_proxy :0")
			return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: "Proxy cleared.\n" + out}}}, nil
		}
		host, _ := args["host"].(string)
		port, _ := args["port"].(string)
		if host == "" || port == "" {
			return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: "host and port required for set action"}}, IsError: true}, nil
		}
		out, _ := cvdExec(ctx, cvdHome, adb, "shell", "settings put global http_proxy "+host+":"+port)
		return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: fmt.Sprintf("Proxy set to %s:%s\n%s", host, port, out)}}}, nil
	})

	// ── cuttlefish_install_cert ─────────────────────────────────────────
	mcpServer.RegisterTool(mcp.Tool{
		Name:             builtin.ToolCuttlefishCert,
		Description:      "Install a CA certificate into the Android system trust store on the Cuttlefish device. Required for HTTPS traffic interception. The device must have root access (Cuttlefish userdebug builds support this).",
		ShortDescription: "Install CA cert on Android device",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"cert_path": map[string]interface{}{
					"type":        "string",
					"description": "Path to the PEM-encoded CA certificate file on the host",
				},
			},
			"required": []string{"cert_path"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		certPath, _ := args["cert_path"].(string)
		out, err := cvdExec(ctx, cvdHome, filepath.Join(cvdHome, "cvd-api.sh"), "install-cert", certPath)
		if err != nil {
			return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: "Cert install failed: " + err.Error() + "\n" + out}}, IsError: true}, nil
		}
		return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: out}}}, nil
	})

	// ── cuttlefish_snapshot ─────────────────────────────────────────────
	mcpServer.RegisterTool(mcp.Tool{
		Name:             builtin.ToolCuttlefishSnapshot,
		Description:      "Save, restore, or list device state snapshots. Use 'save' before destructive testing, 'restore' to revert to a known-good state.",
		ShortDescription: "Manage Cuttlefish device snapshots",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"save", "restore", "list"},
					"description": "Snapshot action",
				},
				"name": map[string]interface{}{
					"type":        "string",
					"description": "Snapshot name (default: 'default')",
				},
			},
			"required": []string{"action"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		action, _ := args["action"].(string)
		cmdArgs := []string{action}
		if name, ok := args["name"].(string); ok && name != "" {
			cmdArgs = append(cmdArgs, name)
		}
		out, err := cvdExec(ctx, cvdHome, filepath.Join(cvdHome, "cvd-snapshot.sh"), cmdArgs...)
		if err != nil {
			return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: "Snapshot failed: " + err.Error() + "\n" + out}}, IsError: true}, nil
		}
		return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: out}}}, nil
	})

	// ── cuttlefish_packages ─────────────────────────────────────────────
	mcpServer.RegisterTool(mcp.Tool{
		Name:             builtin.ToolCuttlefishPackages,
		Description:      "List, inspect, or manage packages on the Cuttlefish device. Actions: list (all/third-party), info (package details), permissions, activities, clear-data, force-stop, enable, disable.",
		ShortDescription: "Manage packages on Android device",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"list", "list-third-party", "info", "permissions", "activities", "clear-data", "force-stop", "enable", "disable", "uninstall"},
					"description": "Package management action",
				},
				"package": map[string]interface{}{
					"type":        "string",
					"description": "Package name (required for info/permissions/activities/clear-data/force-stop/enable/disable/uninstall)",
				},
			},
			"required": []string{"action"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		action, _ := args["action"].(string)
		pkg, _ := args["package"].(string)
		var out string
		var err error
		switch action {
		case "list":
			out, err = cvdExec(ctx, cvdHome, adb, "shell", "pm list packages")
		case "list-third-party":
			out, err = cvdExec(ctx, cvdHome, adb, "shell", "pm list packages -3")
		case "info":
			out, err = cvdExec(ctx, cvdHome, adb, "shell", "dumpsys package "+pkg)
		case "permissions":
			out, err = cvdExec(ctx, cvdHome, adb, "shell", "dumpsys package "+pkg+" | grep -A 100 'granted=true'")
		case "activities":
			out, err = cvdExec(ctx, cvdHome, adb, "shell", "dumpsys package "+pkg+" | grep -A 5 'Activity'")
		case "clear-data":
			out, err = cvdExec(ctx, cvdHome, adb, "shell", "pm clear "+pkg)
		case "force-stop":
			out, err = cvdExec(ctx, cvdHome, adb, "shell", "am force-stop "+pkg)
		case "enable":
			out, err = cvdExec(ctx, cvdHome, adb, "shell", "pm enable "+pkg)
		case "disable":
			out, err = cvdExec(ctx, cvdHome, adb, "shell", "pm disable-user --user 0 "+pkg)
		case "uninstall":
			out, err = cvdExec(ctx, cvdHome, adb, "shell", "pm uninstall "+pkg)
		default:
			return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: "Unknown action: " + action}}, IsError: true}, nil
		}
		if err != nil {
			return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: out}}}, nil
		}
		return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: out}}}, nil
	})

	// ── cuttlefish_droidrun ─────────────────────────────────────────────
	mcpServer.RegisterTool(mcp.Tool{
		Name:             builtin.ToolCuttlefishDroidRun,
		Description:      "Run DroidRun AI agent on the Cuttlefish device. Give a natural language goal and the agent will autonomously interact with the Android UI to accomplish it (e.g. 'Open Settings and find device info', 'Install and test the login flow of the app'). Requires DroidRun and its dependencies to be installed (pip install droidrun).",
		ShortDescription: "Run DroidRun AI agent on Cuttlefish",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"goal": map[string]interface{}{
					"type":        "string",
					"description": "Natural language goal for the DroidRun agent",
				},
				"install_apk": map[string]interface{}{
					"type":        "string",
					"description": "APK path to install before running the goal (optional)",
				},
				"config": map[string]interface{}{
					"type":        "string",
					"description": "Path to custom DroidRun config YAML (optional)",
				},
			},
			"required": []string{"goal"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		goal, _ := args["goal"].(string)
		cmdArgs := []string{bridgeScript}
		if apk, ok := args["install_apk"].(string); ok && apk != "" {
			cmdArgs = append(cmdArgs, "--install", apk)
		}
		if cfg, ok := args["config"].(string); ok && cfg != "" {
			cmdArgs = append(cmdArgs, "--config", cfg)
		}
		cmdArgs = append(cmdArgs, goal)
		out, err := cvdExec(ctx, cvdHome, "python3", cmdArgs...)
		if err != nil {
			return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: "DroidRun failed: " + err.Error() + "\n" + out}}, IsError: true}, nil
		}
		return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: out}}}, nil
	})

	logger.Info("registered Cuttlefish MCP tools", zap.Int("count", 16))
}
