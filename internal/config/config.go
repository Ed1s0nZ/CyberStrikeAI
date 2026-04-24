package config

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Version     string                `yaml:"version,omitempty" json:"version,omitempty"`       // Version number displayed in the frontend, e.g. v1.3.3
	Provider    string                `yaml:"provider,omitempty" json:"provider,omitempty"`     // Top-level LLM provider: "openai" (default) or "claude-cli"
	Server      ServerConfig          `yaml:"server"`
	Log         LogConfig             `yaml:"log"`
	Debug       DebugConfig           `yaml:"debug"`
	MCP         MCPConfig             `yaml:"mcp"`
	OpenAI      OpenAIConfig          `yaml:"openai"`
	ClaudeCLI   ClaudeCLIConfig       `yaml:"claude_cli,omitempty" json:"claude_cli,omitempty"` // Claude CLI provider settings
	FOFA        FofaConfig            `yaml:"fofa,omitempty" json:"fofa,omitempty"`
	ZoomEye     ZoomEyeConfig         `yaml:"zoomeye,omitempty" json:"zoomeye,omitempty"`
	Shodan      ShodanConfig          `yaml:"shodan,omitempty" json:"shodan,omitempty"`
	Censys      CensysConfig          `yaml:"censys,omitempty" json:"censys,omitempty"`
	Agent       AgentConfig           `yaml:"agent"`
	Security    SecurityConfig        `yaml:"security"`
	Database    DatabaseConfig        `yaml:"database"`
	Auth        AuthConfig            `yaml:"auth"`
	ExternalMCP ExternalMCPConfig     `yaml:"external_mcp,omitempty"`
	Knowledge   KnowledgeConfig       `yaml:"knowledge,omitempty"`
	Robots      RobotsConfig          `yaml:"robots,omitempty" json:"robots,omitempty"`         // Bot configuration (Telegram)
	RolesDir    string                `yaml:"roles_dir,omitempty" json:"roles_dir,omitempty"`   // Role configuration file directory (new approach)
	Roles       map[string]RoleConfig `yaml:"roles,omitempty" json:"roles,omitempty"`           // Backward-compatible: supports defining roles in the main config file
	SkillsDir   string                `yaml:"skills_dir,omitempty" json:"skills_dir,omitempty"` // Skills configuration file directory
	AgentsDir   string                `yaml:"agents_dir,omitempty" json:"agents_dir,omitempty"` // Multi-agent sub-agent Markdown definitions directory
	MultiAgent  MultiAgentConfig      `yaml:"multi_agent,omitempty" json:"multi_agent,omitempty"`
}

// MultiAgentConfig holds multi-agent orchestration settings (coexists with single Agent /agent-loop).
type MultiAgentConfig struct {
	Enabled                 bool                  `yaml:"enabled" json:"enabled"`
	DefaultMode             string                `yaml:"default_mode" json:"default_mode"` // single | multi
	RobotUseMultiAgent      bool                  `yaml:"robot_use_multi_agent" json:"robot_use_multi_agent"`
	BatchUseMultiAgent      bool                  `yaml:"batch_use_multi_agent" json:"batch_use_multi_agent"`
	MaxIteration            int                   `yaml:"max_iteration" json:"max_iteration"`
	SubAgentMaxIterations   int                   `yaml:"sub_agent_max_iterations" json:"sub_agent_max_iterations"`
	WithoutGeneralSubAgent  bool                  `yaml:"without_general_sub_agent" json:"without_general_sub_agent"`
	WithoutWriteTodos       bool                  `yaml:"without_write_todos" json:"without_write_todos"`
	OrchestratorInstruction string                `yaml:"orchestrator_instruction" json:"orchestrator_instruction"`
	SubAgents               []MultiAgentSubConfig `yaml:"sub_agents" json:"sub_agents"`
}

