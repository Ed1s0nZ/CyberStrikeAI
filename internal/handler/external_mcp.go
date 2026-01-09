package handler

import (
	"fmt"
	"net/http"
	"os"
	"sync"

	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/mcp"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// ExternalMCPHandler 外部MCP处理器
type ExternalMCPHandler struct {
	manager    *mcp.ExternalMCPManager
	config     *config.Config
	configPath string
	logger     *zap.Logger
	mu         sync.RWMutex
}

// NewExternalMCPHandler 创建外部MCP处理器
func NewExternalMCPHandler(manager *mcp.ExternalMCPManager, cfg *config.Config, configPath string, logger *zap.Logger) *ExternalMCPHandler {
	return &ExternalMCPHandler{
		manager:    manager,
		config:     cfg,
		configPath: configPath,
		logger:     logger,
	}
}

// GetExternalMCPs 获取所有外部MCP配置
func (h *ExternalMCPHandler) GetExternalMCPs(c *gin.Context) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	configs := h.manager.GetConfigs()
	
	// 获取所有外部MCP的工具数量
	toolCounts := h.manager.GetToolCounts()
	
	// 转换为响应格式
	result := make(map[string]ExternalMCPResponse)
	for name, cfg := range configs {
		client, exists := h.manager.GetClient(name)
		status := "disconnected"
		if exists {
			status = client.GetStatus()
		} else if h.isEnabled(cfg) {
			status = "disconnected"
		} else {
			status = "disabled"
		}
		
		toolCount := toolCounts[name]
		errorMsg := ""
		if status == "error" {
			errorMsg = h.manager.GetError(name)
		}
		
		result[name] = ExternalMCPResponse{
			Config:    cfg,
			Status:    status,
			ToolCount: toolCount,
			Error:     errorMsg,
		}
	}
	
	c.JSON(http.StatusOK, gin.H{
		"servers": result,
		"stats":   h.manager.GetStats(),
	})
}

// GetExternalMCP 获取单个外部MCP配置
func (h *ExternalMCPHandler) GetExternalMCP(c *gin.Context) {
	name := c.Param("name")
	
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	configs := h.manager.GetConfigs()
	cfg, exists := configs[name]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "外部MCP配置不存在"})
		return
	}
	
	client, clientExists := h.manager.GetClient(name)
	status := "disconnected"
	if clientExists {
		status = client.GetStatus()
	} else if h.isEnabled(cfg) {
		status = "disconnected"
	} else {
		status = "disabled"
	}
	
	// 获取工具数量
	toolCount := 0
	if clientExists && client.IsConnected() {
		if count, err := h.manager.GetToolCount(name); err == nil {
			toolCount = count
		}
	}
	
	// 获取错误信息
	errorMsg := ""
	if status == "error" {
		errorMsg = h.manager.GetError(name)
	}
	
	c.JSON(http.StatusOK, ExternalMCPResponse{
		Config:    cfg,
		Status:    status,
		ToolCount: toolCount,
		Error:     errorMsg,
	})
}

// AddOrUpdateExternalMCP 添加或更新外部MCP配置
func (h *ExternalMCPHandler) AddOrUpdateExternalMCP(c *gin.Context) {
	var req AddOrUpdateExternalMCPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数: " + err.Error()})
		return
	}
	
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "名称不能为空"})
		return
	}
	
	// 验证配置
	if err := h.validateConfig(req.Config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	h.mu.Lock()
	defer h.mu.Unlock()
	
	// 添加或更新配置
	if err := h.manager.AddOrUpdateConfig(name, req.Config); err != nil {
		h.logger.Error("添加或更新外部MCP配置失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "添加或更新配置失败: " + err.Error()})
		return
	}
	
	// 更新内存中的配置
	if h.config.ExternalMCP.Servers == nil {
		h.config.ExternalMCP.Servers = make(map[string]config.ExternalMCPServerConfig)
	}
	
	// 如果用户提供了 disabled 或 enabled 字段，保留它们以保持向后兼容
	// 同时将值迁移到 external_mcp_enable
	cfg := req.Config
	
	if req.Config.Disabled {
		// 用户设置了 disabled: true
		cfg.ExternalMCPEnable = false
		cfg.Disabled = true
		cfg.Enabled = false
	} else if req.Config.Enabled {
		// 用户设置了 enabled: true
		cfg.ExternalMCPEnable = true
		cfg.Enabled = true
		cfg.Disabled = false
	} else if !req.Config.ExternalMCPEnable {
		// 用户没有设置任何字段，且 external_mcp_enable 为 false
		// 检查现有配置是否有旧字段
		if existingCfg, exists := h.config.ExternalMCP.Servers[name]; exists {
			// 保留现有的旧字段
			cfg.Enabled = existingCfg.Enabled
			cfg.Disabled = existingCfg.Disabled
		}
	} else {
		// 用户通过新字段启用了（external_mcp_enable: true），但没有设置旧字段
		// 为了向后兼容，我们设置 enabled: true
		// 这样即使原始配置中有 disabled: false，也会被转换为 enabled: true
		cfg.Enabled = true
		cfg.Disabled = false
	}
	
	h.config.ExternalMCP.Servers[name] = cfg
	
	// 保存到配置文件
	if err := h.saveConfig(); err != nil {
		h.logger.Error("保存配置失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存配置失败: " + err.Error()})
		return
	}
	
	h.logger.Info("外部MCP配置已更新", zap.String("name", name))
	c.JSON(http.StatusOK, gin.H{"message": "配置已更新"})
}

