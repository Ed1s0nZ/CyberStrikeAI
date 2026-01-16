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
	"cyberstrike-ai/internal/security"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// KnowledgeToolRegistrar 知识库工具注册器接口
type KnowledgeToolRegistrar func() error

// VulnerabilityToolRegistrar 漏洞工具注册器接口
type VulnerabilityToolRegistrar func() error

// SkillsToolRegistrar Skills工具注册器接口
type SkillsToolRegistrar func() error

// RetrieverUpdater 检索器更新接口
type RetrieverUpdater interface {
	UpdateConfig(config *knowledge.RetrievalConfig)
}

// KnowledgeInitializer 知识库初始化器接口
type KnowledgeInitializer func() (*KnowledgeHandler, error)

// AppUpdater App更新接口（用于更新App中的知识库组件）
type AppUpdater interface {
	UpdateKnowledgeComponents(handler *KnowledgeHandler, manager interface{}, retriever interface{}, indexer interface{})
}

// ConfigHandler 配置处理器
type ConfigHandler struct {
	configPath                 string
	config                     *config.Config
	mcpServer                  *mcp.Server
	executor                   *security.Executor
	agent                      AgentUpdater               // Agent接口，用于更新Agent配置
	attackChainHandler         AttackChainUpdater         // 攻击链处理器接口，用于更新配置
	externalMCPMgr             *mcp.ExternalMCPManager    // 外部MCP管理器
	knowledgeToolRegistrar     KnowledgeToolRegistrar     // 知识库工具注册器（可选）
	vulnerabilityToolRegistrar VulnerabilityToolRegistrar // 漏洞工具注册器（可选）
	skillsToolRegistrar        SkillsToolRegistrar        // Skills工具注册器（可选）
	retrieverUpdater           RetrieverUpdater           // 检索器更新器（可选）
	knowledgeInitializer       KnowledgeInitializer       // 知识库初始化器（可选）
	appUpdater                 AppUpdater                 // App更新器（可选）
	logger                     *zap.Logger
	mu                         sync.RWMutex
	lastEmbeddingConfig        *config.EmbeddingConfig // 上一次的嵌入模型配置（用于检测变更）
}

// AttackChainUpdater 攻击链处理器更新接口
type AttackChainUpdater interface {
	UpdateConfig(cfg *config.OpenAIConfig)
}

// AgentUpdater Agent更新接口
type AgentUpdater interface {
	UpdateConfig(cfg *config.OpenAIConfig)
	UpdateMaxIterations(maxIterations int)
}

// NewConfigHandler 创建新的配置处理器
func NewConfigHandler(configPath string, cfg *config.Config, mcpServer *mcp.Server, executor *security.Executor, agent AgentUpdater, attackChainHandler AttackChainUpdater, externalMCPMgr *mcp.ExternalMCPManager, logger *zap.Logger) *ConfigHandler {
	// 保存初始的嵌入模型配置（如果知识库已启用）
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

// SetKnowledgeToolRegistrar 设置知识库工具注册器
func (h *ConfigHandler) SetKnowledgeToolRegistrar(registrar KnowledgeToolRegistrar) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.knowledgeToolRegistrar = registrar
}

// SetVulnerabilityToolRegistrar 设置漏洞工具注册器
func (h *ConfigHandler) SetVulnerabilityToolRegistrar(registrar VulnerabilityToolRegistrar) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.vulnerabilityToolRegistrar = registrar
}

// SetSkillsToolRegistrar 设置Skills工具注册器
func (h *ConfigHandler) SetSkillsToolRegistrar(registrar SkillsToolRegistrar) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.skillsToolRegistrar = registrar
}

// SetRetrieverUpdater 设置检索器更新器
func (h *ConfigHandler) SetRetrieverUpdater(updater RetrieverUpdater) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.retrieverUpdater = updater
}

// SetKnowledgeInitializer 设置知识库初始化器
func (h *ConfigHandler) SetKnowledgeInitializer(initializer KnowledgeInitializer) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.knowledgeInitializer = initializer
}

// SetAppUpdater 设置App更新器
func (h *ConfigHandler) SetAppUpdater(updater AppUpdater) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.appUpdater = updater
}

// GetConfigResponse 获取配置响应
type GetConfigResponse struct {
	OpenAI    config.OpenAIConfig    `json:"openai"`
	MCP       config.MCPConfig       `json:"mcp"`
	Tools     []ToolConfigInfo       `json:"tools"`
	Agent     config.AgentConfig     `json:"agent"`
	Knowledge config.KnowledgeConfig `json:"knowledge"`
}