// MultiAgentSubConfig holds sub-agent configuration.
type MultiAgentSubConfig struct {
	ID            string   `yaml:"id" json:"id"`
	Name          string   `yaml:"name" json:"name"`
	Description   string   `yaml:"description" json:"description"`
	Instruction   string   `yaml:"instruction" json:"instruction"`
	BindRole      string   `yaml:"bind_role,omitempty" json:"bind_role,omitempty"`
	RoleTools     []string `yaml:"role_tools" json:"role_tools"`
	MaxIterations int      `yaml:"max_iterations" json:"max_iterations"`
	Kind          string   `yaml:"kind,omitempty" json:"kind,omitempty"`
}

// MultiAgentPublic is the public-facing subset of MultiAgentConfig.
type MultiAgentPublic struct {
	Enabled            bool   `json:"enabled"`
	DefaultMode        string `json:"default_mode"`
	RobotUseMultiAgent bool   `json:"robot_use_multi_agent"`
	BatchUseMultiAgent bool   `json:"batch_use_multi_agent"`
	SubAgentCount      int    `json:"sub_agent_count"`
}

// MultiAgentAPIUpdate holds fields updatable via API (does not overwrite sub_agents).
type MultiAgentAPIUpdate struct {
	Enabled            bool   `json:"enabled"`
	DefaultMode        string `json:"default_mode"`
	RobotUseMultiAgent bool   `json:"robot_use_multi_agent"`
	BatchUseMultiAgent bool   `json:"batch_use_multi_agent"`
}

// MCPServerURL returns the URL for CyberStrikeAI's MCP server, or "" if MCP is disabled.
func (c *Config) MCPServerURL() string {
	if c == nil || !c.MCP.Enabled {
		return ""
	}
	host := c.MCP.Host
	if host == "0.0.0.0" {
		host = "127.0.0.1"
	}
	return fmt.Sprintf("http://%s:%d/mcp", host, c.MCP.Port)
}

// EffectiveProvider returns the active LLM provider, defaulting to "openai".
func (c *Config) EffectiveProvider() string {
	if c == nil || strings.TrimSpace(c.Provider) == "" {
		return "openai"
	}
	return strings.TrimSpace(c.Provider)
}

// ClaudeCLIConfig holds settings for the Claude CLI provider.
type ClaudeCLIConfig struct {
	Workdir      string   `yaml:"workdir,omitempty" json:"workdir,omitempty"`             // Working directory for claude binary (default: cwd)
	MaxTurns     int      `yaml:"max_turns,omitempty" json:"max_turns,omitempty"`         // Max agentic turns (0 = unlimited)
	AllowedTools []string `yaml:"allowed_tools,omitempty" json:"allowed_tools,omitempty"` // Restrict claude to these tools (empty = all)
}

// RobotsConfig holds bot configuration (Telegram only).
type RobotsConfig struct {
	Telegram RobotTelegramConfig `yaml:"telegram,omitempty" json:"telegram,omitempty"` // Telegram
}

