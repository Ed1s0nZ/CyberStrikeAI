package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// MonitorStorage 监控数据存储接口
type MonitorStorage interface {
	SaveToolExecution(exec *ToolExecution) error
	LoadToolExecutions() ([]*ToolExecution, error)
	GetToolExecution(id string) (*ToolExecution, error)
	SaveToolStats(toolName string, stats *ToolStats) error
	LoadToolStats() (map[string]*ToolStats, error)
	UpdateToolStats(toolName string, totalCalls, successCalls, failedCalls int, lastCallTime *time.Time) error
}

// Server MCP服务器
type Server struct {
	tools                 map[string]ToolHandler
	toolDefs              map[string]Tool // 工具定义
	executions            map[string]*ToolExecution
	stats                 map[string]*ToolStats
	prompts               map[string]*Prompt   // 提示词模板
	resources             map[string]*Resource // 资源
	storage               MonitorStorage       // 可选的持久化存储
	mu                    sync.RWMutex
	logger                *zap.Logger
	maxExecutionsInMemory int // 内存中最大执行记录数
	sseClients            map[string]*sseClient
}

type sseClient struct {
	id   string
	send chan []byte
}

// ToolHandler 工具处理函数
type ToolHandler func(ctx context.Context, args map[string]interface{}) (*ToolResult, error)

// NewServer 创建新的MCP服务器
func NewServer(logger *zap.Logger) *Server {
	return NewServerWithStorage(logger, nil)
}

// NewServerWithStorage 创建新的MCP服务器（带持久化存储）
func NewServerWithStorage(logger *zap.Logger, storage MonitorStorage) *Server {
	s := &Server{
		tools:                 make(map[string]ToolHandler),
		toolDefs:              make(map[string]Tool),
		executions:            make(map[string]*ToolExecution),
		stats:                 make(map[string]*ToolStats),
		prompts:               make(map[string]*Prompt),
		resources:             make(map[string]*Resource),
		storage:               storage,
		logger:                logger,
		maxExecutionsInMemory: 1000, // 默认最多在内存中保留1000条执行记录
		sseClients:            make(map[string]*sseClient),
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

// ClearTools 清空所有工具（用于重新加载配置）
func (s *Server) ClearTools() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 清空工具和工具定义
	s.tools = make(map[string]ToolHandler)
	s.toolDefs = make(map[string]Tool)

	// 清空工具相关的资源（保留其他资源）
	newResources := make(map[string]*Resource)
	for uri, resource := range s.resources {
		// 保留非工具资源
		if !strings.HasPrefix(uri, "tool://") {
			newResources[uri] = resource
		}
	}
	s.resources = newResources
}

// HandleHTTP 处理HTTP请求
func (s *Server) HandleHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet && strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
		s.handleSSE(w, r)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 官方 MCP SSE 规范：带 sessionid 的 POST 表示消息发往该 SSE 会话，响应通过 SSE 流返回
	if sessionID := r.URL.Query().Get("sessionid"); sessionID != "" {
		s.serveSSESessionMessage(w, r, sessionID)
		return
	}

	// 简单 POST：请求体为 JSON-RPC，响应在 body 中返回
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

	response := s.handleMessage(&msg)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// serveSSESessionMessage 处理发往 SSE 会话的 POST：读取 JSON-RPC 请求，处理后将响应通过该会话的 SSE 流推送
func (s *Server) serveSSESessionMessage(w http.ResponseWriter, r *http.Request, sessionID string) {
	s.mu.RLock()
	client, exists := s.sseClients[sessionID]
	s.mu.RUnlock()
	if !exists || client == nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	var msg Message
	if err := json.Unmarshal(body, &msg); err != nil {
		http.Error(w, "failed to parse body", http.StatusBadRequest)
		return
	}

	response := s.handleMessage(&msg)
	if response == nil {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	respBytes, err := json.Marshal(response)
	if err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}

	select {
	case client.send <- respBytes:
		w.WriteHeader(http.StatusAccepted)
	default:
		http.Error(w, "session send buffer full", http.StatusServiceUnavailable)
	}
}

// handleSSE 处理 SSE 连接，兼容官方 MCP 2024-11-05 SSE 规范：
// 1. 首个事件必须为 event: endpoint，data 为客户端 POST 消息的 URL（含 sessionid）
// 2. 后续事件为 event: message，data 为 JSON-RPC 响应
func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	sessionID := uuid.New().String()
	client := &sseClient{
		id:   sessionID,
		send: make(chan []byte, 32),
	}

	s.addSSEClient(client)
	defer s.removeSSEClient(client.id)

	// 官方规范：首个事件为 endpoint，data 为消息端点 URL（客户端将向该 URL POST 请求）
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if r.URL.Scheme != "" {
		scheme = r.URL.Scheme
	}
	endpointURL := fmt.Sprintf("%s://%s%s?sessionid=%s", scheme, r.Host, r.URL.Path, sessionID)
	fmt.Fprintf(w, "event: endpoint\ndata: %s\n\n", endpointURL)
	flusher.Flush()

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case msg, ok := <-client.send:
			if !ok {
				return
			}
			fmt.Fprintf(w, "event: message\ndata: %s\n\n", msg)
			flusher.Flush()
		case <-ticker.C:
			fmt.Fprintf(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}

// addSSEClient 注册SSE客户端
func (s *Server) addSSEClient(client *sseClient) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sseClients[client.id] = client
}

