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
	clients      map[string]ExternalMCPClient
	configs      map[string]config.ExternalMCPServerConfig
	logger       *zap.Logger
	storage      MonitorStorage            // 可选的持久化存储
	executions   map[string]*ToolExecution // 执行记录
	stats        map[string]*ToolStats     // 工具统计信息
	errors       map[string]string         // 错误信息
	toolCounts   map[string]int            // 工具数量缓存
	toolCountsMu sync.RWMutex              // 工具数量缓存的锁
	stopRefresh  chan struct{}             // 停止后台刷新的信号
	refreshWg    sync.WaitGroup            // 等待后台刷新goroutine完成
	mu           sync.RWMutex
}

// NewExternalMCPManager 创建外部MCP管理器
func NewExternalMCPManager(logger *zap.Logger) *ExternalMCPManager {
	return NewExternalMCPManagerWithStorage(logger, nil)
}

// NewExternalMCPManagerWithStorage 创建外部MCP管理器（带持久化存储）
func NewExternalMCPManagerWithStorage(logger *zap.Logger, storage MonitorStorage) *ExternalMCPManager {
	manager := &ExternalMCPManager{
		clients:     make(map[string]ExternalMCPClient),
		configs:     make(map[string]config.ExternalMCPServerConfig),
		logger:      logger,
		storage:     storage,
		executions:  make(map[string]*ToolExecution),
		stats:       make(map[string]*ToolStats),
		errors:      make(map[string]string),
		toolCounts:  make(map[string]int),
		stopRefresh: make(chan struct{}),
	}
	// 启动后台刷新工具数量的goroutine
	manager.startToolCountRefresh()
	return manager
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

	// 清理工具数量缓存
	m.toolCountsMu.Lock()
	delete(m.toolCounts, name)
	m.toolCountsMu.Unlock()

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
	existingClient, hasClient := m.clients[name]
	m.mu.RUnlock()

	if hasClient {
		// 检查客户端是否已连接
		if existingClient.IsConnected() {
			// 客户端已连接，直接返回成功（目标状态已达成）
			// 更新配置为启用（确保配置一致）
			m.mu.Lock()
			serverCfg.ExternalMCPEnable = true
			m.configs[name] = serverCfg
			m.mu.Unlock()
			return nil
		}
		// 如果有客户端但未连接，先关闭
		existingClient.Close()
		m.mu.Lock()
		delete(m.clients, name)
		m.mu.Unlock()
	}

	// 更新配置为启用
	m.mu.Lock()
	serverCfg.ExternalMCPEnable = true
	m.configs[name] = serverCfg
	// 清除之前的错误信息（重新启动时）
	delete(m.errors, name)
	m.mu.Unlock()

	// 立即创建客户端并设置为"connecting"状态，这样前端可以立即看到状态
	client := m.createClient(serverCfg)
	if client == nil {
		return fmt.Errorf("无法创建客户端：不支持的传输模式")
	}

	// 设置状态为connecting
	m.setClientStatus(client, "connecting")

	// 立即保存客户端，这样前端查询时就能看到"connecting"状态
	m.mu.Lock()
	m.clients[name] = client
	m.mu.Unlock()

	// 在后台异步进行实际连接
	go func() {
		if err := m.doConnect(name, serverCfg, client); err != nil {
			m.logger.Error("连接外部MCP客户端失败",
				zap.String("name", name),
				zap.Error(err),
			)
			// 连接失败，设置状态为error并保存错误信息
			m.setClientStatus(client, "error")
			m.mu.Lock()
			m.errors[name] = err.Error()
			m.mu.Unlock()
			// 触发工具数量刷新（连接失败，工具数量应为0）
			m.triggerToolCountRefresh()
		} else {
			// 连接成功，清除错误信息
			m.mu.Lock()
			delete(m.errors, name)
			m.mu.Unlock()
			// 延迟再刷新工具数量，避免 SSE/Streamable 连接尚未就绪时立即请求导致 EOF（如值得买等远端）
			time.Sleep(2 * time.Second)
			m.triggerToolCountRefresh()
		}
	}()

	return nil
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

	// 清除错误信息
	delete(m.errors, name)

	// 更新工具数量缓存（停止后工具数量为0）
	m.toolCountsMu.Lock()
	m.toolCounts[name] = 0
	m.toolCountsMu.Unlock()

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

// GetError 获取错误信息
func (m *ExternalMCPManager) GetError(name string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.errors[name]
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

// GetToolCount 获取指定外部MCP的工具数量（从缓存读取，不阻塞）
func (m *ExternalMCPManager) GetToolCount(name string) (int, error) {
	// 先从缓存读取
	m.toolCountsMu.RLock()
	if count, exists := m.toolCounts[name]; exists {
		m.toolCountsMu.RUnlock()
		return count, nil
	}
	m.toolCountsMu.RUnlock()

	// 如果缓存中没有，检查客户端状态
	client, exists := m.GetClient(name)
	if !exists {
		return 0, fmt.Errorf("客户端不存在: %s", name)
	}

	if !client.IsConnected() {
		// 未连接，缓存为0
		m.toolCountsMu.Lock()
		m.toolCounts[name] = 0
		m.toolCountsMu.Unlock()
		return 0, nil
	}

	// 如果已连接但缓存中没有，触发异步刷新并返回0（避免阻塞）
	m.triggerToolCountRefresh()
	return 0, nil
}

// GetToolCounts 获取所有外部MCP的工具数量（从缓存读取，不阻塞）
func (m *ExternalMCPManager) GetToolCounts() map[string]int {
	m.toolCountsMu.RLock()
	defer m.toolCountsMu.RUnlock()

	// 返回缓存的副本，避免外部修改
	result := make(map[string]int)
	for k, v := range m.toolCounts {
		result[k] = v
	}
	return result
}

// refreshToolCounts 刷新工具数量缓存（后台异步执行）
func (m *ExternalMCPManager) refreshToolCounts() {
	m.mu.RLock()
	clients := make(map[string]ExternalMCPClient)
	for k, v := range m.clients {
		clients[k] = v
	}
	m.mu.RUnlock()

	newCounts := make(map[string]int)

	// 使用goroutine并发获取每个客户端的工具数量，避免串行阻塞
	type countResult struct {
		name  string
		count int
	}
	resultChan := make(chan countResult, len(clients))

	for name, client := range clients {
		go func(n string, c ExternalMCPClient) {
			if !c.IsConnected() {
				resultChan <- countResult{name: n, count: 0}
				return
			}

			// 使用合理的超时时间（15秒），既能应对网络延迟，又不会过长阻塞
			// 由于这是后台异步刷新，超时不会影响前端响应
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			tools, err := c.ListTools(ctx)
			cancel()

			if err != nil {
				errStr := err.Error()
				// SSE 连接 EOF：远端可能关闭了流或未按规范在流上推送响应，仅首次用 Warn 提示
				if strings.Contains(errStr, "EOF") || strings.Contains(errStr, "client is closing") {
					m.logger.Warn("获取外部MCP工具数量失败（SSE 流已关闭或服务端未在流上返回 tools/list 响应）",
						zap.String("name", n),
						zap.String("hint", "若为 SSE 连接，请确认服务端保持 GET 流打开并按 MCP 规范以 event: message 推送 JSON-RPC 响应"),
						zap.Error(err),
					)
				} else {
					m.logger.Warn("获取外部MCP工具数量失败，请检查连接或服务端 tools/list",
						zap.String("name", n),
						zap.Error(err),
					)
				}
				resultChan <- countResult{name: n, count: -1} // -1 表示使用旧值
				return
			}

			resultChan <- countResult{name: n, count: len(tools)}
		}(name, client)
	}

	// 收集结果
	m.toolCountsMu.RLock()
	oldCounts := make(map[string]int)
	for k, v := range m.toolCounts {
		oldCounts[k] = v
	}
	m.toolCountsMu.RUnlock()

	for i := 0; i < len(clients); i++ {
		result := <-resultChan
		if result.count >= 0 {
			newCounts[result.name] = result.count
		} else {
			// 获取失败，保留旧值
			if oldCount, exists := oldCounts[result.name]; exists {
				newCounts[result.name] = oldCount
			} else {
				newCounts[result.name] = 0
			}
		}
	}

	// 更新缓存
	m.toolCountsMu.Lock()
	// 更新所有获取到的值
	for name, count := range newCounts {
		m.toolCounts[name] = count
	}
	// 对于未连接的客户端，设置为0
	for name, client := range clients {
		if !client.IsConnected() {
			m.toolCounts[name] = 0
		}
	}
	m.toolCountsMu.Unlock()
}

// startToolCountRefresh 启动后台刷新工具数量的goroutine
func (m *ExternalMCPManager) startToolCountRefresh() {
	m.refreshWg.Add(1)
	go func() {
		defer m.refreshWg.Done()
		ticker := time.NewTicker(10 * time.Second) // 每10秒刷新一次
		defer ticker.Stop()

		// 立即执行一次刷新
		m.refreshToolCounts()

		for {
			select {
			case <-ticker.C:
				m.refreshToolCounts()
			case <-m.stopRefresh:
				return
			}
		}
	}()
}

// triggerToolCountRefresh 触发立即刷新工具数量（异步）
func (m *ExternalMCPManager) triggerToolCountRefresh() {
	go m.refreshToolCounts()
}

// createClient 创建客户端（不连接）。统一使用官方 MCP Go SDK 的 lazy 客户端，连接在 Initialize 时完成。
func (m *ExternalMCPManager) createClient(serverCfg config.ExternalMCPServerConfig) ExternalMCPClient {
	transport := serverCfg.Transport
	if transport == "" {
		if serverCfg.Command != "" {
			transport = "stdio"
		} else if serverCfg.URL != "" {
			transport = "http"
		} else {
			return nil
		}
	}

	switch transport {
	case "http":
		if serverCfg.URL == "" {
			return nil
		}
		return newLazySDKClient(serverCfg, m.logger)
	case "simple_http":
		// 简单 HTTP（一次 POST 一次响应），用于自建 MCP 等
		if serverCfg.URL == "" {
			return nil
		}
		return newLazySDKClient(serverCfg, m.logger)
	case "stdio":
		if serverCfg.Command == "" {
			return nil
		}
		return newLazySDKClient(serverCfg, m.logger)
	case "sse":
		if serverCfg.URL == "" {
			return nil
		}
		return newLazySDKClient(serverCfg, m.logger)
	default:
		return nil
	}
}

// doConnect 执行实际连接
func (m *ExternalMCPManager) doConnect(name string, serverCfg config.ExternalMCPServerConfig, client ExternalMCPClient) error {
	timeout := time.Duration(serverCfg.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	// 初始化连接
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := client.Initialize(ctx); err != nil {
		return err
	}

	m.logger.Info("外部MCP客户端已连接",
		zap.String("name", name),
	)

	return nil
}

// setClientStatus 设置客户端状态（通过类型断言）
func (m *ExternalMCPManager) setClientStatus(client ExternalMCPClient, status string) {
	if c, ok := client.(*lazySDKClient); ok {
		c.setStatus(status)
	}
}

// connectClient 连接客户端（异步）- 保留用于向后兼容
func (m *ExternalMCPManager) connectClient(name string, serverCfg config.ExternalMCPServerConfig) error {
	client := m.createClient(serverCfg)
	if client == nil {
		return fmt.Errorf("无法创建客户端：不支持的传输模式")
	}

	// 设置状态为connecting
	m.setClientStatus(client, "connecting")

	// 初始化连接
	timeout := time.Duration(serverCfg.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

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
	)

	// 连接成功，触发工具数量刷新
	m.triggerToolCountRefresh()

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

	// 清理所有工具数量缓存
	m.toolCountsMu.Lock()
	m.toolCounts = make(map[string]int)
	m.toolCountsMu.Unlock()

	// 停止后台刷新（使用 select 避免重复关闭 channel）
	select {
	case <-m.stopRefresh:
		// 已经关闭，不需要再次关闭
	default:
		close(m.stopRefresh)
		m.refreshWg.Wait()
	}
}
