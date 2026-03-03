package config

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Version     string                `yaml:"version,omitempty" json:"version,omitempty"` // Version number displayed in the frontend, e.g. v1.3.3
	Server      ServerConfig          `yaml:"server"`
	Log         LogConfig             `yaml:"log"`
	MCP         MCPConfig             `yaml:"mcp"`
	OpenAI      OpenAIConfig          `yaml:"openai"`
	FOFA        FofaConfig            `yaml:"fofa,omitempty" json:"fofa,omitempty"`
	Agent       AgentConfig           `yaml:"agent"`
	Security    SecurityConfig        `yaml:"security"`
	Database    DatabaseConfig        `yaml:"database"`
	Auth        AuthConfig            `yaml:"auth"`
	ExternalMCP ExternalMCPConfig     `yaml:"external_mcp,omitempty"`
	Knowledge   KnowledgeConfig       `yaml:"knowledge,omitempty"`
	Robots      RobotsConfig          `yaml:"robots,omitempty" json:"robots,omitempty"`         // Bot configuration for DingTalk, Lark/Feishu, etc.
	RolesDir    string                `yaml:"roles_dir,omitempty" json:"roles_dir,omitempty"`   // Role configuration file directory (new approach)
	Roles       map[string]RoleConfig `yaml:"roles,omitempty" json:"roles,omitempty"`           // Backward-compatible: supports defining roles in the main config file
	SkillsDir   string                `yaml:"skills_dir,omitempty" json:"skills_dir,omitempty"` // Skills configuration file directory
}

// RobotsConfig holds bot configuration for DingTalk, Lark/Feishu, etc.
type RobotsConfig struct {
	Wecom   RobotWecomConfig   `yaml:"wecom,omitempty" json:"wecom,omitempty"`     // WeCom (Enterprise WeChat)
	Dingtalk RobotDingtalkConfig `yaml:"dingtalk,omitempty" json:"dingtalk,omitempty"` // DingTalk
	Lark    RobotLarkConfig    `yaml:"lark,omitempty" json:"lark,omitempty"`     // Lark (Feishu)
}

// RobotWecomConfig holds the WeCom (Enterprise WeChat) bot configuration.
type RobotWecomConfig struct {
	Enabled       bool   `yaml:"enabled" json:"enabled"`
	Token         string `yaml:"token" json:"token"`                     // Callback URL verification token
	EncodingAESKey string `yaml:"encoding_aes_key" json:"encoding_aes_key"` // EncodingAESKey
	CorpID        string `yaml:"corp_id" json:"corp_id"`               // Enterprise ID
	Secret        string `yaml:"secret" json:"secret"`                  // Application Secret
	AgentID       int64  `yaml:"agent_id" json:"agent_id"`              // Application AgentId
}

// RobotDingtalkConfig holds the DingTalk bot configuration.
type RobotDingtalkConfig struct {
	Enabled      bool   `yaml:"enabled" json:"enabled"`
	ClientID     string `yaml:"client_id" json:"client_id"`         // Application Key (AppKey)
	ClientSecret string `yaml:"client_secret" json:"client_secret"` // Application Secret
}