// removeSSEClient 移除SSE客户端
func (s *Server) removeSSEClient(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if client, exists := s.sseClients[id]; exists {
		close(client.send)
		delete(s.sseClients, id)
	}
}

// handleMessage 处理MCP消息
func (s *Server) handleMessage(msg *Message) *Message {
	// 检查是否是通知（notification）- 通知没有id字段，不需要响应
	isNotification := msg.ID.Value() == nil || msg.ID.String() == ""

	// 如果不是通知且ID为空，生成新的UUID
	if !isNotification && msg.ID.String() == "" {
		msg.ID = MessageID{value: uuid.New().String()}
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
	case "notifications/initialized":
		// 通知类型，不需要响应
		s.logger.Debug("收到 initialized 通知")
		return nil
	case "":
		// 空方法名，可能是通知，不返回错误
		if isNotification {
			s.logger.Debug("收到无方法名的通知消息")
			return nil
		}
		fallthrough
	default:
		// 如果是通知，不返回错误响应
		if isNotification {
			s.logger.Debug("收到未知通知", zap.String("method", msg.Method))
			return nil
		}
		// 对于请求，返回方法未找到错误
		return &Message{
			ID:      msg.ID,
			Type:    MessageTypeError,
			Version: "2.0",
			Error:   &Error{Code: -32601, Message: "Method not found"},
		}
	}
}

