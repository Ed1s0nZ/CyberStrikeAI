package plugins

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"cyberstrike-ai/internal/config"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// PluginManifest describes a plugin's metadata and requirements.
type PluginManifest struct {
	Name         string            `yaml:"name" json:"name"`
	Version      string            `yaml:"version" json:"version"`
	Description  string            `yaml:"description" json:"description"`
	Author       string            `yaml:"author" json:"author"`
	URL          string            `yaml:"url" json:"url"`
	Provides     PluginProvides    `yaml:"provides" json:"provides"`
	Config       []PluginConfigVar `yaml:"config" json:"config"`
	Requirements string            `yaml:"requirements" json:"requirements"`
	Frontend     PluginFrontend    `yaml:"frontend" json:"frontend"`
}

// PluginFrontend describes frontend assets a plugin provides.
type PluginFrontend struct {
	NavItems []PluginNavItem `yaml:"nav_items" json:"nav_items"` // sidebar nav items
	Pages    []string        `yaml:"pages" json:"pages"`         // HTML page files in web/pages/
	Scripts  []string        `yaml:"scripts" json:"scripts"`     // JS files in web/js/
	Styles   []string        `yaml:"styles" json:"styles"`       // CSS files in web/css/
}

// PluginNavItem describes a sidebar navigation item added by a plugin.
type PluginNavItem struct {
	ID    string `yaml:"id" json:"id"`       // page ID (e.g. "my-plugin-page")
	Label string `yaml:"label" json:"label"` // display text
	Icon  string `yaml:"icon" json:"icon"`   // SVG or emoji
	I18n  string `yaml:"i18n" json:"i18n"`   // i18n key for label
}

// PluginProvides declares which resource types a plugin provides.
type PluginProvides struct {
	Tools     bool `yaml:"tools" json:"tools"`
	Skills    bool `yaml:"skills" json:"skills"`
	Agents    bool `yaml:"agents" json:"agents"`
	Roles     bool `yaml:"roles" json:"roles"`
	Knowledge bool `yaml:"knowledge" json:"knowledge"`
}

// PluginConfigVar describes a single configuration variable for a plugin.
type PluginConfigVar struct {
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description" json:"description"`
	Required    bool   `yaml:"required" json:"required"`
	Value       string `yaml:"-" json:"value,omitempty"` // runtime value, not in yaml
}

// PluginState holds the runtime state of a discovered plugin.
type PluginState struct {
	Manifest   PluginManifest `json:"manifest"`
	Enabled    bool           `json:"enabled"`
	Dir        string         `json:"dir"`
	Installed  bool           `json:"installed"`
	ConfigSet  bool           `json:"config_set"`
	ToolCount  int            `json:"tool_count"`
	SkillCount int            `json:"skill_count"`
	AgentCount int            `json:"agent_count"`
	Error      string         `json:"error,omitempty"`
}

// Manager discovers, loads, enables, and disables plugins.
type Manager struct {
	pluginsDir string
	plugins    map[string]*PluginState
	mu         sync.RWMutex
	logger     *zap.Logger
	db         *DB

	// pluginEnvVars maps tool names to environment variables to inject when
	// executing that tool. Updated when a plugin is enabled/disabled.
	pluginEnvVars map[string][]string
}

// NewManager creates a new plugin manager.
func NewManager(pluginsDir string, sqlDB *sql.DB, logger *zap.Logger) (*Manager, error) {
	db, err := NewDB(sqlDB, logger)
	if err != nil {
		return nil, fmt.Errorf("plugin manager db: %w", err)
	}

	m := &Manager{
		pluginsDir:    pluginsDir,
		plugins:       make(map[string]*PluginState),
		logger:        logger,
		db:            db,
		pluginEnvVars: make(map[string][]string),
	}

	return m, nil
}

