package handler

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/knowledge"
	"cyberstrike-ai/internal/mcp"
	"cyberstrike-ai/internal/openai"
	"cyberstrike-ai/internal/security"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// KnowledgeToolRegistrar knowledge base tool registrar interface
type KnowledgeToolRegistrar func() error

// VulnerabilityToolRegistrar vulnerability tool registrar interface
type VulnerabilityToolRegistrar func() error

// SkillsToolRegistrar Skills tool registrar interface
type SkillsToolRegistrar func() error

// BuiltinToolRegistrar generic registrar for builtin tools (memory, time, etc.)
type BuiltinToolRegistrar func() error

// RetrieverUpdater retriever updater interface
type RetrieverUpdater interface {
	UpdateConfig(config *knowledge.RetrievalConfig)
}

// KnowledgeInitializer knowledge base initializer interface
type KnowledgeInitializer func() (*KnowledgeHandler, error)

// AppUpdater App updater interface (for updating knowledge base components in App)
type AppUpdater interface {
	UpdateKnowledgeComponents(handler *KnowledgeHandler, manager interface{}, retriever interface{}, indexer interface{})
}

// RobotRestarter robot connection restarter (for restarting Lark long connections after config is applied)
type RobotRestarter interface {
	RestartRobotConnections()
}

// ConfigHandler configuration handler
type ConfigHandler struct {
	configPath                 string
	config                     *config.Config
	mcpServer                  *mcp.Server
	executor                   *security.Executor
	agent                      AgentUpdater               // Agent interface for updating Agent config
	attackChainHandler         AttackChainUpdater         // attack chain handler interface for updating config
	externalMCPMgr             *mcp.ExternalMCPManager    // external MCP manager
	knowledgeToolRegistrar     KnowledgeToolRegistrar     // knowledge base tool registrar (optional)
	vulnerabilityToolRegistrar VulnerabilityToolRegistrar // vulnerability tool registrar (optional)
	skillsToolRegistrar        SkillsToolRegistrar        // Skills tool registrar (optional)
	memoryToolRegistrar        BuiltinToolRegistrar       // memory tool registrar (optional)
	timeToolRegistrar          BuiltinToolRegistrar       // time tool registrar (optional)
	retrieverUpdater           RetrieverUpdater           // retriever updater (optional)
	knowledgeInitializer       KnowledgeInitializer       // knowledge base initializer (optional)
	appUpdater                 AppUpdater                 // App updater (optional)
	robotRestarter             RobotRestarter             // robot connection restarter (optional), restarts Lark when ApplyConfig is called
	logger                     *zap.Logger
	mu                         sync.RWMutex
	lastEmbeddingConfig        *config.EmbeddingConfig // last embedding model config (for detecting changes)
}

// AttackChainUpdater attack chain handler update interface
type AttackChainUpdater interface {
	UpdateConfig(cfg *config.OpenAIConfig)
}

// AgentUpdater Agent update interface
type AgentUpdater interface {
	UpdateConfig(cfg *config.OpenAIConfig)
	UpdateMaxIterations(maxIterations int)
}

// NewConfigHandler creates a new configuration handler
func NewConfigHandler(configPath string, cfg *config.Config, mcpServer *mcp.Server, executor *security.Executor, agent AgentUpdater, attackChainHandler AttackChainUpdater, externalMCPMgr *mcp.ExternalMCPManager, logger *zap.Logger) *ConfigHandler {
	// Save initial embedding model config (if knowledge base is enabled)
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

// SetKnowledgeToolRegistrar sets the knowledge base tool registrar
func (h *ConfigHandler) SetKnowledgeToolRegistrar(registrar KnowledgeToolRegistrar) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.knowledgeToolRegistrar = registrar
}

// SetVulnerabilityToolRegistrar sets the vulnerability tool registrar
func (h *ConfigHandler) SetVulnerabilityToolRegistrar(registrar VulnerabilityToolRegistrar) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.vulnerabilityToolRegistrar = registrar
}

// SetSkillsToolRegistrar sets the Skills tool registrar
func (h *ConfigHandler) SetSkillsToolRegistrar(registrar SkillsToolRegistrar) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.skillsToolRegistrar = registrar
}

// SetMemoryToolRegistrar sets the persistent memory tool registrar
func (h *ConfigHandler) SetMemoryToolRegistrar(registrar BuiltinToolRegistrar) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.memoryToolRegistrar = registrar
}

// SetTimeToolRegistrar sets the time awareness tool registrar
func (h *ConfigHandler) SetTimeToolRegistrar(registrar BuiltinToolRegistrar) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.timeToolRegistrar = registrar
}

// SetRetrieverUpdater sets the retriever updater
func (h *ConfigHandler) SetRetrieverUpdater(updater RetrieverUpdater) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.retrieverUpdater = updater
}

// SetKnowledgeInitializer sets the knowledge base initializer
func (h *ConfigHandler) SetKnowledgeInitializer(initializer KnowledgeInitializer) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.knowledgeInitializer = initializer
}

// SetAppUpdater sets the App updater
func (h *ConfigHandler) SetAppUpdater(updater AppUpdater) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.appUpdater = updater
}

// SetRobotRestarter sets the robot connection restarter (used to restart Lark long connections when ApplyConfig is called)
func (h *ConfigHandler) SetRobotRestarter(restarter RobotRestarter) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.robotRestarter = restarter
}