// RobotTelegramConfig holds the Telegram bot configuration.
type RobotTelegramConfig struct {
	Enabled        bool    `yaml:"enabled" json:"enabled"`
	BotToken       string  `yaml:"bot_token" json:"bot_token"`
	AllowedUserIDs []int64 `yaml:"allowed_user_ids,omitempty" json:"allowed_user_ids,omitempty"`
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type LogConfig struct {
	Level  string `yaml:"level"`
	Output string `yaml:"output"`
}

type DebugConfig struct {
	Enabled    bool `yaml:"enabled" json:"enabled"`
	RetainDays int  `yaml:"retain_days" json:"retain_days"`
}

type MCPConfig struct {
	Enabled         bool   `yaml:"enabled"`
	Host            string `yaml:"host"`
	Port            int    `yaml:"port"`
	AuthHeader      string `yaml:"auth_header,omitempty"`       // Auth header name (empty = no auth)
	AuthHeaderValue string `yaml:"auth_header_value,omitempty"` // Auth header value
}

type OpenAIConfig struct {
	Provider         string `yaml:"provider,omitempty" json:"provider,omitempty"` // "openai" (default) or "anthropic"
	APIKey           string `yaml:"api_key" json:"api_key"`
	BaseURL          string `yaml:"base_url" json:"base_url"`
	Model            string `yaml:"model" json:"model"`
	ToolModel        string `yaml:"tool_model,omitempty" json:"tool_model,omitempty"`
	ToolBaseURL      string `yaml:"tool_base_url,omitempty" json:"tool_base_url,omitempty"`
	ToolAPIKey       string `yaml:"tool_api_key,omitempty" json:"tool_api_key,omitempty"`
	SummaryModel     string `yaml:"summary_model,omitempty" json:"summary_model,omitempty"`
	SummaryBaseURL   string `yaml:"summary_base_url,omitempty" json:"summary_base_url,omitempty"`
	SummaryAPIKey    string `yaml:"summary_api_key,omitempty" json:"summary_api_key,omitempty"`
	MaxTotalTokens   int    `yaml:"max_total_tokens,omitempty" json:"max_total_tokens,omitempty"`
	RateLimitDelayMs int    `yaml:"rate_limit_delay_ms,omitempty" json:"rate_limit_delay_ms,omitempty"`
	// Main model sampling (orchestrator/planner - creative, exploratory)
	Temperature float64 `yaml:"temperature,omitempty" json:"temperature,omitempty"` // 0.0-2.0 (0=default). Higher = more creative planning.
	TopP        float64 `yaml:"top_p,omitempty" json:"top_p,omitempty"`             // 0.0-1.0 (0=default).
	TopK        int     `yaml:"top_k,omitempty" json:"top_k,omitempty"`             // Anthropic only.
	// Tool model sampling (executor - precise, deterministic)
	ToolTemperature float64 `yaml:"tool_temperature,omitempty" json:"tool_temperature,omitempty"` // 0.0-2.0 (0=inherit main). Lower = more precise tool args.
	ToolTopP        float64 `yaml:"tool_top_p,omitempty" json:"tool_top_p,omitempty"`             // 0.0-1.0 (0=inherit main).
	// Summary model sampling (compressor - factual, concise)
	SummaryTemperature float64 `yaml:"summary_temperature,omitempty" json:"summary_temperature,omitempty"` // 0.0-2.0 (0=inherit main). Low recommended for accurate summaries.
	SummaryTopP        float64 `yaml:"summary_top_p,omitempty" json:"summary_top_p,omitempty"`             // 0.0-1.0 (0=inherit main).
}

// ApplyModelDefaults normalizes model fields:
// - If Model is empty, use defaultMainModel.
// - If ToolModel is empty, fall back to Model.
// - If SummaryModel is empty, fall back to Model.
func (c *OpenAIConfig) ApplyModelDefaults(defaultMainModel string) {
	if c == nil {
		return
	}
	if strings.TrimSpace(defaultMainModel) == "" {
		defaultMainModel = "gpt-4"
	}
	if strings.TrimSpace(c.Model) == "" {
		c.Model = defaultMainModel
	}
	if strings.TrimSpace(c.ToolModel) == "" {
		c.ToolModel = c.Model
	}
	if strings.TrimSpace(c.SummaryModel) == "" {
		c.SummaryModel = c.Model
	}
}

// EffectiveToolConfig returns the base URL and API key to use for tool-calling requests.
// Falls back to the main config values when tool-specific ones are empty.
func (c *OpenAIConfig) EffectiveToolConfig() (baseURL, apiKey string) {
	baseURL = c.ToolBaseURL
	if baseURL == "" {
		baseURL = c.BaseURL
	}
	apiKey = c.ToolAPIKey
	if apiKey == "" {
		apiKey = c.APIKey
	}
	return
}

// EffectiveSummaryConfig returns the base URL and API key to use for summarization requests.
// Falls back to the main config values when summary-specific ones are empty.
func (c *OpenAIConfig) EffectiveSummaryConfig() (baseURL, apiKey string) {
	baseURL = c.SummaryBaseURL
	if baseURL == "" {
		baseURL = c.BaseURL
	}
	apiKey = c.SummaryAPIKey
	if apiKey == "" {
		apiKey = c.APIKey
	}
	return
}

type FofaConfig struct {
	// Email is the FOFA account email; APIKey is the FOFA API Key (read-only key recommended)
	Email   string `yaml:"email,omitempty" json:"email,omitempty"`
	APIKey  string `yaml:"api_key,omitempty" json:"api_key,omitempty"`
	BaseURL string `yaml:"base_url,omitempty" json:"base_url,omitempty"` // Default: https://fofa.info/api/v1/search/all
}

type ZoomEyeConfig struct {
	APIKey string `yaml:"api_key,omitempty" json:"api_key,omitempty"` // ZoomEye API key (from https://www.zoomeye.ai/profile)
}

type ShodanConfig struct {
	APIKey string `yaml:"api_key,omitempty" json:"api_key,omitempty"` // Shodan API key (from https://account.shodan.io)
}

type CensysConfig struct {
	APIID     string `yaml:"api_id,omitempty" json:"api_id,omitempty"`         // Censys API ID
	APISecret string `yaml:"api_secret,omitempty" json:"api_secret,omitempty"` // Censys API Secret
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
	MaxIterations           int                 `yaml:"max_iterations" json:"max_iterations"`
	LargeResultThreshold    int                 `yaml:"large_result_threshold" json:"large_result_threshold"`         // Large-result threshold (bytes), default 50 KB
	ResultStorageDir        string              `yaml:"result_storage_dir" json:"result_storage_dir"`                 // Result storage directory, default tmp
	ToolTimeoutMinutes      int                 `yaml:"tool_timeout_minutes" json:"tool_timeout_minutes"`             // Per-tool max execution time in minutes (0 = unlimited)
	ParallelToolExecution   bool                `yaml:"parallel_tool_execution" json:"parallel_tool_execution"`       // Execute multiple tool calls concurrently (default true)
	MaxParallelTools        int                 `yaml:"max_parallel_tools" json:"max_parallel_tools"`                 // Maximum concurrent tool calls (0 = unlimited)
	ToolRetryCount          int                 `yaml:"tool_retry_count" json:"tool_retry_count"`                     // Number of retries on transient tool errors (default 0)
	ParallelToolWaitSeconds int                 `yaml:"parallel_tool_wait_seconds" json:"parallel_tool_wait_seconds"` // Max wait per parallel tool before deferring (default 45s)
	TimeAwareness           TimeAwarenessConfig `yaml:"time_awareness" json:"time_awareness"`                         // Temporal context injection settings
	Memory                  MemoryConfig        `yaml:"memory" json:"memory"`                                         // Persistent memory settings
	FileManager             FileManagerConfig   `yaml:"file_manager" json:"file_manager"`                             // File manager settings
	Cuttlefish              CuttlefishConfig    `yaml:"cuttlefish" json:"cuttlefish"`                                 // Android VM (Cuttlefish) settings
	SSLStrip                SSLStripConfig      `yaml:"sslstrip" json:"sslstrip"`                                     // SSLStrip MITM tool settings
	Proxy                   ProxyConfig         `yaml:"proxy" json:"proxy"`                                           // Global proxy middleware for tool traffic routing
}

// TimeAwarenessConfig controls whether and how the agent injects time context.
type TimeAwarenessConfig struct {
	Enabled    bool   `yaml:"enabled" json:"enabled"` // Inject current date/time into every system prompt (default true)
	EnabledSet bool   `yaml:"-" json:"-"`
	Timezone   string `yaml:"timezone" json:"timezone"` // IANA timezone name, e.g. "America/New_York" (default "UTC")
}

// MemoryConfig controls persistent cross-conversation memory behaviour.
type MemoryConfig struct {
	Enabled    bool `yaml:"enabled" json:"enabled"` // Enable the persistent memory store (default true)
	EnabledSet bool `yaml:"-" json:"-"`
	MaxEntries int  `yaml:"max_entries" json:"max_entries"` // Hard cap on stored memory entries, 0 = unlimited
}

// FileManagerConfig controls the file manager module.
type FileManagerConfig struct {
	Enabled    bool   `yaml:"enabled" json:"enabled"` // Enable file manager (default true)
	EnabledSet bool   `yaml:"-" json:"-"`
	StorageDir string `yaml:"storage_dir" json:"storage_dir"` // Directory for managed file storage (default "managed_files")
}

func markEnabledPresence(node *yaml.Node, target *bool) {
	if node == nil || node.Kind != yaml.MappingNode || target == nil {
		return
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Kind == yaml.ScalarNode && node.Content[i].Value == "enabled" {
			*target = true
			return
		}
	}
}

func (c *MCPConfig) UnmarshalYAML(value *yaml.Node) error {
	type raw MCPConfig
	var out raw
	if err := value.Decode(&out); err != nil {
		return err
	}
	*c = MCPConfig(out)
	return nil
}

func (c *TimeAwarenessConfig) UnmarshalYAML(value *yaml.Node) error {
	type raw TimeAwarenessConfig
	var out raw
	if err := value.Decode(&out); err != nil {
		return err
	}
	*c = TimeAwarenessConfig(out)
	markEnabledPresence(value, &c.EnabledSet)
	return nil
}

func (c *MemoryConfig) UnmarshalYAML(value *yaml.Node) error {
	type raw MemoryConfig
	var out raw
	if err := value.Decode(&out); err != nil {
		return err
	}
	*c = MemoryConfig(out)
	markEnabledPresence(value, &c.EnabledSet)
	return nil
}

func (c *FileManagerConfig) UnmarshalYAML(value *yaml.Node) error {
	type raw FileManagerConfig
	var out raw
	if err := value.Decode(&out); err != nil {
		return err
	}
	*c = FileManagerConfig(out)
	markEnabledPresence(value, &c.EnabledSet)
	return nil
}

// CuttlefishConfig controls the Cuttlefish Android VM integration.
type CuttlefishConfig struct {
	Enabled         bool   `yaml:"enabled" json:"enabled"`                   // Enable Cuttlefish tools (default true)
	CvdHome         string `yaml:"cvd_home" json:"cvd_home"`                 // Cuttlefish workspace directory (default ~/cuttlefish-workspace)
	MemoryMB        int    `yaml:"memory_mb" json:"memory_mb"`               // VM RAM in MB (default 8192)
	CPUs            int    `yaml:"cpus" json:"cpus"`                         // VM CPU count (default 4)
	DiskMB          int    `yaml:"disk_mb" json:"disk_mb"`                   // Data partition size in MB (default 16000)
	GPUMode         string `yaml:"gpu_mode" json:"gpu_mode"`                 // GPU mode: guest_swiftshader, drm_virgl (default guest_swiftshader)
	AutoLaunch      bool   `yaml:"auto_launch" json:"auto_launch"`           // Auto-launch VM on server start (default false)
	RussianIdentity bool   `yaml:"russian_identity" json:"russian_identity"` // Apply Russian phone identity on boot (default true)
	WebRTCPort      int    `yaml:"webrtc_port" json:"webrtc_port"`           // WebRTC display port (default 8443)
	DroidRunPath    string `yaml:"droidrun_path" json:"droidrun_path"`       // Path to DroidRun installation (default ~/droidrun)
	DroidRunConfig  string `yaml:"droidrun_config" json:"droidrun_config"`   // Path to DroidRun config YAML (default <cvd_home>/droidrun/config.yaml)
	BridgeScript    string `yaml:"bridge_script" json:"bridge_script"`       // Path to droidrun-bridge.py (auto-detected if empty)
	ProxyPort       int    `yaml:"proxy_port" json:"proxy_port"`             // DroidRun proxy HTTP service port (default 18090)
	ProxyAutoStart  bool   `yaml:"proxy_auto_start" json:"proxy_auto_start"` // Auto-start DroidRun proxy when VM launches (default true)
	ScreenshotDir   string `yaml:"screenshot_dir" json:"screenshot_dir"`     // Directory for screenshots from proxy (default /tmp/droidrun_screenshots)
	VisionEnabled   bool   `yaml:"vision_enabled" json:"vision_enabled"`     // Include screenshots in state responses for VL models (default true)
}

// SSLStripConfig controls SSLStrip MITM tool integration.
type SSLStripConfig struct {
	Enabled    bool   `yaml:"enabled" json:"enabled"`         // Enable SSLStrip tool (default true)
	ListenPort int    `yaml:"listen_port" json:"listen_port"` // Default listen port (default 10000)
	LogDir     string `yaml:"log_dir" json:"log_dir"`         // Directory for SSLStrip capture logs (default /tmp)
	AutoProxy  bool   `yaml:"auto_proxy" json:"auto_proxy"`   // Auto-configure Cuttlefish proxy when SSLStrip starts (default false)
}

// ProxyConfig controls global proxy middleware for all tool executions.
// All tools spawned by the executor inherit proxy environment variables.
// Tools that don't respect env vars (nmap, masscan) use proxychains wrapper.
type ProxyConfig struct {
	Enabled      bool     `yaml:"enabled" json:"enabled"`                     // Enable proxy middleware (default false)
	Type         string   `yaml:"type" json:"type"`                           // Proxy type: tor, socks5, socks5h, http, https (default socks5h)
	Host         string   `yaml:"host" json:"host"`                           // Proxy host (default 127.0.0.1)
	Port         int      `yaml:"port" json:"port"`                           // Proxy port (default 9050 for tor, 1080 for socks5)
	Username     string   `yaml:"username,omitempty" json:"username"`         // Auth username (optional, for socks5/http with auth)
	Password     string   `yaml:"password,omitempty" json:"password"`         // Auth password (optional)
	NoProxy      string   `yaml:"no_proxy" json:"no_proxy"`                   // Comma-separated bypass list (default: localhost,127.0.0.1,*.local)
	ExemptTools  []string `yaml:"exempt_tools,omitempty" json:"exempt_tools"` // Tools that bypass proxy (e.g. tools that need direct connection)
	TorAutoStart bool     `yaml:"tor_auto_start" json:"tor_auto_start"`       // Auto-start tor service if type=tor and tor not running (default false)
	ProxyChains  bool     `yaml:"proxychains" json:"proxychains"`             // Wrap non-env-aware tools (nmap, masscan) with proxychains (default false)
	DNSProxy     bool     `yaml:"dns_proxy" json:"dns_proxy"`                 // Route DNS through proxy (socks5h, tor). True by default for tor. (default true)
	HealthCheck  bool     `yaml:"health_check" json:"health_check"`           // Verify proxy works at startup (default true)
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
	Name        string      `yaml:"name"`                // Parameter name
	Type        string      `yaml:"type"`                // Parameter type: string, int, bool, array
	Description string      `yaml:"description"`         // Parameter description
	Required    bool        `yaml:"required,omitempty"`  // Whether the parameter is required
	Default     interface{} `yaml:"default,omitempty"`   // Default value
	ItemType    string      `yaml:"item_type,omitempty"` // When type is array, the element type (string, number, object)
	Flag        string      `yaml:"flag,omitempty"`      // Command-line flag, e.g. "-u", "--url", "-p"
	Position    *int        `yaml:"position,omitempty"`  // Position of a positional parameter (0-based)
	Format      string      `yaml:"format,omitempty"`    // Parameter format: "flag", "positional", "combined" (flag=value), "template"
	Template    string      `yaml:"template,omitempty"`  // Template string, e.g. "{flag} {value}" or "{value}"
	Options     []string    `yaml:"options,omitempty"`   // List of allowed values (for enum parameters)
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

	// Ensure omitted model fields always fall back to defaults.
	cfg.ApplyModelDefaults()

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
			// Env-var resolution happens lazily at connection time inside the mcp
			// package (createSDKClient) so the templated strings stay in the
			// stored config and saveConfig never writes resolved secrets to disk.
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

// generateRandomToken generates a random hex string for MCP auth.
func generateRandomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// persistMCPAuth writes MCP auth_header / auth_header_value back to the config file.
func persistMCPAuth(path string, mcp *MCPConfig) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")
	inMcpBlock := false
	mcpIndent := -1

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !inMcpBlock {
			if strings.HasPrefix(trimmed, "mcp:") {
				inMcpBlock = true
				mcpIndent = len(line) - len(strings.TrimLeft(line, " "))
			}
			continue
		}
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		leadingSpaces := len(line) - len(strings.TrimLeft(line, " "))
		if leadingSpaces <= mcpIndent {
			inMcpBlock = false
			mcpIndent = -1
			if strings.HasPrefix(trimmed, "mcp:") {
				inMcpBlock = true
				mcpIndent = leadingSpaces
			}
			continue
		}

		prefix := line[:leadingSpaces]
		rest := strings.TrimSpace(line[leadingSpaces:])
		comment := ""
		if idx := strings.Index(line, "#"); idx >= 0 {
			comment = strings.TrimRight(line[idx:], " ")
		}
		withComment := ""
		if comment != "" {
			if !strings.HasPrefix(comment, " ") {
				withComment = " "
			}
			withComment += comment
		}

		if strings.HasPrefix(rest, "auth_header_value:") {
			lines[i] = fmt.Sprintf("%sauth_header_value: %q%s", prefix, mcp.AuthHeaderValue, withComment)
		} else if strings.HasPrefix(rest, "auth_header:") {
			lines[i] = fmt.Sprintf("%sauth_header: %q%s", prefix, mcp.AuthHeader, withComment)
		}
	}

	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644)
}

