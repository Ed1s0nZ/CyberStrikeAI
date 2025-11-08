package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/mcp"
	"go.uber.org/zap"
)

// Agent AI代理
type Agent struct {
	openAIClient *http.Client
	config       *config.OpenAIConfig
	mcpServer    *mcp.Server
	logger       *zap.Logger
}

// NewAgent 创建新的Agent
func NewAgent(cfg *config.OpenAIConfig, mcpServer *mcp.Server, logger *zap.Logger) *Agent {
	return &Agent{
		openAIClient: &http.Client{Timeout: 5 * time.Minute},
		config:       cfg,
		mcpServer:    mcpServer,
		logger:       logger,
	}
}

// ChatMessage 聊天消息
type ChatMessage struct {
	Role      string     `json:"role"`
	Content   string     `json:"content,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string    `json:"tool_call_id,omitempty"`
}

// MarshalJSON 自定义JSON序列化，将tool_calls中的arguments转换为JSON字符串
func (cm ChatMessage) MarshalJSON() ([]byte, error) {
	// 构建序列化结构
	aux := map[string]interface{}{
		"role": cm.Role,
	}

	// 添加content（如果存在）
	if cm.Content != "" {
		aux["content"] = cm.Content
	}

	// 添加tool_call_id（如果存在）
	if cm.ToolCallID != "" {
		aux["tool_call_id"] = cm.ToolCallID
	}

	// 转换tool_calls，将arguments转换为JSON字符串
	if len(cm.ToolCalls) > 0 {
		toolCallsJSON := make([]map[string]interface{}, len(cm.ToolCalls))
		for i, tc := range cm.ToolCalls {
			// 将arguments转换为JSON字符串
			argsJSON := ""
			if tc.Function.Arguments != nil {
				argsBytes, err := json.Marshal(tc.Function.Arguments)
				if err != nil {
					return nil, err
				}
				argsJSON = string(argsBytes)
			}
			
			toolCallsJSON[i] = map[string]interface{}{
				"id":   tc.ID,
				"type": tc.Type,
				"function": map[string]interface{}{
					"name":      tc.Function.Name,
					"arguments": argsJSON,
				},
			}
		}
		aux["tool_calls"] = toolCallsJSON
	}

	return json.Marshal(aux)
}

// OpenAIRequest OpenAI API请求
type OpenAIRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Tools    []Tool        `json:"tools,omitempty"`
}

// OpenAIResponse OpenAI API响应
type OpenAIResponse struct {
	ID      string   `json:"id"`
	Choices []Choice `json:"choices"`
	Error   *Error   `json:"error,omitempty"`
}

// Choice 选择
type Choice struct {
	Message      MessageWithTools `json:"message"`
	FinishReason string           `json:"finish_reason"`
}

// MessageWithTools 带工具调用的消息
type MessageWithTools struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
}

// Tool OpenAI工具定义
type Tool struct {
	Type     string                 `json:"type"`
	Function FunctionDefinition     `json:"function"`
}

// FunctionDefinition 函数定义
type FunctionDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// Error OpenAI错误
type Error struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

// ToolCall 工具调用
type ToolCall struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Function FunctionCall           `json:"function"`
}

// FunctionCall 函数调用
type FunctionCall struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// UnmarshalJSON 自定义JSON解析，处理arguments可能是字符串或对象的情况
func (fc *FunctionCall) UnmarshalJSON(data []byte) error {
	type Alias FunctionCall
	aux := &struct {
		Name      string      `json:"name"`
		Arguments interface{} `json:"arguments"`
		*Alias
	}{
		Alias: (*Alias)(fc),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	fc.Name = aux.Name

	// 处理arguments可能是字符串或对象的情况
	switch v := aux.Arguments.(type) {
	case map[string]interface{}:
		fc.Arguments = v
	case string:
		// 如果是字符串，尝试解析为JSON
		if err := json.Unmarshal([]byte(v), &fc.Arguments); err != nil {
			// 如果解析失败，创建一个包含原始字符串的map
			fc.Arguments = map[string]interface{}{
				"raw": v,
			}
		}
	case nil:
		fc.Arguments = make(map[string]interface{})
	default:
		// 其他类型，尝试转换为map
		fc.Arguments = map[string]interface{}{
			"value": v,
		}
	}

	return nil
}

// AgentLoopResult Agent Loop执行结果
type AgentLoopResult struct {
	Response      string
	MCPExecutionIDs []string
}

// AgentLoop 执行Agent循环
func (a *Agent) AgentLoop(ctx context.Context, userInput string, historyMessages []ChatMessage) (*AgentLoopResult, error) {
	messages := []ChatMessage{
		{
			Role:    "system",
			Content: "你是一个专业的网络安全渗透测试专家。你可以使用各种安全工具进行自主渗透测试。分析目标并选择最佳测试策略。当需要执行工具时，使用提供的工具函数。",
		},
	}
	
	// 添加历史消息（数据库只保存user和assistant消息）
	a.logger.Info("处理历史消息",
		zap.Int("count", len(historyMessages)),
	)
	addedCount := 0
	for i, msg := range historyMessages {
		// 只添加有内容的消息
		if msg.Content != "" {
			messages = append(messages, ChatMessage{
				Role:    msg.Role,
				Content: msg.Content,
			})
			addedCount++
			contentPreview := msg.Content
			if len(contentPreview) > 50 {
				contentPreview = contentPreview[:50] + "..."
			}
			a.logger.Info("添加历史消息到上下文",
				zap.Int("index", i),
				zap.String("role", msg.Role),
				zap.String("content", contentPreview),
			)
		}
	}
	
	a.logger.Info("构建消息数组",
		zap.Int("historyMessages", len(historyMessages)),
		zap.Int("addedMessages", addedCount),
		zap.Int("totalMessages", len(messages)),
	)
	
	// 添加当前用户消息
	messages = append(messages, ChatMessage{
		Role:    "user",
		Content: userInput,
	})

	result := &AgentLoopResult{
		MCPExecutionIDs: make([]string, 0),
	}

	maxIterations := 10
	for i := 0; i < maxIterations; i++ {
		// 获取可用工具
		tools := a.getAvailableTools()

		// 记录每次调用OpenAI
		if i == 0 {
			a.logger.Info("调用OpenAI",
				zap.Int("iteration", i+1),
				zap.Int("messagesCount", len(messages)),
			)
			// 记录前几条消息的内容（用于调试）
			for j, msg := range messages {
				if j >= 5 { // 只记录前5条
					break
				}
				contentPreview := msg.Content
				if len(contentPreview) > 100 {
					contentPreview = contentPreview[:100] + "..."
				}
				a.logger.Debug("消息内容",
					zap.Int("index", j),
					zap.String("role", msg.Role),
					zap.String("content", contentPreview),
				)
			}
		} else {
			a.logger.Info("调用OpenAI",
				zap.Int("iteration", i+1),
				zap.Int("messagesCount", len(messages)),
			)
		}

		// 调用OpenAI
		response, err := a.callOpenAI(ctx, messages, tools)
		if err != nil {
			result.Response = ""
			return result, fmt.Errorf("调用OpenAI失败: %w", err)
		}

		if response.Error != nil {
			result.Response = ""
			return result, fmt.Errorf("OpenAI错误: %s", response.Error.Message)
		}

		if len(response.Choices) == 0 {
			result.Response = ""
			return result, fmt.Errorf("没有收到响应")
		}

		choice := response.Choices[0]

		// 检查是否有工具调用
		if len(choice.Message.ToolCalls) > 0 {
			// 添加assistant消息（包含工具调用）
			messages = append(messages, ChatMessage{
				Role:      "assistant",
				Content:   choice.Message.Content,
				ToolCalls: choice.Message.ToolCalls,
			})

			// 执行所有工具调用
			for _, toolCall := range choice.Message.ToolCalls {
				// 执行工具
				execResult, err := a.executeToolViaMCP(ctx, toolCall.Function.Name, toolCall.Function.Arguments)
				if err != nil {
					messages = append(messages, ChatMessage{
						Role:      "tool",
						ToolCallID: toolCall.ID,
						Content:   fmt.Sprintf("工具执行失败: %v", err),
					})
				} else {
					messages = append(messages, ChatMessage{
						Role:      "tool",
						ToolCallID: toolCall.ID,
						Content:   execResult.Result,
					})
					// 收集执行ID
					if execResult.ExecutionID != "" {
						result.MCPExecutionIDs = append(result.MCPExecutionIDs, execResult.ExecutionID)
					}
				}
			}
			continue
		}

		// 添加assistant响应
		messages = append(messages, ChatMessage{
			Role:    "assistant",
			Content: choice.Message.Content,
		})

		// 如果完成，返回结果
		if choice.FinishReason == "stop" {
			result.Response = choice.Message.Content
			return result, nil
		}
	}

	result.Response = "达到最大迭代次数"
	return result, nil
}

// getAvailableTools 获取可用工具
func (a *Agent) getAvailableTools() []Tool {
	// 从MCP服务器获取工具列表
	executions := a.mcpServer.GetAllExecutions()
	toolNames := make(map[string]bool)
	for _, exec := range executions {
		toolNames[exec.ToolName] = true
	}

	tools := []Tool{
		{
			Type: "function",
			Function: FunctionDefinition{
				Name:        "nmap",
				Description: "使用nmap进行网络扫描，发现开放端口和服务。支持IP地址、域名或URL（会自动提取域名）。使用TCP连接扫描，不需要root权限。",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"target": map[string]interface{}{
							"type":        "string",
							"description": "目标IP地址、域名或URL（如 https://example.com）。如果是URL，会自动提取域名部分。",
						},
						"ports": map[string]interface{}{
							"type":        "string",
							"description": "要扫描的端口范围，例如: 1-1000 或 80,443,8080。如果不指定，将扫描常用端口。",
						},
					},
					"required": []string{"target"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDefinition{
				Name:        "sqlmap",
				Description: "使用sqlmap检测SQL注入漏洞",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"url": map[string]interface{}{
							"type":        "string",
							"description": "目标URL",
						},
					},
					"required": []string{"url"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDefinition{
				Name:        "nikto",
				Description: "使用nikto扫描Web服务器漏洞",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"target": map[string]interface{}{
							"type":        "string",
							"description": "目标URL",
						},
					},
					"required": []string{"target"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDefinition{
				Name:        "dirb",
				Description: "使用dirb进行目录扫描",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"url": map[string]interface{}{
							"type":        "string",
							"description": "目标URL",
						},
					},
					"required": []string{"url"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDefinition{
				Name:        "exec",
				Description: "执行系统命令（谨慎使用，仅用于必要的系统操作）",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"command": map[string]interface{}{
							"type":        "string",
							"description": "要执行的系统命令",
						},
						"shell": map[string]interface{}{
							"type":        "string",
							"description": "使用的shell（可选，默认为sh）",
						},
						"workdir": map[string]interface{}{
							"type":        "string",
							"description": "工作目录（可选）",
						},
					},
					"required": []string{"command"},
				},
			},
		},
	}

	return tools
}

// callOpenAI 调用OpenAI API
func (a *Agent) callOpenAI(ctx context.Context, messages []ChatMessage, tools []Tool) (*OpenAIResponse, error) {
	reqBody := OpenAIRequest{
		Model:    a.config.Model,
		Messages: messages,
	}

	if len(tools) > 0 {
		reqBody.Tools = tools
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", a.config.BaseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.config.APIKey)

	resp, err := a.openAIClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// 记录响应内容（用于调试）
	if resp.StatusCode != http.StatusOK {
		a.logger.Warn("OpenAI API返回非200状态码",
			zap.Int("status", resp.StatusCode),
			zap.String("body", string(body)),
		)
	}

	var response OpenAIResponse
	if err := json.Unmarshal(body, &response); err != nil {
		a.logger.Error("解析OpenAI响应失败",
			zap.Error(err),
			zap.String("body", string(body)),
		)
		return nil, fmt.Errorf("解析响应失败: %w, 响应内容: %s", err, string(body))
	}

	return &response, nil
}

// parseToolCall 解析工具调用
func (a *Agent) parseToolCall(content string) (map[string]interface{}, error) {
	// 简单解析，实际应该更复杂
	// 格式: [TOOL_CALL]tool_name:arg1=value1,arg2=value2
	if !strings.HasPrefix(content, "[TOOL_CALL]") {
		return nil, fmt.Errorf("不是有效的工具调用格式")
	}

	parts := strings.Split(content[len("[TOOL_CALL]"):], ":")
	if len(parts) < 2 {
		return nil, fmt.Errorf("工具调用格式错误")
	}

	toolName := strings.TrimSpace(parts[0])
	argsStr := strings.TrimSpace(parts[1])

	args := make(map[string]interface{})
	argPairs := strings.Split(argsStr, ",")
	for _, pair := range argPairs {
		kv := strings.Split(pair, "=")
		if len(kv) == 2 {
			args[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}

	args["_tool_name"] = toolName
	return args, nil
}

// ToolExecutionResult 工具执行结果
type ToolExecutionResult struct {
	Result      string
	ExecutionID string
}

// executeToolViaMCP 通过MCP执行工具
func (a *Agent) executeToolViaMCP(ctx context.Context, toolName string, args map[string]interface{}) (*ToolExecutionResult, error) {
	a.logger.Info("通过MCP执行工具",
		zap.String("tool", toolName),
		zap.Any("args", args),
	)

	// 通过MCP服务器调用工具
	result, executionID, err := a.mcpServer.CallTool(ctx, toolName, args)
	if err != nil {
		return nil, fmt.Errorf("工具执行失败: %w", err)
	}

	// 格式化结果
	var resultText strings.Builder
	for _, content := range result.Content {
		resultText.WriteString(content.Text)
		resultText.WriteString("\n")
	}

	return &ToolExecutionResult{
		Result:      resultText.String(),
		ExecutionID: executionID,
	}, nil
}

