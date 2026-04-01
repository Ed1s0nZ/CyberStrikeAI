package handler

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/mcp"
	"cyberstrike-ai/internal/plugins"
	"cyberstrike-ai/internal/security"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// PluginsHandler handles REST API requests for plugin management.
type PluginsHandler struct {
	manager   *plugins.Manager
	executor  *security.Executor
	mcpServer *mcp.Server
	secConfig *config.SecurityConfig
	logger    *zap.Logger
}

// NewPluginsHandler creates a new PluginsHandler.
func NewPluginsHandler(
	manager *plugins.Manager,
	executor *security.Executor,
	mcpServer *mcp.Server,
	secConfig *config.SecurityConfig,
	logger *zap.Logger,
) *PluginsHandler {
	return &PluginsHandler{
		manager:   manager,
		executor:  executor,
		mcpServer: mcpServer,
		secConfig: secConfig,
		logger:    logger,
	}
}

// ListPlugins returns all discovered plugins with their state.
// GET /api/plugins
func (h *PluginsHandler) ListPlugins(c *gin.Context) {
	pluginsList := h.manager.GetPlugins()

	// Mask config values for security
	for i := range pluginsList {
		for j := range pluginsList[i].Manifest.Config {
			if pluginsList[i].Manifest.Config[j].Value != "" {
				pluginsList[i].Manifest.Config[j].Value = "********"
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"plugins": pluginsList,
		"count":   len(pluginsList),
	})
}

// EnablePlugin enables a plugin and registers its tools with the MCP server.
// POST /api/plugins/:name/enable
func (h *PluginsHandler) EnablePlugin(c *gin.Context) {
	name := c.Param("name")

	if err := h.manager.EnablePlugin(name); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Hot-load: register plugin tools with MCP server
	tools, err := h.manager.GetPluginTools(name)
	if err != nil {
		h.logger.Warn("failed to load tools after enabling plugin",
			zap.String("plugin", name),
			zap.Error(err),
		)
	} else {
		h.registerPluginTools(name, tools)
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    fmt.Sprintf("plugin %q enabled", name),
		"tools_loaded": len(tools),
	})
}

// DisablePlugin disables a plugin and unregisters its tools.
// POST /api/plugins/:name/disable
func (h *PluginsHandler) DisablePlugin(c *gin.Context) {
	name := c.Param("name")

	// Get tools before disabling so we know which to unregister
	tools, _ := h.manager.GetPluginTools(name)

	if err := h.manager.DisablePlugin(name); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Hot-unload: unregister plugin tools from MCP server
	h.unregisterPluginTools(tools)

	c.JSON(http.StatusOK, gin.H{
		"message":        fmt.Sprintf("plugin %q disabled", name),
		"tools_unloaded": len(tools),
	})
}

// GetPluginConfig returns a plugin's config variables (values masked).
// GET /api/plugins/:name/config
func (h *PluginsHandler) GetPluginConfig(c *gin.Context) {
	name := c.Param("name")

	ps := h.manager.GetPlugin(name)
	if ps == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("plugin %q not found", name)})
		return
	}

	// Mask values
	maskedConfig := make([]plugins.PluginConfigVar, len(ps.Manifest.Config))
	for i, cv := range ps.Manifest.Config {
		maskedConfig[i] = cv
		if cv.Value != "" {
			maskedConfig[i].Value = "********"
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"plugin":     name,
		"config":     maskedConfig,
		"config_set": ps.ConfigSet,
	})
}

// SetPluginConfig sets config variables for a plugin.
// POST /api/plugins/:name/config
// Body: {"CENSYS_API_ID": "xxx", "CENSYS_API_SECRET": "yyy"}
func (h *PluginsHandler) SetPluginConfig(c *gin.Context) {
	name := c.Param("name")

	var body map[string]string
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	var errors []string
	for key, value := range body {
		if err := h.manager.SetPluginConfig(name, key, value); err != nil {
			errors = append(errors, err.Error())
		}
	}

	if len(errors) > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "some config keys failed",
			"errors":  errors,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("config updated for plugin %q", name),
		"keys_set": len(body),
	})
}

// InstallRequirements installs Python requirements for a plugin.
// POST /api/plugins/:name/install
func (h *PluginsHandler) InstallRequirements(c *gin.Context) {
	name := c.Param("name")

	if err := h.manager.InstallRequirements(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("requirements installed for plugin %q", name),
	})
}