// SecuritySettingsResponse exposes the subset of SecurityConfig that is user-configurable.
type SecuritySettingsResponse struct {
	ToolDescriptionMode string `json:"tool_description_mode"`
}

// GetConfigResponse get configuration response
type GetConfigResponse struct {
	OpenAI    config.OpenAIConfig      `json:"openai"`
	FOFA      config.FofaConfig        `json:"fofa"`
	ZoomEye   config.ZoomEyeConfig     `json:"zoomeye"`
	Shodan    config.ShodanConfig      `json:"shodan"`
	Censys    config.CensysConfig      `json:"censys"`
	MCP       config.MCPConfig         `json:"mcp"`
	Tools     []ToolConfigInfo         `json:"tools"`
	Agent     config.AgentConfig       `json:"agent"`
	Knowledge config.KnowledgeConfig   `json:"knowledge"`
	Robots    config.RobotsConfig      `json:"robots,omitempty"`
	Security  SecuritySettingsResponse `json:"security"`
}

// ToolConfigInfo tool configuration info
type ToolConfigInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
	IsExternal  bool   `json:"is_external,omitempty"`  // whether it is an external MCP tool
	ExternalMCP string `json:"external_mcp,omitempty"` // external MCP name (if it is an external tool)
	RoleEnabled *bool  `json:"role_enabled,omitempty"` // whether this tool is enabled in the current role (nil means no role specified or all tools used)
}

// GetConfig gets the current configuration
func (h *ConfigHandler) GetConfig(c *gin.Context) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Get tool list (including internal and external tools)
	// First get tools from config file
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

	// Get all registered tools from MCP server (including directly registered tools, such as knowledge retrieval tools)
	if h.mcpServer != nil {
		mcpTools := h.mcpServer.GetAllTools()
		for _, mcpTool := range mcpTools {
			// Skip tools already in config file (to avoid duplicates)
			if configToolMap[mcpTool.Name] {
				continue
			}
			// Add tools directly registered to the MCP server (such as knowledge retrieval tools)
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
				Enabled:     true, // directly registered tools are enabled by default
				IsExternal:  false,
			})
		}
	}

	// Get external MCP tools
	if h.externalMCPMgr != nil {
		ctx := context.Background()
		externalTools := h.getExternalMCPTools(ctx)
		for _, toolInfo := range externalTools {
			tools = append(tools, toolInfo)
		}
	}

	c.JSON(http.StatusOK, GetConfigResponse{
		OpenAI:    h.config.OpenAI,
		FOFA:      h.config.FOFA,
		ZoomEye:   h.config.ZoomEye,
		Shodan:    h.config.Shodan,
		Censys:    h.config.Censys,
		MCP:       h.config.MCP,
		Tools:     tools,
		Agent:     h.config.Agent,
		Knowledge: h.config.Knowledge,
		Robots:    h.config.Robots,
		Security: SecuritySettingsResponse{
			ToolDescriptionMode: h.config.Security.ToolDescriptionMode,
		},
	})
}

// GetToolsResponse get tools list response (paginated)
type GetToolsResponse struct {
	Tools        []ToolConfigInfo `json:"tools"`
	Total        int              `json:"total"`
	TotalEnabled int              `json:"total_enabled"` // total number of enabled tools
	Page         int              `json:"page"`
	PageSize     int              `json:"page_size"`
	TotalPages   int              `json:"total_pages"`
}

