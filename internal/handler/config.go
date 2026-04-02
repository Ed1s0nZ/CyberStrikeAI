package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"cyberstrike-ai/internal/agents"
	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/knowledge"
	"cyberstrike-ai/internal/mcp"
	"cyberstrike-ai/internal/security"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// KnowledgeToolRegistrar knowledge base
type KnowledgeToolRegistrar func() error

// VulnerabilityToolRegistrar vulnerability tool registrar interface
type VulnerabilityToolRegistrar func() error

// WebshellToolRegistrar WebShell tool registrar interface（ApplyConfig ）
type WebshellToolRegistrar func() error

// SkillsToolRegistrar Skills tool registrar interface
type SkillsToolRegistrar func() error

// RetrieverUpdater retriever
type RetrieverUpdater interface {
	UpdateConfig(config *knowledge.RetrievalConfig)
}

// KnowledgeInitializer knowledge base
type KnowledgeInitializer func() (*KnowledgeHandler, error)

// AppUpdater App updater interface（Appknowledge base）
type AppUpdater interface {
	UpdateKnowledgeComponents(handler *KnowledgeHandler, manager interface{}, retriever interface{}, indexer interface{})
}

// RobotRestarter restarts robot connections (Telegram bot).
type RobotRestarter interface {
	RestartRobotConnections()
}

// ConfigHandler config handler
type ConfigHandler struct {
	configPath                 string
	config                     *config.Config
	mcpServer                  *mcp.Server
	executor                   *security.Executor
	agent AgentUpdater // Agent interface，update Agent config
	attackChainHandler         AttackChainUpdater         // attack chain handler interface，for updating config
	externalMCPMgr *mcp.ExternalMCPManager // external MCP management
	knowledgeToolRegistrar KnowledgeToolRegistrar // knowledge base（）
	vulnerabilityToolRegistrar VulnerabilityToolRegistrar // vulnerability tool registrar (optional)
	webshellToolRegistrar      WebshellToolRegistrar      // WebShell tool registrar (optional)
	skillsToolRegistrar        SkillsToolRegistrar        // Skills tool registrar (optional)
	retrieverUpdater RetrieverUpdater // retriever（）
	knowledgeInitializer KnowledgeInitializer // knowledge base（）
	appUpdater                 AppUpdater                 // App updater (optional)
	robotRestarter RobotRestarter // robot connection restarter; ApplyConfig restarts Telegram bot
	logger                     *zap.Logger
	mu                         sync.RWMutex
	lastEmbeddingConfig        *config.EmbeddingConfig // previous embedding model config（for detecting changes）
}

// AttackChainUpdater attack chain handler updater interface
type AttackChainUpdater interface {
	UpdateConfig(cfg *config.OpenAIConfig)
}

// AgentUpdater Agent updater interface
type AgentUpdater interface {
	UpdateConfig(cfg *config.OpenAIConfig)
	UpdateMaxIterations(maxIterations int)
}

// NewConfigHandler creates a new config handler
func NewConfigHandler(configPath string, cfg *config.Config, mcpServer *mcp.Server, executor *security.Executor, agent AgentUpdater, attackChainHandler AttackChainUpdater, externalMCPMgr *mcp.ExternalMCPManager, logger *zap.Logger) *ConfigHandler {
	// save initial embedding model config（knowledge base）
	var lastEmbeddingConfig *config.EmbeddingConfig
	if cfg.Knowledge.Enabled {
		lastEmbeddingConfig = &config.EmbeddingConfig{
			Provider: cfg.Knowledge.Embedding.Provider,
			Model:    cfg.Knowledge.Embedding.Model,
			BaseURL:  cfg.Knowledge.Embedding.BaseURL,
			APIKey:   cfg.Knowledge.Embedding.APIKey,
		}
	}
	return &ConfigHandler{
		configPath:          configPath,
		config:              cfg,
		mcpServer:           mcpServer,
		executor:            executor,
		agent:               agent,
		attackChainHandler:  attackChainHandler,
		externalMCPMgr:      externalMCPMgr,
		logger:              logger,
		lastEmbeddingConfig: lastEmbeddingConfig,
	}
}

// SetKnowledgeToolRegistrar knowledge base
func (h *ConfigHandler) SetKnowledgeToolRegistrar(registrar KnowledgeToolRegistrar) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.knowledgeToolRegistrar = registrar
}

// SetVulnerabilityToolRegistrar set vulnerability tool registrar
func (h *ConfigHandler) SetVulnerabilityToolRegistrar(registrar VulnerabilityToolRegistrar) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.vulnerabilityToolRegistrar = registrar
}

// SetWebshellToolRegistrar set WebShell tool registrar
func (h *ConfigHandler) SetWebshellToolRegistrar(registrar WebshellToolRegistrar) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.webshellToolRegistrar = registrar
}

// SetSkillsToolRegistrar set Skills tool registrar
func (h *ConfigHandler) SetSkillsToolRegistrar(registrar SkillsToolRegistrar) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.skillsToolRegistrar = registrar
}

// SetRetrieverUpdater retriever
func (h *ConfigHandler) SetRetrieverUpdater(updater RetrieverUpdater) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.retrieverUpdater = updater
}

// SetKnowledgeInitializer knowledge base
func (h *ConfigHandler) SetKnowledgeInitializer(initializer KnowledgeInitializer) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.knowledgeInitializer = initializer
}

// SetAppUpdater set App updater
func (h *ConfigHandler) SetAppUpdater(updater AppUpdater) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.appUpdater = updater
}

// SetRobotRestarter sets the robot connection restarter (called by ApplyConfig to restart Telegram bot).
func (h *ConfigHandler) SetRobotRestarter(restarter RobotRestarter) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.robotRestarter = restarter
}

// GetConfigResponse get config response
type GetConfigResponse struct {
	OpenAI     config.OpenAIConfig     `json:"openai"`
	FOFA       config.FofaConfig       `json:"fofa"`
	MCP        config.MCPConfig        `json:"mcp"`
	Tools      []ToolConfigInfo        `json:"tools"`
	Agent      config.AgentConfig      `json:"agent"`
	Knowledge  config.KnowledgeConfig  `json:"knowledge"`
	Robots     config.RobotsConfig     `json:"robots,omitempty"`
	MultiAgent config.MultiAgentPublic `json:"multi_agent,omitempty"`
}

// ToolConfigInfo tool config info
type ToolConfigInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
	IsExternal  bool   `json:"is_external,omitempty"`  // whether external MCP tool
	ExternalMCP string `json:"external_mcp,omitempty"` // external MCP name（if external tool）
	RoleEnabled *bool `json:"role_enabled,omitempty"` // currentrole（nilrole）
}