// RobotLarkConfig holds the Lark (Feishu) bot configuration.
type RobotLarkConfig struct {
	Enabled   bool   `yaml:"enabled" json:"enabled"`
	AppID     string `yaml:"app_id" json:"app_id"`         // Application App ID
	AppSecret string `yaml:"app_secret" json:"app_secret"` // Application App Secret
	VerifyToken string `yaml:"verify_token" json:"verify_token"` // Event subscription Verification Token (optional)
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type LogConfig struct {
	Level  string `yaml:"level"`
	Output string `yaml:"output"`
}

type MCPConfig struct {
	Enabled bool   `yaml:"enabled"`
	Host    string `yaml:"host"`
	Port    int    `yaml:"port"`
}

type OpenAIConfig struct {
	APIKey         string `yaml:"api_key" json:"api_key"`
	BaseURL        string `yaml:"base_url" json:"base_url"`
	Model          string `yaml:"model" json:"model"`
	MaxTotalTokens int    `yaml:"max_total_tokens,omitempty" json:"max_total_tokens,omitempty"`
}

type FofaConfig struct {
	// Email is the FOFA account email; APIKey is the FOFA API Key (read-only key recommended)
	Email   string `yaml:"email,omitempty" json:"email,omitempty"`
	APIKey  string `yaml:"api_key,omitempty" json:"api_key,omitempty"`
	BaseURL string `yaml:"base_url,omitempty" json:"base_url,omitempty"` // Default: https://fofa.info/api/v1/search/all
}

type SecurityConfig struct {
	Tools               []ToolConfig `yaml:"tools,omitempty"`                 // Backward-compatible: supports defining tools in the main config file
	ToolsDir            string       `yaml:"tools_dir,omitempty"`             // Tool configuration file directory (new approach)
	ToolDescriptionMode string       `yaml:"tool_description_mode,omitempty"` // Tool description mode: "short" | "full", default short
}

type DatabaseConfig struct {
	Path            string `yaml:"path"`                        // Session database path
	KnowledgeDBPath string `yaml:"knowledge_db_path,omitempty"` // Knowledge base database path (optional; uses session database if empty)
}

type AgentConfig struct {
	MaxIterations          int    `yaml:"max_iterations" json:"max_iterations"`
	LargeResultThreshold   int    `yaml:"large_result_threshold" json:"large_result_threshold"`     // Large-result threshold (bytes), default 50 KB
	ResultStorageDir       string `yaml:"result_storage_dir" json:"result_storage_dir"`             // Result storage directory, default tmp
	ParallelToolExecution  bool   `yaml:"parallel_tool_execution" json:"parallel_tool_execution"`   // Execute multiple tool calls concurrently (default true)
	MaxParallelTools       int    `yaml:"max_parallel_tools" json:"max_parallel_tools"`             // Maximum concurrent tool calls (0 = unlimited)
	ToolRetryCount         int    `yaml:"tool_retry_count" json:"tool_retry_count"`                 // Number of retries on transient tool errors (default 0)
}

type AuthConfig struct {
	Password                    string `yaml:"password" json:"password"`
	SessionDurationHours        int    `yaml:"session_duration_hours" json:"session_duration_hours"`
	GeneratedPassword           string `yaml:"-" json:"-"`
	GeneratedPasswordPersisted  bool   `yaml:"-" json:"-"`
	GeneratedPasswordPersistErr string `yaml:"-" json:"-"`
}

// ExternalMCPConfig holds external MCP configuration.
type ExternalMCPConfig struct {
	Servers map[string]ExternalMCPServerConfig `yaml:"servers,omitempty" json:"servers,omitempty"`
}

// ExternalMCPServerConfig holds configuration for an external MCP server.
type ExternalMCPServerConfig struct {
	// stdio mode configuration
	Command string            `yaml:"command,omitempty" json:"command,omitempty"`
	Args    []string          `yaml:"args,omitempty" json:"args,omitempty"`
	Env     map[string]string `yaml:"env,omitempty" json:"env,omitempty"` // Environment variables (for stdio mode)

	// HTTP mode configuration
	Transport string            `yaml:"transport,omitempty" json:"transport,omitempty"` // "stdio" | "sse" | "http"(Streamable) | "simple_http"(custom/simple POST endpoint, e.g. http://127.0.0.1:8081/mcp)
	URL       string            `yaml:"url,omitempty" json:"url,omitempty"`
	Headers   map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"` // HTTP/SSE request headers (e.g. x-api-key)

	// General configuration
	Description       string          `yaml:"description,omitempty" json:"description,omitempty"`
	Timeout           int             `yaml:"timeout,omitempty" json:"timeout,omitempty"`                         // Timeout in seconds
	ExternalMCPEnable bool            `yaml:"external_mcp_enable,omitempty" json:"external_mcp_enable,omitempty"` // Whether to enable the external MCP server
	ToolEnabled       map[string]bool `yaml:"tool_enabled,omitempty" json:"tool_enabled,omitempty"`               // Per-tool enabled state (tool name -> enabled)

	// Backward-compatible fields (deprecated; retained for reading old configs)
	Enabled  bool `yaml:"enabled,omitempty" json:"enabled,omitempty"`   // Deprecated; use external_mcp_enable
	Disabled bool `yaml:"disabled,omitempty" json:"disabled,omitempty"` // Deprecated; use external_mcp_enable
}
type ToolConfig struct {
	Name             string            `yaml:"name"`
	Command          string            `yaml:"command"`
	Args             []string          `yaml:"args,omitempty"`              // Fixed arguments (optional)
	ShortDescription string            `yaml:"short_description,omitempty"` // Short description (used in tool lists to reduce token consumption)
	Description      string            `yaml:"description"`                 // Detailed description (used in tool documentation)
	Enabled          bool              `yaml:"enabled"`
	Parameters       []ParameterConfig `yaml:"parameters,omitempty"`         // Parameter definitions (optional)
	ArgMapping       string            `yaml:"arg_mapping,omitempty"`        // Argument mapping mode: "auto", "manual", "template" (optional)
	AllowedExitCodes []int             `yaml:"allowed_exit_codes,omitempty"` // Allowed exit codes (some tools return non-zero exit codes even on success)
}

// ParameterConfig holds the configuration for a single tool parameter.
type ParameterConfig struct {
	Name        string      `yaml:"name"`               // Parameter name
	Type        string      `yaml:"type"`               // Parameter type: string, int, bool, array
	Description string      `yaml:"description"`        // Parameter description
	Required    bool        `yaml:"required,omitempty"` // Whether the parameter is required
	Default     interface{} `yaml:"default,omitempty"`  // Default value
	Flag        string      `yaml:"flag,omitempty"`     // Command-line flag, e.g. "-u", "--url", "-p"
	Position    *int        `yaml:"position,omitempty"` // Position of a positional parameter (0-based)
	Format      string      `yaml:"format,omitempty"`   // Parameter format: "flag", "positional", "combined" (flag=value), "template"
	Template    string      `yaml:"template,omitempty"` // Template string, e.g. "{flag} {value}" or "{value}"
	Options     []string    `yaml:"options,omitempty"`  // List of allowed values (for enum parameters)
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if cfg.Auth.SessionDurationHours <= 0 {
		cfg.Auth.SessionDurationHours = 12
	}

	if strings.TrimSpace(cfg.Auth.Password) == "" {
		password, err := generateStrongPassword(24)
		if err != nil {
			return nil, fmt.Errorf("failed to generate default password: %w", err)
		}

		cfg.Auth.Password = password
		cfg.Auth.GeneratedPassword = password

		if err := PersistAuthPassword(path, password); err != nil {
			cfg.Auth.GeneratedPasswordPersisted = false
			cfg.Auth.GeneratedPasswordPersistErr = err.Error()
		} else {
			cfg.Auth.GeneratedPasswordPersisted = true
		}
	}

	// If a tools directory is configured, load tool configs from the directory
	if cfg.Security.ToolsDir != "" {
		configDir := filepath.Dir(path)
		toolsDir := cfg.Security.ToolsDir

		// If relative, resolve relative to the config file's directory
		if !filepath.IsAbs(toolsDir) {
			toolsDir = filepath.Join(configDir, toolsDir)
		}

		tools, err := LoadToolsFromDir(toolsDir)
		if err != nil {
			return nil, fmt.Errorf("failed to load tool configs from tools directory: %w", err)
		}

		// Merge tool configs: tools from directory take precedence; tools from main config are supplementary
		existingTools := make(map[string]bool)
		for _, tool := range tools {
			existingTools[tool.Name] = true
		}

		// Add tools from main config that are not present in the directory (backward compatibility)
		for _, tool := range cfg.Security.Tools {
			if !existingTools[tool.Name] {
				tools = append(tools, tool)
			}
		}

		cfg.Security.Tools = tools
	}

	// Migrate external MCP config: migrate old enabled/disabled fields to external_mcp_enable
	if cfg.ExternalMCP.Servers != nil {
		for name, serverCfg := range cfg.ExternalMCP.Servers {
			// If external_mcp_enable is already set, skip migration.
			// Otherwise migrate from enabled/disabled fields.
			// Note: since ExternalMCPEnable is a bool, its zero value is false, so we check the
			// old enabled/disabled fields to determine whether migration is needed.
			if serverCfg.Disabled {
				// Old config used disabled; migrate to external_mcp_enable
				serverCfg.ExternalMCPEnable = false
			} else if serverCfg.Enabled {
				// Old config used enabled; migrate to external_mcp_enable
				serverCfg.ExternalMCPEnable = true
			} else {
				// Neither set; default to enabled
				serverCfg.ExternalMCPEnable = true
			}
			cfg.ExternalMCP.Servers[name] = serverCfg
		}
	}

	// Load role configs from the roles directory
	if cfg.RolesDir != "" {
		configDir := filepath.Dir(path)
		rolesDir := cfg.RolesDir

		// If relative, resolve relative to the config file's directory
		if !filepath.IsAbs(rolesDir) {
			rolesDir = filepath.Join(configDir, rolesDir)
		}

		roles, err := LoadRolesFromDir(rolesDir)
		if err != nil {
			return nil, fmt.Errorf("failed to load role configs from roles directory: %w", err)
		}

		cfg.Roles = roles
	} else {
		// If roles_dir is not configured, initialize to empty map
		if cfg.Roles == nil {
			cfg.Roles = make(map[string]RoleConfig)
		}
	}

	return &cfg, nil
}

func generateStrongPassword(length int) (string, error) {
	if length <= 0 {
		length = 24
	}

	bytesLen := length
	randomBytes := make([]byte, bytesLen)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}

	password := base64.RawURLEncoding.EncodeToString(randomBytes)
	if len(password) > length {
		password = password[:length]
	}
	return password, nil
}