// ScanPlugins discovers all plugins in the plugins directory.
func (m *Manager) ScanPlugins() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, err := os.Stat(m.pluginsDir); os.IsNotExist(err) {
		m.logger.Info("plugins directory does not exist, creating", zap.String("dir", m.pluginsDir))
		if mkErr := os.MkdirAll(m.pluginsDir, 0755); mkErr != nil {
			return fmt.Errorf("create plugins dir: %w", mkErr)
		}
		return nil
	}

	entries, err := os.ReadDir(m.pluginsDir)
	if err != nil {
		return fmt.Errorf("read plugins dir: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pluginDir := filepath.Join(m.pluginsDir, entry.Name())
		manifestPath := filepath.Join(pluginDir, "plugin.yaml")

		if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
			// also try plugin.yml
			manifestPath = filepath.Join(pluginDir, "plugin.yml")
			if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
				continue
			}
		}

		state, loadErr := m.loadPlugin(pluginDir, manifestPath)
		if loadErr != nil {
			m.logger.Warn("failed to load plugin",
				zap.String("dir", pluginDir),
				zap.Error(loadErr),
			)
			m.plugins[entry.Name()] = &PluginState{
				Manifest: PluginManifest{Name: entry.Name()},
				Dir:      pluginDir,
				Error:    loadErr.Error(),
			}
			continue
		}

		m.plugins[state.Manifest.Name] = state
		m.logger.Info("plugin discovered",
			zap.String("name", state.Manifest.Name),
			zap.String("version", state.Manifest.Version),
			zap.Bool("enabled", state.Enabled),
			zap.Int("tools", state.ToolCount),
		)
	}

	return nil
}

// loadPlugin reads the manifest and merges persisted state.
func (m *Manager) loadPlugin(pluginDir, manifestPath string) (*PluginState, error) {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}

	var manifest PluginManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	if manifest.Name == "" {
		return nil, fmt.Errorf("manifest missing name")
	}

	// Auto-detect provides from directories if not explicitly set
	manifest.Provides = m.autoDetectProvides(pluginDir, manifest.Provides)

	state := &PluginState{
		Manifest: manifest,
		Dir:      pluginDir,
	}

	// Count resources
	state.ToolCount = m.countYAMLFiles(filepath.Join(pluginDir, "tools"))
	state.SkillCount = m.countSubDirs(filepath.Join(pluginDir, "skills"))
	state.AgentCount = m.countMarkdownFiles(filepath.Join(pluginDir, "agents"))

	// Load persisted state from DB
	dbState, err := m.db.GetPluginState(manifest.Name)
	if err != nil {
		m.logger.Warn("failed to load plugin state from db", zap.String("name", manifest.Name), zap.Error(err))
	}
	if dbState != nil {
		state.Enabled = dbState.Enabled
		state.Installed = dbState.Installed
	}

	// Load config values from DB
	configValues, err := m.db.GetPluginConfig(manifest.Name)
	if err != nil {
		m.logger.Warn("failed to load plugin config from db", zap.String("name", manifest.Name), zap.Error(err))
	}
	if configValues != nil {
		for i := range manifest.Config {
			if val, ok := configValues[manifest.Config[i].Name]; ok {
				manifest.Config[i].Value = val
			}
		}
		state.Manifest = manifest
	}

	// Check if all required config vars are set
	state.ConfigSet = m.checkConfigComplete(state)

	return state, nil
}

func (m *Manager) autoDetectProvides(pluginDir string, explicit PluginProvides) PluginProvides {
	result := explicit
	if !result.Tools {
		if info, err := os.Stat(filepath.Join(pluginDir, "tools")); err == nil && info.IsDir() {
			result.Tools = true
		}
	}
	if !result.Skills {
		if info, err := os.Stat(filepath.Join(pluginDir, "skills")); err == nil && info.IsDir() {
			result.Skills = true
		}
	}
	if !result.Agents {
		if info, err := os.Stat(filepath.Join(pluginDir, "agents")); err == nil && info.IsDir() {
			result.Agents = true
		}
	}
	if !result.Roles {
		if info, err := os.Stat(filepath.Join(pluginDir, "roles")); err == nil && info.IsDir() {
			result.Roles = true
		}
	}
	if !result.Knowledge {
		if info, err := os.Stat(filepath.Join(pluginDir, "knowledge")); err == nil && info.IsDir() {
			result.Knowledge = true
		}
	}
	return result
}

func (m *Manager) checkConfigComplete(state *PluginState) bool {
	for _, cv := range state.Manifest.Config {
		if cv.Required && cv.Value == "" {
			return false
		}
	}
	return true
}

func (m *Manager) countYAMLFiles(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() && (strings.HasSuffix(e.Name(), ".yaml") || strings.HasSuffix(e.Name(), ".yml")) {
			count++
		}
	}
	return count
}

func (m *Manager) countSubDirs(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if e.IsDir() {
			count++
		}
	}
	return count
}

func (m *Manager) countMarkdownFiles(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			count++
		}
	}
	return count
}