// GetConfig current
func (h *ConfigHandler) GetConfig(c *gin.Context) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// list（including internal and external tools）
	// first get tools from config file
	configToolMap := make(map[string]bool)
	tools := make([]ToolConfigInfo, 0, len(h.config.Security.Tools))
	for _, tool := range h.config.Security.Tools {
		configToolMap[tool.Name] = true
		tools = append(tools, ToolConfigInfo{
			Name:        tool.Name,
			Description: h.pickToolDescription(tool.ShortDescription, tool.Description),
			Enabled:     tool.Enabled,
			IsExternal:  false,
		})
	}

	// get all registered tools from MCP server（including directly registered tools，such as knowledge retrieval tools）
	if h.mcpServer != nil {
		mcpTools := h.mcpServer.GetAllTools()
		for _, mcpTool := range mcpTools {
			// skip（）
			if configToolMap[mcpTool.Name] {
				continue
			}
			// addMCP（such as knowledge retrieval tools）
			description := mcpTool.ShortDescription
			if description == "" {
				description = mcpTool.Description
			}
			if len(description) > 10000 {
				description = description[:10000] + "..."
			}
			tools = append(tools, ToolConfigInfo{
				Name:        mcpTool.Name,
				Description: description,
				Enabled: true, // default
				IsExternal:  false,
			})
		}
	}

	// get external MCP tools
	if h.externalMCPMgr != nil {
		ctx := context.Background()
		externalTools := h.getExternalMCPTools(ctx)
		for _, toolInfo := range externalTools {
			tools = append(tools, toolInfo)
		}
	}

	subAgentCount := len(h.config.MultiAgent.SubAgents)
	agentsDir := strings.TrimSpace(h.config.AgentsDir)
	if agentsDir == "" {
		agentsDir = "agents"
	}
	if !filepath.IsAbs(agentsDir) {
		agentsDir = filepath.Join(filepath.Dir(h.configPath), agentsDir)
	}
	if load, err := agents.LoadMarkdownAgentsDir(agentsDir); err == nil {
		subAgentCount = len(agents.MergeYAMLAndMarkdown(h.config.MultiAgent.SubAgents, load.SubAgents))
	}
	multiPub := config.MultiAgentPublic{
		Enabled:            h.config.MultiAgent.Enabled,
		DefaultMode:        h.config.MultiAgent.DefaultMode,
		RobotUseMultiAgent: h.config.MultiAgent.RobotUseMultiAgent,
		BatchUseMultiAgent: h.config.MultiAgent.BatchUseMultiAgent,
		SubAgentCount:      subAgentCount,
	}
	if strings.TrimSpace(multiPub.DefaultMode) == "" {
		multiPub.DefaultMode = "single"
	}

	c.JSON(http.StatusOK, GetConfigResponse{
		OpenAI:     h.config.OpenAI,
		FOFA:       h.config.FOFA,
		MCP:        h.config.MCP,
		Tools:      tools,
		Agent:      h.config.Agent,
		Knowledge:  h.config.Knowledge,
		Robots:     h.config.Robots,
		MultiAgent: multiPub,
	})
}

// GetToolsResponse list（）
type GetToolsResponse struct {
	Tools        []ToolConfigInfo `json:"tools"`
	Total        int              `json:"total"`
	TotalEnabled int              `json:"total_enabled"` // total enabled tools
	Page         int              `json:"page"`
	PageSize     int              `json:"page_size"`
	TotalPages   int              `json:"total_pages"`
}

// GetTools list（）
func (h *ConfigHandler) GetTools(c *gin.Context) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// parsepagination params
	page := 1
	pageSize := 20
	if pageStr := c.Query("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}
	if pageSizeStr := c.Query("page_size"); pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 && ps <= 100 {
			pageSize = ps
		}
	}

	// parsesearch params
	searchTerm := c.Query("search")
	searchTermLower := ""
	if searchTerm != "" {
		searchTermLower = strings.ToLower(searchTerm)
	}

	// parserole，status
	roleName := c.Query("role")
	var roleToolsSet map[string]bool // role
	var roleUsesAllTools bool = true // role（defaultrole）
	if roleName != "" && roleName != "default" && h.config.Roles != nil {
		if role, exists := h.config.Roles[roleName]; exists && role.Enabled {
			if len(role.Tools) > 0 {
				// rolelist，
				roleToolsSet = make(map[string]bool)
				for _, toolKey := range role.Tools {
					roleToolsSet[toolKey] = true
				}
				roleUsesAllTools = false
			}
		}
	}

	// get all internal tools and apply search filter
	configToolMap := make(map[string]bool)
	allTools := make([]ToolConfigInfo, 0, len(h.config.Security.Tools))
	for _, tool := range h.config.Security.Tools {
		configToolMap[tool.Name] = true
		toolInfo := ToolConfigInfo{
			Name:        tool.Name,
			Description: h.pickToolDescription(tool.ShortDescription, tool.Description),
			Enabled:     tool.Enabled,
			IsExternal:  false,
		}

		// rolestatus
		if roleName != "" {
			if roleUsesAllTools {
				// role，role_enabled=true
				if tool.Enabled {
					roleEnabled := true
					toolInfo.RoleEnabled = &roleEnabled
				} else {
					roleEnabled := false
					toolInfo.RoleEnabled = &roleEnabled
				}
			} else {
				// rolelist，list
				// internal tools use tool name as key
				if roleToolsSet[tool.Name] {
					roleEnabled := tool.Enabled // rolelist
					toolInfo.RoleEnabled = &roleEnabled
				} else {
					// rolelist，mark as false
					roleEnabled := false
					toolInfo.RoleEnabled = &roleEnabled
				}
			}
		}

		// if keywords, apply search filter
		if searchTermLower != "" {
			nameLower := strings.ToLower(toolInfo.Name)
			descLower := strings.ToLower(toolInfo.Description)
			if !strings.Contains(nameLower, searchTermLower) && !strings.Contains(descLower, searchTermLower) {
				continue // match，skip
			}
		}

		allTools = append(allTools, toolInfo)
	}

	// get all registered tools from MCP server（including directly registered tools，such as knowledge retrieval tools）
	if h.mcpServer != nil {
		mcpTools := h.mcpServer.GetAllTools()
		for _, mcpTool := range mcpTools {
			// skip（）
			if configToolMap[mcpTool.Name] {
				continue
			}

			description := mcpTool.ShortDescription
			if description == "" {
				description = mcpTool.Description
			}
			if len(description) > 10000 {
				description = description[:10000] + "..."
			}

			toolInfo := ToolConfigInfo{
				Name:        mcpTool.Name,
				Description: description,
				Enabled: true, // default
				IsExternal:  false,
			}

			// rolestatus
			if roleName != "" {
				if roleUsesAllTools {
					// role，default
					roleEnabled := true
					toolInfo.RoleEnabled = &roleEnabled
				} else {
					// rolelist，list
					// internal tools use tool name as key
					if roleToolsSet[mcpTool.Name] {
						roleEnabled := true // rolelist
						toolInfo.RoleEnabled = &roleEnabled
					} else {
						// rolelist，mark as false
						roleEnabled := false
						toolInfo.RoleEnabled = &roleEnabled
					}
				}
			}

			// if keywords, apply search filter
			if searchTermLower != "" {
				nameLower := strings.ToLower(toolInfo.Name)
				descLower := strings.ToLower(toolInfo.Description)
				if !strings.Contains(nameLower, searchTermLower) && !strings.Contains(descLower, searchTermLower) {
					continue // match，skip
				}
			}

			allTools = append(allTools, toolInfo)
		}
	}

	// get external MCP tools
	if h.externalMCPMgr != nil {
		// context
		ctx := context.Background()
		externalTools := h.getExternalMCPTools(ctx)

		// role
		for _, toolInfo := range externalTools {
			// 
			if searchTermLower != "" {
				nameLower := strings.ToLower(toolInfo.Name)
				descLower := strings.ToLower(toolInfo.Description)
				if !strings.Contains(nameLower, searchTermLower) && !strings.Contains(descLower, searchTermLower) {
					continue // match，skip
				}
			}

			// rolestatus
			if roleName != "" {
				if roleUsesAllTools {
					// role，role_enabled=true
					roleEnabled := toolInfo.Enabled
					toolInfo.RoleEnabled = &roleEnabled
				} else {
					// rolelist，list
					// "mcpName::toolName" formatkey
					externalToolKey := fmt.Sprintf("%s::%s", toolInfo.ExternalMCP, toolInfo.Name)
					if roleToolsSet[externalToolKey] {
						roleEnabled := toolInfo.Enabled // rolelist
						toolInfo.RoleEnabled = &roleEnabled
					} else {
						// rolelist，mark as false
						roleEnabled := false
						toolInfo.RoleEnabled = &roleEnabled
					}
				}
			}

			allTools = append(allTools, toolInfo)
		}
	}

	// rolelist，（list，）
	// ：，， role_enabled status
	// ，currentrole

	total := len(allTools)
	// count enabled tools（role）
	totalEnabled := 0
	for _, tool := range allTools {
		if tool.RoleEnabled != nil && *tool.RoleEnabled {
			totalEnabled++
		} else if tool.RoleEnabled == nil && tool.Enabled {
			// role，
			totalEnabled++
		}
	}

	totalPages := (total + pageSize - 1) / pageSize
	if totalPages == 0 {
		totalPages = 1
	}

	// calculate pagination range
	offset := (page - 1) * pageSize
	end := offset + pageSize
	if end > total {
		end = total
	}

	var tools []ToolConfigInfo
	if offset < total {
		tools = allTools[offset:end]
	} else {
		tools = []ToolConfigInfo{}
	}

	c.JSON(http.StatusOK, GetToolsResponse{
		Tools:        tools,
		Total:        total,
		TotalEnabled: totalEnabled,
		Page:         page,
		PageSize:     pageSize,
		TotalPages:   totalPages,
	})
}