func PersistAuthPassword(path, password string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	inAuthBlock := false
	authIndent := -1

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !inAuthBlock {
			if strings.HasPrefix(trimmed, "auth:") {
				inAuthBlock = true
				authIndent = len(line) - len(strings.TrimLeft(line, " "))
			}
			continue
		}

		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		leadingSpaces := len(line) - len(strings.TrimLeft(line, " "))
		if leadingSpaces <= authIndent {
			// Left the auth block
			inAuthBlock = false
			authIndent = -1
			// Continue looking for other auth blocks (theoretically there are none)
			if strings.HasPrefix(trimmed, "auth:") {
				inAuthBlock = true
				authIndent = leadingSpaces
			}
			continue
		}

		if strings.HasPrefix(strings.TrimSpace(line), "password:") {
			prefix := line[:len(line)-len(strings.TrimLeft(line, " "))]
			comment := ""
			if idx := strings.Index(line, "#"); idx >= 0 {
				comment = strings.TrimRight(line[idx:], " ")
			}

			newLine := fmt.Sprintf("%spassword: %s", prefix, password)
			if comment != "" {
				if !strings.HasPrefix(comment, " ") {
					newLine += " "
				}
				newLine += comment
			}
			lines[i] = newLine
			break
		}
	}

	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644)
}