// DeleteExternalMCP 删除外部MCP配置
func (h *ExternalMCPHandler) DeleteExternalMCP(c *gin.Context) {
	name := c.Param("name")
	
	h.mu.Lock()
	defer h.mu.Unlock()
	
	// 移除配置
	if err := h.manager.RemoveConfig(name); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "配置不存在"})
		return
	}
	
	// 从内存配置中删除
	if h.config.ExternalMCP.Servers != nil {
		delete(h.config.ExternalMCP.Servers, name)
	}
	
	// 保存到配置文件
	if err := h.saveConfig(); err != nil {
		h.logger.Error("保存配置失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存配置失败: " + err.Error()})
		return
	}
	
	h.logger.Info("外部MCP配置已删除", zap.String("name", name))
	c.JSON(http.StatusOK, gin.H{"message": "配置已删除"})
}

// StartExternalMCP 启动外部MCP
func (h *ExternalMCPHandler) StartExternalMCP(c *gin.Context) {
	name := c.Param("name")
	
	h.mu.Lock()
	defer h.mu.Unlock()
	
	// 更新配置为启用
	if h.config.ExternalMCP.Servers == nil {
		h.config.ExternalMCP.Servers = make(map[string]config.ExternalMCPServerConfig)
	}
	cfg := h.config.ExternalMCP.Servers[name]
	cfg.ExternalMCPEnable = true
	h.config.ExternalMCP.Servers[name] = cfg
	
	// 保存到配置文件
	if err := h.saveConfig(); err != nil {
		h.logger.Error("保存配置失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存配置失败: " + err.Error()})
		return
	}
	
	// 启动客户端（立即创建客户端并设置状态为connecting，实际连接在后台进行）
	h.logger.Info("开始启动外部MCP", zap.String("name", name))
	if err := h.manager.StartClient(name); err != nil {
		h.logger.Error("启动外部MCP失败", zap.String("name", name), zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
			"status": "error",
		})
		return
	}
	
	// 获取客户端状态（应该是connecting）
	client, exists := h.manager.GetClient(name)
	status := "connecting"
	if exists {
		status = client.GetStatus()
	}
	
	// 立即返回，不等待连接完成
	// 客户端会在后台异步连接，用户可以通过状态查询接口查看连接状态
	c.JSON(http.StatusOK, gin.H{
		"message": "外部MCP启动请求已提交，正在后台连接中",
		"status":  status,
	})
}

// StopExternalMCP 停止外部MCP
func (h *ExternalMCPHandler) StopExternalMCP(c *gin.Context) {
	name := c.Param("name")
	
	h.mu.Lock()
	defer h.mu.Unlock()
	
	// 停止客户端
	if err := h.manager.StopClient(name); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	// 更新配置
	if h.config.ExternalMCP.Servers == nil {
		h.config.ExternalMCP.Servers = make(map[string]config.ExternalMCPServerConfig)
	}
	cfg := h.config.ExternalMCP.Servers[name]
	cfg.ExternalMCPEnable = false
	h.config.ExternalMCP.Servers[name] = cfg
	
	// 保存到配置文件
	if err := h.saveConfig(); err != nil {
		h.logger.Error("保存配置失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存配置失败: " + err.Error()})
		return
	}
	
	h.logger.Info("外部MCP已停止", zap.String("name", name))
	c.JSON(http.StatusOK, gin.H{"message": "外部MCP已停止"})
}