// UploadPlugin handles plugin zip upload and extraction.
// POST /api/plugins/upload
func (h *PluginsHandler) UploadPlugin(c *gin.Context) {
	file, err := c.FormFile("plugin")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing plugin file in upload"})
		return
	}

	if !strings.HasSuffix(file.Filename, ".zip") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "only .zip files are supported"})
		return
	}

	// Save to temp location
	tmpPath := filepath.Join(os.TempDir(), "cyberstrike-plugin-"+file.Filename)
	if err := c.SaveUploadedFile(file, tmpPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save uploaded file"})
		return
	}
	defer os.Remove(tmpPath)

	// Extract zip
	pluginName, err := h.extractPluginZip(tmpPath)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Re-scan plugins
	if err := h.manager.ScanPlugins(); err != nil {
		h.logger.Warn("failed to rescan plugins after upload", zap.Error(err))
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("plugin %q uploaded and extracted", pluginName),
		"name":    pluginName,
	})
}

// DeletePlugin removes a plugin directory and state.
// DELETE /api/plugins/:name
func (h *PluginsHandler) DeletePlugin(c *gin.Context) {
	name := c.Param("name")

	// Unregister tools first if plugin was enabled
	tools, _ := h.manager.GetPluginTools(name)
	h.unregisterPluginTools(tools)

	if err := h.manager.DeletePlugin(name); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("plugin %q deleted", name),
	})
}

// registerPluginTools adds plugin tools to the security config and MCP server.
func (h *PluginsHandler) registerPluginTools(pluginName string, tools []config.ToolConfig) {
	for _, tool := range tools {
		if !tool.Enabled {
			continue
		}

		// Add to security config tools list
		h.secConfig.Tools = append(h.secConfig.Tools, tool)

		// Register with executor and MCP server
		h.executor.RegisterSingleTool(h.mcpServer, &tool)

		h.logger.Info("plugin tool registered",
			zap.String("plugin", pluginName),
			zap.String("tool", tool.Name),
		)
	}
}

// unregisterPluginTools removes plugin tools from the MCP server and security config.
func (h *PluginsHandler) unregisterPluginTools(tools []config.ToolConfig) {
	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
		h.mcpServer.UnregisterTool(tool.Name)
	}

	// Remove from security config tools list
	filtered := make([]config.ToolConfig, 0, len(h.secConfig.Tools))
	for _, t := range h.secConfig.Tools {
		if !toolNames[t.Name] {
			filtered = append(filtered, t)
		}
	}
	h.secConfig.Tools = filtered

	// Rebuild executor index
	h.executor.RebuildToolIndex()
}

// GetPluginI18n serves a plugin's i18n translation JSON file.
// GET /api/plugins/:name/i18n/:lang
func (h *PluginsHandler) GetPluginI18n(c *gin.Context) {
	name := c.Param("name")
	lang := c.Param("lang") // "en-US" or "uk-UA"

	filePath := filepath.Join(h.manager.PluginsDir(), name, "i18n", lang+".json")
	if _, err := os.Stat(filePath); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "translation not found"})
		return
	}
	c.File(filePath)
}

// ServePluginStatic serves a plugin's static web assets (JS, CSS, HTML pages).
// GET /api/plugins/:name/web/*filepath
func (h *PluginsHandler) ServePluginStatic(c *gin.Context) {
	name := c.Param("name")
	fp := c.Param("filepath")

	// Security: prevent path traversal
	clean := filepath.Clean(fp)
	if strings.Contains(clean, "..") {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	fullPath := filepath.Join(h.manager.PluginsDir(), name, "web", clean)
	if _, err := os.Stat(fullPath); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.File(fullPath)
}

// extractPluginZip extracts a plugin zip to the plugins directory. Returns the
// plugin name (top-level directory in the zip).
func (h *PluginsHandler) extractPluginZip(zipPath string) (string, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", fmt.Errorf("open zip: %w", err)
	}
	defer r.Close()

	// Determine the plugin name from the zip's top-level directory
	var pluginName string
	for _, f := range r.File {
		parts := strings.SplitN(f.Name, "/", 2)
		if len(parts) > 0 && parts[0] != "" {
			pluginName = parts[0]
			break
		}
	}
	if pluginName == "" {
		return "", fmt.Errorf("zip has no top-level directory")
	}

	destDir := h.manager.PluginsDir()

	for _, f := range r.File {
		// Sanitize path to prevent zip slip
		destPath := filepath.Join(destDir, f.Name)
		if !strings.HasPrefix(destPath, filepath.Clean(destDir)+string(os.PathSeparator)) {
			return "", fmt.Errorf("illegal file path in zip: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(destPath, 0755); err != nil {
				return "", fmt.Errorf("create dir: %w", err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return "", fmt.Errorf("create parent dir: %w", err)
		}

		outFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return "", fmt.Errorf("create file: %w", err)
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return "", fmt.Errorf("open zip entry: %w", err)
		}

		// Limit extraction size to 100MB per file
		_, err = io.Copy(outFile, io.LimitReader(rc, 100*1024*1024))
		rc.Close()
		outFile.Close()
		if err != nil {
			return "", fmt.Errorf("extract file: %w", err)
		}
	}

	return pluginName, nil
}
