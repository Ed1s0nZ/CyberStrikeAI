package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Server MCP服务器
type Server struct {
	tools      map[string]ToolHandler
	toolDefs   map[string]Tool  // 工具定义
	executions map[string]*ToolExecution
	stats      map[string]*ToolStats
	prompts    map[string]*Prompt  // 提示词模板
	resources  map[string]*Resource // 资源
	mu         sync.RWMutex
	logger     *zap.Logger
}

// ToolHandler 工具处理函数
type ToolHandler func(ctx context.Context, args map[string]interface{}) (*ToolResult, error)

// NewServer 创建新的MCP服务器
func NewServer(logger *zap.Logger) *Server {
	s := &Server{
		tools:      make(map[string]ToolHandler),
		toolDefs:   make(map[string]Tool),
		executions: make(map[string]*ToolExecution),
		stats:      make(map[string]*ToolStats),
		prompts:    make(map[string]*Prompt),
		resources:  make(map[string]*Resource),
		logger:     logger,
	}
	
	// 初始化默认提示词和资源
	s.initDefaultPrompts()
	s.initDefaultResources()
	
	return s
}

// RegisterTool 注册工具
func (s *Server) RegisterTool(tool Tool, handler ToolHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tools[tool.Name] = handler
	s.toolDefs[tool.Name] = tool
	
	// 自动为工具创建资源文档
	resourceURI := fmt.Sprintf("tool://%s", tool.Name)
	s.resources[resourceURI] = &Resource{
		URI:         resourceURI,
		Name:        fmt.Sprintf("%s工具文档", tool.Name),
		Description: tool.Description,
		MimeType:    "text/plain",
	}
}

// HandleHTTP 处理HTTP请求
func (s *Server) HandleHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.sendError(w, nil, -32700, "Parse error", err.Error())
		return
	}

	var msg Message
	if err := json.Unmarshal(body, &msg); err != nil {
		s.sendError(w, nil, -32700, "Parse error", err.Error())
		return
	}

	// 处理消息
	response := s.handleMessage(&msg)
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleMessage 处理MCP消息
func (s *Server) handleMessage(msg *Message) *Message {
	if msg.ID == "" {
		msg.ID = uuid.New().String()
	}

	switch msg.Method {
	case "initialize":
		return s.handleInitialize(msg)
	case "tools/list":
		return s.handleListTools(msg)
	case "tools/call":
		return s.handleCallTool(msg)
	case "prompts/list":
		return s.handleListPrompts(msg)
	case "prompts/get":
		return s.handleGetPrompt(msg)
	case "resources/list":
		return s.handleListResources(msg)
	case "resources/read":
		return s.handleReadResource(msg)
	case "sampling/request":
		return s.handleSamplingRequest(msg)
	default:
		return &Message{
			ID:    msg.ID,
			Type:  MessageTypeError,
			Error: &Error{Code: -32601, Message: "Method not found"},
		}
	}
}

// handleInitialize 处理初始化请求
func (s *Server) handleInitialize(msg *Message) *Message {
	var req InitializeRequest
	if err := json.Unmarshal(msg.Params, &req); err != nil {
		return &Message{
			ID:    msg.ID,
			Type:  MessageTypeError,
			Error: &Error{Code: -32602, Message: "Invalid params"},
		}
	}

	response := InitializeResponse{
		ProtocolVersion: ProtocolVersion,
		Capabilities: ServerCapabilities{
			Tools: map[string]interface{}{
				"listChanged": true,
			},
			Prompts: map[string]interface{}{
				"listChanged": true,
			},
			Resources: map[string]interface{}{
				"subscribe":   true,
				"listChanged": true,
			},
			Sampling: map[string]interface{}{},
		},
		ServerInfo: ServerInfo{
			Name:    "CyberStrikeAI",
			Version: "1.0.0",
		},
	}

	result, _ := json.Marshal(response)
	return &Message{
		ID:      msg.ID,
		Type:    MessageTypeResponse,
		Version: "2.0",
		Result:  result,
	}
}

// handleListTools 处理列出工具请求
func (s *Server) handleListTools(msg *Message) *Message {
	s.mu.RLock()
	tools := make([]Tool, 0, len(s.toolDefs))
	for _, tool := range s.toolDefs {
		tools = append(tools, tool)
	}
	s.mu.RUnlock()

	response := ListToolsResponse{Tools: tools}
	result, _ := json.Marshal(response)
	return &Message{
		ID:      msg.ID,
		Type:    MessageTypeResponse,
		Version: "2.0",
		Result:  result,
	}
}