// handleInitialize 处理初始化请求
func (s *Server) handleInitialize(msg *Message) *Message {
	var req InitializeRequest
	if err := json.Unmarshal(msg.Params, &req); err != nil {
		return &Message{
			ID:      msg.ID,
			Type:    MessageTypeError,
			Version: "2.0",
			Error:   &Error{Code: -32602, Message: "Invalid params"},
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
			ID:      msg.ID,
			Type:    MessageTypeError,
			Version: "2.0",
			Error:   &Error{Code: -32602, Message: "Invalid params"},
		}
	}

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
	// 如果内存中的执行记录超过限制，清理最旧的记录
	s.cleanupOldExecutions()
	s.mu.Unlock()

	if s.storage != nil {
		if err := s.storage.SaveToolExecution(execution); err != nil {
			s.logger.Warn("保存执行记录到数据库失败", zap.Error(err))
		}
	}

	s.mu.RLock()
	handler, exists := s.tools[req.Name]
	s.mu.RUnlock()

	if !exists {
		execution.Status = "failed"
		execution.Error = "Tool not found"
		now := time.Now()
		execution.EndTime = &now
		execution.Duration = now.Sub(execution.StartTime)

		if s.storage != nil {
			if err := s.storage.SaveToolExecution(execution); err != nil {
				s.logger.Warn("保存执行记录到数据库失败", zap.Error(err))
			}
			s.mu.Lock()
			delete(s.executions, executionID)
			s.mu.Unlock()
		}

		s.updateStats(req.Name, true)

		return &Message{
			ID:      msg.ID,
			Type:    MessageTypeError,
			Version: "2.0",
			Error:   &Error{Code: -32601, Message: "Tool not found"},
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	s.logger.Info("开始执行工具",
		zap.String("toolName", req.Name),
		zap.Any("arguments", req.Arguments),
	)

	result, err := handler(ctx, req.Arguments)
	now := time.Now()
	var failed bool
	var finalResult *ToolResult

	s.mu.Lock()
	execution.EndTime = &now
	execution.Duration = now.Sub(execution.StartTime)

	if err != nil {
		execution.Status = "failed"
		execution.Error = err.Error()
		failed = true
	} else if result != nil && result.IsError {
		execution.Status = "failed"
		if len(result.Content) > 0 {
			execution.Error = result.Content[0].Text
		} else {
			execution.Error = "工具执行返回错误结果"
		}
		execution.Result = result
		failed = true
	} else {
		execution.Status = "completed"
		if result == nil {
			result = &ToolResult{
				Content: []Content{
					{Type: "text", Text: "工具执行完成，但未返回结果"},
				},
			}
		}
		execution.Result = result
		failed = false
	}

	finalResult = execution.Result
	s.mu.Unlock()

	if s.storage != nil {
		if err := s.storage.SaveToolExecution(execution); err != nil {
			s.logger.Warn("保存执行记录到数据库失败", zap.Error(err))
		}
	}

	s.updateStats(req.Name, failed)

	if s.storage != nil {
		s.mu.Lock()
		delete(s.executions, executionID)
		s.mu.Unlock()
	}

	if err != nil {
		s.logger.Error("工具执行失败",
			zap.String("toolName", req.Name),
			zap.Error(err),
		)

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

	if finalResult != nil && finalResult.IsError {
		s.logger.Warn("工具执行返回错误结果",
			zap.String("toolName", req.Name),
		)

		errorResult, _ := json.Marshal(CallToolResponse{
			Content: finalResult.Content,
			IsError: true,
		})
		return &Message{
			ID:      msg.ID,
			Type:    MessageTypeResponse,
			Version: "2.0",
			Result:  errorResult,
		}
	}

	if finalResult == nil {
		finalResult = &ToolResult{
			Content: []Content{
				{Type: "text", Text: "工具执行完成，但未返回结果"},
			},
		}
	}

	resultJSON, _ := json.Marshal(CallToolResponse{
		Content: finalResult.Content,
		IsError: false,
	})

	s.logger.Info("工具执行完成",
		zap.String("toolName", req.Name),
		zap.Bool("isError", finalResult.IsError),
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
	now := time.Now()
	if s.storage != nil {
		totalCalls := 1
		successCalls := 0
		failedCalls := 0
		if failed {
			failedCalls = 1
		} else {
			successCalls = 1
		}
		if err := s.storage.UpdateToolStats(toolName, totalCalls, successCalls, failedCalls, &now); err != nil {
			s.logger.Warn("保存统计信息到数据库失败", zap.Error(err))
		}
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stats[toolName] == nil {
		s.stats[toolName] = &ToolStats{
			ToolName: toolName,
		}
	}

	stats := s.stats[toolName]
	stats.TotalCalls++
	stats.LastCallTime = &now

	if failed {
		stats.FailedCalls++
	} else {
		stats.SuccessCalls++
	}
}

// GetExecution 获取执行记录（先从内存查找，再从数据库查找）
func (s *Server) GetExecution(id string) (*ToolExecution, bool) {
	s.mu.RLock()
	exec, exists := s.executions[id]
	s.mu.RUnlock()

	if exists {
		return exec, true
	}

	if s.storage != nil {
		exec, err := s.storage.GetToolExecution(id)
		if err == nil {
			return exec, true
		}
	}

	return nil, false
}

// loadHistoricalData 从数据库加载历史数据
func (s *Server) loadHistoricalData() {
	if s.storage == nil {
		return
	}

	// 加载历史执行记录（最近1000条）
	executions, err := s.storage.LoadToolExecutions()
	if err != nil {
		s.logger.Warn("加载历史执行记录失败", zap.Error(err))
	} else {
		s.mu.Lock()
		for _, exec := range executions {
			// 只加载最近 maxExecutionsInMemory 条，避免内存占用过大
			if len(s.executions) < s.maxExecutionsInMemory {
				s.executions[exec.ID] = exec
			} else {
				break
			}
		}
		s.mu.Unlock()
		s.logger.Info("加载历史执行记录", zap.Int("count", len(executions)))
	}

	// 加载历史统计信息
	stats, err := s.storage.LoadToolStats()
	if err != nil {
		s.logger.Warn("加载历史统计信息失败", zap.Error(err))
	} else {
		s.mu.Lock()
		for k, v := range stats {
			s.stats[k] = v
		}
		s.mu.Unlock()
		s.logger.Info("加载历史统计信息", zap.Int("count", len(stats)))
	}
}

// GetAllExecutions 获取所有执行记录（合并内存和数据库）
func (s *Server) GetAllExecutions() []*ToolExecution {
	if s.storage != nil {
		dbExecutions, err := s.storage.LoadToolExecutions()
		if err == nil {
			execMap := make(map[string]*ToolExecution)
			for _, exec := range dbExecutions {
				if _, exists := execMap[exec.ID]; !exists {
					execMap[exec.ID] = exec
				}
			}

			s.mu.RLock()
			for id, exec := range s.executions {
				if _, exists := execMap[id]; !exists {
					execMap[id] = exec
				}
			}
			s.mu.RUnlock()

			result := make([]*ToolExecution, 0, len(execMap))
			for _, exec := range execMap {
				result = append(result, exec)
			}
			return result
		} else {
			s.logger.Warn("从数据库加载执行记录失败", zap.Error(err))
		}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	memExecutions := make([]*ToolExecution, 0, len(s.executions))
	for _, exec := range s.executions {
		memExecutions = append(memExecutions, exec)
	}
	return memExecutions
}

// GetStats 获取统计信息（合并内存和数据库）
func (s *Server) GetStats() map[string]*ToolStats {
	if s.storage != nil {
		dbStats, err := s.storage.LoadToolStats()
		if err == nil {
			return dbStats
		}
		s.logger.Warn("从数据库加载统计信息失败", zap.Error(err))
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	memStats := make(map[string]*ToolStats)
	for k, v := range s.stats {
		statCopy := *v
		memStats[k] = &statCopy
	}

	return memStats
}

// GetAllTools 获取所有已注册的工具（用于Agent动态获取工具列表）
func (s *Server) GetAllTools() []Tool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tools := make([]Tool, 0, len(s.toolDefs))
	for _, tool := range s.toolDefs {
		tools = append(tools, tool)
	}
	return tools
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
	// 如果内存中的执行记录超过限制，清理最旧的记录
	s.cleanupOldExecutions()
	s.mu.Unlock()

	if s.storage != nil {
		if err := s.storage.SaveToolExecution(execution); err != nil {
			s.logger.Warn("保存执行记录到数据库失败", zap.Error(err))
		}
	}

	result, err := handler(ctx, args)

	s.mu.Lock()
	now := time.Now()
	execution.EndTime = &now
	execution.Duration = now.Sub(execution.StartTime)
	var failed bool
	var finalResult *ToolResult

	if err != nil {
		execution.Status = "failed"
		execution.Error = err.Error()
		failed = true
	} else if result != nil && result.IsError {
		execution.Status = "failed"
		if len(result.Content) > 0 {
			execution.Error = result.Content[0].Text
		} else {
			execution.Error = "工具执行返回错误结果"
		}
		execution.Result = result
		failed = true
		finalResult = result
	} else {
		execution.Status = "completed"
		if result == nil {
			result = &ToolResult{
				Content: []Content{
					{Type: "text", Text: "工具执行完成，但未返回结果"},
				},
			}
		}
		execution.Result = result
		finalResult = result
		failed = false
	}

	if finalResult == nil {
		finalResult = execution.Result
	}
	s.mu.Unlock()

	if s.storage != nil {
		if err := s.storage.SaveToolExecution(execution); err != nil {
			s.logger.Warn("保存执行记录到数据库失败", zap.Error(err))
		}
	}

	s.updateStats(toolName, failed)

	if s.storage != nil {
		s.mu.Lock()
		delete(s.executions, executionID)
		s.mu.Unlock()
	}

	if err != nil {
		return nil, executionID, err
	}

	return finalResult, executionID, nil
}

// cleanupOldExecutions 清理旧的执行记录，防止内存无限增长
func (s *Server) cleanupOldExecutions() {
	if len(s.executions) <= s.maxExecutionsInMemory {
		return
	}

	// 按开始时间排序，找出最旧的记录
	type execWithTime struct {
		id        string
		startTime time.Time
	}
	execs := make([]execWithTime, 0, len(s.executions))
	for id, exec := range s.executions {
		execs = append(execs, execWithTime{
			id:        id,
			startTime: exec.StartTime,
		})
	}

	// 使用 sort 包进行高效排序（最旧的在前）
	sort.Slice(execs, func(i, j int) bool {
		return execs[i].startTime.Before(execs[j].startTime)
	})

	// 删除最旧的记录，保留 maxExecutionsInMemory 条
	toDelete := len(s.executions) - s.maxExecutionsInMemory
	for i := 0; i < toDelete; i++ {
		delete(s.executions, execs[i].id)
	}

	s.logger.Debug("清理旧的执行记录",
		zap.Int("before", len(execs)),
		zap.Int("after", len(s.executions)),
		zap.Int("deleted", toDelete),
	)
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
			ID:      msg.ID,
			Type:    MessageTypeError,
			Version: "2.0",
			Error:   &Error{Code: -32602, Message: "Invalid params"},
		}
	}

	s.mu.RLock()
	prompt, exists := s.prompts[req.Name]
	s.mu.RUnlock()

	if !exists {
		return &Message{
			ID:      msg.ID,
			Type:    MessageTypeError,
			Version: "2.0",
			Error:   &Error{Code: -32601, Message: "Prompt not found"},
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
			ID:      msg.ID,
			Type:    MessageTypeError,
			Version: "2.0",
			Error:   &Error{Code: -32602, Message: "Invalid params"},
		}
	}

	s.mu.RLock()
	resource, exists := s.resources[req.URI]
	s.mu.RUnlock()

	if !exists {
		return &Message{
			ID:      msg.ID,
			Type:    MessageTypeError,
			Version: "2.0",
			Error:   &Error{Code: -32601, Message: "Resource not found"},
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
// 注意：硬编码的工具文档已移除，现在只使用工具定义中的信息
func (s *Server) generateToolDocumentation(toolName string, resource *Resource) string {
	// 获取工具定义以获取更详细的信息
	s.mu.RLock()
	tool, hasTool := s.toolDefs[toolName]
	s.mu.RUnlock()

	// 使用工具定义中的描述信息
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

// handleSamplingRequest 处理采样请求
func (s *Server) handleSamplingRequest(msg *Message) *Message {
	var req SamplingRequest
	if err := json.Unmarshal(msg.Params, &req); err != nil {
		return &Message{
			ID:      msg.ID,
			Type:    MessageTypeError,
			Version: "2.0",
			Error:   &Error{Code: -32602, Message: "Invalid params"},
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

// HandleStdio 处理标准输入输出（用于 stdio 传输模式）
// MCP 协议使用换行分隔的 JSON-RPC 消息
func (s *Server) HandleStdio() error {
	decoder := json.NewDecoder(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)
	// 注意：不设置缩进，MCP 协议期望紧凑的 JSON 格式

	for {
		var msg Message
		if err := decoder.Decode(&msg); err != nil {
			if err == io.EOF {
				break
			}
			// 日志输出到 stderr，避免干扰 stdout 的 JSON-RPC 通信
			s.logger.Error("读取消息失败", zap.Error(err))
			// 发送错误响应
			errorMsg := Message{
				ID:      msg.ID,
				Type:    MessageTypeError,
				Version: "2.0",
				Error:   &Error{Code: -32700, Message: "Parse error", Data: err.Error()},
			}
			if err := encoder.Encode(errorMsg); err != nil {
				return fmt.Errorf("发送错误响应失败: %w", err)
			}
			continue
		}

		// 处理消息
		response := s.handleMessage(&msg)

		// 如果是通知（response 为 nil），不需要发送响应
		if response == nil {
			continue
		}

		// 发送响应
		if err := encoder.Encode(response); err != nil {
			return fmt.Errorf("发送响应失败: %w", err)
		}
	}

	return nil
}

// sendError 发送错误响应
func (s *Server) sendError(w http.ResponseWriter, id interface{}, code int, message, data string) {
	var msgID MessageID
	if id != nil {
		msgID = MessageID{value: id}
	}
	response := Message{
		ID:      msgID,
		Type:    MessageTypeError,
		Version: "2.0",
		Error:   &Error{Code: code, Message: message, Data: data},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