// EnsureMCPAuth auto-generates a random MCP auth key when MCP is enabled but auth_header_value is empty.
func EnsureMCPAuth(path string, cfg *Config) error {
	if !cfg.MCP.Enabled || strings.TrimSpace(cfg.MCP.AuthHeaderValue) != "" {
		return nil
	}
	token, err := generateRandomToken()
	if err != nil {
		return fmt.Errorf("failed to generate MCP auth key: %w", err)
	}
	cfg.MCP.AuthHeaderValue = token
	if strings.TrimSpace(cfg.MCP.AuthHeader) == "" {
		cfg.MCP.AuthHeader = "X-MCP-Token"
	}
	return persistMCPAuth(path, &cfg.MCP)
}

// PrintMCPConfigJSON prints MCP config JSON to terminal for use in Cursor / Claude Code.
func PrintMCPConfigJSON(mcp MCPConfig) {
	if !mcp.Enabled {
		return
	}
	hostForURL := strings.TrimSpace(mcp.Host)
	if hostForURL == "" || hostForURL == "0.0.0.0" {
		hostForURL = "localhost"
	}
	url := fmt.Sprintf("http://%s:%d/mcp", hostForURL, mcp.Port)
	headers := map[string]string{}
	if mcp.AuthHeader != "" {
		headers[mcp.AuthHeader] = mcp.AuthHeaderValue
	}
	serverEntry := map[string]interface{}{
		"url": url,
	}
	if len(headers) > 0 {
		serverEntry["headers"] = headers
	}
	serverEntry["type"] = "http"
	out := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"cyberstrike-ai": serverEntry,
		},
	}
	b, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println("[CyberStrikeAI] MCP config (copy to Cursor / Claude Code):")
	fmt.Println("  Cursor: place in ~/.cursor/mcp.json mcpServers, or project .cursor/mcp.json")
	fmt.Println("  Claude Code: place in .mcp.json or ~/.claude.json mcpServers")
	fmt.Println("----------------------------------------------------------------")
	fmt.Println(string(b))
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
			MaxIterations:      30, // Default maximum iteration count
			ToolTimeoutMinutes: 10, // Per-tool default max 10 minutes
			TimeAwareness: TimeAwarenessConfig{
				Enabled:    true,
				EnabledSet: true,
				Timezone:   "UTC",
			},
			Memory: MemoryConfig{
				Enabled:    true,
				EnabledSet: true,
				MaxEntries: 200,
			},
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
				SimilarityThreshold: 0.65, // lower threshold to 0.65,reduce missed detections
				HybridWeight:        0.7,
			},
			Indexing: IndexingConfig{
				ChunkSize:        768, // increase to 768,better context preservation
				ChunkOverlap:     50,
				MaxChunksPerItem: 20,  // limit each knowledge item to max 20 chunks, avoid excessive quota consumption
				MaxRPM:           100, // default 100 RPM, 429 error
				RateLimitDelayMs: 600, // 600ms interval, corresponding to 100 RPM
				MaxRetries:       3,
				RetryDelayMs:     1000,
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
	Indexing  IndexingConfig  `yaml:"indexing,omitempty" json:"indexing,omitempty"` // index build config
}