// GetExternalMCPStats 获取统计信息
func (h *ExternalMCPHandler) GetExternalMCPStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, stats)
}

// validateConfig 验证配置
func (h *ExternalMCPHandler) validateConfig(cfg config.ExternalMCPServerConfig) error {
	transport := cfg.Transport
	if transport == "" {
		// 如果没有指定transport，根据是否有command或url判断
		if cfg.Command != "" {
			transport = "stdio"
		} else if cfg.URL != "" {
			transport = "http"
		} else {
			return fmt.Errorf("需要指定command（stdio模式）或url（http/sse模式）")
		}
	}
	
	switch transport {
	case "http":
		if cfg.URL == "" {
			return fmt.Errorf("HTTP模式需要URL")
		}
	case "stdio":
		if cfg.Command == "" {
			return fmt.Errorf("stdio模式需要command")
		}
	case "sse":
		if cfg.URL == "" {
			return fmt.Errorf("SSE模式需要URL")
		}
	default:
		return fmt.Errorf("不支持的传输模式: %s，支持的模式: http, stdio, sse", transport)
	}
	
	return nil
}

// isEnabled 检查是否启用
func (h *ExternalMCPHandler) isEnabled(cfg config.ExternalMCPServerConfig) bool {
	// 优先使用 ExternalMCPEnable 字段
	// 如果没有设置，检查旧的 enabled/disabled 字段（向后兼容）
	if cfg.ExternalMCPEnable {
		return true
	}
	// 向后兼容：检查旧字段
	if cfg.Disabled {
		return false
	}
	if cfg.Enabled {
		return true
	}
	// 都没有设置，默认为启用
	return true
}

// saveConfig 保存配置到文件
func (h *ExternalMCPHandler) saveConfig() error {
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

	// 在更新前，读取原始配置中的 enabled/disabled 字段，以便保持向后兼容
	originalConfigs := make(map[string]map[string]bool)
	externalMCPNode := findMapValue(root.Content[0], "external_mcp")
	if externalMCPNode != nil && externalMCPNode.Kind == yaml.MappingNode {
		serversNode := findMapValue(externalMCPNode, "servers")
		if serversNode != nil && serversNode.Kind == yaml.MappingNode {
			// 遍历现有的服务器配置，保存 enabled/disabled 字段
			for i := 0; i < len(serversNode.Content); i += 2 {
				if i+1 >= len(serversNode.Content) {
					break
				}
				nameNode := serversNode.Content[i]
				serverNode := serversNode.Content[i+1]
				if nameNode.Kind == yaml.ScalarNode && serverNode.Kind == yaml.MappingNode {
					serverName := nameNode.Value
					originalConfigs[serverName] = make(map[string]bool)
					// 检查是否有 enabled 字段
					if enabledVal := findBoolInMap(serverNode, "enabled"); enabledVal != nil {
						originalConfigs[serverName]["enabled"] = *enabledVal
					}
					// 检查是否有 disabled 字段
					if disabledVal := findBoolInMap(serverNode, "disabled"); disabledVal != nil {
						originalConfigs[serverName]["disabled"] = *disabledVal
					}
				}
			}
		}
	}

	// 更新外部MCP配置
	updateExternalMCPConfig(root, h.config.ExternalMCP, originalConfigs)

	if err := writeYAMLDocument(h.configPath, root); err != nil {
		return fmt.Errorf("保存配置文件失败: %w", err)
	}

	h.logger.Info("配置已保存", zap.String("path", h.configPath))
	return nil
}