func PrintGeneratedPasswordWarning(password string, persisted bool, persistErr string) {
	if strings.TrimSpace(password) == "" {
		return
	}

	if persisted {
		fmt.Println("[CyberStrikeAI] A web login password has been automatically generated and written to config.")
	} else {
		if persistErr != "" {
			fmt.Printf("[CyberStrikeAI] WARNING: Could not automatically write password to config file: %s\n", persistErr)
		} else {
			fmt.Println("[CyberStrikeAI] WARNING: Could not automatically write password to config file.")
		}
		fmt.Println("Please manually write the following random password to auth.password in config.yaml:")
	}

	fmt.Println("----------------------------------------------------------------")
	fmt.Println("CyberStrikeAI Auto-Generated Web Password")
	fmt.Printf("Password: %s\n", password)
	fmt.Println("WARNING: Anyone with this password can fully control CyberStrikeAI.")
	fmt.Println("Please store it securely and change it in config.yaml as soon as possible.")
	fmt.Println("----------------------------------------------------------------")
}

// LoadToolsFromDir loads all tool configuration files from a directory.
func LoadToolsFromDir(dir string) ([]ToolConfig, error) {
	var tools []ToolConfig

	// Return an empty list (no error) if the directory does not exist
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return tools, nil
	}

	// Read all .yaml and .yml files in the directory
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read tools directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}

		filePath := filepath.Join(dir, name)
		tool, err := LoadToolFromFile(filePath)
		if err != nil {
			// Log the error but continue loading other files
			fmt.Printf("Warning: failed to load tool config file %s: %v\n", filePath, err)
			continue
		}

		tools = append(tools, *tool)
	}

	return tools, nil
}