// IndexingConfig index build config(knowledge base)
type IndexingConfig struct {
	// chunking config
	ChunkSize        int `yaml:"chunk_size,omitempty" json:"chunk_size,omitempty"`                   // max tokens per chunk (estimated),default 512
	ChunkOverlap     int `yaml:"chunk_overlap,omitempty" json:"chunk_overlap,omitempty"`             // overlap tokens between chunks,default 50
	MaxChunksPerItem int `yaml:"max_chunks_per_item,omitempty" json:"max_chunks_per_item,omitempty"` // ,0

	// rate limit config(to avoid API rate limits)
	RateLimitDelayMs int `yaml:"rate_limit_delay_ms,omitempty" json:"rate_limit_delay_ms,omitempty"` // request interval time(),0
	MaxRPM           int `yaml:"max_rpm,omitempty" json:"max_rpm,omitempty"`                         // ,0

	// retry config(error)
	MaxRetries   int `yaml:"max_retries,omitempty" json:"max_retries,omitempty"`       // max retry count,default 3
	RetryDelayMs int `yaml:"retry_delay_ms,omitempty" json:"retry_delay_ms,omitempty"` // retry delay(),default 1000

	// batch processing config(,current,)
	BatchSize int `yaml:"batch_size,omitempty" json:"batch_size,omitempty"` // ,0
}

// EmbeddingConfig holds the embedding model configuration.
type EmbeddingConfig struct {
	Provider  string `yaml:"provider" json:"provider"`     // Embedding model provider
	Model     string `yaml:"model" json:"model"`           // Model name
	BaseURL   string `yaml:"base_url" json:"base_url"`     // API Base URL
	APIKey    string `yaml:"api_key" json:"api_key"`       // API Key (inherited from OpenAI config)
	MaxTokens int    `yaml:"max_tokens" json:"max_tokens"` // Embedding model max token limit (0 = default 512); chunks are sized to fit within this limit
}

// ApplyModelDefaults normalizes model-related fields across config sections.
// It ensures all model fields have a valid fallback when omitted by user config.
func (c *Config) ApplyModelDefaults() {
	if c == nil {
		return
	}
	defaultCfg := Default()

	// Main/tool/summary model fallback chain.
	c.OpenAI.ApplyModelDefaults(defaultCfg.OpenAI.Model)

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
