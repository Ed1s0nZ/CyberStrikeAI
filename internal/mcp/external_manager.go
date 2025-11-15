package mcp

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"cyberstrike-ai/internal/config"

	"github.com/google/uuid"

	"go.uber.org/zap"
)

// ExternalMCPManager 外部MCP管理器
type ExternalMCPManager struct {
	clients    map[string]ExternalMCPClient
	configs    map[string]config.ExternalMCPServerConfig
	logger     *zap.Logger
	storage    MonitorStorage            // 可选的持久化存储
	executions map[string]*ToolExecution // 执行记录
	stats      map[string]*ToolStats     // 工具统计信息
	mu         sync.RWMutex
}

// NewExternalMCPManager 创建外部MCP管理器
func NewExternalMCPManager(logger *zap.Logger) *ExternalMCPManager {
	return NewExternalMCPManagerWithStorage(logger, nil)
}

// NewExternalMCPManagerWithStorage 创建外部MCP管理器（带持久化存储）
func NewExternalMCPManagerWithStorage(logger *zap.Logger, storage MonitorStorage) *ExternalMCPManager {
	return &ExternalMCPManager{
		clients:    make(map[string]ExternalMCPClient),
		configs:    make(map[string]config.ExternalMCPServerConfig),
		logger:     logger,
		storage:    storage,
		executions: make(map[string]*ToolExecution),
		stats:      make(map[string]*ToolStats),
	}
}

// LoadConfigs 加载配置
func (m *ExternalMCPManager) LoadConfigs(cfg *config.ExternalMCPConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if cfg == nil || cfg.Servers == nil {
		return
	}

	m.configs = make(map[string]config.ExternalMCPServerConfig)
	for name, serverCfg := range cfg.Servers {
		m.configs[name] = serverCfg
	}
}

// GetConfigs 获取所有配置
func (m *ExternalMCPManager) GetConfigs() map[string]config.ExternalMCPServerConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]config.ExternalMCPServerConfig)
	for k, v := range m.configs {
		result[k] = v
	}
	return result
}

// AddOrUpdateConfig 添加或更新配置
func (m *ExternalMCPManager) AddOrUpdateConfig(name string, serverCfg config.ExternalMCPServerConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 如果已存在客户端，先关闭
	if client, exists := m.clients[name]; exists {
		client.Close()
		delete(m.clients, name)
	}

	m.configs[name] = serverCfg

	// 如果启用，自动连接
	if m.isEnabled(serverCfg) {
		go m.connectClient(name, serverCfg)
	}

	return nil
}

// RemoveConfig 移除配置
func (m *ExternalMCPManager) RemoveConfig(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 关闭客户端
	if client, exists := m.clients[name]; exists {
		client.Close()
		delete(m.clients, name)
	}

	delete(m.configs, name)
	return nil
}

// StartClient 启动客户端
func (m *ExternalMCPManager) StartClient(name string) error {
	m.mu.Lock()
	serverCfg, exists := m.configs[name]
	m.mu.Unlock()

	if !exists {
		return fmt.Errorf("配置不存在: %s", name)
	}

	// 检查是否已经有连接的客户端
	m.mu.RLock()
	_, hasClient := m.clients[name]
	m.mu.RUnlock()

	if hasClient {
		// 检查客户端是否已连接
		if client, ok := m.GetClient(name); ok && client.IsConnected() {
			// 客户端已连接，直接返回成功（目标状态已达成）
			// 更新配置为启用（确保配置一致）
			m.mu.Lock()
			serverCfg.ExternalMCPEnable = true
			m.configs[name] = serverCfg
			m.mu.Unlock()
			return nil
		}
		// 如果有客户端但未连接，先关闭
		if client, ok := m.GetClient(name); ok {
			client.Close()
			m.mu.Lock()
			delete(m.clients, name)
			m.mu.Unlock()
		}
	}

	// 更新配置为启用
	m.mu.Lock()
	serverCfg.ExternalMCPEnable = true
	m.configs[name] = serverCfg
	m.mu.Unlock()

	// 连接客户端
	return m.connectClient(name, serverCfg)
}

// StopClient 停止客户端
func (m *ExternalMCPManager) StopClient(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	serverCfg, exists := m.configs[name]
	if !exists {
		return fmt.Errorf("配置不存在: %s", name)
	}

	// 关闭客户端
	if client, exists := m.clients[name]; exists {
		client.Close()
		delete(m.clients, name)
	}

	// 更新配置为禁用
	serverCfg.ExternalMCPEnable = false
	m.configs[name] = serverCfg

	return nil
}