// UpdateConfigRequest update config request
type UpdateConfigRequest struct {
	OpenAI     *config.OpenAIConfig        `json:"openai,omitempty"`
	FOFA       *config.FofaConfig          `json:"fofa,omitempty"`
	MCP        *config.MCPConfig           `json:"mcp,omitempty"`
	Tools      []ToolEnableStatus          `json:"tools,omitempty"`
	Agent      *config.AgentConfig         `json:"agent,omitempty"`
	Knowledge  *config.KnowledgeConfig     `json:"knowledge,omitempty"`
	Robots     *config.RobotsConfig        `json:"robots,omitempty"`
	MultiAgent *config.MultiAgentAPIUpdate `json:"multi_agent,omitempty"`
}

// ToolEnableStatus status
type ToolEnableStatus struct {
	Name        string `json:"name"`
	Enabled     bool   `json:"enabled"`
	IsExternal  bool   `json:"is_external,omitempty"`  // whether external MCP tool
	ExternalMCP string `json:"external_mcp,omitempty"` // external MCP name（if external tool）
}

// UpdateConfig update config
func (h *ConfigHandler) UpdateConfig(c *gin.Context) {
	var req UpdateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request parameters: " + err.Error()})
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	// update OpenAI config
	if req.OpenAI != nil {
		h.config.OpenAI = *req.OpenAI
		h.logger.Info("update OpenAI config",
			zap.String("base_url", h.config.OpenAI.BaseURL),
			zap.String("model", h.config.OpenAI.Model),
		)
	}

	// update FOFA config
	if req.FOFA != nil {
		h.config.FOFA = *req.FOFA
		h.logger.Info("update FOFA config", zap.String("email", h.config.FOFA.Email))
	}

	// update MCP config
	if req.MCP != nil {
		h.config.MCP = *req.MCP
		h.logger.Info("update MCP config",
			zap.Bool("enabled", h.config.MCP.Enabled),
			zap.String("host", h.config.MCP.Host),
			zap.Int("port", h.config.MCP.Port),
		)
	}

	// update Agent config
	if req.Agent != nil {
		h.config.Agent = *req.Agent
		h.logger.Info("update Agent config",
			zap.Int("max_iterations", h.config.Agent.MaxIterations),
		)
	}

	// update Knowledge config
	if req.Knowledge != nil {
		// save old embedding model config（for detecting changes）
		if h.config.Knowledge.Enabled {
			h.lastEmbeddingConfig = &config.EmbeddingConfig{
				Provider: h.config.Knowledge.Embedding.Provider,
				Model:    h.config.Knowledge.Embedding.Model,
				BaseURL:  h.config.Knowledge.Embedding.BaseURL,
				APIKey:   h.config.Knowledge.Embedding.APIKey,
			}
		}
		h.config.Knowledge = *req.Knowledge
		h.logger.Info("update Knowledge config",
			zap.Bool("enabled", h.config.Knowledge.Enabled),
			zap.String("base_path", h.config.Knowledge.BasePath),
			zap.String("embedding_model", h.config.Knowledge.Embedding.Model),
			zap.Int("retrieval_top_k", h.config.Knowledge.Retrieval.TopK),
			zap.Float64("similarity_threshold", h.config.Knowledge.Retrieval.SimilarityThreshold),
			zap.Float64("hybrid_weight", h.config.Knowledge.Retrieval.HybridWeight),
		)
	}

	// update robot config
	if req.Robots != nil {
		h.config.Robots = *req.Robots
		h.logger.Info("update robot config",
			zap.Bool("telegram_enabled", h.config.Robots.Telegram.Enabled),
		)
	}

	// multi-agent scalar（sub_agents config.yaml ）
	if req.MultiAgent != nil {
		h.config.MultiAgent.Enabled = req.MultiAgent.Enabled
		dm := strings.TrimSpace(req.MultiAgent.DefaultMode)
		if dm == "multi" || dm == "single" {
			h.config.MultiAgent.DefaultMode = dm
		}
		h.config.MultiAgent.RobotUseMultiAgent = req.MultiAgent.RobotUseMultiAgent
		h.config.MultiAgent.BatchUseMultiAgent = req.MultiAgent.BatchUseMultiAgent
		h.logger.Info("update multi-agent config",
			zap.Bool("enabled", h.config.MultiAgent.Enabled),
			zap.String("default_mode", h.config.MultiAgent.DefaultMode),
			zap.Bool("robot_use_multi_agent", h.config.MultiAgent.RobotUseMultiAgent),
			zap.Bool("batch_use_multi_agent", h.config.MultiAgent.BatchUseMultiAgent),
		)
	}

	// status
	if req.Tools != nil {
		// separate internal and external tools
		internalToolMap := make(map[string]bool)
		// status：MCP -> -> status
		externalMCPToolMap := make(map[string]map[string]bool)

		for _, toolStatus := range req.Tools {
			if toolStatus.IsExternal && toolStatus.ExternalMCP != "" {
				// ：status
				mcpName := toolStatus.ExternalMCP
				if externalMCPToolMap[mcpName] == nil {
					externalMCPToolMap[mcpName] = make(map[string]bool)
				}
				externalMCPToolMap[mcpName][toolStatus.Name] = toolStatus.Enabled
			} else {
				// internal tool
				internalToolMap[toolStatus.Name] = toolStatus.Enabled
			}
		}

		// internal toolstatus
		for i := range h.config.Security.Tools {
			if enabled, ok := internalToolMap[h.config.Security.Tools[i].Name]; ok {
				h.config.Security.Tools[i].Enabled = enabled
				h.logger.Info("status",
					zap.String("tool", h.config.Security.Tools[i].Name),
					zap.Bool("enabled", enabled),
				)
			}
		}

		// MCPstatus
		if h.externalMCPMgr != nil {
			for mcpName, toolStates := range externalMCPToolMap {
				// update configstatus
				if h.config.ExternalMCP.Servers == nil {
					h.config.ExternalMCP.Servers = make(map[string]config.ExternalMCPServerConfig)
				}
				cfg, exists := h.config.ExternalMCP.Servers[mcpName]
				if !exists {
					h.logger.Warn("external MCP config not found", zap.String("mcp", mcpName))
					continue
				}

				// ToolEnabled map
				if cfg.ToolEnabled == nil {
					cfg.ToolEnabled = make(map[string]bool)
				}

				// status
				for toolName, enabled := range toolStates {
					cfg.ToolEnabled[toolName] = enabled
					h.logger.Info("status",
						zap.String("mcp", mcpName),
						zap.String("tool", toolName),
						zap.Bool("enabled", enabled),
					)
				}

				// ，MCP
				hasEnabledTool := false
				for _, enabled := range cfg.ToolEnabled {
					if enabled {
						hasEnabledTool = true
						break
					}
				}

				// MCP，，MCP
				// MCP，status（）
				if !cfg.ExternalMCPEnable && hasEnabledTool {
					cfg.ExternalMCPEnable = true
					h.logger.Info("auto-enable external MCP（）", zap.String("mcp", mcpName))
				}

				h.config.ExternalMCP.Servers[mcpName] = cfg
			}

			// sync update externalMCPMgr config， GetConfigs() returns
			// update uniformly outside loop, avoid repeated calls
			h.externalMCPMgr.LoadConfigs(&h.config.ExternalMCP)

			// MCPstatus（，）
			for mcpName := range externalMCPToolMap {
				cfg := h.config.ExternalMCP.Servers[mcpName]
				// if MCP needs to be enabled, ensure client started
				if cfg.ExternalMCPEnable {
					// start external MCP (if not started)- async execution, avoid blocking
					client, exists := h.externalMCPMgr.GetClient(mcpName)
					if !exists || !client.IsConnected() {
						go func(name string) {
							if err := h.externalMCPMgr.StartClient(name); err != nil {
								h.logger.Warn("failed to start external MCP",
									zap.String("mcp", name),
									zap.Error(err),
								)
							} else {
								h.logger.Info("start external MCP",
									zap.String("mcp", name),
								)
							}
						}(mcpName)
					}
				}
			}
		}
	}

	// save config to file
	if err := h.saveConfig(); err != nil {
		h.logger.Error("failed to save config", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save config: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "config updated"})
}

