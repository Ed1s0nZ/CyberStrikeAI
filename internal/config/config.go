package config

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Version     string                `yaml:"version,omitempty" json:"version,omitempty"` // 前端显示的版本号，如 v1.3.3
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
	Robots      RobotsConfig          `yaml:"robots,omitempty" json:"robots,omitempty"`         // 企业微信/钉钉/飞书等机器人配置
	RolesDir    string                `yaml:"roles_dir,omitempty" json:"roles_dir,omitempty"`   // 角色配置文件目录（新方式）
	Roles       map[string]RoleConfig `yaml:"roles,omitempty" json:"roles,omitempty"`           // 向后兼容：支持在主配置文件中定义角色
	SkillsDir   string                `yaml:"skills_dir,omitempty" json:"skills_dir,omitempty"` // Skills配置文件目录
}

// RobotsConfig 机器人配置（企业微信、钉钉、飞书等）
type RobotsConfig struct {
	Wecom   RobotWecomConfig   `yaml:"wecom,omitempty" json:"wecom,omitempty"`     // 企业微信
	Dingtalk RobotDingtalkConfig `yaml:"dingtalk,omitempty" json:"dingtalk,omitempty"` // 钉钉
	Lark    RobotLarkConfig    `yaml:"lark,omitempty" json:"lark,omitempty"`     // 飞书
}

// RobotWecomConfig 企业微信机器人配置
type RobotWecomConfig struct {
	Enabled       bool   `yaml:"enabled" json:"enabled"`
	Token         string `yaml:"token" json:"token"`                     // 回调 URL 校验 Token
	EncodingAESKey string `yaml:"encoding_aes_key" json:"encoding_aes_key"` // EncodingAESKey
	CorpID        string `yaml:"corp_id" json:"corp_id"`               // 企业 ID
	Secret        string `yaml:"secret" json:"secret"`                  // 应用 Secret
	AgentID       int64  `yaml:"agent_id" json:"agent_id"`              // 应用 AgentId
}

// RobotDingtalkConfig 钉钉机器人配置
type RobotDingtalkConfig struct {
	Enabled      bool   `yaml:"enabled" json:"enabled"`
	ClientID     string `yaml:"client_id" json:"client_id"`         // 应用 Key (AppKey)
	ClientSecret string `yaml:"client_secret" json:"client_secret"` // 应用 Secret
}

// RobotLarkConfig 飞书机器人配置
type RobotLarkConfig struct {
	Enabled   bool   `yaml:"enabled" json:"enabled"`
	AppID     string `yaml:"app_id" json:"app_id"`         // 应用 App ID
	AppSecret string `yaml:"app_secret" json:"app_secret"` // 应用 App Secret
	VerifyToken string `yaml:"verify_token" json:"verify_token"` // 事件订阅 Verification Token（可选）
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
	Enabled         bool   `yaml:"enabled"`
	Host            string `yaml:"host"`
	Port            int    `yaml:"port"`
	AuthHeader      string `yaml:"auth_header,omitempty"`      // 鉴权 header 名，留空表示不鉴权
	AuthHeaderValue string `yaml:"auth_header_value,omitempty"` // 鉴权 header 值，需与请求中该 header 一致
}

type OpenAIConfig struct {
	APIKey         string `yaml:"api_key" json:"api_key"`
	APIKeyKeychain string `yaml:"api_key_keychain,omitempty" json:"api_key_keychain,omitempty"` // macOS Keychain service name (account: "cyberstrike")
	BaseURL        string `yaml:"base_url" json:"base_url"`
	Model          string `yaml:"model" json:"model"`
	MaxTotalTokens int    `yaml:"max_total_tokens,omitempty" json:"max_total_tokens,omitempty"`
	Provider       string `yaml:"provider,omitempty" json:"provider,omitempty"` // "openai" (default) or "anthropic"
}

type FofaConfig struct {
	// Email 为 FOFA 账号邮箱；APIKey 为 FOFA API Key（建议使用只读权限的 Key）
	Email   string `yaml:"email,omitempty" json:"email,omitempty"`
	APIKey  string `yaml:"api_key,omitempty" json:"api_key,omitempty"`
	BaseURL string `yaml:"base_url,omitempty" json:"base_url,omitempty"` // 默认 https://fofa.info/api/v1/search/all
}