// LoadToolFromFile loads a tool configuration from a single file.
func LoadToolFromFile(path string) (*ToolConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var tool ToolConfig
	if err := yaml.Unmarshal(data, &tool); err != nil {
		return nil, fmt.Errorf("failed to parse tool config: %w", err)
	}

	// Validate required fields
	if tool.Name == "" {
		return nil, fmt.Errorf("tool name must not be empty")
	}
	if tool.Command == "" {
		return nil, fmt.Errorf("tool command must not be empty")
	}

	return &tool, nil
}

// LoadRolesFromDir loads all role configuration files from a directory.
func LoadRolesFromDir(dir string) (map[string]RoleConfig, error) {
	roles := make(map[string]RoleConfig)

	// Return an empty map (no error) if the directory does not exist
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return roles, nil
	}

	// Read all .yaml and .yml files in the directory
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read roles directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}

		filePath := filepath.Join(dir, name)
		role, err := LoadRoleFromFile(filePath)
		if err != nil {
			// Log the error but continue loading other files
			fmt.Printf("Warning: failed to load role config file %s: %v\n", filePath, err)
			continue
		}

		// Use the role name as the key
		roleName := role.Name
		if roleName == "" {
			// If the role name is empty, use the filename (without extension) as the name
			roleName = strings.TrimSuffix(strings.TrimSuffix(name, ".yaml"), ".yml")
			role.Name = roleName
		}

		roles[roleName] = *role
	}

	return roles, nil
}

// LoadRoleFromFile loads a role configuration from a single file.
func LoadRoleFromFile(path string) (*RoleConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var role RoleConfig
	if err := yaml.Unmarshal(data, &role); err != nil {
		return nil, fmt.Errorf("failed to parse role config: %w", err)
	}

	// Handle the icon field: if it contains a Unicode escape sequence (e.g. \U0001F3C6),
	// convert it to the actual Unicode character. The Go yaml library may not automatically
	// handle \U escape sequences, so we do it manually.
	if role.Icon != "" {
		icon := role.Icon
		// Strip possible surrounding quotes
		icon = strings.Trim(icon, `"`)

		// Check for Unicode escape format \U0001F3C6 (8-digit hex) or \uXXXX (4-digit hex)
		if len(icon) >= 3 && icon[0] == '\\' {
			if icon[1] == 'U' && len(icon) >= 10 {
				// \U0001F3C6 format (8-digit hex)
				if codePoint, err := strconv.ParseInt(icon[2:10], 16, 32); err == nil {
					role.Icon = string(rune(codePoint))
				}
			} else if icon[1] == 'u' && len(icon) >= 6 {
				// \uXXXX format (4-digit hex)
				if codePoint, err := strconv.ParseInt(icon[2:6], 16, 32); err == nil {
					role.Icon = string(rune(codePoint))
				}
			}
		}
	}

	// Validate required fields; if name is empty, derive it from the filename
	if role.Name == "" {
		baseName := filepath.Base(path)
		role.Name = strings.TrimSuffix(strings.TrimSuffix(baseName, ".yaml"), ".yml")
	}

	return &role, nil
}