// ToolConfigInfo 工具配置信息
type ToolConfigInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
	IsExternal  bool   `json:"is_external,omitempty"`  // 是否为外部MCP工具
	ExternalMCP string `json:"external_mcp,omitempty"` // 外部MCP名称（如果是外部工具）
	RoleEnabled *bool  `json:"role_enabled,omitempty"` // 该工具在当前角色中是否启用（nil表示未指定角色或使用所有工具）
}

// GetConfig 获取当前配置
func (h *ConfigHandler) GetConfig(c *gin.Context) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// 获取工具列表（包含内部和外部工具）
	// 首先从配置文件获取工具
	configToolMap := make(map[string]bool)
	tools := make([]ToolConfigInfo, 0, len(h.config.Security.Tools))
	for _, tool := range h.config.Security.Tools {
		configToolMap[tool.Name] = true
		tools = append(tools, ToolConfigInfo{
			Name:        tool.Name,
			Description: tool.ShortDescription,
			Enabled:     tool.Enabled,
			IsExternal:  false,
		})
		// 如果没有简短描述，使用详细描述的前100个字符
		if tools[len(tools)-1].Description == "" {
			desc := tool.Description
			if len(desc) > 100 {
				desc = desc[:100] + "..."
			}
			tools[len(tools)-1].Description = desc
		}
	}

	// 从MCP服务器获取所有已注册的工具（包括直接注册的工具，如知识检索工具）
	if h.mcpServer != nil {
		mcpTools := h.mcpServer.GetAllTools()
		for _, mcpTool := range mcpTools {
			// 跳过已经在配置文件中的工具（避免重复）
			if configToolMap[mcpTool.Name] {
				continue
			}
			// 添加直接注册到MCP服务器的工具（如知识检索工具）
			description := mcpTool.ShortDescription
			if description == "" {
				description = mcpTool.Description
			}
			if len(description) > 100 {
				description = description[:100] + "..."
			}
			tools = append(tools, ToolConfigInfo{
				Name:        mcpTool.Name,
				Description: description,
				Enabled:     true, // 直接注册的工具默认启用
				IsExternal:  false,
			})
		}
	}

	// 获取外部MCP工具
	if h.externalMCPMgr != nil {
		// 增加超时时间到30秒，因为通过代理连接远程服务器可能需要更长时间
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		externalTools, err := h.externalMCPMgr.GetAllTools(ctx)
		if err == nil {
			externalMCPConfigs := h.externalMCPMgr.GetConfigs()
			for _, externalTool := range externalTools {
				var mcpName, actualToolName string
				if idx := strings.Index(externalTool.Name, "::"); idx > 0 {
					mcpName = externalTool.Name[:idx]
					actualToolName = externalTool.Name[idx+2:]
				} else {
					continue
				}

				enabled := false
				if cfg, exists := externalMCPConfigs[mcpName]; exists {
					// 首先检查外部MCP是否启用
					if !cfg.ExternalMCPEnable && !(cfg.Enabled && !cfg.Disabled) {
						enabled = false // MCP未启用，所有工具都禁用
					} else {
						// MCP已启用，检查单个工具的启用状态
						// 如果ToolEnabled为空或未设置该工具，默认为启用（向后兼容）
						if cfg.ToolEnabled == nil {
							enabled = true // 未设置工具状态，默认为启用
						} else if toolEnabled, exists := cfg.ToolEnabled[actualToolName]; exists {
							enabled = toolEnabled // 使用配置的工具状态
						} else {
							enabled = true // 工具未在配置中，默认为启用
						}
					}
				}

				client, exists := h.externalMCPMgr.GetClient(mcpName)
				if !exists || !client.IsConnected() {
					enabled = false
				}

				description := externalTool.ShortDescription
				if description == "" {
					description = externalTool.Description
				}
				if len(description) > 100 {
					description = description[:100] + "..."
				}

				tools = append(tools, ToolConfigInfo{
					Name:        actualToolName,
					Description: description,
					Enabled:     enabled,
					IsExternal:  true,
					ExternalMCP: mcpName,
				})
			}
		}
	}

	c.JSON(http.StatusOK, GetConfigResponse{
		OpenAI:    h.config.OpenAI,
		MCP:       h.config.MCP,
		Tools:     tools,
		Agent:     h.config.Agent,
		Knowledge: h.config.Knowledge,
	})
}

// GetToolsResponse 获取工具列表响应（分页）
type GetToolsResponse struct {
	Tools        []ToolConfigInfo `json:"tools"`
	Total        int              `json:"total"`
	TotalEnabled int              `json:"total_enabled"` // 已启用的工具总数
	Page         int              `json:"page"`
	PageSize     int              `json:"page_size"`
	TotalPages   int              `json:"total_pages"`
}