// GetTools gets the tool list (supports pagination and search)
func (h *ConfigHandler) GetTools(c *gin.Context) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Parse pagination parameters
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

	// Parse search parameters
	searchTerm := c.Query("search")
	searchTermLower := ""
	if searchTerm != "" {
		searchTermLower = strings.ToLower(searchTerm)
	}

	// Parse role parameter, for filtering tools and annotating enabled status
	roleName := c.Query("role")
	var roleToolsSet map[string]bool // set of tools configured for the role
	var roleUsesAllTools bool = true // whether the role uses all tools (default role)
	if roleName != "" && roleName != "Default" && h.config.Roles != nil {
		if role, exists := h.config.Roles[roleName]; exists && role.Enabled {
			if len(role.Tools) > 0 {
				// Role has configured a tool list, only use those tools
				roleToolsSet = make(map[string]bool)
				for _, toolKey := range role.Tools {
					roleToolsSet[toolKey] = true
				}
				roleUsesAllTools = false
			}
		}
	}

	// Get all internal tools and apply search filtering
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

		// Annotate tool status based on role config
		if roleName != "" {
			if roleUsesAllTools {
				// Role uses all tools, annotate enabled tools as role_enabled=true
				if tool.Enabled {
					roleEnabled := true
					toolInfo.RoleEnabled = &roleEnabled
				} else {
					roleEnabled := false
					toolInfo.RoleEnabled = &roleEnabled
				}
			} else {
				// Role has configured a tool list, check if tool is in the list
				// Internal tools use tool name as key
				if roleToolsSet[tool.Name] {
					roleEnabled := tool.Enabled // tool must be in role list and enabled itself
					toolInfo.RoleEnabled = &roleEnabled
				} else {
					// Not in role list, mark as false
					roleEnabled := false
					toolInfo.RoleEnabled = &roleEnabled
				}
			}
		}

		// If there is a keyword, apply search filter
		if searchTermLower != "" {
			nameLower := strings.ToLower(toolInfo.Name)
			descLower := strings.ToLower(toolInfo.Description)
			if !strings.Contains(nameLower, searchTermLower) && !strings.Contains(descLower, searchTermLower) {
				continue // no match, skip
			}
		}

		allTools = append(allTools, toolInfo)
	}

	// Get all registered tools from MCP server (including directly registered tools, such as knowledge retrieval tools)
	if h.mcpServer != nil {
		mcpTools := h.mcpServer.GetAllTools()
		for _, mcpTool := range mcpTools {
			// Skip tools already in config file (to avoid duplicates)
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
				Enabled:     true, // directly registered tools are enabled by default
				IsExternal:  false,
			}

			// Annotate tool status based on role config
			if roleName != "" {
				if roleUsesAllTools {
					// Role uses all tools, directly registered tools are enabled by default
					roleEnabled := true
					toolInfo.RoleEnabled = &roleEnabled
				} else {
					// Role has configured a tool list, check if tool is in the list
					// Internal tools use tool name as key
					if roleToolsSet[mcpTool.Name] {
						roleEnabled := true // in role list and tool itself is enabled
						toolInfo.RoleEnabled = &roleEnabled
					} else {
						// Not in role list, mark as false
						roleEnabled := false
						toolInfo.RoleEnabled = &roleEnabled
					}
				}
			}

			// If there is a keyword, apply search filter
			if searchTermLower != "" {
				nameLower := strings.ToLower(toolInfo.Name)
				descLower := strings.ToLower(toolInfo.Description)
				if !strings.Contains(nameLower, searchTermLower) && !strings.Contains(descLower, searchTermLower) {
					continue // no match, skip
				}
			}

			allTools = append(allTools, toolInfo)
		}
	}

	// Get external MCP tools
	if h.externalMCPMgr != nil {
		// Create context for fetching external tools
		ctx := context.Background()
		externalTools := h.getExternalMCPTools(ctx)

		// Apply search filtering and role config
		for _, toolInfo := range externalTools {
			// Search filtering
			if searchTermLower != "" {
				nameLower := strings.ToLower(toolInfo.Name)
				descLower := strings.ToLower(toolInfo.Description)
				if !strings.Contains(nameLower, searchTermLower) && !strings.Contains(descLower, searchTermLower) {
					continue // no match, skip
				}
			}

			// Annotate tool status based on role config
			if roleName != "" {
				if roleUsesAllTools {
					// Role uses all tools, annotate enabled tools as role_enabled=true
					roleEnabled := toolInfo.Enabled
					toolInfo.RoleEnabled = &roleEnabled
				} else {
					// Role has configured a tool list, check if tool is in the list
					// External tools use "mcpName::toolName" format as key
					externalToolKey := fmt.Sprintf("%s::%s", toolInfo.ExternalMCP, toolInfo.Name)
					if roleToolsSet[externalToolKey] {
						roleEnabled := toolInfo.Enabled // tool must be in role list and enabled itself
						toolInfo.RoleEnabled = &roleEnabled
					} else {
						// Not in role list, mark as false
						roleEnabled := false
						toolInfo.RoleEnabled = &roleEnabled
					}
				}
			}

			allTools = append(allTools, toolInfo)
		}
	}

	// If the role has configured a tool list, filter tools (keep only listed tools, but retain others and mark as disabled)
	// Note: here we do not directly filter out tools, but retain all tools, annotating status via role_enabled field
	// This way the frontend can display all tools and annotate which tools are available in the current role

	total := len(allTools)
	// Count enabled tools (enabled tools in the role)
	totalEnabled := 0
	for _, tool := range allTools {
		if tool.RoleEnabled != nil && *tool.RoleEnabled {
			totalEnabled++
		} else if tool.RoleEnabled == nil && tool.Enabled {
			// If no role specified, count all enabled tools
			totalEnabled++
		}
	}

	totalPages := (total + pageSize - 1) / pageSize
	if totalPages == 0 {
		totalPages = 1
	}

	// Calculate pagination range
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

// SecuritySettingsRequest holds the user-configurable subset of SecurityConfig.
type SecuritySettingsRequest struct {
	ToolDescriptionMode string `json:"tool_description_mode"`
}

// UpdateConfigRequest update configuration request
type UpdateConfigRequest struct {
	OpenAI    *config.OpenAIConfig     `json:"openai,omitempty"`
	FOFA      *config.FofaConfig       `json:"fofa,omitempty"`
	ZoomEye   *config.ZoomEyeConfig    `json:"zoomeye,omitempty"`
	Shodan    *config.ShodanConfig     `json:"shodan,omitempty"`
	Censys    *config.CensysConfig     `json:"censys,omitempty"`
	MCP       *config.MCPConfig        `json:"mcp,omitempty"`
	Tools     []ToolEnableStatus       `json:"tools,omitempty"`
	Agent     *config.AgentConfig      `json:"agent,omitempty"`
	Knowledge *config.KnowledgeConfig  `json:"knowledge,omitempty"`
	Robots    *config.RobotsConfig     `json:"robots,omitempty"`
	Security  *SecuritySettingsRequest `json:"security,omitempty"`
}

type DiscoverModelsRequest struct {
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key,omitempty"`
}

// DiscoverModels fetches available models from an OpenAI-compatible endpoint.
func (h *ConfigHandler) DiscoverModels(c *gin.Context) {
	var req DiscoverModelsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request parameters: " + err.Error()})
		return
	}
	baseURL := strings.TrimSpace(req.BaseURL)
	if baseURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "base_url is required"})
		return
	}

	models, err := openai.FetchModels(baseURL, strings.TrimSpace(req.APIKey), nil)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"base_url": baseURL,
		"models":   models,
	})
}