// ApplyConfig apply config（load）
func (h *ConfigHandler) ApplyConfig(c *gin.Context) {
	// knowledge base（，）
	var needInitKnowledge bool
	var knowledgeInitializer KnowledgeInitializer

	h.mu.RLock()
	needInitKnowledge = h.config.Knowledge.Enabled && h.knowledgeToolRegistrar == nil && h.knowledgeInitializer != nil
	if needInitKnowledge {
		knowledgeInitializer = h.knowledgeInitializer
	}
	h.mu.RUnlock()

	// knowledge base，（）
	if needInitKnowledge {
		h.logger.Info("knowledge base，knowledge base")
		if _, err := knowledgeInitializer(); err != nil {
			h.logger.Error("knowledge base", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "knowledge base: " + err.Error()})
			return
		}
		h.logger.Info("knowledge base，")
	}

	// check if embedding model config changed（，）
	var needReinitKnowledge bool
	var reinitKnowledgeInitializer KnowledgeInitializer
	h.mu.RLock()
	if h.config.Knowledge.Enabled && h.knowledgeInitializer != nil && h.lastEmbeddingConfig != nil {
		// check if embedding model config changed
		currentEmbedding := h.config.Knowledge.Embedding
		if currentEmbedding.Provider != h.lastEmbeddingConfig.Provider ||
			currentEmbedding.Model != h.lastEmbeddingConfig.Model ||
			currentEmbedding.BaseURL != h.lastEmbeddingConfig.BaseURL ||
			currentEmbedding.APIKey != h.lastEmbeddingConfig.APIKey {
			needReinitKnowledge = true
			reinitKnowledgeInitializer = h.knowledgeInitializer
			h.logger.Info("detected embedding model config change，knowledge base",
				zap.String("old_model", h.lastEmbeddingConfig.Model),
				zap.String("new_model", currentEmbedding.Model),
				zap.String("old_base_url", h.lastEmbeddingConfig.BaseURL),
				zap.String("new_base_url", currentEmbedding.BaseURL),
			)
		}
	}
	h.mu.RUnlock()

	// knowledge base（），
	if needReinitKnowledge {
		h.logger.Info("knowledge base（）")
		if _, err := reinitKnowledgeInitializer(); err != nil {
			h.logger.Error("knowledge base", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "knowledge base: " + err.Error()})
			return
		}
		h.logger.Info("knowledge base")
	}

	// now acquire write lock, execute fast operations
	h.mu.Lock()
	defer h.mu.Unlock()

	// knowledge base，record
	if needReinitKnowledge && h.config.Knowledge.Enabled {
		h.lastEmbeddingConfig = &config.EmbeddingConfig{
			Provider: h.config.Knowledge.Embedding.Provider,
			Model:    h.config.Knowledge.Embedding.Model,
			BaseURL:  h.config.Knowledge.Embedding.BaseURL,
			APIKey:   h.config.Knowledge.Embedding.APIKey,
		}
		h.logger.Info("record")
	}

	// re-register tools（status）
	h.logger.Info("re-register tools")

	// clear tools in MCP server
	h.mcpServer.ClearTools()

	// re-register security tools
	h.executor.RegisterTools(h.mcpServer)

	// record（built-in tool, must register）
	if h.vulnerabilityToolRegistrar != nil {
		h.logger.Info("record")
		if err := h.vulnerabilityToolRegistrar(); err != nil {
			h.logger.Error("record", zap.Error(err))
		} else {
			h.logger.Info("record")
		}
	}

	// re-register WebShell tools（built-in tool, must register）
	if h.webshellToolRegistrar != nil {
		h.logger.Info("re-register WebShell tools")
		if err := h.webshellToolRegistrar(); err != nil {
			h.logger.Error("re-register WebShell tools", zap.Error(err))
		} else {
			h.logger.Info("WebShell tools re-registered")
		}
	}

	// re-register Skills tools（built-in tool, must register）
	if h.skillsToolRegistrar != nil {
		h.logger.Info("re-register Skills tools")
		if err := h.skillsToolRegistrar(); err != nil {
			h.logger.Error("re-register Skills tools", zap.Error(err))
		} else {
			h.logger.Info("Skills tools re-registered")
		}
	}

	// knowledge base，knowledge base
	if h.config.Knowledge.Enabled && h.knowledgeToolRegistrar != nil {
		h.logger.Info("knowledge base")
		if err := h.knowledgeToolRegistrar(); err != nil {
			h.logger.Error("knowledge base", zap.Error(err))
		} else {
			h.logger.Info("knowledge base")
		}
	}

	// update Agent OpenAI config
	if h.agent != nil {
		h.agent.UpdateConfig(&h.config.OpenAI)
		h.agent.UpdateMaxIterations(h.config.Agent.MaxIterations)
		h.logger.Info("Agent config updated")
	}

	// update AttackChainHandler OpenAI config
	if h.attackChainHandler != nil {
		h.attackChainHandler.UpdateConfig(&h.config.OpenAI)
		h.logger.Info("AttackChainHandler config updated")
	}

	// retriever（knowledge base）
	if h.config.Knowledge.Enabled && h.retrieverUpdater != nil {
		retrievalConfig := &knowledge.RetrievalConfig{
			TopK:                h.config.Knowledge.Retrieval.TopK,
			SimilarityThreshold: h.config.Knowledge.Retrieval.SimilarityThreshold,
			HybridWeight:        h.config.Knowledge.Retrieval.HybridWeight,
		}
		h.retrieverUpdater.UpdateConfig(retrievalConfig)
		h.logger.Info("retrieverconfig updated",
			zap.Int("top_k", retrievalConfig.TopK),
			zap.Float64("similarity_threshold", retrievalConfig.SimilarityThreshold),
			zap.Float64("hybrid_weight", retrievalConfig.HybridWeight),
		)
	}

	// record（knowledge base）
	if h.config.Knowledge.Enabled {
		h.lastEmbeddingConfig = &config.EmbeddingConfig{
			Provider: h.config.Knowledge.Embedding.Provider,
			Model:    h.config.Knowledge.Embedding.Model,
			BaseURL:  h.config.Knowledge.Embedding.BaseURL,
			APIKey:   h.config.Knowledge.Embedding.APIKey,
		}
	}

	// restart Telegram bot to apply config changes immediately (no service restart needed)
	if h.robotRestarter != nil {
		h.robotRestarter.RestartRobotConnections()
		h.logger.Info("triggered robot connection restart (Telegram)")
	}

	h.logger.Info("config applied",
		zap.Int("tools_count", len(h.config.Security.Tools)),
	)

	c.JSON(http.StatusOK, gin.H{
		"message":     "config applied",
		"tools_count": len(h.config.Security.Tools),
	})
}

