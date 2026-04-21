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

// ExternalMCPHandler handles external MCP operations
type ExternalMCPHandler struct {
	manager    *mcp.ExternalMCPManager
	config     *config.Config
	configPath string
	logger     *zap.Logger
	mu         sync.RWMutex
}

// NewExternalMCPHandler creates a new external MCP handler
func NewExternalMCPHandler(manager *mcp.ExternalMCPManager, cfg *config.Config, configPath string, logger *zap.Logger) *ExternalMCPHandler {
	return &ExternalMCPHandler{
		manager:    manager,
		config:     cfg,
		configPath: configPath,
		logger:     logger,
	}
}

// GetExternalMCPs retrieves all external MCP configurations
func (h *ExternalMCPHandler) GetExternalMCPs(c *gin.Context) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	configs := h.manager.GetConfigs()

	// get tool counts for all external MCPs
	toolCounts := h.manager.GetToolCounts()

	// convert to response format
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

// GetExternalMCP retrieves a single external MCP configuration
func (h *ExternalMCPHandler) GetExternalMCP(c *gin.Context) {
	name := c.Param("name")

	h.mu.RLock()
	defer h.mu.RUnlock()

	configs := h.manager.GetConfigs()
	cfg, exists := configs[name]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "external MCP configuration not found"})
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

	// get tool count
	toolCount := 0
	if clientExists && client.IsConnected() {
		if count, err := h.manager.GetToolCount(name); err == nil {
			toolCount = count
		}
	}

	// get error message
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

// AddOrUpdateExternalMCP adds or updates an external MCP configuration
func (h *ExternalMCPHandler) AddOrUpdateExternalMCP(c *gin.Context) {
	var req AddOrUpdateExternalMCPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request parameters: " + err.Error()})
		return
	}

	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name cannot be empty"})
		return
	}

	// validate configuration
	if err := h.validateConfig(req.Config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	// add or update configuration
	if err := h.manager.AddOrUpdateConfig(name, req.Config); err != nil {
		h.logger.Error("failed to add or update external MCP configuration", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to add or update configuration: " + err.Error()})
		return
	}

	// update in-memory configuration
	if h.config.ExternalMCP.Servers == nil {
		h.config.ExternalMCP.Servers = make(map[string]config.ExternalMCPServerConfig)
	}

	// if the user provided disabled or enabled fields, retain them for backward compatibility
	// also migrate the values to external_mcp_enable
	cfg := req.Config

	if req.Config.Disabled {
		// user set disabled: true
		cfg.ExternalMCPEnable = false
		cfg.Disabled = true
		cfg.Enabled = false
	} else if req.Config.Enabled {
		// user set enabled: true
		cfg.ExternalMCPEnable = true
		cfg.Enabled = true
		cfg.Disabled = false
	} else if !req.Config.ExternalMCPEnable {
		// user did not set any field and external_mcp_enable is false
		// check if existing configuration has legacy fields
		if existingCfg, exists := h.config.ExternalMCP.Servers[name]; exists {
			// retain existing legacy fields
			cfg.Enabled = existingCfg.Enabled
			cfg.Disabled = existingCfg.Disabled
		}
	} else {
		// user enabled via the new field (external_mcp_enable: true) without setting legacy fields
		// for backward compatibility, set enabled: true
		// this way even if the original config has disabled: false, it is converted to enabled: true
		cfg.Enabled = true
		cfg.Disabled = false
	}

	// Env-var resolution happens lazily at connection time inside the mcp
	// package - do NOT expand here, because cfg is then stored in h.config
	// and written back to disk by saveConfig, which would persist resolved
	// secrets into config.yaml and defeat the point of using ${VAR} refs.
	h.config.ExternalMCP.Servers[name] = cfg

	// save to config file
	if err := h.saveConfig(); err != nil {
		h.logger.Error("failed to save configuration", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save configuration: " + err.Error()})
		return
	}

	h.logger.Info("external MCP configuration updated", zap.String("name", name))
	c.JSON(http.StatusOK, gin.H{"message": "configuration updated"})
}

// DeleteExternalMCP deletes an external MCP configuration
func (h *ExternalMCPHandler) DeleteExternalMCP(c *gin.Context) {
	name := c.Param("name")

	h.mu.Lock()
	defer h.mu.Unlock()

	// remove configuration
	if err := h.manager.RemoveConfig(name); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "configuration not found"})
		return
	}

	// delete from in-memory configuration
	if h.config.ExternalMCP.Servers != nil {
		delete(h.config.ExternalMCP.Servers, name)
	}

	// save to config file
	if err := h.saveConfig(); err != nil {
		h.logger.Error("failed to save configuration", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save configuration: " + err.Error()})
		return
	}

	h.logger.Info("external MCP configuration deleted", zap.String("name", name))
	c.JSON(http.StatusOK, gin.H{"message": "configuration deleted"})
}