// GetTools 获取工具列表（支持分页和搜索）
func (h *ConfigHandler) GetTools(c *gin.Context) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// 解析分页参数
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

	// 解析搜索参数
	searchTerm := c.Query("search")
	searchTermLower := ""
	if searchTerm != "" {
		searchTermLower = strings.ToLower(searchTerm)
	}

	// 解析角色参数，用于过滤工具并标注启用状态
	roleName := c.Query("role")
	var roleToolsSet map[string]bool // 角色配置的工具集合
	var roleUsesAllTools bool = true // 角色是否使用所有工具（默认角色）
	if roleName != "" && roleName != "默认" && h.config.Roles != nil {
		if role, exists := h.config.Roles[roleName]; exists && role.Enabled {
			if len(role.Tools) > 0 {
				// 角色配置了工具列表，只使用这些工具
				roleToolsSet = make(map[string]bool)
				for _, toolKey := range role.Tools {
					roleToolsSet[toolKey] = true
				}
				roleUsesAllTools = false
			}
		}
	}

	// 获取所有内部工具并应用搜索过滤
	configToolMap := make(map[string]bool)
	allTools := make([]ToolConfigInfo, 0, len(h.config.Security.Tools))
	for _, tool := range h.config.Security.Tools {
		configToolMap[tool.Name] = true
		toolInfo := ToolConfigInfo{
			Name:        tool.Name,
			Description: tool.ShortDescription,
			Enabled:     tool.Enabled,
			IsExternal:  false,
		}
		// 如果没有简短描述，使用详细描述的前100个字符
		if toolInfo.Description == "" {
			desc := tool.Description
			if len(desc) > 100 {
				desc = desc[:100] + "..."
			}
			toolInfo.Description = desc
		}

		// 根据角色配置标注工具状态
		if roleName != "" {
			if roleUsesAllTools {
				// 角色使用所有工具，标注启用的工具为role_enabled=true
				if tool.Enabled {
					roleEnabled := true
					toolInfo.RoleEnabled = &roleEnabled
				} else {
					roleEnabled := false
					toolInfo.RoleEnabled = &roleEnabled
				}
			} else {
				// 角色配置了工具列表，检查工具是否在列表中
				// 内部工具使用工具名称作为key
				if roleToolsSet[tool.Name] {
					roleEnabled := tool.Enabled // 工具必须在角色列表中且本身启用
					toolInfo.RoleEnabled = &roleEnabled
				} else {
					// 不在角色列表中，标记为false
					roleEnabled := false
					toolInfo.RoleEnabled = &roleEnabled
				}
			}
		}

		// 如果有关键词，进行搜索过滤
		if searchTermLower != "" {
			nameLower := strings.ToLower(toolInfo.Name)
			descLower := strings.ToLower(toolInfo.Description)
			if !strings.Contains(nameLower, searchTermLower) && !strings.Contains(descLower, searchTermLower) {
				continue // 不匹配，跳过
			}
		}

		allTools = append(allTools, toolInfo)
	}

	// 从MCP服务器获取所有已注册的工具（包括直接注册的工具，如知识检索工具）
	if h.mcpServer != nil {
		mcpTools := h.mcpServer.GetAllTools()
		for _, mcpTool := range mcpTools {
			// 跳过已经在配置文件中的工具（避免重复）
			if configToolMap[mcpTool.Name] {
				continue
			}

			description := mcpTool.ShortDescription
			if description == "" {
				description = mcpTool.Description
			}
			if len(description) > 100 {
				description = description[:100] + "..."
			}

			toolInfo := ToolConfigInfo{
				Name:        mcpTool.Name,
				Description: description,
				Enabled:     true, // 直接注册的工具默认启用
				IsExternal:  false,
			}

			// 根据角色配置标注工具状态
			if roleName != "" {
				if roleUsesAllTools {
					// 角色使用所有工具，直接注册的工具默认启用
					roleEnabled := true
					toolInfo.RoleEnabled = &roleEnabled
				} else {
					// 角色配置了工具列表，检查工具是否在列表中
					// 内部工具使用工具名称作为key
					if roleToolsSet[mcpTool.Name] {
						roleEnabled := true // 在角色列表中且工具本身启用
						toolInfo.RoleEnabled = &roleEnabled
					} else {
						// 不在角色列表中，标记为false
						roleEnabled := false
						toolInfo.RoleEnabled = &roleEnabled
					}
				}
			}

			// 如果有关键词，进行搜索过滤
			if searchTermLower != "" {
				nameLower := strings.ToLower(toolInfo.Name)
				descLower := strings.ToLower(toolInfo.Description)
				if !strings.Contains(nameLower, searchTermLower) && !strings.Contains(descLower, searchTermLower) {
					continue // 不匹配，跳过
				}
			}

			allTools = append(allTools, toolInfo)
		}
	}

	// 获取外部MCP工具
	if h.externalMCPMgr != nil {
		// 增加超时时间到30秒，因为通过代理连接远程服务器可能需要更长时间
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		externalTools, err := h.externalMCPMgr.GetAllTools(ctx)
		if err != nil {
			h.logger.Warn("获取外部MCP工具失败", zap.Error(err))
		} else {
			// 获取外部MCP配置，用于判断启用状态
			externalMCPConfigs := h.externalMCPMgr.GetConfigs()

			for _, externalTool := range externalTools {
				// 解析工具名称：mcpName::toolName
				var mcpName, actualToolName string
				if idx := strings.Index(externalTool.Name, "::"); idx > 0 {
					mcpName = externalTool.Name[:idx]
					actualToolName = externalTool.Name[idx+2:]
				} else {
					continue // 跳过格式不正确的工具
				}

				// 获取外部工具的启用状态
				enabled := false
				if cfg, exists := externalMCPConfigs[mcpName]; exists {
					// 首先检查外部MCP是否启用
					if !cfg.ExternalMCPEnable && !(cfg.Enabled && !cfg.Disabled) {
						enabled = false // MCP未启用，所有工具都禁用
					} else {
						// MCP已启用，检查单个工具的启用状态
						// 如果ToolEnabled为空或未设置该工具，默认为启用（向后兼容）
						if cfg.ToolEnabled == nil {
							enabled = true // 未设置工具状态，默认为启用
						} else if toolEnabled, exists := cfg.ToolEnabled[actualToolName]; exists {
							enabled = toolEnabled // 使用配置的工具状态
						} else {
							enabled = true // 工具未在配置中，默认为启用
						}
					}
				}

				// 检查外部MCP是否已连接
				client, exists := h.externalMCPMgr.GetClient(mcpName)
				if !exists || !client.IsConnected() {
					enabled = false // 未连接时视为禁用
				}

				description := externalTool.ShortDescription
				if description == "" {
					description = externalTool.Description
				}
				if len(description) > 100 {
					description = description[:100] + "..."
				}

				// 如果有关键词，进行搜索过滤
				if searchTermLower != "" {
					nameLower := strings.ToLower(actualToolName)
					descLower := strings.ToLower(description)
					if !strings.Contains(nameLower, searchTermLower) && !strings.Contains(descLower, searchTermLower) {
						continue // 不匹配，跳过
					}
				}

				toolInfo := ToolConfigInfo{
					Name:        actualToolName, // 显示实际工具名称，不带前缀
					Description: description,
					Enabled:     enabled,
					IsExternal:  true,
					ExternalMCP: mcpName,
				}

				// 根据角色配置标注工具状态
				if roleName != "" {
					if roleUsesAllTools {
						// 角色使用所有工具，标注启用的工具为role_enabled=true
						toolInfo.RoleEnabled = &enabled
					} else {
						// 角色配置了工具列表，检查工具是否在列表中
						// 外部工具使用 "mcpName::toolName" 格式作为key
						externalToolKey := externalTool.Name // 这是 "mcpName::toolName" 格式
						if roleToolsSet[externalToolKey] {
							roleEnabled := enabled // 工具必须在角色列表中且本身启用
							toolInfo.RoleEnabled = &roleEnabled
						} else {
							// 不在角色列表中，标记为false
							roleEnabled := false
							toolInfo.RoleEnabled = &roleEnabled
						}
					}
				}

				allTools = append(allTools, toolInfo)
			}
		}
	}

	// 如果角色配置了工具列表，过滤工具（只保留列表中的工具，但保留其他工具并标记为禁用）
	// 注意：这里我们不直接过滤掉工具，而是保留所有工具，但通过 role_enabled 字段标注状态
	// 这样前端可以显示所有工具，并标注哪些工具在当前角色中可用

	total := len(allTools)
	// 统计已启用的工具数（在角色中的启用工具数）
	totalEnabled := 0
	for _, tool := range allTools {
		if tool.RoleEnabled != nil && *tool.RoleEnabled {
			totalEnabled++
		} else if tool.RoleEnabled == nil && tool.Enabled {
			// 如果未指定角色，统计所有启用的工具
			totalEnabled++
		}
	}

	totalPages := (total + pageSize - 1) / pageSize
	if totalPages == 0 {
		totalPages = 1
	}

	// 计算分页范围
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