// handleCallTool 处理工具调用请求
func (s *Server) handleCallTool(msg *Message) *Message {
	var req CallToolRequest
	if err := json.Unmarshal(msg.Params, &req); err != nil {
		return &Message{
			ID:    msg.ID,
			Type:  MessageTypeError,
			Error: &Error{Code: -32602, Message: "Invalid params"},
		}
	}

	// 创建执行记录
	executionID := uuid.New().String()
	execution := &ToolExecution{
		ID:        executionID,
		ToolName:  req.Name,
		Arguments: req.Arguments,
		Status:    "running",
		StartTime: time.Now(),
	}

	s.mu.Lock()
	s.executions[executionID] = execution
	s.mu.Unlock()

	// 更新统计
	s.updateStats(req.Name, false)

	// 执行工具
	s.mu.RLock()
	handler, exists := s.tools[req.Name]
	s.mu.RUnlock()

	if !exists {
		execution.Status = "failed"
		execution.Error = "Tool not found"
		now := time.Now()
		execution.EndTime = &now
		return &Message{
			ID:    msg.ID,
			Type:  MessageTypeError,
			Error: &Error{Code: -32601, Message: "Tool not found"},
		}
	}

	// 同步执行所有工具，确保错误能正确返回
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	s.logger.Info("开始执行工具",
		zap.String("toolName", req.Name),
		zap.Any("arguments", req.Arguments),
	)

	result, err := handler(ctx, req.Arguments)
	
	s.mu.Lock()
	now := time.Now()
	execution.EndTime = &now
	execution.Duration = now.Sub(execution.StartTime)
	
	if err != nil {
		execution.Status = "failed"
		execution.Error = err.Error()
		s.updateStats(req.Name, true)
		s.mu.Unlock()
		
		s.logger.Error("工具执行失败",
			zap.String("toolName", req.Name),
			zap.Error(err),
		)
		
		// 返回错误结果
		errorResult, _ := json.Marshal(CallToolResponse{
			Content: []Content{
				{Type: "text", Text: fmt.Sprintf("工具执行失败: %v", err)},
			},
			IsError: true,
		})
		return &Message{
			ID:      msg.ID,
			Type:    MessageTypeResponse,
			Version: "2.0",
			Result:  errorResult,
		}
	}
	
	// 检查result是否为错误
	if result != nil && result.IsError {
		execution.Status = "failed"
		if len(result.Content) > 0 {
			execution.Error = result.Content[0].Text
		}
		s.updateStats(req.Name, true)
	} else {
		execution.Status = "completed"
		execution.Result = result
		s.updateStats(req.Name, false)
	}
	s.mu.Unlock()
	
	// 返回执行结果
	if result == nil {
		result = &ToolResult{
			Content: []Content{
				{Type: "text", Text: "工具执行完成，但未返回结果"},
			},
		}
	}
	
	resultJSON, _ := json.Marshal(CallToolResponse{
		Content: result.Content,
		IsError: result.IsError,
	})
	
	s.logger.Info("工具执行完成",
		zap.String("toolName", req.Name),
		zap.Bool("isError", result.IsError),
	)
	
	return &Message{
		ID:      msg.ID,
		Type:    MessageTypeResponse,
		Version: "2.0",
		Result:  resultJSON,
	}
}

// updateStats 更新统计信息
func (s *Server) updateStats(toolName string, failed bool) {
	if s.stats[toolName] == nil {
		s.stats[toolName] = &ToolStats{
			ToolName: toolName,
		}
	}

	stats := s.stats[toolName]
	stats.TotalCalls++
	now := time.Now()
	stats.LastCallTime = &now

	if failed {
		stats.FailedCalls++
	} else {
		stats.SuccessCalls++
	}
}

// GetExecution 获取执行记录
func (s *Server) GetExecution(id string) (*ToolExecution, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	exec, exists := s.executions[id]
	return exec, exists
}