// saveConfig save config to file
func (h *ConfigHandler) saveConfig() error {
	// 
	data, err := os.ReadFile(h.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	if err := os.WriteFile(h.configPath+".backup", data, 0644); err != nil {
		h.logger.Warn("failed to create config backup", zap.Error(err))
	}

	root, err := loadYAMLDocument(h.configPath)
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}

	updateAgentConfig(root, h.config.Agent.MaxIterations)
	updateProxyConfig(root, h.config.Agent.Proxy)
	updateMCPConfig(root, h.config.MCP)
	updateOpenAIConfig(root, h.config.OpenAI)
	updateFOFAConfig(root, h.config.FOFA)
	updateKnowledgeConfig(root, h.config.Knowledge)
	updateRobotsConfig(root, h.config.Robots)
	updateMultiAgentConfig(root, h.config.MultiAgent)
	// update external MCP config（external_mcp.go，）
	// read original config for backward compatibility
	originalConfigs := make(map[string]map[string]bool)
	externalMCPNode := findMapValue(root, "external_mcp")
	if externalMCPNode != nil && externalMCPNode.Kind == yaml.MappingNode {
		serversNode := findMapValue(externalMCPNode, "servers")
		if serversNode != nil && serversNode.Kind == yaml.MappingNode {
			for i := 0; i < len(serversNode.Content); i += 2 {
				if i+1 >= len(serversNode.Content) {
					break
				}
				nameNode := serversNode.Content[i]
				serverNode := serversNode.Content[i+1]
				if nameNode.Kind == yaml.ScalarNode && serverNode.Kind == yaml.MappingNode {
					serverName := nameNode.Value
					originalConfigs[serverName] = make(map[string]bool)
					if enabledVal := findBoolInMap(serverNode, "enabled"); enabledVal != nil {
						originalConfigs[serverName]["enabled"] = *enabledVal
					}
					if disabledVal := findBoolInMap(serverNode, "disabled"); disabledVal != nil {
						originalConfigs[serverName]["disabled"] = *disabledVal
					}
				}
			}
		}
	}
	updateExternalMCPConfig(root, h.config.ExternalMCP, originalConfigs)

	if err := writeYAMLDocument(h.configPath, root); err != nil {
		return fmt.Errorf("failed to save config file: %w", err)
	}

	// update tool configenabledstatus
	if h.config.Security.ToolsDir != "" {
		configDir := filepath.Dir(h.configPath)
		toolsDir := h.config.Security.ToolsDir
		if !filepath.IsAbs(toolsDir) {
			toolsDir = filepath.Join(configDir, toolsDir)
		}

		for _, tool := range h.config.Security.Tools {
			toolFile := filepath.Join(toolsDir, tool.Name+".yaml")
			// check if file exists
			if _, err := os.Stat(toolFile); os.IsNotExist(err) {
				// .yml
				toolFile = filepath.Join(toolsDir, tool.Name+".yml")
				if _, err := os.Stat(toolFile); os.IsNotExist(err) {
					h.logger.Warn("tool config file not found", zap.String("tool", tool.Name))
					continue
				}
			}

			toolDoc, err := loadYAMLDocument(toolFile)
			if err != nil {
				h.logger.Warn("parse", zap.String("tool", tool.Name), zap.Error(err))
				continue
			}

			setBoolInMap(toolDoc.Content[0], "enabled", tool.Enabled)

			if err := writeYAMLDocument(toolFile, toolDoc); err != nil {
				h.logger.Warn("failed to save tool config file", zap.String("tool", tool.Name), zap.Error(err))
				continue
			}

			h.logger.Info("update tool config", zap.String("tool", tool.Name), zap.Bool("enabled", tool.Enabled))
		}
	}

	h.logger.Info("config saved", zap.String("path", h.configPath))
	return nil
}

func loadYAMLDocument(path string) (*yaml.Node, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if len(bytes.TrimSpace(data)) == 0 {
		return newEmptyYAMLDocument(), nil
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}

	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return newEmptyYAMLDocument(), nil
	}

	if doc.Content[0].Kind != yaml.MappingNode {
		root := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		doc.Content = []*yaml.Node{root}
	}

	return &doc, nil
}

func newEmptyYAMLDocument() *yaml.Node {
	root := &yaml.Node{
		Kind:    yaml.DocumentNode,
		Content: []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}},
	}
	return root
}

func writeYAMLDocument(path string, doc *yaml.Node) error {
	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(doc); err != nil {
		return err
	}
	if err := encoder.Close(); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0644)
}

func updateAgentConfig(doc *yaml.Node, maxIterations int) {
	root := doc.Content[0]
	agentNode := ensureMap(root, "agent")
	setIntInMap(agentNode, "max_iterations", maxIterations)
}

func updateProxyConfig(doc *yaml.Node, cfg config.ProxyConfig) {
	root := doc.Content[0]
	agentNode := ensureMap(root, "agent")
	proxyNode := ensureMap(agentNode, "proxy")
	setBoolInMap(proxyNode, "enabled", cfg.Enabled)
	setStringInMap(proxyNode, "type", cfg.Type)
	setStringInMap(proxyNode, "host", cfg.Host)
	setIntInMap(proxyNode, "port", cfg.Port)
	setStringInMap(proxyNode, "username", cfg.Username)
	setStringInMap(proxyNode, "password", cfg.Password)
	setStringInMap(proxyNode, "no_proxy", cfg.NoProxy)
	setBoolInMap(proxyNode, "proxychains", cfg.ProxyChains)
	setBoolInMap(proxyNode, "dns_proxy", cfg.DNSProxy)
	setBoolInMap(proxyNode, "tor_auto_start", cfg.TorAutoStart)
	setBoolInMap(proxyNode, "health_check", cfg.HealthCheck)
}

func updateMCPConfig(doc *yaml.Node, cfg config.MCPConfig) {
	root := doc.Content[0]
	mcpNode := ensureMap(root, "mcp")
	setBoolInMap(mcpNode, "enabled", cfg.Enabled)
	setStringInMap(mcpNode, "host", cfg.Host)
	setIntInMap(mcpNode, "port", cfg.Port)
}

func updateOpenAIConfig(doc *yaml.Node, cfg config.OpenAIConfig) {
	root := doc.Content[0]
	openaiNode := ensureMap(root, "openai")
	if cfg.Provider != "" {
		setStringInMap(openaiNode, "provider", cfg.Provider)
	}
	setStringInMap(openaiNode, "api_key", cfg.APIKey)
	setStringInMap(openaiNode, "base_url", cfg.BaseURL)
	setStringInMap(openaiNode, "model", cfg.Model)
	setStringInMap(openaiNode, "tool_model", cfg.ToolModel)
	setStringInMap(openaiNode, "tool_base_url", cfg.ToolBaseURL)
	setStringInMap(openaiNode, "tool_api_key", cfg.ToolAPIKey)
	setStringInMap(openaiNode, "summary_model", cfg.SummaryModel)
	setStringInMap(openaiNode, "summary_base_url", cfg.SummaryBaseURL)
	setStringInMap(openaiNode, "summary_api_key", cfg.SummaryAPIKey)
}

func updateFOFAConfig(doc *yaml.Node, cfg config.FofaConfig) {
	root := doc.Content[0]
	fofaNode := ensureMap(root, "fofa")
	setStringInMap(fofaNode, "base_url", cfg.BaseURL)
	setStringInMap(fofaNode, "email", cfg.Email)
	setStringInMap(fofaNode, "api_key", cfg.APIKey)
}

func updateKnowledgeConfig(doc *yaml.Node, cfg config.KnowledgeConfig) {
	root := doc.Content[0]
	knowledgeNode := ensureMap(root, "knowledge")
	setBoolInMap(knowledgeNode, "enabled", cfg.Enabled)
	setStringInMap(knowledgeNode, "base_path", cfg.BasePath)

	// update embedding config
	embeddingNode := ensureMap(knowledgeNode, "embedding")
	setStringInMap(embeddingNode, "provider", cfg.Embedding.Provider)
	setStringInMap(embeddingNode, "model", cfg.Embedding.Model)
	if cfg.Embedding.BaseURL != "" {
		setStringInMap(embeddingNode, "base_url", cfg.Embedding.BaseURL)
	}
	if cfg.Embedding.APIKey != "" {
		setStringInMap(embeddingNode, "api_key", cfg.Embedding.APIKey)
	}

	// update retrieval config
	retrievalNode := ensureMap(knowledgeNode, "retrieval")
	setIntInMap(retrievalNode, "top_k", cfg.Retrieval.TopK)
	setFloatInMap(retrievalNode, "similarity_threshold", cfg.Retrieval.SimilarityThreshold)
	setFloatInMap(retrievalNode, "hybrid_weight", cfg.Retrieval.HybridWeight)

	// update indexing config
	indexingNode := ensureMap(knowledgeNode, "indexing")
	setIntInMap(indexingNode, "chunk_size", cfg.Indexing.ChunkSize)
	setIntInMap(indexingNode, "chunk_overlap", cfg.Indexing.ChunkOverlap)
	setIntInMap(indexingNode, "max_chunks_per_item", cfg.Indexing.MaxChunksPerItem)
	setIntInMap(indexingNode, "max_rpm", cfg.Indexing.MaxRPM)
	setIntInMap(indexingNode, "rate_limit_delay_ms", cfg.Indexing.RateLimitDelayMs)
	setIntInMap(indexingNode, "max_retries", cfg.Indexing.MaxRetries)
	setIntInMap(indexingNode, "retry_delay_ms", cfg.Indexing.RetryDelayMs)
}