type SecurityConfig struct {
	Tools               []ToolConfig `yaml:"tools,omitempty"`                 // 向后兼容：支持在主配置文件中定义工具
	ToolsDir            string       `yaml:"tools_dir,omitempty"`             // 工具配置文件目录（新方式）
	ToolDescriptionMode string       `yaml:"tool_description_mode,omitempty"` // 工具描述模式: "short" | "full"，默认 short
}

type DatabaseConfig struct {
	Path            string `yaml:"path"`                        // 会话数据库路径
	KnowledgeDBPath string `yaml:"knowledge_db_path,omitempty"` // 知识库数据库路径（可选，为空则使用会话数据库）
}

type AgentConfig struct {
	MaxIterations        int    `yaml:"max_iterations" json:"max_iterations"`
	LargeResultThreshold int    `yaml:"large_result_threshold" json:"large_result_threshold"` // 大结果阈值（字节），默认50KB
	ResultStorageDir     string `yaml:"result_storage_dir" json:"result_storage_dir"`         // 结果存储目录，默认tmp
	ToolTimeoutMinutes   int    `yaml:"tool_timeout_minutes" json:"tool_timeout_minutes"`     // 单次工具执行最大时长（分钟），超时自动终止，防止长时间挂起；0 表示不限制（不推荐）
}

type AuthConfig struct {
	Password                    string `yaml:"password" json:"password"`
	SessionDurationHours        int    `yaml:"session_duration_hours" json:"session_duration_hours"`
	GeneratedPassword           string `yaml:"-" json:"-"`
	GeneratedPasswordPersisted  bool   `yaml:"-" json:"-"`
	GeneratedPasswordPersistErr string `yaml:"-" json:"-"`
}

// ExternalMCPConfig 外部MCP配置
type ExternalMCPConfig struct {
	Servers map[string]ExternalMCPServerConfig `yaml:"servers,omitempty" json:"servers,omitempty"`
}

// ExternalMCPServerConfig 外部MCP服务器配置
type ExternalMCPServerConfig struct {
	// stdio模式配置
	Command string            `yaml:"command,omitempty" json:"command,omitempty"`
	Args    []string          `yaml:"args,omitempty" json:"args,omitempty"`
	Env     map[string]string `yaml:"env,omitempty" json:"env,omitempty"` // 环境变量（用于stdio模式）

	// HTTP模式配置
	Transport string            `yaml:"transport,omitempty" json:"transport,omitempty"` // "stdio" | "sse" | "http"(Streamable) | "simple_http"(自建/简单POST端点，如本机 http://127.0.0.1:8081/mcp)
	URL       string            `yaml:"url,omitempty" json:"url,omitempty"`
	Headers   map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"` // HTTP/SSE 请求头（如 x-api-key）

	// 通用配置
	Description       string          `yaml:"description,omitempty" json:"description,omitempty"`
	Timeout           int             `yaml:"timeout,omitempty" json:"timeout,omitempty"`                         // 超时时间（秒）
	ExternalMCPEnable bool            `yaml:"external_mcp_enable,omitempty" json:"external_mcp_enable,omitempty"` // 是否启用外部MCP
	ToolEnabled       map[string]bool `yaml:"tool_enabled,omitempty" json:"tool_enabled,omitempty"`               // 每个工具的启用状态（工具名称 -> 是否启用）

	// 向后兼容字段（已废弃，保留用于读取旧配置）
	Enabled  bool `yaml:"enabled,omitempty" json:"enabled,omitempty"`   // 已废弃，使用 external_mcp_enable
	Disabled bool `yaml:"disabled,omitempty" json:"disabled,omitempty"` // 已废弃，使用 external_mcp_enable
}
type ToolConfig struct {
	Name             string            `yaml:"name"`
	Command          string            `yaml:"command"`
	Args             []string          `yaml:"args,omitempty"`              // 固定参数（可选）
	ShortDescription string            `yaml:"short_description,omitempty"` // 简短描述（用于工具列表，减少token消耗）
	Description      string            `yaml:"description"`                 // 详细描述（用于工具文档）
	Enabled          bool              `yaml:"enabled"`
	Parameters       []ParameterConfig `yaml:"parameters,omitempty"`         // 参数定义（可选）
	ArgMapping       string            `yaml:"arg_mapping,omitempty"`        // 参数映射方式: "auto", "manual", "template"（可选）
	AllowedExitCodes []int             `yaml:"allowed_exit_codes,omitempty"` // 允许的退出码列表（某些工具在成功时也返回非零退出码）
}