// GetAllExecutions 获取所有执行记录
func (s *Server) GetAllExecutions() []*ToolExecution {
	s.mu.RLock()
	defer s.mu.RUnlock()
	executions := make([]*ToolExecution, 0, len(s.executions))
	for _, exec := range s.executions {
		executions = append(executions, exec)
	}
	return executions
}

// GetStats 获取统计信息
func (s *Server) GetStats() map[string]*ToolStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	stats := make(map[string]*ToolStats)
	for k, v := range s.stats {
		stats[k] = v
	}
	return stats
}

// CallTool 直接调用工具（用于内部调用）
func (s *Server) CallTool(ctx context.Context, toolName string, args map[string]interface{}) (*ToolResult, string, error) {
	s.mu.RLock()
	handler, exists := s.tools[toolName]
	s.mu.RUnlock()

	if !exists {
		return nil, "", fmt.Errorf("工具 %s 未找到", toolName)
	}

	// 创建执行记录
	executionID := uuid.New().String()
	execution := &ToolExecution{
		ID:        executionID,
		ToolName:  toolName,
		Arguments: args,
		Status:    "running",
		StartTime: time.Now(),
	}

	s.mu.Lock()
	s.executions[executionID] = execution
	s.mu.Unlock()

	// 更新统计
	s.updateStats(toolName, false)

	// 执行工具
	result, err := handler(ctx, args)

	s.mu.Lock()
	now := time.Now()
	execution.EndTime = &now
	execution.Duration = now.Sub(execution.StartTime)

	if err != nil {
		execution.Status = "failed"
		execution.Error = err.Error()
		s.updateStats(toolName, true)
		s.mu.Unlock()
		return nil, executionID, err
	} else {
		execution.Status = "completed"
		execution.Result = result
		s.updateStats(toolName, false)
		s.mu.Unlock()
		return result, executionID, nil
	}
}

// initDefaultPrompts 初始化默认提示词模板
func (s *Server) initDefaultPrompts() {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// 网络安全测试提示词
	s.prompts["security_scan"] = &Prompt{
		Name:        "security_scan",
		Description: "生成网络安全扫描任务的提示词",
		Arguments: []PromptArgument{
			{Name: "target", Description: "扫描目标（IP地址或域名）", Required: true},
			{Name: "scan_type", Description: "扫描类型（port, vuln, web等）", Required: false},
		},
	}
	
	// 渗透测试提示词
	s.prompts["penetration_test"] = &Prompt{
		Name:        "penetration_test",
		Description: "生成渗透测试任务的提示词",
		Arguments: []PromptArgument{
			{Name: "target", Description: "测试目标", Required: true},
			{Name: "scope", Description: "测试范围", Required: false},
		},
	}
}

// initDefaultResources 初始化默认资源
// 注意：工具资源现在在 RegisterTool 时自动创建，此函数保留用于其他非工具资源
func (s *Server) initDefaultResources() {
	// 工具资源已改为在 RegisterTool 时自动创建，无需在此硬编码
}

// handleListPrompts 处理列出提示词请求
func (s *Server) handleListPrompts(msg *Message) *Message {
	s.mu.RLock()
	prompts := make([]Prompt, 0, len(s.prompts))
	for _, prompt := range s.prompts {
		prompts = append(prompts, *prompt)
	}
	s.mu.RUnlock()
	
	response := ListPromptsResponse{
		Prompts: prompts,
	}
	result, _ := json.Marshal(response)
	return &Message{
		ID:      msg.ID,
		Type:    MessageTypeResponse,
		Version: "2.0",
		Result:  result,
	}
}

// handleGetPrompt 处理获取提示词请求
func (s *Server) handleGetPrompt(msg *Message) *Message {
	var req GetPromptRequest
	if err := json.Unmarshal(msg.Params, &req); err != nil {
		return &Message{
			ID:    msg.ID,
			Type:  MessageTypeError,
			Error: &Error{Code: -32602, Message: "Invalid params"},
		}
	}
	
	s.mu.RLock()
	prompt, exists := s.prompts[req.Name]
	s.mu.RUnlock()
	
	if !exists {
		return &Message{
			ID:    msg.ID,
			Type:  MessageTypeError,
			Error: &Error{Code: -32601, Message: "Prompt not found"},
		}
	}
	
	// 根据提示词名称生成消息
	messages := s.generatePromptMessages(prompt, req.Arguments)
	
	response := GetPromptResponse{
		Messages: messages,
	}
	result, _ := json.Marshal(response)
	return &Message{
		ID:      msg.ID,
		Type:    MessageTypeResponse,
		Version: "2.0",
		Result:  result,
	}
}