// UpdateConfigRequest 更新配置请求
type UpdateConfigRequest struct {
	OpenAI    *config.OpenAIConfig    `json:"openai,omitempty"`
	MCP       *config.MCPConfig       `json:"mcp,omitempty"`
	Tools     []ToolEnableStatus      `json:"tools,omitempty"`
	Agent     *config.AgentConfig     `json:"agent,omitempty"`
	Knowledge *config.KnowledgeConfig `json:"knowledge,omitempty"`
}

// ToolEnableStatus 工具启用状态
type ToolEnableStatus struct {
	Name        string `json:"name"`
	Enabled     bool   `json:"enabled"`
	IsExternal  bool   `json:"is_external,omitempty"`  // 是否为外部MCP工具
	ExternalMCP string `json:"external_mcp,omitempty"` // 外部MCP名称（如果是外部工具）
}

// UpdateConfig 更新配置
func (h *ConfigHandler) UpdateConfig(c *gin.Context) {
	var req UpdateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数: " + err.Error()})
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	// 更新OpenAI配置
	if req.OpenAI != nil {
		h.config.OpenAI = *req.OpenAI
		h.logger.Info("更新OpenAI配置",
			zap.String("base_url", h.config.OpenAI.BaseURL),
			zap.String("model", h.config.OpenAI.Model),
		)
	}

	// 更新MCP配置
	if req.MCP != nil {
		h.config.MCP = *req.MCP
		h.logger.Info("更新MCP配置",
			zap.Bool("enabled", h.config.MCP.Enabled),
			zap.String("host", h.config.MCP.Host),
			zap.Int("port", h.config.MCP.Port),
		)
	}

	// 更新Agent配置
	if req.Agent != nil {
		h.config.Agent = *req.Agent
		h.logger.Info("更新Agent配置",
			zap.Int("max_iterations", h.config.Agent.MaxIterations),
		)
	}

	// 更新Knowledge配置
	if req.Knowledge != nil {
		// 保存旧的嵌入模型配置（用于检测变更）
		if h.config.Knowledge.Enabled {
			h.lastEmbeddingConfig = &config.EmbeddingConfig{
				Provider: h.config.Knowledge.Embedding.Provider,
				Model:    h.config.Knowledge.Embedding.Model,
				BaseURL:  h.config.Knowledge.Embedding.BaseURL,
				APIKey:   h.config.Knowledge.Embedding.APIKey,
			}
		}
		h.config.Knowledge = *req.Knowledge
		h.logger.Info("更新Knowledge配置",
			zap.Bool("enabled", h.config.Knowledge.Enabled),
			zap.String("base_path", h.config.Knowledge.BasePath),
			zap.String("embedding_model", h.config.Knowledge.Embedding.Model),
			zap.Int("retrieval_top_k", h.config.Knowledge.Retrieval.TopK),
			zap.Float64("similarity_threshold", h.config.Knowledge.Retrieval.SimilarityThreshold),
			zap.Float64("hybrid_weight", h.config.Knowledge.Retrieval.HybridWeight),
		)
	}

	// 更新工具启用状态
	if req.Tools != nil {
		// 分离内部工具和外部工具
		internalToolMap := make(map[string]bool)
		// 外部工具状态：MCP名称 -> 工具名称 -> 启用状态
		externalMCPToolMap := make(map[string]map[string]bool)

		for _, toolStatus := range req.Tools {
			if toolStatus.IsExternal && toolStatus.ExternalMCP != "" {
				// 外部工具：保存每个工具的独立状态
				mcpName := toolStatus.ExternalMCP
				if externalMCPToolMap[mcpName] == nil {
					externalMCPToolMap[mcpName] = make(map[string]bool)
				}
				externalMCPToolMap[mcpName][toolStatus.Name] = toolStatus.Enabled
			} else {
				// 内部工具
				internalToolMap[toolStatus.Name] = toolStatus.Enabled
			}
		}

		// 更新内部工具状态
		for i := range h.config.Security.Tools {
			if enabled, ok := internalToolMap[h.config.Security.Tools[i].Name]; ok {
				h.config.Security.Tools[i].Enabled = enabled
				h.logger.Info("更新工具启用状态",
					zap.String("tool", h.config.Security.Tools[i].Name),
					zap.Bool("enabled", enabled),
				)
			}
		}

		// 更新外部MCP工具状态
		if h.externalMCPMgr != nil {
			for mcpName, toolStates := range externalMCPToolMap {
				// 更新配置中的工具启用状态
				if h.config.ExternalMCP.Servers == nil {
					h.config.ExternalMCP.Servers = make(map[string]config.ExternalMCPServerConfig)
				}
				cfg, exists := h.config.ExternalMCP.Servers[mcpName]
				if !exists {
					h.logger.Warn("外部MCP配置不存在", zap.String("mcp", mcpName))
					continue
				}

				// 初始化ToolEnabled map
				if cfg.ToolEnabled == nil {
					cfg.ToolEnabled = make(map[string]bool)
				}

				// 更新每个工具的启用状态
				for toolName, enabled := range toolStates {
					cfg.ToolEnabled[toolName] = enabled
					h.logger.Info("更新外部工具启用状态",
						zap.String("mcp", mcpName),
						zap.String("tool", toolName),
						zap.Bool("enabled", enabled),
					)
				}

				// 检查是否有任何工具启用，如果有则启用MCP
				hasEnabledTool := false
				for _, enabled := range cfg.ToolEnabled {
					if enabled {
						hasEnabledTool = true
						break
					}
				}

				// 如果MCP之前未启用，但现在有工具启用，则启用MCP
				// 如果MCP之前已启用，保持启用状态（允许部分工具禁用）
				if !cfg.ExternalMCPEnable && hasEnabledTool {
					cfg.ExternalMCPEnable = true
					h.logger.Info("自动启用外部MCP（因为有工具启用）", zap.String("mcp", mcpName))
				}

				h.config.ExternalMCP.Servers[mcpName] = cfg
			}

			// 同步更新 externalMCPMgr 中的配置，确保 GetConfigs() 返回最新配置
			// 在循环外部统一更新，避免重复调用
			h.externalMCPMgr.LoadConfigs(&h.config.ExternalMCP)

			// 处理MCP连接状态（异步启动，避免阻塞）
			for mcpName := range externalMCPToolMap {
				cfg := h.config.ExternalMCP.Servers[mcpName]
				// 如果MCP需要启用，确保客户端已启动
				if cfg.ExternalMCPEnable {
					// 启动外部MCP（如果未启动）- 异步执行，避免阻塞
					client, exists := h.externalMCPMgr.GetClient(mcpName)
					if !exists || !client.IsConnected() {
						go func(name string) {
							if err := h.externalMCPMgr.StartClient(name); err != nil {
								h.logger.Warn("启动外部MCP失败",
									zap.String("mcp", name),
									zap.Error(err),
								)
							} else {
								h.logger.Info("启动外部MCP",
									zap.String("mcp", name),
								)
							}
						}(mcpName)
					}
				}
			}
		}
	}

	// 保存配置到文件
	if err := h.saveConfig(); err != nil {
		h.logger.Error("保存配置失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存配置失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "配置已更新"})
}

// ApplyConfig 应用配置（重新加载并重启相关服务）
func (h *ConfigHandler) ApplyConfig(c *gin.Context) {
	// 先检查是否需要动态初始化知识库（在锁外执行，避免阻塞其他请求）
	var needInitKnowledge bool
	var knowledgeInitializer KnowledgeInitializer

	h.mu.RLock()
	needInitKnowledge = h.config.Knowledge.Enabled && h.knowledgeToolRegistrar == nil && h.knowledgeInitializer != nil
	if needInitKnowledge {
		knowledgeInitializer = h.knowledgeInitializer
	}
	h.mu.RUnlock()

	// 如果需要动态初始化知识库，在锁外执行（这是耗时操作）
	if needInitKnowledge {
		h.logger.Info("检测到知识库从禁用变为启用，开始动态初始化知识库组件")
		if _, err := knowledgeInitializer(); err != nil {
			h.logger.Error("动态初始化知识库失败", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "初始化知识库失败: " + err.Error()})
			return
		}
		h.logger.Info("知识库动态初始化完成，工具已注册")
	}

	// 检查嵌入模型配置是否变更（需要在锁外执行，避免阻塞）
	var needReinitKnowledge bool
	var reinitKnowledgeInitializer KnowledgeInitializer
	h.mu.RLock()
	if h.config.Knowledge.Enabled && h.knowledgeInitializer != nil && h.lastEmbeddingConfig != nil {
		// 检查嵌入模型配置是否变更
		currentEmbedding := h.config.Knowledge.Embedding
		if currentEmbedding.Provider != h.lastEmbeddingConfig.Provider ||
			currentEmbedding.Model != h.lastEmbeddingConfig.Model ||
			currentEmbedding.BaseURL != h.lastEmbeddingConfig.BaseURL ||
			currentEmbedding.APIKey != h.lastEmbeddingConfig.APIKey {
			needReinitKnowledge = true
			reinitKnowledgeInitializer = h.knowledgeInitializer
			h.logger.Info("检测到嵌入模型配置变更，需要重新初始化知识库组件",
				zap.String("old_model", h.lastEmbeddingConfig.Model),
				zap.String("new_model", currentEmbedding.Model),
				zap.String("old_base_url", h.lastEmbeddingConfig.BaseURL),
				zap.String("new_base_url", currentEmbedding.BaseURL),
			)
		}
	}
	h.mu.RUnlock()

	// 如果需要重新初始化知识库（嵌入模型配置变更），在锁外执行
	if needReinitKnowledge {
		h.logger.Info("开始重新初始化知识库组件（嵌入模型配置已变更）")
		if _, err := reinitKnowledgeInitializer(); err != nil {
			h.logger.Error("重新初始化知识库失败", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "重新初始化知识库失败: " + err.Error()})
			return
		}
		h.logger.Info("知识库组件重新初始化完成")
	}

	// 现在获取写锁，执行快速的操作
	h.mu.Lock()
	defer h.mu.Unlock()

	// 如果重新初始化了知识库，更新嵌入模型配置记录
	if needReinitKnowledge && h.config.Knowledge.Enabled {
		h.lastEmbeddingConfig = &config.EmbeddingConfig{
			Provider: h.config.Knowledge.Embedding.Provider,
			Model:    h.config.Knowledge.Embedding.Model,
			BaseURL:  h.config.Knowledge.Embedding.BaseURL,
			APIKey:   h.config.Knowledge.Embedding.APIKey,
		}
		h.logger.Info("已更新嵌入模型配置记录")
	}

	// 重新注册工具（根据新的启用状态）
	h.logger.Info("重新注册工具")

	// 清空MCP服务器中的工具
	h.mcpServer.ClearTools()

	// 重新注册安全工具
	h.executor.RegisterTools(h.mcpServer)

	// 重新注册漏洞记录工具（内置工具，必须注册）
	if h.vulnerabilityToolRegistrar != nil {
		h.logger.Info("重新注册漏洞记录工具")
		if err := h.vulnerabilityToolRegistrar(); err != nil {
			h.logger.Error("重新注册漏洞记录工具失败", zap.Error(err))
		} else {
			h.logger.Info("漏洞记录工具已重新注册")
		}
	}

	// 重新注册Skills工具（内置工具，必须注册）
	if h.skillsToolRegistrar != nil {
		h.logger.Info("重新注册Skills工具")
		if err := h.skillsToolRegistrar(); err != nil {
			h.logger.Error("重新注册Skills工具失败", zap.Error(err))
		} else {
			h.logger.Info("Skills工具已重新注册")
		}
	}

	// 如果知识库启用，重新注册知识库工具
	if h.config.Knowledge.Enabled && h.knowledgeToolRegistrar != nil {
		h.logger.Info("重新注册知识库工具")
		if err := h.knowledgeToolRegistrar(); err != nil {
			h.logger.Error("重新注册知识库工具失败", zap.Error(err))
		} else {
			h.logger.Info("知识库工具已重新注册")
		}
	}

	// 更新Agent的OpenAI配置
	if h.agent != nil {
		h.agent.UpdateConfig(&h.config.OpenAI)
		h.agent.UpdateMaxIterations(h.config.Agent.MaxIterations)
		h.logger.Info("Agent配置已更新")
	}

	// 更新AttackChainHandler的OpenAI配置
	if h.attackChainHandler != nil {
		h.attackChainHandler.UpdateConfig(&h.config.OpenAI)
		h.logger.Info("AttackChainHandler配置已更新")
	}

	// 更新检索器配置（如果知识库启用）
	if h.config.Knowledge.Enabled && h.retrieverUpdater != nil {
		retrievalConfig := &knowledge.RetrievalConfig{
			TopK:                h.config.Knowledge.Retrieval.TopK,
			SimilarityThreshold: h.config.Knowledge.Retrieval.SimilarityThreshold,
			HybridWeight:        h.config.Knowledge.Retrieval.HybridWeight,
		}
		h.retrieverUpdater.UpdateConfig(retrievalConfig)
		h.logger.Info("检索器配置已更新",
			zap.Int("top_k", retrievalConfig.TopK),
			zap.Float64("similarity_threshold", retrievalConfig.SimilarityThreshold),
			zap.Float64("hybrid_weight", retrievalConfig.HybridWeight),
		)
	}

	// 更新嵌入模型配置记录（如果知识库启用）
	if h.config.Knowledge.Enabled {
		h.lastEmbeddingConfig = &config.EmbeddingConfig{
			Provider: h.config.Knowledge.Embedding.Provider,
			Model:    h.config.Knowledge.Embedding.Model,
			BaseURL:  h.config.Knowledge.Embedding.BaseURL,
			APIKey:   h.config.Knowledge.Embedding.APIKey,
		}
	}

	h.logger.Info("配置已应用",
		zap.Int("tools_count", len(h.config.Security.Tools)),
	)

	c.JSON(http.StatusOK, gin.H{
		"message":     "配置已应用",
		"tools_count": len(h.config.Security.Tools),
	})
}