// EnablePlugin enables a plugin and persists state.
func (m *Manager) EnablePlugin(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, ok := m.plugins[name]
	if !ok {
		return fmt.Errorf("plugin %q not found", name)
	}
	if state.Error != "" {
		return fmt.Errorf("plugin %q has load error: %s", name, state.Error)
	}

	state.Enabled = true
	if err := m.db.SetPluginEnabled(name, true); err != nil {
		return fmt.Errorf("persist enable state: %w", err)
	}

	m.logger.Info("plugin enabled", zap.String("name", name))
	return nil
}

// DisablePlugin disables a plugin and persists state.
func (m *Manager) DisablePlugin(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, ok := m.plugins[name]
	if !ok {
		return fmt.Errorf("plugin %q not found", name)
	}

	state.Enabled = false
	if err := m.db.SetPluginEnabled(name, false); err != nil {
		return fmt.Errorf("persist disable state: %w", err)
	}

	// Remove env vars for this plugin's tools
	m.removePluginEnvVars(name)

	m.logger.Info("plugin disabled", zap.String("name", name))
	return nil
}

// SetPluginConfig sets a configuration variable for a plugin.
func (m *Manager) SetPluginConfig(pluginName, key, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, ok := m.plugins[pluginName]
	if !ok {
		return fmt.Errorf("plugin %q not found", pluginName)
	}

	// Validate the key exists in manifest
	found := false
	for i := range state.Manifest.Config {
		if state.Manifest.Config[i].Name == key {
			state.Manifest.Config[i].Value = value
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("config key %q not found in plugin %q manifest", key, pluginName)
	}

	if err := m.db.SetPluginConfigValue(pluginName, key, value); err != nil {
		return fmt.Errorf("persist config: %w", err)
	}

	// Re-check config completeness
	state.ConfigSet = m.checkConfigComplete(state)

	m.logger.Info("plugin config set",
		zap.String("plugin", pluginName),
		zap.String("key", key),
	)
	return nil
}

// GetPlugins returns all discovered plugins (snapshot).
func (m *Manager) GetPlugins() []PluginState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]PluginState, 0, len(m.plugins))
	for _, ps := range m.plugins {
		result = append(result, *ps)
	}
	return result
}

// GetPlugin returns a single plugin's state, or nil if not found.
func (m *Manager) GetPlugin(name string) *PluginState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ps, ok := m.plugins[name]
	if !ok {
		return nil
	}
	copy := *ps
	return &copy
}

// GetPluginTools loads tool configs from a plugin's tools/ directory. It
// replaces {{PLUGIN_DIR}} in args with the plugin's absolute directory path.
func (m *Manager) GetPluginTools(name string) ([]config.ToolConfig, error) {
	m.mu.RLock()
	state, ok := m.plugins[name]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("plugin %q not found", name)
	}

	toolsDir := filepath.Join(state.Dir, "tools")
	tools, err := config.LoadToolsFromDir(toolsDir)
	if err != nil {
		return nil, fmt.Errorf("load tools from plugin %q: %w", name, err)
	}

	absDir, _ := filepath.Abs(state.Dir)

	for i := range tools {
		// Replace {{PLUGIN_DIR}} in args
		for j := range tools[i].Args {
			tools[i].Args[j] = strings.ReplaceAll(tools[i].Args[j], "{{PLUGIN_DIR}}", absDir)
		}
		// Replace {{PLUGIN_DIR}} in command
		tools[i].Command = strings.ReplaceAll(tools[i].Command, "{{PLUGIN_DIR}}", absDir)
	}

	return tools, nil
}

// GetPluginScriptsDir returns the absolute path to a plugin's scripts/ dir.
func (m *Manager) GetPluginScriptsDir(name string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state, ok := m.plugins[name]
	if !ok {
		return ""
	}
	return filepath.Join(state.Dir, "scripts")
}

// GetEnabledPluginTools returns tool configs for all enabled plugins, merged.
func (m *Manager) GetEnabledPluginTools() []config.ToolConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var allTools []config.ToolConfig
	for name, state := range m.plugins {
		if !state.Enabled || state.Error != "" {
			continue
		}

		tools, err := m.getPluginToolsLocked(name, state)
		if err != nil {
			m.logger.Warn("failed to load tools for enabled plugin",
				zap.String("plugin", name),
				zap.Error(err),
			)
			continue
		}

		// Build env vars for each tool
		envVars := m.buildPluginEnvVars(state)
		for _, tool := range tools {
			m.pluginEnvVars[tool.Name] = envVars
		}

		allTools = append(allTools, tools...)
	}

	return allTools
}