// ParameterConfig 参数配置
type ParameterConfig struct {
	Name        string      `yaml:"name"`                  // 参数名称
	Type        string      `yaml:"type"`                  // 参数类型: string, int, bool, array
	Description string      `yaml:"description"`           // 参数描述
	Required    bool        `yaml:"required,omitempty"`    // 是否必需
	Default     interface{} `yaml:"default,omitempty"`      // 默认值
	ItemType    string      `yaml:"item_type,omitempty"`    // 当 type 为 array 时，数组元素类型，如 string, number, object
	Flag        string      `yaml:"flag,omitempty"`         // 命令行标志，如 "-u", "--url", "-p"
	Position    *int        `yaml:"position,omitempty"`     // 位置参数的位置（从0开始）
	Format      string      `yaml:"format,omitempty"`      // 参数格式: "flag", "positional", "combined" (flag=value), "template"
	Template    string      `yaml:"template,omitempty"`    // 模板字符串，如 "{flag} {value}" 或 "{value}"
	Options     []string    `yaml:"options,omitempty"`     // 可选值列表（用于枚举）
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	if cfg.Auth.SessionDurationHours <= 0 {
		cfg.Auth.SessionDurationHours = 12
	}

	if strings.TrimSpace(cfg.Auth.Password) == "" {
		password, err := generateStrongPassword(24)
		if err != nil {
			return nil, fmt.Errorf("生成默认密码失败: %w", err)
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

	// If api_key is empty but api_key_keychain is set, resolve from macOS Keychain
	if strings.TrimSpace(cfg.OpenAI.APIKey) == "" && strings.TrimSpace(cfg.OpenAI.APIKeyKeychain) != "" {
		key, err := readFromKeychain(cfg.OpenAI.APIKeyKeychain)
		if err != nil {
			return nil, fmt.Errorf("failed to read API key from macOS Keychain (service: %s): %w", cfg.OpenAI.APIKeyKeychain, err)
		}
		cfg.OpenAI.APIKey = key
	}

	// 如果配置了工具目录，从目录加载工具配置
	if cfg.Security.ToolsDir != "" {
		configDir := filepath.Dir(path)
		toolsDir := cfg.Security.ToolsDir

		// 如果是相对路径，相对于配置文件所在目录
		if !filepath.IsAbs(toolsDir) {
			toolsDir = filepath.Join(configDir, toolsDir)
		}

		tools, err := LoadToolsFromDir(toolsDir)
		if err != nil {
			return nil, fmt.Errorf("从工具目录加载工具配置失败: %w", err)
		}

		// 合并工具配置：目录中的工具优先，主配置中的工具作为补充
		existingTools := make(map[string]bool)
		for _, tool := range tools {
			existingTools[tool.Name] = true
		}

		// 添加主配置中不存在于目录中的工具（向后兼容）
		for _, tool := range cfg.Security.Tools {
			if !existingTools[tool.Name] {
				tools = append(tools, tool)
			}
		}

		cfg.Security.Tools = tools
	}

	// 迁移外部MCP配置：将旧的 enabled/disabled 字段迁移到 external_mcp_enable
	if cfg.ExternalMCP.Servers != nil {
		for name, serverCfg := range cfg.ExternalMCP.Servers {
			// 如果已经设置了 external_mcp_enable，跳过迁移
			// 否则从 enabled/disabled 字段迁移
			// 注意：由于 ExternalMCPEnable 是 bool 类型，零值为 false，所以需要检查是否真的设置了
			// 这里我们通过检查旧的 enabled/disabled 字段来判断是否需要迁移
			if serverCfg.Disabled {
				// 旧配置使用 disabled，迁移到 external_mcp_enable
				serverCfg.ExternalMCPEnable = false
			} else if serverCfg.Enabled {
				// 旧配置使用 enabled，迁移到 external_mcp_enable
				serverCfg.ExternalMCPEnable = true
			} else {
				// 都没有设置，默认为启用
				serverCfg.ExternalMCPEnable = true
			}
			cfg.ExternalMCP.Servers[name] = serverCfg
		}
	}

	// 从角色目录加载角色配置
	if cfg.RolesDir != "" {
		configDir := filepath.Dir(path)
		rolesDir := cfg.RolesDir

		// 如果是相对路径，相对于配置文件所在目录
		if !filepath.IsAbs(rolesDir) {
			rolesDir = filepath.Join(configDir, rolesDir)
		}

		roles, err := LoadRolesFromDir(rolesDir)
		if err != nil {
			return nil, fmt.Errorf("从角色目录加载角色配置失败: %w", err)
		}

		cfg.Roles = roles
	} else {
		// 如果未配置 roles_dir，初始化为空 map
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
			// 离开 auth 块
			inAuthBlock = false
			authIndent = -1
			// 继续寻找其它 auth 块（理论上没有）
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
		fmt.Println("[CyberStrikeAI] ✅ 已为您自动生成并写入 Web 登录密码。")
	} else {
		if persistErr != "" {
			fmt.Printf("[CyberStrikeAI] ⚠️ 无法自动写入配置文件中的密码: %s\n", persistErr)
		} else {
			fmt.Println("[CyberStrikeAI] ⚠️ 无法自动写入配置文件中的密码。")
		}
		fmt.Println("请手动将以下随机密码写入 config.yaml 的 auth.password：")
	}

	fmt.Println("----------------------------------------------------------------")
	fmt.Println("CyberStrikeAI Auto-Generated Web Password")
	fmt.Printf("Password: %s\n", password)
	fmt.Println("WARNING: Anyone with this password can fully control CyberStrikeAI.")
	fmt.Println("Please store it securely and change it in config.yaml as soon as possible.")
	fmt.Println("警告：持有此密码的人将拥有对 CyberStrikeAI 的完全控制权限。")
	fmt.Println("请妥善保管，并尽快在 config.yaml 中修改 auth.password！")
	fmt.Println("----------------------------------------------------------------")
}

// generateRandomToken 生成用于 MCP 鉴权的随机字符串（64 位十六进制）
func generateRandomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// persistMCPAuth 将 MCP 的 auth_header / auth_header_value 写回配置文件
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

// EnsureMCPAuth 在 MCP 启用且 auth_header_value 为空时，自动生成随机密钥并写回配置
func EnsureMCPAuth(path string, cfg *Config) error {
	if !cfg.MCP.Enabled || strings.TrimSpace(cfg.MCP.AuthHeaderValue) != "" {
		return nil
	}
	token, err := generateRandomToken()
	if err != nil {
		return fmt.Errorf("生成 MCP 鉴权密钥失败: %w", err)
	}
	cfg.MCP.AuthHeaderValue = token
	if strings.TrimSpace(cfg.MCP.AuthHeader) == "" {
		cfg.MCP.AuthHeader = "X-MCP-Token"
	}
	return persistMCPAuth(path, &cfg.MCP)
}

// PrintMCPConfigJSON 向终端输出 MCP 配置的 JSON，可直接复制到 Cursor / Claude Code 的 mcp 配置中使用
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
	// Claude Code 需要 type: "http"
	serverEntry["type"] = "http"
	out := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"cyberstrike-ai": serverEntry,
		},
	}
	b, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println("[CyberStrikeAI] MCP 配置（可复制到 Cursor / Claude Code 使用）：")
	fmt.Println("  Cursor: 放入 ~/.cursor/mcp.json 的 mcpServers，或项目 .cursor/mcp.json")
	fmt.Println("  Claude Code: 放入 .mcp.json 或 ~/.claude.json 的 mcpServers")
	fmt.Println("----------------------------------------------------------------")
	fmt.Println(string(b))
	fmt.Println("----------------------------------------------------------------")
}