// StartExternalMCP starts an external MCP
func (h *ExternalMCPHandler) StartExternalMCP(c *gin.Context) {
	name := c.Param("name")

	h.mu.Lock()
	defer h.mu.Unlock()

	// update configuration to enabled
	if h.config.ExternalMCP.Servers == nil {
		h.config.ExternalMCP.Servers = make(map[string]config.ExternalMCPServerConfig)
	}
	cfg := h.config.ExternalMCP.Servers[name]
	cfg.ExternalMCPEnable = true
	h.config.ExternalMCP.Servers[name] = cfg

	// save to config file
	if err := h.saveConfig(); err != nil {
		h.logger.Error("failed to save configuration", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save configuration: " + err.Error()})
		return
	}

	// start the client (immediately create the client and set status to connecting, actual connection happens in the background)
	h.logger.Info("starting external MCP", zap.String("name", name))
	if err := h.manager.StartClient(name); err != nil {
		h.logger.Error("failed to start external MCP", zap.String("name", name), zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":  err.Error(),
			"status": "error",
		})
		return
	}

	// get client status (should be connecting)
	client, exists := h.manager.GetClient(name)
	status := "connecting"
	if exists {
		status = client.GetStatus()
	}

	// return immediately without waiting for connection to complete
	// the client will connect asynchronously in the background; users can check connection status via the status query endpoint
	c.JSON(http.StatusOK, gin.H{
		"message": "external MCP start request submitted, connecting in the background",
		"status":  status,
	})
}

// StopExternalMCP stops an external MCP
func (h *ExternalMCPHandler) StopExternalMCP(c *gin.Context) {
	name := c.Param("name")

	h.mu.Lock()
	defer h.mu.Unlock()

	// stop the client
	if err := h.manager.StopClient(name); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// update configuration
	if h.config.ExternalMCP.Servers == nil {
		h.config.ExternalMCP.Servers = make(map[string]config.ExternalMCPServerConfig)
	}
	cfg := h.config.ExternalMCP.Servers[name]
	cfg.ExternalMCPEnable = false
	h.config.ExternalMCP.Servers[name] = cfg

	// save to config file
	if err := h.saveConfig(); err != nil {
		h.logger.Error("failed to save configuration", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save configuration: " + err.Error()})
		return
	}

	h.logger.Info("external MCP stopped", zap.String("name", name))
	c.JSON(http.StatusOK, gin.H{"message": "external MCP stopped"})
}

// GetExternalMCPStats retrieves statistics
func (h *ExternalMCPHandler) GetExternalMCPStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, stats)
}

// validateConfig validates the configuration
func (h *ExternalMCPHandler) validateConfig(cfg config.ExternalMCPServerConfig) error {
	transport := cfg.Transport
	if transport == "" {
		// if transport is not specified, infer from command or url
		if cfg.Command != "" {
			transport = "stdio"
		} else if cfg.URL != "" {
			transport = "http"
		} else {
			return fmt.Errorf("command (stdio mode) or url (http/sse mode) is required")
		}
	}

	switch transport {
	case "http":
		if cfg.URL == "" {
			return fmt.Errorf("HTTP mode requires a URL")
		}
	case "stdio":
		if cfg.Command == "" {
			return fmt.Errorf("stdio mode requires a command")
		}
	case "sse":
		if cfg.URL == "" {
			return fmt.Errorf("SSE mode requires a URL")
		}
	default:
		return fmt.Errorf("unsupported transport mode: %s, supported modes: http, stdio, sse", transport)
	}

	return nil
}

// isEnabled checks whether the configuration is enabled
func (h *ExternalMCPHandler) isEnabled(cfg config.ExternalMCPServerConfig) bool {
	// prefer ExternalMCPEnable field
	// if not set, check legacy enabled/disabled fields (backward compatibility)
	if cfg.ExternalMCPEnable {
		return true
	}
	// backward compatibility: check legacy fields
	if cfg.Disabled {
		return false
	}
	if cfg.Enabled {
		return true
	}
	// neither field set, default to enabled
	return true
}