func Default() *Config {
	return &Config{
		Server: ServerConfig{
			Host: "0.0.0.0",
			Port: 8080,
		},
		Log: LogConfig{
			Level:  "info",
			Output: "stdout",
		},
		MCP: MCPConfig{
			Enabled: true,
			Host:    "0.0.0.0",
			Port:    8081,
		},
		OpenAI: OpenAIConfig{
			BaseURL:        "https://api.openai.com/v1",
			Model:          "gpt-4",
			MaxTotalTokens: 120000,
		},
		Agent: AgentConfig{
			MaxIterations: 30, // Default maximum iteration count
		},
		Security: SecurityConfig{
			Tools:    []ToolConfig{}, // Tool configs should be loaded from config.yaml or the tools/ directory
			ToolsDir: "tools",        // Default tools directory
		},
		Database: DatabaseConfig{
			Path:            "data/conversations.db",
			KnowledgeDBPath: "data/knowledge.db", // Default knowledge base database path
		},
		Auth: AuthConfig{
			SessionDurationHours: 12,
		},
		Knowledge: KnowledgeConfig{
			Enabled:  true,
			BasePath: "knowledge_base",
			Embedding: EmbeddingConfig{
				Provider: "openai",
				Model:    "text-embedding-3-small",
				BaseURL:  "https://api.openai.com/v1",
			},
			Retrieval: RetrievalConfig{
				TopK:                5,
				SimilarityThreshold: 0.7,
				HybridWeight:        0.7,
			},
		},
	}
}

// KnowledgeConfig holds the knowledge base configuration.
type KnowledgeConfig struct {
	Enabled   bool            `yaml:"enabled" json:"enabled"`     // Whether to enable knowledge retrieval
	BasePath  string          `yaml:"base_path" json:"base_path"` // Knowledge base path
	Embedding EmbeddingConfig `yaml:"embedding" json:"embedding"`
	Retrieval RetrievalConfig `yaml:"retrieval" json:"retrieval"`
}

// EmbeddingConfig holds the embedding model configuration.
type EmbeddingConfig struct {
	Provider string `yaml:"provider" json:"provider"` // Embedding model provider
	Model    string `yaml:"model" json:"model"`       // Model name
	BaseURL  string `yaml:"base_url" json:"base_url"` // API Base URL
	APIKey   string `yaml:"api_key" json:"api_key"`   // API Key (inherited from OpenAI config)
}

// RetrievalConfig holds the retrieval configuration.
type RetrievalConfig struct {
	TopK                int     `yaml:"top_k" json:"top_k"`                               // Top-K retrieval count
	SimilarityThreshold float64 `yaml:"similarity_threshold" json:"similarity_threshold"` // Similarity threshold
	HybridWeight        float64 `yaml:"hybrid_weight" json:"hybrid_weight"`               // Vector retrieval weight (0–1)
}

// RolesConfig holds role configuration (deprecated; use map[string]RoleConfig instead).
// Retained for backward compatibility, but direct use of map[string]RoleConfig is recommended.
type RolesConfig struct {
	Roles map[string]RoleConfig `yaml:"roles,omitempty" json:"roles,omitempty"`
}

// RoleConfig holds configuration for a single role.
type RoleConfig struct {
	Name        string   `yaml:"name" json:"name"`                         // Role name
	Description string   `yaml:"description" json:"description"`           // Role description
	UserPrompt  string   `yaml:"user_prompt" json:"user_prompt"`           // User prompt (prepended to user messages)
	Icon        string   `yaml:"icon,omitempty" json:"icon,omitempty"`     // Role icon (optional)
	Tools       []string `yaml:"tools,omitempty" json:"tools,omitempty"`   // Associated tool list (toolKey format, e.g. "toolName" or "mcpName::toolName")
	MCPs        []string `yaml:"mcps,omitempty" json:"mcps,omitempty"`     // Backward-compatible: associated MCP server list (deprecated; use tools instead)
	Skills      []string `yaml:"skills,omitempty" json:"skills,omitempty"` // Associated skills list (skill names whose content is read before task execution)
	Enabled     bool     `yaml:"enabled" json:"enabled"`                   // Whether the role is enabled
}