// generatePromptMessages 生成提示词消息
func (s *Server) generatePromptMessages(prompt *Prompt, args map[string]interface{}) []PromptMessage {
	messages := []PromptMessage{}
	
	switch prompt.Name {
	case "security_scan":
		target, _ := args["target"].(string)
		scanType, _ := args["scan_type"].(string)
		if scanType == "" {
			scanType = "comprehensive"
		}
		
		content := fmt.Sprintf(`请对目标 %s 执行%s安全扫描。包括：
1. 端口扫描和服务识别
2. 漏洞检测
3. Web应用安全测试
4. 生成详细的安全报告`, target, scanType)
		
		messages = append(messages, PromptMessage{
			Role:    "user",
			Content: content,
		})
		
	case "penetration_test":
		target, _ := args["target"].(string)
		scope, _ := args["scope"].(string)
		
		content := fmt.Sprintf(`请对目标 %s 执行渗透测试。`, target)
		if scope != "" {
			content += fmt.Sprintf("测试范围：%s", scope)
		}
		content += "\n请按照OWASP Top 10进行全面的安全测试。"
		
		messages = append(messages, PromptMessage{
			Role:    "user",
			Content: content,
		})
		
	default:
		messages = append(messages, PromptMessage{
			Role:    "user",
			Content: "请执行安全测试任务",
		})
	}
	
	return messages
}

// handleListResources 处理列出资源请求
func (s *Server) handleListResources(msg *Message) *Message {
	s.mu.RLock()
	resources := make([]Resource, 0, len(s.resources))
	for _, resource := range s.resources {
		resources = append(resources, *resource)
	}
	s.mu.RUnlock()
	
	response := ListResourcesResponse{
		Resources: resources,
	}
	result, _ := json.Marshal(response)
	return &Message{
		ID:      msg.ID,
		Type:    MessageTypeResponse,
		Version: "2.0",
		Result:  result,
	}
}

// handleReadResource 处理读取资源请求
func (s *Server) handleReadResource(msg *Message) *Message {
	var req ReadResourceRequest
	if err := json.Unmarshal(msg.Params, &req); err != nil {
		return &Message{
			ID:    msg.ID,
			Type:  MessageTypeError,
			Error: &Error{Code: -32602, Message: "Invalid params"},
		}
	}
	
	s.mu.RLock()
	resource, exists := s.resources[req.URI]
	s.mu.RUnlock()
	
	if !exists {
		return &Message{
			ID:    msg.ID,
			Type:  MessageTypeError,
			Error: &Error{Code: -32601, Message: "Resource not found"},
		}
	}
	
	// 生成资源内容
	content := s.generateResourceContent(resource)
	
	response := ReadResourceResponse{
		Contents: []ResourceContent{content},
	}
	result, _ := json.Marshal(response)
	return &Message{
		ID:      msg.ID,
		Type:    MessageTypeResponse,
		Version: "2.0",
		Result:  result,
	}
}

// generateResourceContent 生成资源内容
func (s *Server) generateResourceContent(resource *Resource) ResourceContent {
	content := ResourceContent{
		URI:      resource.URI,
		MimeType: resource.MimeType,
	}
	
	// 如果是工具资源，生成详细文档
	if strings.HasPrefix(resource.URI, "tool://") {
		toolName := strings.TrimPrefix(resource.URI, "tool://")
		content.Text = s.generateToolDocumentation(toolName, resource)
	} else {
		// 其他资源使用描述或默认内容
		content.Text = resource.Description
	}
	
	return content
}