// saveConfig 保存配置到文件
func (h *ConfigHandler) saveConfig() error {
	// 读取现有配置文件并创建备份
	data, err := os.ReadFile(h.configPath)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	if err := os.WriteFile(h.configPath+".backup", data, 0644); err != nil {
		h.logger.Warn("创建配置备份失败", zap.Error(err))
	}

	root, err := loadYAMLDocument(h.configPath)
	if err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	updateAgentConfig(root, h.config.Agent.MaxIterations)
	updateMCPConfig(root, h.config.MCP)
	updateOpenAIConfig(root, h.config.OpenAI)
	updateKnowledgeConfig(root, h.config.Knowledge)
	// 更新外部MCP配置（使用external_mcp.go中的函数，同一包中可直接调用）
	// 读取原始配置以保持向后兼容
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
		return fmt.Errorf("保存配置文件失败: %w", err)
	}

	// 更新工具配置文件中的enabled状态
	if h.config.Security.ToolsDir != "" {
		configDir := filepath.Dir(h.configPath)
		toolsDir := h.config.Security.ToolsDir
		if !filepath.IsAbs(toolsDir) {
			toolsDir = filepath.Join(configDir, toolsDir)
		}

		for _, tool := range h.config.Security.Tools {
			toolFile := filepath.Join(toolsDir, tool.Name+".yaml")
			// 检查文件是否存在
			if _, err := os.Stat(toolFile); os.IsNotExist(err) {
				// 尝试.yml扩展名
				toolFile = filepath.Join(toolsDir, tool.Name+".yml")
				if _, err := os.Stat(toolFile); os.IsNotExist(err) {
					h.logger.Warn("工具配置文件不存在", zap.String("tool", tool.Name))
					continue
				}
			}

			toolDoc, err := loadYAMLDocument(toolFile)
			if err != nil {
				h.logger.Warn("解析工具配置失败", zap.String("tool", tool.Name), zap.Error(err))
				continue
			}

			setBoolInMap(toolDoc.Content[0], "enabled", tool.Enabled)

			if err := writeYAMLDocument(toolFile, toolDoc); err != nil {
				h.logger.Warn("保存工具配置文件失败", zap.String("tool", tool.Name), zap.Error(err))
				continue
			}

			h.logger.Info("更新工具配置", zap.String("tool", tool.Name), zap.Bool("enabled", tool.Enabled))
		}
	}

	h.logger.Info("配置已保存", zap.String("path", h.configPath))
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
}