// updateExternalMCPConfig 更新外部MCP配置
func updateExternalMCPConfig(doc *yaml.Node, cfg config.ExternalMCPConfig, originalConfigs map[string]map[string]bool) {
	root := doc.Content[0]
	externalMCPNode := ensureMap(root, "external_mcp")
	serversNode := ensureMap(externalMCPNode, "servers")
	
	// 清空现有服务器配置
	serversNode.Content = nil
	
	// 添加新的服务器配置
	for name, serverCfg := range cfg.Servers {
		// 添加服务器名称键
		nameNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: name}
		serverNode := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		serversNode.Content = append(serversNode.Content, nameNode, serverNode)
		
		// 设置服务器配置字段
		if serverCfg.Command != "" {
			setStringInMap(serverNode, "command", serverCfg.Command)
		}
		if len(serverCfg.Args) > 0 {
			setStringArrayInMap(serverNode, "args", serverCfg.Args)
		}
		// 保存 env 字段（环境变量）
		if serverCfg.Env != nil && len(serverCfg.Env) > 0 {
			envNode := ensureMap(serverNode, "env")
			for envKey, envValue := range serverCfg.Env {
				setStringInMap(envNode, envKey, envValue)
			}
		}
		if serverCfg.Transport != "" {
			setStringInMap(serverNode, "transport", serverCfg.Transport)
		}
		if serverCfg.URL != "" {
			setStringInMap(serverNode, "url", serverCfg.URL)
		}
		if serverCfg.Description != "" {
			setStringInMap(serverNode, "description", serverCfg.Description)
		}
		if serverCfg.Timeout > 0 {
			setIntInMap(serverNode, "timeout", serverCfg.Timeout)
		}
		// 保存 external_mcp_enable 字段（新字段）
		setBoolInMap(serverNode, "external_mcp_enable", serverCfg.ExternalMCPEnable)
		// 保存 tool_enabled 字段（每个工具的启用状态）
		if serverCfg.ToolEnabled != nil && len(serverCfg.ToolEnabled) > 0 {
			toolEnabledNode := ensureMap(serverNode, "tool_enabled")
			for toolName, enabled := range serverCfg.ToolEnabled {
				setBoolInMap(toolEnabledNode, toolName, enabled)
			}
		}
		// 保留旧的 enabled/disabled 字段以保持向后兼容
		originalFields, hasOriginal := originalConfigs[name]
		
		// 如果原始配置中有 enabled 字段，保留它
		if hasOriginal {
			if enabledVal, hasEnabled := originalFields["enabled"]; hasEnabled {
				setBoolInMap(serverNode, "enabled", enabledVal)
			}
			// 如果原始配置中有 disabled 字段，保留它
			// 注意：由于 omitempty，disabled: false 不会被保存，但 disabled: true 会被保存
			if disabledVal, hasDisabled := originalFields["disabled"]; hasDisabled {
				if disabledVal {
					setBoolInMap(serverNode, "disabled", disabledVal)
				} else {
					// 如果原始配置中有 disabled: false，我们保存 enabled: true 来等效表示
					// 因为 disabled: false 等价于 enabled: true
					setBoolInMap(serverNode, "enabled", true)
				}
			}
		}
		
		// 如果用户在当前请求中明确设置了这些字段，也保存它们
		if serverCfg.Enabled {
			setBoolInMap(serverNode, "enabled", serverCfg.Enabled)
		}
		if serverCfg.Disabled {
			setBoolInMap(serverNode, "disabled", serverCfg.Disabled)
		} else if !hasOriginal && serverCfg.ExternalMCPEnable {
			// 如果用户通过新字段启用了，且原始配置中没有旧字段，保存 enabled: true 以保持向后兼容
			setBoolInMap(serverNode, "enabled", true)
		}
	}
}

// setStringArrayInMap 设置字符串数组
func setStringArrayInMap(mapNode *yaml.Node, key string, values []string) {
	_, valueNode := ensureKeyValue(mapNode, key)
	valueNode.Kind = yaml.SequenceNode
	valueNode.Tag = "!!seq"
	valueNode.Content = nil
	for _, v := range values {
		itemNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v}
		valueNode.Content = append(valueNode.Content, itemNode)
	}
}

// AddOrUpdateExternalMCPRequest 添加或更新外部MCP请求
type AddOrUpdateExternalMCPRequest struct {
	Config config.ExternalMCPServerConfig `json:"config"`
}

// ExternalMCPResponse 外部MCP响应
type ExternalMCPResponse struct {
	Config    config.ExternalMCPServerConfig `json:"config"`
	Status    string                         `json:"status"` // "connected", "disconnected", "disabled", "error", "connecting"
	ToolCount int                            `json:"tool_count"` // 工具数量
	Error     string                         `json:"error,omitempty"` // 错误信息（仅在status为error时存在）
}