// generateToolDocumentation 生成工具文档
func (s *Server) generateToolDocumentation(toolName string, resource *Resource) string {
	// 获取工具定义以获取更详细的信息
	s.mu.RLock()
	tool, hasTool := s.toolDefs[toolName]
	s.mu.RUnlock()
	
	// 为常见工具生成详细文档
	switch toolName {
	case "nmap":
		return `Nmap (Network Mapper) 是一个强大的网络扫描工具。

主要功能：
- 端口扫描：发现目标主机开放的端口
- 服务识别：识别运行在端口上的服务
- 版本检测：检测服务版本信息
- 操作系统检测：识别目标操作系统

常用命令：
- nmap -sT target          # TCP连接扫描
- nmap -sV target          # 版本检测
- nmap -sC target          # 默认脚本扫描
- nmap -p 1-1000 target    # 扫描指定端口范围

参数说明：
- target: 目标IP地址或域名（必需）
- ports: 端口范围，例如: 1-1000（可选）`
		
	case "sqlmap":
		return `SQLMap 是一个自动化的SQL注入检测和利用工具。

主要功能：
- 自动检测SQL注入漏洞
- 数据库指纹识别
- 数据提取
- 文件系统访问

常用命令：
- sqlmap -u "http://target.com/page?id=1"  # 检测URL参数
- sqlmap -u "http://target.com" --forms    # 检测表单
- sqlmap -u "http://target.com" --dbs      # 列出数据库

参数说明：
- url: 目标URL（必需）`
		
	case "nikto":
		return `Nikto 是一个Web服务器扫描工具。

主要功能：
- Web服务器漏洞扫描
- 检测过时的服务器软件
- 检测危险文件和程序
- 检测服务器配置问题

常用命令：
- nikto -h target           # 扫描目标主机
- nikto -h target -p 80,443 # 扫描指定端口

参数说明：
- target: 目标URL（必需）`
		
	case "dirb":
		return `Dirb 是一个Web内容扫描器。

主要功能：
- 扫描Web目录和文件
- 发现隐藏的目录和文件
- 支持自定义字典

常用命令：
- dirb url                  # 扫描目标URL
- dirb url -w wordlist.txt  # 使用自定义字典

参数说明：
- target: 目标URL（必需）`
		
	case "exec":
		return `Exec 工具用于执行系统命令。

⚠️ 警告：此工具可以执行任意系统命令，请谨慎使用！

参数说明：
- command: 要执行的系统命令（必需）
- shell: 使用的shell，默认为sh（可选）
- workdir: 工作目录（可选）`
		
	default:
		// 对于其他工具，使用工具定义中的描述信息
		if hasTool {
			doc := fmt.Sprintf("%s\n\n", resource.Description)
			if tool.InputSchema != nil {
				if props, ok := tool.InputSchema["properties"].(map[string]interface{}); ok {
					doc += "参数说明：\n"
					for paramName, paramInfo := range props {
						if paramMap, ok := paramInfo.(map[string]interface{}); ok {
							if desc, ok := paramMap["description"].(string); ok {
								doc += fmt.Sprintf("- %s: %s\n", paramName, desc)
							}
						}
					}
				}
			}
			return doc
		}
		return resource.Description
	}
}

// handleSamplingRequest 处理采样请求
func (s *Server) handleSamplingRequest(msg *Message) *Message {
	var req SamplingRequest
	if err := json.Unmarshal(msg.Params, &req); err != nil {
		return &Message{
			ID:    msg.ID,
			Type:  MessageTypeError,
			Error: &Error{Code: -32602, Message: "Invalid params"},
		}
	}
	
	// 注意：采样功能通常需要连接到实际的LLM服务
	// 这里返回一个占位符响应，实际实现需要集成LLM API
	s.logger.Warn("Sampling request received but not fully implemented",
		zap.Any("request", req),
	)
	
	response := SamplingResponse{
		Content: []SamplingContent{
			{
				Type: "text",
				Text: "采样功能需要配置LLM服务。请使用Agent Loop API进行AI对话。",
			},
		},
		StopReason: "length",
	}
	result, _ := json.Marshal(response)
	return &Message{
		ID:      msg.ID,
		Type:    MessageTypeResponse,
		Version: "2.0",
		Result:  result,
	}
}

// RegisterPrompt 注册提示词模板
func (s *Server) RegisterPrompt(prompt *Prompt) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prompts[prompt.Name] = prompt
}

// RegisterResource 注册资源
func (s *Server) RegisterResource(resource *Resource) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resources[resource.URI] = resource
}

// sendError 发送错误响应
func (s *Server) sendError(w http.ResponseWriter, id interface{}, code int, message, data string) {
	response := Message{
		ID:    fmt.Sprintf("%v", id),
		Type:  MessageTypeError,
		Error: &Error{Code: code, Message: message, Data: data},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