// LoadToolsFromDir 从目录加载所有工具配置文件
func LoadToolsFromDir(dir string) ([]ToolConfig, error) {
	var tools []ToolConfig

	// 检查目录是否存在
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return tools, nil // 目录不存在时返回空列表，不报错
	}

	// 读取目录中的所有 .yaml 和 .yml 文件
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("读取工具目录失败: %w", err)
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
			// 记录错误但继续加载其他文件
			fmt.Printf("警告: 加载工具配置文件 %s 失败: %v\n", filePath, err)
			continue
		}

		tools = append(tools, *tool)
	}

	return tools, nil
}

// LoadToolFromFile 从单个文件加载工具配置
func LoadToolFromFile(path string) (*ToolConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}

	var tool ToolConfig
	if err := yaml.Unmarshal(data, &tool); err != nil {
		return nil, fmt.Errorf("解析工具配置失败: %w", err)
	}

	// 验证必需字段
	if tool.Name == "" {
		return nil, fmt.Errorf("工具名称不能为空")
	}
	if tool.Command == "" {
		return nil, fmt.Errorf("工具命令不能为空")
	}

	return &tool, nil
}

// LoadRolesFromDir 从目录加载所有角色配置文件
func LoadRolesFromDir(dir string) (map[string]RoleConfig, error) {
	roles := make(map[string]RoleConfig)

	// 检查目录是否存在
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return roles, nil // 目录不存在时返回空map，不报错
	}

	// 读取目录中的所有 .yaml 和 .yml 文件
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("读取角色目录失败: %w", err)
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
			// 记录错误但继续加载其他文件
			fmt.Printf("警告: 加载角色配置文件 %s 失败: %v\n", filePath, err)
			continue
		}

		// 使用角色名称作为key
		roleName := role.Name
		if roleName == "" {
			// 如果角色名称为空，使用文件名（去掉扩展名）作为名称
			roleName = strings.TrimSuffix(strings.TrimSuffix(name, ".yaml"), ".yml")
			role.Name = roleName
		}

		roles[roleName] = *role
	}

	return roles, nil
}