func updateKnowledgeConfig(doc *yaml.Node, cfg config.KnowledgeConfig) {
	root := doc.Content[0]
	knowledgeNode := ensureMap(root, "knowledge")
	setBoolInMap(knowledgeNode, "enabled", cfg.Enabled)
	setStringInMap(knowledgeNode, "base_path", cfg.BasePath)

	// 更新嵌入配置
	embeddingNode := ensureMap(knowledgeNode, "embedding")
	setStringInMap(embeddingNode, "provider", cfg.Embedding.Provider)
	setStringInMap(embeddingNode, "model", cfg.Embedding.Model)
	if cfg.Embedding.BaseURL != "" {
		setStringInMap(embeddingNode, "base_url", cfg.Embedding.BaseURL)
	}
	if cfg.Embedding.APIKey != "" {
		setStringInMap(embeddingNode, "api_key", cfg.Embedding.APIKey)
	}

	// 更新检索配置
	retrievalNode := ensureMap(knowledgeNode, "retrieval")
	setIntInMap(retrievalNode, "top_k", cfg.Retrieval.TopK)
	setFloatInMap(retrievalNode, "similarity_threshold", cfg.Retrieval.SimilarityThreshold)
	setFloatInMap(retrievalNode, "hybrid_weight", cfg.Retrieval.HybridWeight)
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
	// 对于0.0到1.0之间的值（如hybrid_weight），使用%.1f确保0.0被明确序列化为"0.0"
	// 对于其他值，使用%g自动选择最合适的格式
	if value >= 0.0 && value <= 1.0 {
		valueNode.Value = fmt.Sprintf("%.1f", value)
	} else {
		valueNode.Value = fmt.Sprintf("%g", value)
	}
}
