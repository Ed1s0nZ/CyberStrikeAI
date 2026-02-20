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
	RolesDir    string                `yaml:"roles_dir,omitempty" json:"roles_dir,omitempty"`   // 角色配置文件目录（新方式）
	Roles       map[string]RoleConfig `yaml:"roles,omitempty" json:"roles,omitempty"`           // 向后兼容：支持在主配置文件中定义角色
	SkillsDir   string                `yaml:"skills_dir,omitempty" json:"skills_dir,omitempty"` // Skills配置文件目录
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
	Name        string      `yaml:"name"`               // 参数名称
	Type        string      `yaml:"type"`               // 参数类型: string, int, bool, array
	Description string      `yaml:"description"`        // 参数描述
	Required    bool        `yaml:"required,omitempty"` // 是否必需
	Default     interface{} `yaml:"default,omitempty"`  // 默认值
	Flag        string      `yaml:"flag,omitempty"`     // 命令行标志，如 "-u", "--url", "-p"
	Position    *int        `yaml:"position,omitempty"` // 位置参数的位置（从0开始）
	Format      string      `yaml:"format,omitempty"`   // 参数格式: "flag", "positional", "combined" (flag=value), "template"
	Template    string      `yaml:"template,omitempty"` // 模板字符串，如 "{flag} {value}" 或 "{value}"
	Options     []string    `yaml:"options,omitempty"`  // 可选值列表（用于枚举）
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
			MaxIterations: 30, // 默认最大迭代次数
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
				SimilarityThreshold: 0.7,
				HybridWeight:        0.7,
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