// LoadRoleFromFile 从单个文件加载角色配置
func LoadRoleFromFile(path string) (*RoleConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}

	var role RoleConfig
	if err := yaml.Unmarshal(data, &role); err != nil {
		return nil, fmt.Errorf("解析角色配置失败: %w", err)
	}

	// 处理 icon 字段：如果包含 Unicode 转义格式（\U0001F3C6），转换为实际的 Unicode 字符
	// Go 的 yaml 库可能不会自动解析 \U 转义序列，需要手动转换
	if role.Icon != "" {
		icon := role.Icon
		// 去除可能的引号
		icon = strings.Trim(icon, `"`)

		// 检查是否是 Unicode 转义格式 \U0001F3C6（8位十六进制）或 \uXXXX（4位十六进制）
		if len(icon) >= 3 && icon[0] == '\\' {
			if icon[1] == 'U' && len(icon) >= 10 {
				// \U0001F3C6 格式（8位十六进制）
				if codePoint, err := strconv.ParseInt(icon[2:10], 16, 32); err == nil {
					role.Icon = string(rune(codePoint))
				}
			} else if icon[1] == 'u' && len(icon) >= 6 {
				// \uXXXX 格式（4位十六进制）
				if codePoint, err := strconv.ParseInt(icon[2:6], 16, 32); err == nil {
					role.Icon = string(rune(codePoint))
				}
			}
		}
	}

	// 验证必需字段
	if role.Name == "" {
		// 如果名称为空，尝试从文件名获取
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
			MaxIterations:      30,  // 默认最大迭代次数
			ToolTimeoutMinutes: 10,  // 单次工具执行默认最多 10 分钟，避免异常长时间占用
		},
		Security: SecurityConfig{
			Tools:    []ToolConfig{}, // 工具配置应该从 config.yaml 或 tools/ 目录加载
			ToolsDir: "tools",        // 默认工具目录
		},
		Database: DatabaseConfig{
			Path:            "data/conversations.db",
			KnowledgeDBPath: "data/knowledge.db", // 默认知识库数据库路径
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
				SimilarityThreshold: 0.65, // 降低阈值到 0.65，减少漏检
				HybridWeight:        0.7,
			},
			Indexing: IndexingConfig{
				ChunkSize:        768,   // 增加到 768，更好的上下文保持
				ChunkOverlap:     50,
				MaxChunksPerItem: 20,    // 限制单个知识项最多 20 个块，避免消耗过多配额
				MaxRPM:           100,   // 默认 100 RPM，避免 429 错误
				RateLimitDelayMs: 600,   // 600ms 间隔，对应 100 RPM
				MaxRetries:       3,
				RetryDelayMs:     1000,
			},
		},
	}
}

// KnowledgeConfig 知识库配置
type KnowledgeConfig struct {
	Enabled   bool            `yaml:"enabled" json:"enabled"`     // 是否启用知识检索
	BasePath  string          `yaml:"base_path" json:"base_path"` // 知识库路径
	Embedding EmbeddingConfig `yaml:"embedding" json:"embedding"`
	Retrieval RetrievalConfig `yaml:"retrieval" json:"retrieval"`
	Indexing  IndexingConfig  `yaml:"indexing,omitempty" json:"indexing,omitempty"` // 索引构建配置
}