// getPluginToolsLocked loads tools without acquiring the lock (caller must hold it).
func (m *Manager) getPluginToolsLocked(name string, state *PluginState) ([]config.ToolConfig, error) {
	toolsDir := filepath.Join(state.Dir, "tools")
	tools, err := config.LoadToolsFromDir(toolsDir)
	if err != nil {
		return nil, err
	}

	absDir, _ := filepath.Abs(state.Dir)

	for i := range tools {
		for j := range tools[i].Args {
			tools[i].Args[j] = strings.ReplaceAll(tools[i].Args[j], "{{PLUGIN_DIR}}", absDir)
		}
		tools[i].Command = strings.ReplaceAll(tools[i].Command, "{{PLUGIN_DIR}}", absDir)
	}

	return tools, nil
}

// buildPluginEnvVars returns environment variable strings for a plugin's config.
func (m *Manager) buildPluginEnvVars(state *PluginState) []string {
	var envVars []string
	for _, cv := range state.Manifest.Config {
		if cv.Value != "" {
			envVars = append(envVars, cv.Name+"="+cv.Value)
		}
	}
	return envVars
}

// removePluginEnvVars removes env var mappings for all tools from a plugin.
func (m *Manager) removePluginEnvVars(name string) {
	state, ok := m.plugins[name]
	if !ok {
		return
	}

	toolsDir := filepath.Join(state.Dir, "tools")
	tools, err := config.LoadToolsFromDir(toolsDir)
	if err != nil {
		return
	}

	for _, tool := range tools {
		delete(m.pluginEnvVars, tool.Name)
	}
}

// GetToolEnvVars returns the plugin environment variables for a given tool name.
// Returns nil if the tool is not from a plugin.
func (m *Manager) GetToolEnvVars(toolName string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.pluginEnvVars[toolName]
}

// InstallRequirements runs pip install for a plugin's requirements.txt in a
// virtual environment inside the plugin directory.
func (m *Manager) InstallRequirements(name string) error {
	m.mu.RLock()
	state, ok := m.plugins[name]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("plugin %q not found", name)
	}

	reqFile := state.Manifest.Requirements
	if reqFile == "" {
		reqFile = "requirements.txt"
	}
	reqPath := filepath.Join(state.Dir, reqFile)

	if _, err := os.Stat(reqPath); os.IsNotExist(err) {
		m.logger.Info("no requirements.txt for plugin, skipping install", zap.String("plugin", name))
		m.mu.Lock()
		state.Installed = true
		m.mu.Unlock()
		_ = m.db.SetPluginInstalled(name, true)
		return nil
	}

	venvDir := filepath.Join(state.Dir, ".venv")
	if _, err := os.Stat(venvDir); os.IsNotExist(err) {
		m.logger.Info("creating venv for plugin", zap.String("plugin", name), zap.String("venv", venvDir))
		cmd := exec.Command("python3", "-m", "venv", venvDir)
		cmd.Dir = state.Dir
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("create venv: %w\noutput: %s", err, string(out))
		}
	}

	pipPath := filepath.Join(venvDir, "bin", "pip")
	m.logger.Info("installing plugin requirements",
		zap.String("plugin", name),
		zap.String("requirements", reqPath),
	)

	cmd := exec.Command(pipPath, "install", "-r", reqPath)
	cmd.Dir = state.Dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("pip install: %w\noutput: %s", err, string(out))
	}

	m.mu.Lock()
	state.Installed = true
	m.mu.Unlock()

	_ = m.db.SetPluginInstalled(name, true)
	m.logger.Info("plugin requirements installed", zap.String("plugin", name))
	return nil
}

// DeletePlugin removes a plugin directory and its persisted state.
func (m *Manager) DeletePlugin(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, ok := m.plugins[name]
	if !ok {
		return fmt.Errorf("plugin %q not found", name)
	}

	// Remove env vars
	m.removePluginEnvVars(name)

	// Remove directory
	if err := os.RemoveAll(state.Dir); err != nil {
		return fmt.Errorf("remove plugin dir: %w", err)
	}

	// Remove DB state
	if err := m.db.DeletePluginState(name); err != nil {
		m.logger.Warn("failed to delete plugin db state", zap.String("name", name), zap.Error(err))
	}

	delete(m.plugins, name)
	m.logger.Info("plugin deleted", zap.String("name", name))
	return nil
}

// PluginsDir returns the plugins base directory.
func (m *Manager) PluginsDir() string {
	return m.pluginsDir
}