// GetClient 获取客户端
func (m *ExternalMCPManager) GetClient(name string) (ExternalMCPClient, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	client, exists := m.clients[name]
	return client, exists
}

// GetAllTools 获取所有外部MCP的工具
func (m *ExternalMCPManager) GetAllTools(ctx context.Context) ([]Tool, error) {
	m.mu.RLock()
	clients := make(map[string]ExternalMCPClient)
	for k, v := range m.clients {
		clients[k] = v
	}
	m.mu.RUnlock()

	var allTools []Tool
	for name, client := range clients {
		if !client.IsConnected() {
			continue
		}

		tools, err := client.ListTools(ctx)
		if err != nil {
			m.logger.Warn("获取外部MCP工具列表失败",
				zap.String("name", name),
				zap.Error(err),
			)
			continue
		}

		// 为工具添加前缀，避免冲突
		for _, tool := range tools {
			tool.Name = fmt.Sprintf("%s::%s", name, tool.Name)
			allTools = append(allTools, tool)
		}
	}

	return allTools, nil
}

// CallTool 调用外部MCP工具（返回执行ID）
func (m *ExternalMCPManager) CallTool(ctx context.Context, toolName string, args map[string]interface{}) (*ToolResult, string, error) {
	// 解析工具名称：name::toolName
	var mcpName, actualToolName string
	if idx := findSubstring(toolName, "::"); idx > 0 {
		mcpName = toolName[:idx]
		actualToolName = toolName[idx+2:]
	} else {
		return nil, "", fmt.Errorf("无效的工具名称格式: %s", toolName)
	}

	client, exists := m.GetClient(mcpName)
	if !exists {
		return nil, "", fmt.Errorf("外部MCP客户端不存在: %s", mcpName)
	}

	if !client.IsConnected() {
		return nil, "", fmt.Errorf("外部MCP客户端未连接: %s", mcpName)
	}

	// 创建执行记录
	executionID := uuid.New().String()
	execution := &ToolExecution{
		ID:        executionID,
		ToolName:  toolName, // 使用完整工具名称（包含MCP名称）
		Arguments: args,
		Status:    "running",
		StartTime: time.Now(),
	}

	m.mu.Lock()
	m.executions[executionID] = execution
	// 如果内存中的执行记录超过限制，清理最旧的记录
	m.cleanupOldExecutions()
	m.mu.Unlock()

	if m.storage != nil {
		if err := m.storage.SaveToolExecution(execution); err != nil {
			m.logger.Warn("保存执行记录到数据库失败", zap.Error(err))
		}
	}

	// 调用工具
	result, err := client.CallTool(ctx, actualToolName, args)

	// 更新执行记录
	m.mu.Lock()
	now := time.Now()
	execution.EndTime = &now
	execution.Duration = now.Sub(execution.StartTime)

	if err != nil {
		execution.Status = "failed"
		execution.Error = err.Error()
	} else if result != nil && result.IsError {
		execution.Status = "failed"
		if len(result.Content) > 0 {
			execution.Error = result.Content[0].Text
		} else {
			execution.Error = "工具执行返回错误结果"
		}
		execution.Result = result
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
	}
	m.mu.Unlock()

	if m.storage != nil {
		if err := m.storage.SaveToolExecution(execution); err != nil {
			m.logger.Warn("保存执行记录到数据库失败", zap.Error(err))
		}
	}

	// 更新统计信息
	failed := err != nil || (result != nil && result.IsError)
	m.updateStats(toolName, failed)

	// 如果使用存储，从内存中删除（已持久化）
	if m.storage != nil {
		m.mu.Lock()
		delete(m.executions, executionID)
		m.mu.Unlock()
	}

	if err != nil {
		return nil, executionID, err
	}

	return result, executionID, nil
}

// cleanupOldExecutions 清理旧的执行记录（保持内存中的记录数量在限制内）
func (m *ExternalMCPManager) cleanupOldExecutions() {
	const maxExecutionsInMemory = 1000
	if len(m.executions) <= maxExecutionsInMemory {
		return
	}

	// 按开始时间排序，删除最旧的记录
	type execTime struct {
		id        string
		startTime time.Time
	}
	var execs []execTime
	for id, exec := range m.executions {
		execs = append(execs, execTime{id: id, startTime: exec.StartTime})
	}

	// 按时间排序
	for i := 0; i < len(execs)-1; i++ {
		for j := i + 1; j < len(execs); j++ {
			if execs[i].startTime.After(execs[j].startTime) {
				execs[i], execs[j] = execs[j], execs[i]
			}
		}
	}

	// 删除最旧的记录
	toDelete := len(m.executions) - maxExecutionsInMemory
	for i := 0; i < toDelete && i < len(execs); i++ {
		delete(m.executions, execs[i].id)
	}
}