// ToolEnableStatus tool enable status
type ToolEnableStatus struct {
	Name        string `json:"name"`
	Enabled     bool   `json:"enabled"`
	IsExternal  bool   `json:"is_external,omitempty"`  // whether it is an external MCP tool
	ExternalMCP string `json:"external_mcp,omitempty"` // external MCP name (if it is an external tool)
}

// UpdateConfig updates the configuration
func (h *ConfigHandler) UpdateConfig(c *gin.Context) {
	var req UpdateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request parameters: " + err.Error()})
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	// Update OpenAI config
	if req.OpenAI != nil {
		h.config.OpenAI = *req.OpenAI
		h.logger.Info("Updating OpenAI config",
			zap.String("base_url", h.config.OpenAI.BaseURL),
			zap.String("model", h.config.OpenAI.Model),
		)
	}

	// Update FOFA config
	if req.FOFA != nil {
		h.config.FOFA = *req.FOFA
		h.logger.Info("Updating FOFA config", zap.String("email", h.config.FOFA.Email))
	}

	// Update ZoomEye config
	if req.ZoomEye != nil {
		h.config.ZoomEye = *req.ZoomEye
		h.logger.Info("Updating ZoomEye config", zap.Bool("has_key", h.config.ZoomEye.APIKey != ""))
	}

	// Update Shodan config
	if req.Shodan != nil {
		h.config.Shodan = *req.Shodan
		h.logger.Info("Updating Shodan config", zap.Bool("has_key", h.config.Shodan.APIKey != ""))
	}

	// Update Censys config
	if req.Censys != nil {
		h.config.Censys = *req.Censys
		h.logger.Info("Updating Censys config", zap.Bool("has_id", h.config.Censys.APIID != ""))
	}

	// Update MCP config
	if req.MCP != nil {
		h.config.MCP = *req.MCP
		h.logger.Info("Updating MCP config",
			zap.Bool("enabled", h.config.MCP.Enabled),
			zap.String("host", h.config.MCP.Host),
			zap.Int("port", h.config.MCP.Port),
		)
	}

	// Update Agent config
	if req.Agent != nil {
		h.config.Agent = *req.Agent
		h.logger.Info("Updating Agent config",
			zap.Int("max_iterations", h.config.Agent.MaxIterations),
		)
	}

	// Update Knowledge config
	if req.Knowledge != nil {
		// Save old embedding model config (for detecting changes)
		if h.config.Knowledge.Enabled {
			h.lastEmbeddingConfig = &config.EmbeddingConfig{
				Provider: h.config.Knowledge.Embedding.Provider,
				Model:    h.config.Knowledge.Embedding.Model,
				BaseURL:  h.config.Knowledge.Embedding.BaseURL,
				APIKey:   h.config.Knowledge.Embedding.APIKey,
			}
		}
		h.config.Knowledge = *req.Knowledge
		h.logger.Info("Updating Knowledge config",
			zap.Bool("enabled", h.config.Knowledge.Enabled),
			zap.String("base_path", h.config.Knowledge.BasePath),
			zap.String("embedding_model", h.config.Knowledge.Embedding.Model),
			zap.Int("retrieval_top_k", h.config.Knowledge.Retrieval.TopK),
			zap.Float64("similarity_threshold", h.config.Knowledge.Retrieval.SimilarityThreshold),
			zap.Float64("hybrid_weight", h.config.Knowledge.Retrieval.HybridWeight),
		)
	}

	// Always normalize model defaults after partial updates so empty fields
	// automatically inherit default/main model values.
	h.config.ApplyModelDefaults()

	// Update robot config
	if req.Robots != nil {
		h.config.Robots = *req.Robots
		h.logger.Info("Updating robot config",
			zap.Bool("wecom_enabled", h.config.Robots.Wecom.Enabled),
			zap.Bool("lark_enabled", h.config.Robots.Lark.Enabled),
		)
	}

	// Update security config
	if req.Security != nil {
		if req.Security.ToolDescriptionMode == "short" || req.Security.ToolDescriptionMode == "full" {
			h.config.Security.ToolDescriptionMode = req.Security.ToolDescriptionMode
		}
		h.logger.Info("Updating security config",
			zap.String("tool_description_mode", h.config.Security.ToolDescriptionMode),
		)
	}

	// Update tool enable status
	if req.Tools != nil {
		// Separate internal tools and external tools
		internalToolMap := make(map[string]bool)
		// External tool status: MCP name -> tool name -> enable status
		externalMCPToolMap := make(map[string]map[string]bool)

		for _, toolStatus := range req.Tools {
			if toolStatus.IsExternal && toolStatus.ExternalMCP != "" {
				// External tool: save individual tool status
				mcpName := toolStatus.ExternalMCP
				if externalMCPToolMap[mcpName] == nil {
					externalMCPToolMap[mcpName] = make(map[string]bool)
				}
				externalMCPToolMap[mcpName][toolStatus.Name] = toolStatus.Enabled
			} else {
				// Internal tool
				internalToolMap[toolStatus.Name] = toolStatus.Enabled
			}
		}

		// Update internal tool status
		for i := range h.config.Security.Tools {
			if enabled, ok := internalToolMap[h.config.Security.Tools[i].Name]; ok {
				h.config.Security.Tools[i].Enabled = enabled
				h.logger.Info("Updating tool enable status",
					zap.String("tool", h.config.Security.Tools[i].Name),
					zap.Bool("enabled", enabled),
				)
			}
		}

		// Update external MCP tool status
		if h.externalMCPMgr != nil {
			for mcpName, toolStates := range externalMCPToolMap {
				// Update tool enable status in config
				if h.config.ExternalMCP.Servers == nil {
					h.config.ExternalMCP.Servers = make(map[string]config.ExternalMCPServerConfig)
				}
				cfg, exists := h.config.ExternalMCP.Servers[mcpName]
				if !exists {
					h.logger.Warn("External MCP config does not exist", zap.String("mcp", mcpName))
					continue
				}

				// Initialize ToolEnabled map
				if cfg.ToolEnabled == nil {
					cfg.ToolEnabled = make(map[string]bool)
				}

				// Update each tool's enable status
				for toolName, enabled := range toolStates {
					cfg.ToolEnabled[toolName] = enabled
					h.logger.Info("Updating external tool enable status",
						zap.String("mcp", mcpName),
						zap.String("tool", toolName),
						zap.Bool("enabled", enabled),
					)
				}

				// Check if any tool is enabled; if so, enable the MCP
				hasEnabledTool := false
				for _, enabled := range cfg.ToolEnabled {
					if enabled {
						hasEnabledTool = true
						break
					}
				}

				// If MCP was previously disabled but now has a tool enabled, enable the MCP
				// If MCP was already enabled, keep it enabled (allow some tools to be disabled)
				if !cfg.ExternalMCPEnable && hasEnabledTool {
					cfg.ExternalMCPEnable = true
					h.logger.Info("Automatically enabling external MCP (because a tool is enabled)", zap.String("mcp", mcpName))
				}

				h.config.ExternalMCP.Servers[mcpName] = cfg
			}

			// Sync update configs in externalMCPMgr to ensure GetConfigs() returns latest config
			// Update uniformly outside the loop to avoid repeated calls
			h.externalMCPMgr.LoadConfigs(&h.config.ExternalMCP)

			// Handle MCP connection status (async start, to avoid blocking)
			for mcpName := range externalMCPToolMap {
				cfg := h.config.ExternalMCP.Servers[mcpName]
				// If MCP needs to be enabled, ensure the client is started
				if cfg.ExternalMCPEnable {
					// Start external MCP (if not started) - execute asynchronously to avoid blocking
					client, exists := h.externalMCPMgr.GetClient(mcpName)
					if !exists || !client.IsConnected() {
						go func(name string) {
							if err := h.externalMCPMgr.StartClient(name); err != nil {
								h.logger.Warn("Failed to start external MCP",
									zap.String("mcp", name),
									zap.Error(err),
								)
							} else {
								h.logger.Info("Started external MCP",
									zap.String("mcp", name),
								)
							}
						}(mcpName)
					}
				}
			}
		}
	}

	// Save config to file
	if err := h.saveConfig(); err != nil {
		h.logger.Error("Failed to save config", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save config: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Configuration updated"})
}

// ApplyConfig applies the configuration (reloads and restarts related services)
func (h *ConfigHandler) ApplyConfig(c *gin.Context) {
	// First check if dynamic knowledge base initialization is needed (execute outside lock to avoid blocking other requests)
	var needInitKnowledge bool
	var knowledgeInitializer KnowledgeInitializer

	h.mu.RLock()
	needInitKnowledge = h.config.Knowledge.Enabled && h.knowledgeToolRegistrar == nil && h.knowledgeInitializer != nil
	if needInitKnowledge {
		knowledgeInitializer = h.knowledgeInitializer
	}
	h.mu.RUnlock()

	// If dynamic knowledge base initialization is needed, execute outside lock (this is a time-consuming operation)
	if needInitKnowledge {
		h.logger.Info("Detected knowledge base changed from disabled to enabled, starting dynamic initialization of knowledge base components")
		if _, err := knowledgeInitializer(); err != nil {
			h.logger.Error("Failed to dynamically initialize knowledge base", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize knowledge base: " + err.Error()})
			return
		}
		h.logger.Info("Knowledge base dynamic initialization complete, tools registered")
	}

	// Check if embedding model config has changed (execute outside lock to avoid blocking)
	var needReinitKnowledge bool
	var reinitKnowledgeInitializer KnowledgeInitializer
	h.mu.RLock()
	if h.config.Knowledge.Enabled && h.knowledgeInitializer != nil && h.lastEmbeddingConfig != nil {
		// Check if embedding model config has changed
		currentEmbedding := h.config.Knowledge.Embedding
		if currentEmbedding.Provider != h.lastEmbeddingConfig.Provider ||
			currentEmbedding.Model != h.lastEmbeddingConfig.Model ||
			currentEmbedding.BaseURL != h.lastEmbeddingConfig.BaseURL ||
			currentEmbedding.APIKey != h.lastEmbeddingConfig.APIKey {
			needReinitKnowledge = true
			reinitKnowledgeInitializer = h.knowledgeInitializer
			h.logger.Info("Detected embedding model config change, need to reinitialize knowledge base components",
				zap.String("old_model", h.lastEmbeddingConfig.Model),
				zap.String("new_model", currentEmbedding.Model),
				zap.String("old_base_url", h.lastEmbeddingConfig.BaseURL),
				zap.String("new_base_url", currentEmbedding.BaseURL),
			)
		}
	}
	h.mu.RUnlock()

	// If knowledge base needs reinitialization (embedding model config changed), execute outside lock
	if needReinitKnowledge {
		h.logger.Info("Starting reinitialization of knowledge base components (embedding model config has changed)")
		if _, err := reinitKnowledgeInitializer(); err != nil {
			h.logger.Error("Failed to reinitialize knowledge base", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reinitialize knowledge base: " + err.Error()})
			return
		}
		h.logger.Info("Knowledge base components reinitialized")
	}

	// Now acquire write lock, execute fast operations
	h.mu.Lock()
	defer h.mu.Unlock()

	// If knowledge base was reinitialized, update embedding model config record
	if needReinitKnowledge && h.config.Knowledge.Enabled {
		h.lastEmbeddingConfig = &config.EmbeddingConfig{
			Provider: h.config.Knowledge.Embedding.Provider,
			Model:    h.config.Knowledge.Embedding.Model,
			BaseURL:  h.config.Knowledge.Embedding.BaseURL,
			APIKey:   h.config.Knowledge.Embedding.APIKey,
		}
		h.logger.Info("Embedding model config record updated")
	}

	// Re-register tools (according to new enable status)
	h.logger.Info("Re-registering tools")

	// Clear tools in MCP server
	h.mcpServer.ClearTools()

	// Re-register security tools
	h.executor.RegisterTools(h.mcpServer)

	// Re-register vulnerability record tool (built-in tool, must be registered)
	if h.vulnerabilityToolRegistrar != nil {
		h.logger.Info("Re-registering vulnerability record tool")
		if err := h.vulnerabilityToolRegistrar(); err != nil {
			h.logger.Error("Failed to re-register vulnerability record tool", zap.Error(err))
		} else {
			h.logger.Info("Vulnerability record tool re-registered")
		}
	}

	// Re-register Skills tools (built-in tools, must be registered)
	if h.skillsToolRegistrar != nil {
		h.logger.Info("Re-registering Skills tools")
		if err := h.skillsToolRegistrar(); err != nil {
			h.logger.Error("Failed to re-register Skills tools", zap.Error(err))
		} else {
			h.logger.Info("Skills tools re-registered")
		}
	}

	// Re-register memory tools (built-in, must be registered)
	if h.memoryToolRegistrar != nil {
		h.logger.Info("Re-registering memory tools")
		if err := h.memoryToolRegistrar(); err != nil {
			h.logger.Error("Failed to re-register memory tools", zap.Error(err))
		} else {
			h.logger.Info("Memory tools re-registered")
		}
	}

	// Re-register time tools (built-in, must be registered)
	if h.timeToolRegistrar != nil {
		h.logger.Info("Re-registering time tools")
		if err := h.timeToolRegistrar(); err != nil {
			h.logger.Error("Failed to re-register time tools", zap.Error(err))
		} else {
			h.logger.Info("Time tools re-registered")
		}
	}

	// If knowledge base is enabled, re-register knowledge base tools
	if h.config.Knowledge.Enabled && h.knowledgeToolRegistrar != nil {
		h.logger.Info("Re-registering knowledge base tools")
		if err := h.knowledgeToolRegistrar(); err != nil {
			h.logger.Error("Failed to re-register knowledge base tools", zap.Error(err))
		} else {
			h.logger.Info("Knowledge base tools re-registered")
		}
	}

	// Update Agent's OpenAI config
	if h.agent != nil {
		h.agent.UpdateConfig(&h.config.OpenAI)
		h.agent.UpdateMaxIterations(h.config.Agent.MaxIterations)
		h.logger.Info("Agent config updated")
	}

	// Update AttackChainHandler's OpenAI config
	if h.attackChainHandler != nil {
		h.attackChainHandler.UpdateConfig(&h.config.OpenAI)
		h.logger.Info("AttackChainHandler config updated")
	}

	// Update retriever config (if knowledge base is enabled)
	if h.config.Knowledge.Enabled && h.retrieverUpdater != nil {
		retrievalConfig := &knowledge.RetrievalConfig{
			TopK:                h.config.Knowledge.Retrieval.TopK,
			SimilarityThreshold: h.config.Knowledge.Retrieval.SimilarityThreshold,
			HybridWeight:        h.config.Knowledge.Retrieval.HybridWeight,
		}
		h.retrieverUpdater.UpdateConfig(retrievalConfig)
		h.logger.Info("Retriever config updated",
			zap.Int("top_k", retrievalConfig.TopK),
			zap.Float64("similarity_threshold", retrievalConfig.SimilarityThreshold),
			zap.Float64("hybrid_weight", retrievalConfig.HybridWeight),
		)
	}

	// Update embedding model config record (if knowledge base is enabled)
	if h.config.Knowledge.Enabled {
		h.lastEmbeddingConfig = &config.EmbeddingConfig{
			Provider: h.config.Knowledge.Embedding.Provider,
			Model:    h.config.Knowledge.Embedding.Model,
			BaseURL:  h.config.Knowledge.Embedding.BaseURL,
			APIKey:   h.config.Knowledge.Embedding.APIKey,
		}
	}

	// Restart Lark long connections so that robot config changes from frontend take effect immediately (without restarting the service)
	if h.robotRestarter != nil {
		h.robotRestarter.RestartRobotConnections()
		h.logger.Info("Triggered robot connection restart (Lark)")
	}

	h.logger.Info("Configuration applied",
		zap.Int("tools_count", len(h.config.Security.Tools)),
	)

	c.JSON(http.StatusOK, gin.H{
		"message":     "Configuration applied",
		"tools_count": len(h.config.Security.Tools),
	})
}

// saveConfig saves the configuration to file
func (h *ConfigHandler) saveConfig() error {
	// Read existing config file and create backup
	data, err := os.ReadFile(h.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	if err := os.WriteFile(h.configPath+".backup", data, 0644); err != nil {
		h.logger.Warn("Failed to create config backup", zap.Error(err))
	}

	root, err := loadYAMLDocument(h.configPath)
	if err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	updateAgentConfig(root, h.config.Agent)
	updateMCPConfig(root, h.config.MCP)
	updateOpenAIConfig(root, h.config.OpenAI)
	updateSecuritySettingsConfig(root, h.config.Security.ToolDescriptionMode)
	updateFOFAConfig(root, h.config.FOFA)
	updateKnowledgeConfig(root, h.config.Knowledge)
	updateRobotsConfig(root, h.config.Robots)
	// Update external MCP config (using function from external_mcp.go, directly callable within the same package)
	// Read original config to maintain backward compatibility
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

	// Update enabled status in tool config files
	if h.config.Security.ToolsDir != "" {
		configDir := filepath.Dir(h.configPath)
		toolsDir := h.config.Security.ToolsDir
		if !filepath.IsAbs(toolsDir) {
			toolsDir = filepath.Join(configDir, toolsDir)
		}

		for _, tool := range h.config.Security.Tools {
			toolFile := filepath.Join(toolsDir, tool.Name+".yaml")
			// Check if file exists
			if _, err := os.Stat(toolFile); os.IsNotExist(err) {
				// Try .yml extension
				toolFile = filepath.Join(toolsDir, tool.Name+".yml")
				if _, err := os.Stat(toolFile); os.IsNotExist(err) {
					h.logger.Warn("Tool config file does not exist", zap.String("tool", tool.Name))
					continue
				}
			}

			toolDoc, err := loadYAMLDocument(toolFile)
			if err != nil {
				h.logger.Warn("Failed to parse tool config", zap.String("tool", tool.Name), zap.Error(err))
				continue
			}

			setBoolInMap(toolDoc.Content[0], "enabled", tool.Enabled)

			if err := writeYAMLDocument(toolFile, toolDoc); err != nil {
				h.logger.Warn("Failed to save tool config file", zap.String("tool", tool.Name), zap.Error(err))
				continue
			}

			h.logger.Info("Tool config updated", zap.String("tool", tool.Name), zap.Bool("enabled", tool.Enabled))
		}
	}

	h.logger.Info("Configuration saved", zap.String("path", h.configPath))
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

func updateAgentConfig(doc *yaml.Node, cfg config.AgentConfig) {
	root := doc.Content[0]
	agentNode := ensureMap(root, "agent")
	setIntInMap(agentNode, "max_iterations", cfg.MaxIterations)
	if cfg.LargeResultThreshold > 0 {
		setIntInMap(agentNode, "large_result_threshold", cfg.LargeResultThreshold)
	}
	if cfg.ResultStorageDir != "" {
		setStringInMap(agentNode, "result_storage_dir", cfg.ResultStorageDir)
	}
	setBoolInMap(agentNode, "parallel_tool_execution", cfg.ParallelToolExecution)
	setIntInMap(agentNode, "max_parallel_tools", cfg.MaxParallelTools)
	setIntInMap(agentNode, "tool_retry_count", cfg.ToolRetryCount)
	if cfg.ParallelToolWaitSeconds > 0 {
		setIntInMap(agentNode, "parallel_tool_wait_seconds", cfg.ParallelToolWaitSeconds)
	}

	// Time awareness
	taNode := ensureMap(agentNode, "time_awareness")
	setBoolInMap(taNode, "enabled", cfg.TimeAwareness.Enabled)
	if cfg.TimeAwareness.Timezone != "" {
		setStringInMap(taNode, "timezone", cfg.TimeAwareness.Timezone)
	}

	// Memory
	memNode := ensureMap(agentNode, "memory")
	setBoolInMap(memNode, "enabled", cfg.Memory.Enabled)
	setIntInMap(memNode, "max_entries", cfg.Memory.MaxEntries)
}

func updateSecuritySettingsConfig(doc *yaml.Node, toolDescriptionMode string) {
	root := doc.Content[0]
	secNode := ensureMap(root, "security")
	if toolDescriptionMode != "" {
		setStringInMap(secNode, "tool_description_mode", toolDescriptionMode)
	}
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
	setStringInMap(openaiNode, "api_key", cfg.APIKey)
	setStringInMap(openaiNode, "base_url", cfg.BaseURL)
	setStringInMap(openaiNode, "model", cfg.Model)
	setStringInMap(openaiNode, "tool_model", cfg.ToolModel)
	setStringInMap(openaiNode, "tool_base_url", cfg.ToolBaseURL)
	setStringInMap(openaiNode, "tool_api_key", cfg.ToolAPIKey)
	setStringInMap(openaiNode, "summary_model", cfg.SummaryModel)
	setStringInMap(openaiNode, "summary_base_url", cfg.SummaryBaseURL)
	setStringInMap(openaiNode, "summary_api_key", cfg.SummaryAPIKey)
	if cfg.MaxTotalTokens > 0 {
		setIntInMap(openaiNode, "max_total_tokens", cfg.MaxTotalTokens)
	}
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

	// Update embedding config
	embeddingNode := ensureMap(knowledgeNode, "embedding")
	setStringInMap(embeddingNode, "provider", cfg.Embedding.Provider)
	setStringInMap(embeddingNode, "model", cfg.Embedding.Model)
	if cfg.Embedding.BaseURL != "" {
		setStringInMap(embeddingNode, "base_url", cfg.Embedding.BaseURL)
	}
	if cfg.Embedding.APIKey != "" {
		setStringInMap(embeddingNode, "api_key", cfg.Embedding.APIKey)
	}

	// Update retrieval config
	retrievalNode := ensureMap(knowledgeNode, "retrieval")
	setIntInMap(retrievalNode, "top_k", cfg.Retrieval.TopK)
	setFloatInMap(retrievalNode, "similarity_threshold", cfg.Retrieval.SimilarityThreshold)
	setFloatInMap(retrievalNode, "hybrid_weight", cfg.Retrieval.HybridWeight)
}

func updateRobotsConfig(doc *yaml.Node, cfg config.RobotsConfig) {
	root := doc.Content[0]
	robotsNode := ensureMap(root, "robots")

	wecomNode := ensureMap(robotsNode, "wecom")
	setBoolInMap(wecomNode, "enabled", cfg.Wecom.Enabled)
	setStringInMap(wecomNode, "token", cfg.Wecom.Token)
	setStringInMap(wecomNode, "encoding_aes_key", cfg.Wecom.EncodingAESKey)
	setStringInMap(wecomNode, "corp_id", cfg.Wecom.CorpID)
	setStringInMap(wecomNode, "secret", cfg.Wecom.Secret)
	setIntInMap(wecomNode, "agent_id", int(cfg.Wecom.AgentID))

	larkNode := ensureMap(robotsNode, "lark")
	setBoolInMap(larkNode, "enabled", cfg.Lark.Enabled)
	setStringInMap(larkNode, "app_id", cfg.Lark.AppID)
	setStringInMap(larkNode, "app_secret", cfg.Lark.AppSecret)
	setStringInMap(larkNode, "verify_token", cfg.Lark.VerifyToken)
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
	// For values between 0.0 and 1.0 (such as hybrid_weight), use %.1f to ensure 0.0 is explicitly serialized as "0.0"
	// For other values, use %g to automatically select the most suitable format
	if value >= 0.0 && value <= 1.0 {
		valueNode.Value = fmt.Sprintf("%.1f", value)
	} else {
		valueNode.Value = fmt.Sprintf("%g", value)
	}
}

// getExternalMCPTools gets the list of external MCP tools (public method)
// Returns a list of ToolConfigInfo with enable status and description information already processed
func (h *ConfigHandler) getExternalMCPTools(ctx context.Context) []ToolConfigInfo {
	var result []ToolConfigInfo

	if h.externalMCPMgr == nil {
		return result
	}

	// Use a shorter timeout (5 seconds) for quick failure, to avoid blocking page loading
	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	externalTools, err := h.externalMCPMgr.GetAllTools(timeoutCtx)
	if err != nil {
		// Log warning but don't block; continue returning cached tools (if any)
		h.logger.Warn("Failed to get external MCP tools (connection may be down), attempting to return cached tools",
			zap.Error(err),
			zap.String("hint", "If external MCP tools are not displayed, check connection status or click the refresh button"),
		)
	}

	// If tools were retrieved (even if there were errors), continue processing
	if len(externalTools) == 0 {
		return result
	}

	externalMCPConfigs := h.externalMCPMgr.GetConfigs()

	for _, externalTool := range externalTools {
		// Parse tool name: mcpName::toolName
		mcpName, actualToolName := h.parseExternalToolName(externalTool.Name)
		if mcpName == "" || actualToolName == "" {
			continue // skip incorrectly formatted tools
		}

		// Calculate enable status
		enabled := h.calculateExternalToolEnabled(mcpName, actualToolName, externalMCPConfigs)

		// Process description information
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

// parseExternalToolName parses external tool name (format: mcpName::toolName)
func (h *ConfigHandler) parseExternalToolName(fullName string) (mcpName, toolName string) {
	idx := strings.Index(fullName, "::")
	if idx > 0 {
		return fullName[:idx], fullName[idx+2:]
	}
	return "", ""
}

// calculateExternalToolEnabled calculates the enable status of an external tool
func (h *ConfigHandler) calculateExternalToolEnabled(mcpName, toolName string, configs map[string]config.ExternalMCPServerConfig) bool {
	cfg, exists := configs[mcpName]
	if !exists {
		return false
	}

	// First check if the external MCP is enabled
	if !cfg.ExternalMCPEnable && !(cfg.Enabled && !cfg.Disabled) {
		return false // MCP not enabled, all tools are disabled
	}

	// MCP is enabled, check individual tool enable status
	// If ToolEnabled is empty or the tool is not set, default to enabled (backward compatible)
	if cfg.ToolEnabled == nil {
		// Tool status not set, default to enabled
	} else if toolEnabled, exists := cfg.ToolEnabled[toolName]; exists {
		// Use configured tool status
		if !toolEnabled {
			return false
		}
	}
	// Tool not in config, default to enabled

	// Finally check if the external MCP is connected
	client, exists := h.externalMCPMgr.GetClient(mcpName)
	if !exists || !client.IsConnected() {
		return false // treat as disabled when not connected
	}

	return true
}

// pickToolDescription selects short or full description based on security.tool_description_mode and limits length
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