func updateRobotsConfig(doc *yaml.Node, cfg config.RobotsConfig) {
	root := doc.Content[0]
	robotsNode := ensureMap(root, "robots")

	tgNode := ensureMap(robotsNode, "telegram")
	setBoolInMap(tgNode, "enabled", cfg.Telegram.Enabled)
	setStringInMap(tgNode, "bot_token", cfg.Telegram.BotToken)
	// allowed_user_ids is a sequence -- handled by yaml marshal if needed
}

func updateMultiAgentConfig(doc *yaml.Node, cfg config.MultiAgentConfig) {
	root := doc.Content[0]
	maNode := ensureMap(root, "multi_agent")
	setBoolInMap(maNode, "enabled", cfg.Enabled)
	setStringInMap(maNode, "default_mode", cfg.DefaultMode)
	setBoolInMap(maNode, "robot_use_multi_agent", cfg.RobotUseMultiAgent)
	setBoolInMap(maNode, "batch_use_multi_agent", cfg.BatchUseMultiAgent)
}

func ensureMap(parent *yaml.Node, path ...string) *yaml.Node {
	current := parent
	for _, key := range path {
		value := findMapValue(current, key)
		if value == nil {
			keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}
			mapNode := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
			current.Content = append(current.Content, keyNode, mapNode)
			value = mapNode
		}

		if value.Kind != yaml.MappingNode {
			value.Kind = yaml.MappingNode
			value.Tag = "!!map"
			value.Style = 0
			value.Content = nil
		}

		current = value
	}

	return current
}

func findMapValue(mapNode *yaml.Node, key string) *yaml.Node {
	if mapNode == nil || mapNode.Kind != yaml.MappingNode {
		return nil
	}

	for i := 0; i < len(mapNode.Content); i += 2 {
		if mapNode.Content[i].Value == key {
			return mapNode.Content[i+1]
		}
	}
	return nil
}

func ensureKeyValue(mapNode *yaml.Node, key string) (*yaml.Node, *yaml.Node) {
	if mapNode == nil || mapNode.Kind != yaml.MappingNode {
		return nil, nil
	}

	for i := 0; i < len(mapNode.Content); i += 2 {
		if mapNode.Content[i].Value == key {
			return mapNode.Content[i], mapNode.Content[i+1]
		}
	}

	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}
	valueNode := &yaml.Node{}
	mapNode.Content = append(mapNode.Content, keyNode, valueNode)
	return keyNode, valueNode
}

func setStringInMap(mapNode *yaml.Node, key, value string) {
	_, valueNode := ensureKeyValue(mapNode, key)
	valueNode.Kind = yaml.ScalarNode
	valueNode.Tag = "!!str"
	valueNode.Style = 0
	valueNode.Value = value
}

func setIntInMap(mapNode *yaml.Node, key string, value int) {
	_, valueNode := ensureKeyValue(mapNode, key)
	valueNode.Kind = yaml.ScalarNode
	valueNode.Tag = "!!int"
	valueNode.Style = 0
	valueNode.Value = fmt.Sprintf("%d", value)
}

func findBoolInMap(mapNode *yaml.Node, key string) *bool {
	if mapNode == nil || mapNode.Kind != yaml.MappingNode {
		return nil
	}

	for i := 0; i < len(mapNode.Content); i += 2 {
		if i+1 >= len(mapNode.Content) {
			break
		}
		keyNode := mapNode.Content[i]
		valueNode := mapNode.Content[i+1]

		if keyNode.Kind == yaml.ScalarNode && keyNode.Value == key {
			if valueNode.Kind == yaml.ScalarNode {
				if valueNode.Value == "true" {
					result := true
					return &result
				} else if valueNode.Value == "false" {
					result := false
					return &result
				}
			}
			return nil
		}
	}
	return nil
}

func setBoolInMap(mapNode *yaml.Node, key string, value bool) {
	_, valueNode := ensureKeyValue(mapNode, key)
	valueNode.Kind = yaml.ScalarNode
	valueNode.Tag = "!!bool"
	valueNode.Style = 0
	if value {
		valueNode.Value = "true"
	} else {
		valueNode.Value = "false"
	}
}

func setFloatInMap(mapNode *yaml.Node, key string, value float64) {
	_, valueNode := ensureKeyValue(mapNode, key)
	valueNode.Kind = yaml.ScalarNode
	valueNode.Tag = "!!float"
	valueNode.Style = 0
	// for values between 0.0 and 1.0（hybrid_weight），use %.1f to ensure 0.0 is explicitly serialized as "0.0"
	// for other values, use %g for auto format selection
	if value >= 0.0 && value <= 1.0 {
		valueNode.Value = fmt.Sprintf("%.1f", value)
	} else {
		valueNode.Value = fmt.Sprintf("%g", value)
	}
}

// getExternalMCPTools get external MCP tool list (public method)
// returns ToolConfigInfo list，statusdescription
func (h *ConfigHandler) getExternalMCPTools(ctx context.Context) []ToolConfigInfo {
	var result []ToolConfigInfo

	if h.externalMCPMgr == nil {
		return result
	}

	// use shorter timeout（5），load
	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	externalTools, err := h.externalMCPMgr.GetAllTools(timeoutCtx)
	if err != nil {
		// record，continuereturns（）
		h.logger.Warn("failed to get external MCP tools（），returns",
			zap.Error(err),
			zap.String("hint", "MCP，status"),
		)
	}

	// got tools（error），continue
	if len(externalTools) == 0 {
		return result
	}

	externalMCPConfigs := h.externalMCPMgr.GetConfigs()

	for _, externalTool := range externalTools {
		// parse：mcpName::toolName
		mcpName, actualToolName := h.parseExternalToolName(externalTool.Name)
		if mcpName == "" || actualToolName == "" {
			continue // skipincorrectly formatted tool
		}

		// status
		enabled := h.calculateExternalToolEnabled(mcpName, actualToolName, externalMCPConfigs)

		// process description
		description := h.pickToolDescription(externalTool.ShortDescription, externalTool.Description)

		result = append(result, ToolConfigInfo{
			Name:        actualToolName,
			Description: description,
			Enabled:     enabled,
			IsExternal:  true,
			ExternalMCP: mcpName,
		})
	}

	return result
}

// parseExternalToolName parse（format：mcpName::toolName）
func (h *ConfigHandler) parseExternalToolName(fullName string) (mcpName, toolName string) {
	idx := strings.Index(fullName, "::")
	if idx > 0 {
		return fullName[:idx], fullName[idx+2:]
	}
	return "", ""
}

// calculateExternalToolEnabled status
func (h *ConfigHandler) calculateExternalToolEnabled(mcpName, toolName string, configs map[string]config.ExternalMCPServerConfig) bool {
	cfg, exists := configs[mcpName]
	if !exists {
		return false
	}

	// first check if external MCP is enabled
	if !cfg.ExternalMCPEnable && !(cfg.Enabled && !cfg.Disabled) {
		return false // MCP not enabled, all tools disabled
	}

	// MCP，status
	// ToolEnabled，default（）
	if cfg.ToolEnabled == nil {
		// status，default
	} else if toolEnabled, exists := cfg.ToolEnabled[toolName]; exists {
		// status
		if !toolEnabled {
			return false
		}
	}
	// tool not in config, default to enabled

	// finally check if external MCP connected
	client, exists := h.externalMCPMgr.GetClient(mcpName)
	if !exists || !client.IsConnected() {
		return false // treat as disabled when not connected
	}

	return true
}

// pickToolDescription select based on security.tool_description_mode short or full description with length limit
func (h *ConfigHandler) pickToolDescription(shortDesc, fullDesc string) string {
	useFull := strings.TrimSpace(strings.ToLower(h.config.Security.ToolDescriptionMode)) == "full"
	description := shortDesc
	if useFull {
		description = fullDesc
	} else if description == "" {
		description = fullDesc
	}
	if len(description) > 10000 {
		description = description[:10000] + "..."
	}
	return description
}