// GetExecution 获取执行记录（先从内存查找，再从数据库查找）
func (m *ExternalMCPManager) GetExecution(id string) (*ToolExecution, bool) {
	m.mu.RLock()
	exec, exists := m.executions[id]
	m.mu.RUnlock()

	if exists {
		return exec, true
	}

	if m.storage != nil {
		exec, err := m.storage.GetToolExecution(id)
		if err == nil {
			return exec, true
		}
	}

	return nil, false
}

// updateStats 更新统计信息
func (m *ExternalMCPManager) updateStats(toolName string, failed bool) {
	now := time.Now()
	if m.storage != nil {
		totalCalls := 1
		successCalls := 0
		failedCalls := 0
		if failed {
			failedCalls = 1
		} else {
			successCalls = 1
		}
		if err := m.storage.UpdateToolStats(toolName, totalCalls, successCalls, failedCalls, &now); err != nil {
			m.logger.Warn("保存统计信息到数据库失败", zap.Error(err))
		}
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.stats[toolName] == nil {
		m.stats[toolName] = &ToolStats{
			ToolName: toolName,
		}
	}

	stats := m.stats[toolName]
	stats.TotalCalls++
	stats.LastCallTime = &now

	if failed {
		stats.FailedCalls++
	} else {
		stats.SuccessCalls++
	}
}

// GetStats 获取MCP服务器统计信息
func (m *ExternalMCPManager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	total := len(m.configs)
	enabled := 0
	disabled := 0
	connected := 0

	for name, cfg := range m.configs {
		if m.isEnabled(cfg) {
			enabled++
			if client, exists := m.clients[name]; exists && client.IsConnected() {
				connected++
			}
		} else {
			disabled++
		}
	}

	return map[string]interface{}{
		"total":     total,
		"enabled":   enabled,
		"disabled":  disabled,
		"connected": connected,
	}
}

// GetToolStats 获取工具统计信息（合并内存和数据库）
// 只返回外部MCP工具的统计信息（工具名称包含 "::"）
func (m *ExternalMCPManager) GetToolStats() map[string]*ToolStats {
	result := make(map[string]*ToolStats)

	// 从数据库加载统计信息（如果使用数据库存储）
	if m.storage != nil {
		dbStats, err := m.storage.LoadToolStats()
		if err == nil {
			// 只保留外部MCP工具的统计信息（工具名称包含 "::"）
			for k, v := range dbStats {
				if findSubstring(k, "::") > 0 {
					result[k] = v
				}
			}
		} else {
			m.logger.Warn("从数据库加载统计信息失败", zap.Error(err))
		}
	}

	// 合并内存中的统计信息
	m.mu.RLock()
	for k, v := range m.stats {
		// 如果数据库中已有该工具的统计信息，合并它们
		if existing, exists := result[k]; exists {
			// 创建新的统计信息对象，避免修改共享对象
			merged := &ToolStats{
				ToolName:     k,
				TotalCalls:   existing.TotalCalls + v.TotalCalls,
				SuccessCalls: existing.SuccessCalls + v.SuccessCalls,
				FailedCalls:  existing.FailedCalls + v.FailedCalls,
			}
			// 使用最新的调用时间
			if v.LastCallTime != nil && (existing.LastCallTime == nil || v.LastCallTime.After(*existing.LastCallTime)) {
				merged.LastCallTime = v.LastCallTime
			} else if existing.LastCallTime != nil {
				timeCopy := *existing.LastCallTime
				merged.LastCallTime = &timeCopy
			}
			result[k] = merged
		} else {
			// 如果数据库中没有，直接使用内存中的统计信息
			statCopy := *v
			result[k] = &statCopy
		}
	}
	m.mu.RUnlock()

	return result
}

// GetToolCount 获取指定外部MCP的工具数量
func (m *ExternalMCPManager) GetToolCount(name string) (int, error) {
	client, exists := m.GetClient(name)
	if !exists {
		return 0, fmt.Errorf("客户端不存在: %s", name)
	}

	if !client.IsConnected() {
		return 0, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tools, err := client.ListTools(ctx)
	if err != nil {
		return 0, fmt.Errorf("获取工具列表失败: %w", err)
	}

	return len(tools), nil
}

// GetToolCounts 获取所有外部MCP的工具数量
func (m *ExternalMCPManager) GetToolCounts() map[string]int {
	m.mu.RLock()
	clients := make(map[string]ExternalMCPClient)
	for k, v := range m.clients {
		clients[k] = v
	}
	m.mu.RUnlock()

	result := make(map[string]int)
	for name, client := range clients {
		if !client.IsConnected() {
			result[name] = 0
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		tools, err := client.ListTools(ctx)
		cancel()

		if err != nil {
			m.logger.Warn("获取外部MCP工具数量失败",
				zap.String("name", name),
				zap.Error(err),
			)
			result[name] = 0
			continue
		}

		result[name] = len(tools)
	}

	return result
}

// connectClient 连接客户端（异步）
func (m *ExternalMCPManager) connectClient(name string, serverCfg config.ExternalMCPServerConfig) error {
	var client ExternalMCPClient

	timeout := time.Duration(serverCfg.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	// 根据传输模式创建客户端
	transport := serverCfg.Transport
	if transport == "" {
		// 如果没有指定transport，根据是否有command或url判断
		if serverCfg.Command != "" {
			transport = "stdio"
		} else if serverCfg.URL != "" {
			transport = "http"
		} else {
			return fmt.Errorf("无法确定传输模式: 需要指定command或url")
		}
	}

	switch transport {
	case "http":
		if serverCfg.URL == "" {
			return fmt.Errorf("HTTP模式需要URL")
		}
		client = NewHTTPMCPClient(serverCfg.URL, timeout, m.logger)
	case "stdio":
		if serverCfg.Command == "" {
			return fmt.Errorf("stdio模式需要command")
		}
		client = NewStdioMCPClient(serverCfg.Command, serverCfg.Args, timeout, m.logger)
	default:
		return fmt.Errorf("不支持的传输模式: %s", transport)
	}

	// 初始化连接
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := client.Initialize(ctx); err != nil {
		m.logger.Error("初始化外部MCP客户端失败",
			zap.String("name", name),
			zap.Error(err),
		)
		return err
	}

	// 保存客户端
	m.mu.Lock()
	m.clients[name] = client
	m.mu.Unlock()

	m.logger.Info("外部MCP客户端已连接",
		zap.String("name", name),
		zap.String("transport", transport),
	)

	return nil
}

// isEnabled 检查是否启用
func (m *ExternalMCPManager) isEnabled(cfg config.ExternalMCPServerConfig) bool {
	// 优先使用 ExternalMCPEnable 字段
	// 如果没有设置，检查旧的 enabled/disabled 字段（向后兼容）
	if cfg.ExternalMCPEnable {
		return true
	}
	// 向后兼容：检查旧字段
	if cfg.Disabled {
		return false
	}
	if cfg.Enabled {
		return true
	}
	// 都没有设置，默认为启用
	return true
}

// findSubstring 查找子字符串（简单实现）
func findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// StartAllEnabled 启动所有启用的客户端
func (m *ExternalMCPManager) StartAllEnabled() {
	m.mu.RLock()
	configs := make(map[string]config.ExternalMCPServerConfig)
	for k, v := range m.configs {
		configs[k] = v
	}
	m.mu.RUnlock()

	for name, cfg := range configs {
		if m.isEnabled(cfg) {
			go func(n string, c config.ExternalMCPServerConfig) {
				if err := m.connectClient(n, c); err != nil {
					// 检查是否是连接被拒绝的错误（服务可能还没启动）
					errStr := strings.ToLower(err.Error())
					isConnectionRefused := strings.Contains(errStr, "connection refused") ||
						strings.Contains(errStr, "dial tcp") ||
						strings.Contains(errStr, "connect: connection refused")

					if isConnectionRefused {
						// 连接被拒绝，说明目标服务可能还没启动，这是正常的
						// 使用 Warn 级别，提示用户这是正常的，可以通过手动启动或等待服务启动后自动连接
						fields := []zap.Field{
							zap.String("name", n),
							zap.String("message", "目标服务可能尚未启动，这是正常的。服务启动后可通过界面手动连接，或等待自动重试"),
							zap.Error(err),
						}

						// 根据传输模式添加相应的信息
						transport := c.Transport
						if transport == "" {
							if c.Command != "" {
								transport = "stdio"
							} else if c.URL != "" {
								transport = "http"
							}
						}

						if transport == "http" && c.URL != "" {
							fields = append(fields, zap.String("url", c.URL))
						} else if transport == "stdio" && c.Command != "" {
							fields = append(fields, zap.String("command", c.Command))
						}

						m.logger.Warn("外部MCP服务暂未就绪", fields...)
					} else {
						// 其他错误，使用 Error 级别
						m.logger.Error("启动外部MCP客户端失败",
							zap.String("name", n),
							zap.Error(err),
						)
					}
				}
			}(name, cfg)
		}
	}
}

// StopAll 停止所有客户端
func (m *ExternalMCPManager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, client := range m.clients {
		client.Close()
		delete(m.clients, name)
	}
}
