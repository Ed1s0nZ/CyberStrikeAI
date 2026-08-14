package config

import "strings"

// VisionConfig 独立视觉模型与 analyze_image 工具参数；enabled 时注册 MCP 工具 analyze_image。
type VisionConfig struct {
	Enabled         bool     `yaml:"enabled" json:"enabled"`
	APIKey          string   `yaml:"api_key,omitempty" json:"api_key,omitempty"`
	BaseURL         string   `yaml:"base_url,omitempty" json:"base_url,omitempty"`
	Model           string   `yaml:"model,omitempty" json:"model,omitempty"`
	Provider        string   `yaml:"provider,omitempty" json:"provider,omitempty"`
	TimeoutSeconds  int      `yaml:"timeout_seconds,omitempty" json:"timeout_seconds,omitempty"`
	MaxImageBytes   int64    `yaml:"max_image_bytes,omitempty" json:"max_image_bytes,omitempty"`
	MaxDimension    int      `yaml:"max_dimension,omitempty" json:"max_dimension,omitempty"`
	JPEGQuality     int      `yaml:"jpeg_quality,omitempty" json:"jpeg_quality,omitempty"`
	MaxPayloadBytes          int64 `yaml:"max_payload_bytes,omitempty" json:"max_payload_bytes,omitempty"`
	SkipPreprocessBelowBytes int64 `yaml:"skip_preprocess_below_bytes,omitempty" json:"skip_preprocess_below_bytes,omitempty"` // 0=始终压缩；默认 2MB 且长边已<=max_dimension 时原图直传
	Detail string `yaml:"detail,omitempty" json:"detail,omitempty"` // low | high | auto
}

func (v VisionConfig) TimeoutSecondsEffective() int {
	if v.TimeoutSeconds <= 0 {
		return 60
	}
	return v.TimeoutSeconds
}

func (v VisionConfig) MaxImageBytesEffective() int64 {
	if v.MaxImageBytes <= 0 {
		return 5 * 1024 * 1024
	}
	return v.MaxImageBytes
}

func (v VisionConfig) MaxDimensionEffective() int {
	if v.MaxDimension <= 0 {
		return 2048
	}
	return v.MaxDimension
}

func (v VisionConfig) JPEGQualityEffective() int {
	if v.JPEGQuality <= 0 || v.JPEGQuality > 100 {
		return 82
	}
	return v.JPEGQuality
}

func (v VisionConfig) MaxPayloadBytesEffective() int64 {
	if v.MaxPayloadBytes <= 0 {
		return 512 * 1024
	}
	return v.MaxPayloadBytes
}

// SkipPreprocessBelowBytesEffective 低于该字节数且长边<=max_dimension、且<=max_payload 时可原图直传；0 表示始终压缩。
func (v VisionConfig) SkipPreprocessBelowBytesEffective() int64 {
	if v.SkipPreprocessBelowBytes < 0 {
		return 0
	}
	return v.SkipPreprocessBelowBytes
}

func (v VisionConfig) DetailEffective() string {
	d := strings.ToLower(strings.TrimSpace(v.Detail))
	switch d {
	case "high", "low", "auto":
		return d
	default:
		return "low"
	}
}

// OpenAICfgEffective 合并主 openai 配置与 vision 覆盖项，供 VL ChatModel 使用。
// vision.api_key / base_url / provider 留空或省略时，沿用 main（openai）对应字段；vision.model 必填（由 Ready 校验）。
func (v VisionConfig) OpenAICfgEffective(main OpenAIConfig) OpenAIConfig {
	out := main
	if k := strings.TrimSpace(v.APIKey); k != "" {
		out.APIKey = k
	}
	if u := strings.TrimSpace(v.BaseURL); u != "" {
		out.BaseURL = u
	}
	if m := strings.TrimSpace(v.Model); m != "" {
		out.Model = m
	}
	if p := strings.TrimSpace(v.Provider); p != "" {
		out.Provider = p
	}
	out.Reasoning.Mode = "off"
	return out
}

// Ready 表示已启用且模型名非空。
func (v VisionConfig) Ready() bool {
	return v.Enabled && strings.TrimSpace(v.Model) != ""
}