// --- API health check / model discovery / rate-limit introspection ---

// TestAPIRequest is the request body for POST /api/config/test-api
type TestAPIRequest struct {
	Provider string `json:"provider"` // "anthropic" or "openai"
	BaseURL  string `json:"base_url"`
	APIKey   string `json:"api_key"`
}

// ModelInfo describes a single model returned by the test endpoint.
type ModelInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// RateLimitInfo holds rate-limit numbers extracted from provider response headers.
type RateLimitInfo struct {
	RequestsPerMinute     int `json:"requests_per_minute,omitempty"`
	InputTokensPerMinute  int `json:"input_tokens_per_minute,omitempty"`
	OutputTokensPerMinute int `json:"output_tokens_per_minute,omitempty"`
}

// TestAPIResponse is the JSON returned by TestAPIEndpoint.
type TestAPIResponse struct {
	Status            string        `json:"status"`
	Provider          string        `json:"provider,omitempty"`
	Models            []ModelInfo   `json:"models,omitempty"`
	RateLimits        *RateLimitInfo `json:"rate_limits,omitempty"`
	RecommendedDelay  int           `json:"recommended_delay_ms"`
	Error             string        `json:"error,omitempty"`
}

// TestAPIEndpoint handles POST /api/config/test-api.
// It probes the given provider API, returns available models and rate limits.
func (h *ConfigHandler) TestAPIEndpoint(c *gin.Context) {
	var req TestAPIRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, TestAPIResponse{Status: "error", Error: "Invalid request body"})
		return
	}

	if req.APIKey == "" {
		c.JSON(http.StatusBadRequest, TestAPIResponse{Status: "error", Error: "API key is required"})
		return
	}

	// Auto-detect provider from key prefix when not explicitly set
	provider := strings.ToLower(strings.TrimSpace(req.Provider))
	if provider == "" {
		if strings.HasPrefix(req.APIKey, "sk-ant-") {
			provider = "anthropic"
		} else {
			provider = "openai"
		}
	}

	httpClient := &http.Client{Timeout: 15 * time.Second}

	switch provider {
	case "anthropic":
		h.testAnthropicAPI(c, httpClient, req)
	default:
		h.testOpenAIAPI(c, httpClient, req)
	}
}

// testAnthropicAPI probes the Anthropic Messages API.
func (h *ConfigHandler) testAnthropicAPI(c *gin.Context, client *http.Client, req TestAPIRequest) {
	baseURL := strings.TrimSpace(req.BaseURL)
	if baseURL == "" {
		baseURL = "https://api.anthropic.com/v1"
	}
	baseURL = strings.TrimRight(baseURL, "/")

	// Minimal health-check request (1 token from Haiku — cheapest).
	body := []byte(`{"model":"claude-haiku-4-5-20251001","max_tokens":1,"messages":[{"role":"user","content":"hi"}]}`)

	httpReq, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, baseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		c.JSON(http.StatusOK, TestAPIResponse{Status: "error", Error: "Failed to build request: " + err.Error()})
		return
	}
	httpReq.Header.Set("x-api-key", req.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(httpReq)
	if err != nil {
		c.JSON(http.StatusOK, classifyHTTPError(err))
		return
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body) // drain body

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		c.JSON(http.StatusOK, TestAPIResponse{Status: "error", Error: fmt.Sprintf("Invalid API key (%d %s)", resp.StatusCode, http.StatusText(resp.StatusCode))})
		return
	}
	if resp.StatusCode >= 400 {
		c.JSON(http.StatusOK, TestAPIResponse{Status: "error", Error: fmt.Sprintf("API returned %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))})
		return
	}

	// Parse rate-limit headers
	rl := parseAnthropicRateLimits(resp.Header)

	// Hardcoded model list (Anthropic has no /models endpoint)
	models := []ModelInfo{
		{ID: "claude-opus-4-20250514", Name: "Claude Opus 4 (most capable)"},
		{ID: "claude-sonnet-4-20250514", Name: "Claude Sonnet 4 (balanced)"},
		{ID: "claude-haiku-4-5-20251001", Name: "Claude Haiku 4.5 (fast, cheap)"},
	}

	delay := recommendedDelay(rl.InputTokensPerMinute)

	c.JSON(http.StatusOK, TestAPIResponse{
		Status:           "ok",
		Provider:         "anthropic",
		Models:           models,
		RateLimits:       &rl,
		RecommendedDelay: delay,
	})
}

// testOpenAIAPI probes the OpenAI-compatible API.
func (h *ConfigHandler) testOpenAIAPI(c *gin.Context, client *http.Client, req TestAPIRequest) {
	baseURL := strings.TrimSpace(req.BaseURL)
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	baseURL = strings.TrimRight(baseURL, "/")

	// Step 1: fetch models from GET /models
	models, modelsErr := h.fetchOpenAIModels(c.Request.Context(), client, baseURL, req.APIKey)

	// Step 2: send a minimal chat completion to get rate-limit headers
	rl, chatErr := h.probeOpenAIChat(c.Request.Context(), client, baseURL, req.APIKey)

	// If both failed, report the most useful error
	if modelsErr != nil && chatErr != nil {
		// Prefer auth error from either call
		for _, e := range []error{modelsErr, chatErr} {
			resp := classifyHTTPError(e)
			if resp.Status == "error" {
				c.JSON(http.StatusOK, resp)
				return
			}
		}
		c.JSON(http.StatusOK, TestAPIResponse{Status: "error", Error: chatErr.Error()})
		return
	}

	delay := recommendedDelay(rl.InputTokensPerMinute)

	var rlPtr *RateLimitInfo
	if rl.RequestsPerMinute > 0 || rl.InputTokensPerMinute > 0 || rl.OutputTokensPerMinute > 0 {
		rlPtr = &rl
	}

	c.JSON(http.StatusOK, TestAPIResponse{
		Status:           "ok",
		Provider:         "openai",
		Models:           models,
		RateLimits:       rlPtr,
		RecommendedDelay: delay,
	})
}

// fetchOpenAIModels calls GET /v1/models and filters to chat-capable models.
func (h *ConfigHandler) fetchOpenAIModels(ctx context.Context, client *http.Client, baseURL, apiKey string) ([]ModelInfo, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/models", nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return nil, fmt.Errorf("invalid API key (%d %s)", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result struct {
		Data []struct {
			ID      string `json:"id"`
			OwnedBy string `json:"owned_by"`
		} `json:"data"`
	}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to parse models response: %w", err)
	}

	// Filter out non-chat models
	excludePrefixes := []string{"embedding", "text-embedding", "whisper", "dall-e", "tts", "davinci", "babbage", "ada", "curie"}
	var models []ModelInfo
	for _, m := range result.Data {
		skip := false
		lower := strings.ToLower(m.ID)
		for _, prefix := range excludePrefixes {
			if strings.HasPrefix(lower, prefix) {
				skip = true
				break
			}
		}
		// Also exclude moderation, search models
		if strings.Contains(lower, "moderation") || strings.Contains(lower, "search") {
			skip = true
		}
		if !skip {
			models = append(models, ModelInfo{ID: m.ID, Name: m.ID})
		}
	}

	return models, nil
}