// saveConfig saves the configuration to file
func (h *ExternalMCPHandler) saveConfig() error {
	// read existing config file and create backup
	data, err := os.ReadFile(h.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	if err := os.WriteFile(h.configPath+".backup", data, 0644); err != nil {
		h.logger.Warn("failed to create config backup", zap.Error(err))
	}

	root, err := loadYAMLDocument(h.configPath)
	if err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	// before updating, read the original config's enabled/disabled fields to maintain backward compatibility
	originalConfigs := make(map[string]map[string]bool)
	externalMCPNode := findMapValue(root.Content[0], "external_mcp")
	if externalMCPNode != nil && externalMCPNode.Kind == yaml.MappingNode {
		serversNode := findMapValue(externalMCPNode, "servers")
		if serversNode != nil && serversNode.Kind == yaml.MappingNode {
			// iterate existing server configs and save enabled/disabled fields
			for i := 0; i < len(serversNode.Content); i += 2 {
				if i+1 >= len(serversNode.Content) {
					break
				}
				nameNode := serversNode.Content[i]
				serverNode := serversNode.Content[i+1]
				if nameNode.Kind == yaml.ScalarNode && serverNode.Kind == yaml.MappingNode {
					serverName := nameNode.Value
					originalConfigs[serverName] = make(map[string]bool)
					// check for enabled field
					if enabledVal := findBoolInMap(serverNode, "enabled"); enabledVal != nil {
						originalConfigs[serverName]["enabled"] = *enabledVal
					}
					// check for disabled field
					if disabledVal := findBoolInMap(serverNode, "disabled"); disabledVal != nil {
						originalConfigs[serverName]["disabled"] = *disabledVal
					}
				}
			}
		}
	}

	// update external MCP configuration
	updateExternalMCPConfig(root, h.config.ExternalMCP, originalConfigs)

	if err := writeYAMLDocument(h.configPath, root); err != nil {
		return fmt.Errorf("failed to save config file: %w", err)
	}

	h.logger.Info("configuration saved", zap.String("path", h.configPath))
	return nil
}

// updateExternalMCPConfig updates the external MCP configuration
func updateExternalMCPConfig(doc *yaml.Node, cfg config.ExternalMCPConfig, originalConfigs map[string]map[string]bool) {
	root := doc.Content[0]
	externalMCPNode := ensureMap(root, "external_mcp")
	serversNode := ensureMap(externalMCPNode, "servers")

	// clear existing server configurations
	serversNode.Content = nil

	// add new server configurations
	for name, serverCfg := range cfg.Servers {
		// add server name key
		nameNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: name}
		serverNode := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		serversNode.Content = append(serversNode.Content, nameNode, serverNode)

		// set server config fields
		if serverCfg.Command != "" {
			setStringInMap(serverNode, "command", serverCfg.Command)
		}
		if len(serverCfg.Args) > 0 {
			setStringArrayInMap(serverNode, "args", serverCfg.Args)
		}
		// save env field (environment variables)
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
		// save headers field (HTTP/SSE request headers)
		if serverCfg.Headers != nil && len(serverCfg.Headers) > 0 {
			headersNode := ensureMap(serverNode, "headers")
			for k, v := range serverCfg.Headers {
				setStringInMap(headersNode, k, v)
			}
		}
		if serverCfg.Description != "" {
			setStringInMap(serverNode, "description", serverCfg.Description)
		}
		if serverCfg.Timeout > 0 {
			setIntInMap(serverNode, "timeout", serverCfg.Timeout)
		}
		// save external_mcp_enable field (new field)
		setBoolInMap(serverNode, "external_mcp_enable", serverCfg.ExternalMCPEnable)
		// save tool_enabled field (per-tool enable status)
		if serverCfg.ToolEnabled != nil && len(serverCfg.ToolEnabled) > 0 {
			toolEnabledNode := ensureMap(serverNode, "tool_enabled")
			for toolName, enabled := range serverCfg.ToolEnabled {
				setBoolInMap(toolEnabledNode, toolName, enabled)
			}
		}
		// retain legacy enabled/disabled fields for backward compatibility
		originalFields, hasOriginal := originalConfigs[name]

		// if original config has enabled field, retain it
		if hasOriginal {
			if enabledVal, hasEnabled := originalFields["enabled"]; hasEnabled {
				setBoolInMap(serverNode, "enabled", enabledVal)
			}
			// if original config has disabled field, retain it
			// note: due to omitempty, disabled: false won't be saved, but disabled: true will
			if disabledVal, hasDisabled := originalFields["disabled"]; hasDisabled {
				if disabledVal {
					setBoolInMap(serverNode, "disabled", disabledVal)
				} else {
					// if original config has disabled: false, save enabled: true as equivalent
					// because disabled: false is equivalent to enabled: true
					setBoolInMap(serverNode, "enabled", true)
				}
			}
		}

		// if the user explicitly set these fields in the current request, also save them
		if serverCfg.Enabled {
			setBoolInMap(serverNode, "enabled", serverCfg.Enabled)
		}
		if serverCfg.Disabled {
			setBoolInMap(serverNode, "disabled", serverCfg.Disabled)
		} else if !hasOriginal && serverCfg.ExternalMCPEnable {
			// if user enabled via the new field and there are no legacy fields in the original config,
			// save enabled: true for backward compatibility
			setBoolInMap(serverNode, "enabled", true)
		}
	}
}

// setStringArrayInMap sets a string array value in a map node
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

// AddOrUpdateExternalMCPRequest is the request for adding or updating an external MCP
type AddOrUpdateExternalMCPRequest struct {
	Config config.ExternalMCPServerConfig `json:"config"`
}

// ExternalMCPResponse is the external MCP response
type ExternalMCPResponse struct {
	Config    config.ExternalMCPServerConfig `json:"config"`
	Status    string                         `json:"status"`          // "connected", "disconnected", "disabled", "error", "connecting"
	ToolCount int                            `json:"tool_count"`      // tool count
	Error     string                         `json:"error,omitempty"` // error message (only present when status is error)
}
