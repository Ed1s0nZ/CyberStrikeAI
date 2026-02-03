package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// ExternalMCPClient 外部 MCP 客户端接口（由 client_sdk.go 基于官方 SDK 实现）
type ExternalMCPClient interface {
	Initialize(ctx context.Context) error
	ListTools(ctx context.Context) ([]Tool, error)
	CallTool(ctx context.Context, name string, args map[string]interface{}) (*ToolResult, error)
	Close() error
	IsConnected() bool
	GetStatus() string
}

// MCP消息类型
const (
	MessageTypeRequest  = "request"
	MessageTypeResponse = "response"
	MessageTypeError    = "error"
	MessageTypeNotify   = "notify"
)

// MCP协议版本
const ProtocolVersion = "2024-11-05"

// MessageID 表示JSON-RPC 2.0的id字段，可以是字符串、数字或null
type MessageID struct {
	value interface{}
}

// UnmarshalJSON 自定义反序列化，支持字符串、数字和null
func (m *MessageID) UnmarshalJSON(data []byte) error {
	// 尝试解析为null
	if string(data) == "null" {
		m.value = nil
		return nil
	}

	// 尝试解析为字符串
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		m.value = str
		return nil
	}

	// 尝试解析为数字
	var num json.Number
	if err := json.Unmarshal(data, &num); err == nil {
		m.value = num
		return nil
	}

	return fmt.Errorf("invalid id type")
}

// MarshalJSON 自定义序列化
func (m MessageID) MarshalJSON() ([]byte, error) {
	if m.value == nil {
		return []byte("null"), nil
	}
	return json.Marshal(m.value)
}

// String 返回字符串表示
func (m MessageID) String() string {
	if m.value == nil {
		return ""
	}
	return fmt.Sprintf("%v", m.value)
}

// Value 返回原始值
func (m MessageID) Value() interface{} {
	return m.value
}

// Message 表示MCP消息（符合JSON-RPC 2.0规范）
type Message struct {
	ID      MessageID       `json:"id,omitempty"`
	Type    string          `json:"-"` // 内部使用，不序列化到JSON
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
	Version string          `json:"jsonrpc,omitempty"` // JSON-RPC 2.0 版本标识
}

// Error 表示MCP错误
type Error struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Tool 表示MCP工具定义
type Tool struct {
	Name             string                 `json:"name"`
	Description      string                 `json:"description"`                // 详细描述
	ShortDescription string                 `json:"shortDescription,omitempty"` // 简短描述（用于工具列表，减少token消耗）
	InputSchema      map[string]interface{} `json:"inputSchema"`
}

// ToolCall 表示工具调用
type ToolCall struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// ToolResult 表示工具执行结果
type ToolResult struct {
	Content []Content `json:"content"`
	IsError bool      `json:"isError,omitempty"`
}

// Content 表示内容
type Content struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// InitializeRequest 初始化请求
type InitializeRequest struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    map[string]interface{} `json:"capabilities"`
	ClientInfo      ClientInfo             `json:"clientInfo"`
}

// ClientInfo 客户端信息
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// InitializeResponse 初始化响应
type InitializeResponse struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      ServerInfo         `json:"serverInfo"`
}

// ServerCapabilities 服务器能力
type ServerCapabilities struct {
	Tools     map[string]interface{} `json:"tools,omitempty"`
	Prompts   map[string]interface{} `json:"prompts,omitempty"`
	Resources map[string]interface{} `json:"resources,omitempty"`
	Sampling  map[string]interface{} `json:"sampling,omitempty"`
}

// ServerInfo 服务器信息
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ListToolsRequest 列出工具请求
type ListToolsRequest struct{}

// ListToolsResponse 列出工具响应
type ListToolsResponse struct {
	Tools []Tool `json:"tools"`
}

// ListPromptsResponse 列出提示词响应
type ListPromptsResponse struct {
	Prompts []Prompt `json:"prompts"`
}

// ListResourcesResponse 列出资源响应
type ListResourcesResponse struct {
	Resources []Resource `json:"resources"`
}

// CallToolRequest 调用工具请求
type CallToolRequest struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// CallToolResponse 调用工具响应
type CallToolResponse struct {
	Content []Content `json:"content"`
	IsError bool      `json:"isError,omitempty"`
}

// ToolExecution 工具执行记录
type ToolExecution struct {
	ID        string                 `json:"id"`
	ToolName  string                 `json:"toolName"`
	Arguments map[string]interface{} `json:"arguments"`
	Status    string                 `json:"status"` // pending, running, completed, failed
	Result    *ToolResult            `json:"result,omitempty"`
	Error     string                 `json:"error,omitempty"`
	StartTime time.Time              `json:"startTime"`
	EndTime   *time.Time             `json:"endTime,omitempty"`
	Duration  time.Duration          `json:"duration,omitempty"`
}

// ToolStats 工具统计信息
type ToolStats struct {
	ToolName     string     `json:"toolName"`
	TotalCalls   int        `json:"totalCalls"`
	SuccessCalls int        `json:"successCalls"`
	FailedCalls  int        `json:"failedCalls"`
	LastCallTime *time.Time `json:"lastCallTime,omitempty"`
}

// Prompt 提示词模板
type Prompt struct {
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Arguments   []PromptArgument `json:"arguments,omitempty"`
}

// PromptArgument 提示词参数
type PromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// GetPromptRequest 获取提示词请求
type GetPromptRequest struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// GetPromptResponse 获取提示词响应
type GetPromptResponse struct {
	Messages []PromptMessage `json:"messages"`
}

// PromptMessage 提示词消息
type PromptMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Resource 资源
type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// ReadResourceRequest 读取资源请求
type ReadResourceRequest struct {
	URI string `json:"uri"`
}

// ReadResourceResponse 读取资源响应
type ReadResourceResponse struct {
	Contents []ResourceContent `json:"contents"`
}

// ResourceContent 资源内容
type ResourceContent struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
	Blob     string `json:"blob,omitempty"`
}

// SamplingRequest 采样请求
type SamplingRequest struct {
	Messages    []SamplingMessage `json:"messages"`
	Model       string            `json:"model,omitempty"`
	MaxTokens   int               `json:"maxTokens,omitempty"`
	Temperature float64           `json:"temperature,omitempty"`
	TopP        float64           `json:"topP,omitempty"`
}

// SamplingMessage 采样消息
type SamplingMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// SamplingResponse 采样响应
type SamplingResponse struct {
	Content    []SamplingContent `json:"content"`
	Model      string            `json:"model,omitempty"`
	StopReason string            `json:"stopReason,omitempty"`
}

// SamplingContent 采样内容
type SamplingContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}