// probeOpenAIChat sends a minimal chat completion to extract rate-limit headers.
func (h *ConfigHandler) probeOpenAIChat(ctx context.Context, client *http.Client, baseURL, apiKey string) (RateLimitInfo, error) {
	body := []byte(`{"model":"gpt-4","max_tokens":1,"messages":[{"role":"user","content":"hi"}]}`)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return RateLimitInfo{}, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(httpReq)
	if err != nil {
		return RateLimitInfo{}, err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return RateLimitInfo{}, fmt.Errorf("invalid API key (%d %s)", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	return parseOpenAIRateLimits(resp.Header), nil
}

// parseAnthropicRateLimits extracts rate-limit info from Anthropic response headers.
func parseAnthropicRateLimits(h http.Header) RateLimitInfo {
	return RateLimitInfo{
		RequestsPerMinute:     headerInt(h, "anthropic-ratelimit-requests-limit"),
		InputTokensPerMinute:  headerInt(h, "anthropic-ratelimit-input-tokens-limit"),
		OutputTokensPerMinute: headerInt(h, "anthropic-ratelimit-output-tokens-limit"),
	}
}

// parseOpenAIRateLimits extracts rate-limit info from OpenAI response headers.
func parseOpenAIRateLimits(h http.Header) RateLimitInfo {
	return RateLimitInfo{
		RequestsPerMinute:    headerInt(h, "x-ratelimit-limit-requests"),
		InputTokensPerMinute: headerInt(h, "x-ratelimit-limit-tokens"),
	}
}

// headerInt parses an integer from a response header, returning 0 on failure.
func headerInt(h http.Header, key string) int {
	v := h.Get(key)
	if v == "" {
		return 0
	}
	n, _ := strconv.Atoi(v)
	return n
}

// recommendedDelay calculates a recommended inter-request delay from input token rate.
func recommendedDelay(inputTokensPerMinute int) int {
	if inputTokensPerMinute <= 0 {
		return 0
	}
	if inputTokensPerMinute < 50000 {
		return 2000
	}
	if inputTokensPerMinute < 100000 {
		return 500
	}
	return 0
}

// classifyHTTPError converts a net/http error into a user-friendly TestAPIResponse.
func classifyHTTPError(err error) TestAPIResponse {
	if err == nil {
		return TestAPIResponse{Status: "ok"}
	}
	// Check for timeout
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return TestAPIResponse{Status: "error", Error: "API endpoint timed out"}
	}
	// Check for DNS / connection errors
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		var dnsErr *net.DNSError
		if errors.As(urlErr.Err, &dnsErr) {
			return TestAPIResponse{Status: "error", Error: "Cannot reach API endpoint. Check your network/DNS."}
		}
		var opErr *net.OpError
		if errors.As(urlErr.Err, &opErr) {
			return TestAPIResponse{Status: "error", Error: "Cannot reach API endpoint. Check your network/DNS."}
		}
	}
	return TestAPIResponse{Status: "error", Error: err.Error()}
}

// ModelHealthResponse is the JSON returned by the model health check endpoint.
type ModelHealthResponse struct {
	Status    string `json:"status"`               // "ok", "error", "unconfigured"
	Provider  string `json:"provider,omitempty"`
	Model     string `json:"model,omitempty"`
	BaseURL   string `json:"base_url,omitempty"`
	Message   string `json:"message,omitempty"`
	LatencyMs int64  `json:"latency_ms,omitempty"`
	ErrorCode string `json:"error_code,omitempty"` // "auth_failed", "dns_error", "timeout", "unconfigured"
}

// ModelHealthCheck tests the configured AI model endpoint and returns status.
// Returns: {status: "ok"|"error"|"unconfigured", provider, model, message, details}
func (h *ConfigHandler) ModelHealthCheck(c *gin.Context) {
	h.mu.RLock()
	cfg := h.config
	h.mu.RUnlock()

	if cfg == nil {
		c.JSON(http.StatusOK, ModelHealthResponse{
			Status:    "error",
			Message:   "Configuration not loaded",
			ErrorCode: "unconfigured",
		})
		return
	}

	provider := strings.ToLower(strings.TrimSpace(cfg.OpenAI.Provider))
	apiKey := strings.TrimSpace(cfg.OpenAI.APIKey)
	baseURL := strings.TrimSpace(cfg.OpenAI.BaseURL)
	model := strings.TrimSpace(cfg.OpenAI.Model)

	// Auto-detect provider from key prefix
	if provider == "" {
		if strings.HasPrefix(apiKey, "sk-ant-") {
			provider = "anthropic"
		} else {
			provider = "openai"
		}
	}

	if provider == "anthropic" && baseURL == "" {
		baseURL = "https://api.anthropic.com/v1"
	} else if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	// Check for unconfigured API key
	if apiKey == "" || apiKey == "YOUR_API_KEY_HERE" || apiKey == "sk-xxxxxx" || apiKey == "sk-xxx" {
		c.JSON(http.StatusOK, ModelHealthResponse{
			Status:    "unconfigured",
			Provider:  provider,
			Model:     model,
			BaseURL:   baseURL,
			Message:   "API key not set. Go to Settings to configure.",
			ErrorCode: "unconfigured",
		})
		return
	}

	httpClient := &http.Client{Timeout: 10 * time.Second}
	start := time.Now()

	var resp *http.Response
	var reqErr error

	switch provider {
	case "anthropic":
		body := []byte(fmt.Sprintf(`{"model":%q,"max_tokens":1,"messages":[{"role":"user","content":"ping"}]}`, model))
		endpoint := strings.TrimRight(baseURL, "/") + "/messages"
		var httpReq *http.Request
		httpReq, reqErr = http.NewRequestWithContext(c.Request.Context(), http.MethodPost, endpoint, bytes.NewReader(body))
		if reqErr == nil {
			httpReq.Header.Set("x-api-key", apiKey)
			httpReq.Header.Set("anthropic-version", "2023-06-01")
			httpReq.Header.Set("Content-Type", "application/json")
			resp, reqErr = httpClient.Do(httpReq)
		}
	default:
		body := []byte(fmt.Sprintf(`{"model":%q,"max_tokens":1,"messages":[{"role":"user","content":"ping"}]}`, model))
		endpoint := strings.TrimRight(baseURL, "/") + "/chat/completions"
		var httpReq *http.Request
		httpReq, reqErr = http.NewRequestWithContext(c.Request.Context(), http.MethodPost, endpoint, bytes.NewReader(body))
		if reqErr == nil {
			httpReq.Header.Set("Content-Type", "application/json")
			httpReq.Header.Set("Authorization", "Bearer "+apiKey)
			resp, reqErr = httpClient.Do(httpReq)
		}
	}

	latencyMs := time.Since(start).Milliseconds()

	if reqErr != nil {
		result := classifyModelHealthError(reqErr)
		result.Provider = provider
		result.Model = model
		result.BaseURL = baseURL
		result.LatencyMs = latencyMs
		c.JSON(http.StatusOK, result)
		return
	}
	defer func() {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()

	switch {
	case resp.StatusCode == 401 || resp.StatusCode == 403:
		c.JSON(http.StatusOK, ModelHealthResponse{
			Status:    "error",
			Provider:  provider,
			Model:     model,
			BaseURL:   baseURL,
			Message:   fmt.Sprintf("Authentication failed (%d %s)", resp.StatusCode, http.StatusText(resp.StatusCode)),
			LatencyMs: latencyMs,
			ErrorCode: "auth_failed",
		})
	case resp.StatusCode >= 400:
		c.JSON(http.StatusOK, ModelHealthResponse{
			Status:    "error",
			Provider:  provider,
			Model:     model,
			BaseURL:   baseURL,
			Message:   fmt.Sprintf("API returned %d %s", resp.StatusCode, http.StatusText(resp.StatusCode)),
			LatencyMs: latencyMs,
			ErrorCode: "api_error",
		})
	default:
		c.JSON(http.StatusOK, ModelHealthResponse{
			Status:    "ok",
			Provider:  provider,
			Model:     model,
			BaseURL:   baseURL,
			Message:   "Model endpoint reachable",
			LatencyMs: latencyMs,
		})
	}
}

// classifyModelHealthError converts a net/http error into a ModelHealthResponse.
func classifyModelHealthError(err error) ModelHealthResponse {
	if err == nil {
		return ModelHealthResponse{Status: "ok", Message: "Model endpoint reachable"}
	}
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return ModelHealthResponse{
			Status:    "error",
			Message:   "Request timed out (10s)",
			ErrorCode: "timeout",
		}
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		var dnsErr *net.DNSError
		if errors.As(urlErr.Err, &dnsErr) {
			return ModelHealthResponse{
				Status:    "error",
				Message:   "DNS resolution failed. Check your network or base URL.",
				ErrorCode: "dns_error",
			}
		}
		var opErr *net.OpError
		if errors.As(urlErr.Err, &opErr) {
			return ModelHealthResponse{
				Status:    "error",
				Message:   "Cannot reach endpoint. Check your network or base URL.",
				ErrorCode: "network_error",
			}
		}
	}
	return ModelHealthResponse{
		Status:    "error",
		Message:   err.Error(),
		ErrorCode: "network_error",
	}
}
