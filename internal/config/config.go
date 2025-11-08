package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Log      LogConfig      `yaml:"log"`
	MCP      MCPConfig      `yaml:"mcp"`
	OpenAI   OpenAIConfig   `yaml:"openai"`
	Security SecurityConfig `yaml:"security"`
	Database DatabaseConfig `yaml:"database"`
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
	APIKey  string `yaml:"api_key"`
	BaseURL string `yaml:"base_url"`
	Model   string `yaml:"model"`
}

type SecurityConfig struct {
	Tools []ToolConfig `yaml:"tools"`
}

type DatabaseConfig struct {
	Path string `yaml:"path"`
}

type ToolConfig struct {
	Name        string            `yaml:"name"`
	Command     string            `yaml:"command"`
	Args        []string          `yaml:"args,omitempty"`        // 固定参数（可选）
	Description string            `yaml:"description"`
	Enabled     bool              `yaml:"enabled"`
	Parameters  []ParameterConfig `yaml:"parameters,omitempty"` // 参数定义（可选）
	ArgMapping  string            `yaml:"arg_mapping,omitempty"` // 参数映射方式: "auto", "manual", "template"（可选）
}

// ParameterConfig 参数配置
type ParameterConfig struct {
	Name        string      `yaml:"name"`                  // 参数名称
	Type        string      `yaml:"type"`                  // 参数类型: string, int, bool, array
	Description string      `yaml:"description"`           // 参数描述
	Required    bool        `yaml:"required,omitempty"`     // 是否必需
	Default     interface{} `yaml:"default,omitempty"`      // 默认值
	Flag        string      `yaml:"flag,omitempty"`         // 命令行标志，如 "-u", "--url", "-p"
	Position    *int        `yaml:"position,omitempty"`    // 位置参数的位置（从0开始）
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

	return &cfg, nil
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
			BaseURL: "https://api.openai.com/v1",
			Model:   "gpt-4",
		},
		Security: SecurityConfig{
			Tools: []ToolConfig{}, // 工具配置应该从 config.yaml 加载，不在此硬编码
		},
		Database: DatabaseConfig{
			Path: "data/conversations.db",
		},
	}
}