// IndexingConfig 索引构建配置（用于控制知识库索引构建时的行为）
type IndexingConfig struct {
	// 分块配置
	ChunkSize       int `yaml:"chunk_size,omitempty" json:"chunk_size,omitempty"`           // 每个块的最大 token 数（估算），默认 512
	ChunkOverlap    int `yaml:"chunk_overlap,omitempty" json:"chunk_overlap,omitempty"`     // 块之间的重叠 token 数，默认 50
	MaxChunksPerItem int `yaml:"max_chunks_per_item,omitempty" json:"max_chunks_per_item,omitempty"` // 单个知识项的最大块数量，0 表示不限制

	// 速率限制配置（用于避免 API 速率限制）
	RateLimitDelayMs int `yaml:"rate_limit_delay_ms,omitempty" json:"rate_limit_delay_ms,omitempty"` // 请求间隔时间（毫秒），0 表示不使用固定延迟
	MaxRPM          int `yaml:"max_rpm,omitempty" json:"max_rpm,omitempty"`                         // 每分钟最大请求数，0 表示不限制

	// 重试配置（用于处理临时错误）
	MaxRetries     int `yaml:"max_retries,omitempty" json:"max_retries,omitempty"`       // 最大重试次数，默认 3
	RetryDelayMs   int `yaml:"retry_delay_ms,omitempty" json:"retry_delay_ms,omitempty"` // 重试间隔（毫秒），默认 1000

	// 批处理配置（用于批量嵌入，当前未使用，保留扩展）
	BatchSize      int `yaml:"batch_size,omitempty" json:"batch_size,omitempty"`         // 批量处理大小，0 表示逐个处理
}

// EmbeddingConfig 嵌入配置
type EmbeddingConfig struct {
	Provider string `yaml:"provider" json:"provider"` // 嵌入模型提供商
	Model    string `yaml:"model" json:"model"`       // 模型名称
	BaseURL  string `yaml:"base_url" json:"base_url"` // API Base URL
	APIKey   string `yaml:"api_key" json:"api_key"`   // API Key（从OpenAI配置继承）
}

// RetrievalConfig 检索配置
type RetrievalConfig struct {
	TopK                int     `yaml:"top_k" json:"top_k"`                               // 检索Top-K
	SimilarityThreshold float64 `yaml:"similarity_threshold" json:"similarity_threshold"` // 相似度阈值
	HybridWeight        float64 `yaml:"hybrid_weight" json:"hybrid_weight"`               // 向量检索权重（0-1）
}

// RolesConfig 角色配置（已废弃，使用 map[string]RoleConfig 替代）
// 保留此类型以兼容旧代码，但建议直接使用 map[string]RoleConfig
type RolesConfig struct {
	Roles map[string]RoleConfig `yaml:"roles,omitempty" json:"roles,omitempty"`
}

// RoleConfig 单个角色配置
type RoleConfig struct {
	Name        string   `yaml:"name" json:"name"`                         // 角色名称
	Description string   `yaml:"description" json:"description"`           // 角色描述
	UserPrompt  string   `yaml:"user_prompt" json:"user_prompt"`           // 用户提示词(追加到用户消息前)
	Icon        string   `yaml:"icon,omitempty" json:"icon,omitempty"`     // 角色图标（可选）
	Tools       []string `yaml:"tools,omitempty" json:"tools,omitempty"`   // 关联的工具列表（toolKey格式，如 "toolName" 或 "mcpName::toolName"）
	MCPs        []string `yaml:"mcps,omitempty" json:"mcps,omitempty"`     // 向后兼容：关联的MCP服务器列表（已废弃，使用tools替代）
	Skills      []string `yaml:"skills,omitempty" json:"skills,omitempty"` // 关联的skills列表（skill名称列表，在执行任务前会读取这些skills的内容）
	Enabled     bool     `yaml:"enabled" json:"enabled"`                   // 是否启用
}

// readFromKeychain reads a secret from macOS Keychain using the `security` CLI.
// serviceName is the Keychain service name; account is always "cyberstrike".
func readFromKeychain(serviceName string) (string, error) {
	if runtime.GOOS != "darwin" {
		return "", fmt.Errorf("keychain is only supported on macOS")
	}
	out, err := exec.Command("security", "find-generic-password", "-a", "cyberstrike", "-s", serviceName, "-w").Output()
	if err != nil {
		return "", fmt.Errorf("keychain lookup failed: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}
