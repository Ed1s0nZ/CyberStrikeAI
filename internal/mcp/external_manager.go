package mcp

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"cyberstrike-ai/internal/config"

	"github.com/google/uuid"

	"go.uber.org/zap"
)

// ExternalMCPManager external MCP manager
type ExternalMCPManager struct {
	clients      map[string]ExternalMCPClient
	configs      map[string]config.ExternalMCPServerConfig
	logger       *zap.Logger
	storage      MonitorStorage            // optional persistent storage
	executions   map[string]*ToolExecution // execution records
	stats        map[string]*ToolStats     // tool statistics
	errors       map[string]string         // error messages
	toolCounts   map[string]int            // tool count cache
	toolCountsMu sync.RWMutex              // lock for tool count cache
	toolCache    map[string][]Tool         // tool list cache: MCP name -> tool list
	toolCacheMu  sync.RWMutex              // lock for tool list cache
	stopRefresh  chan struct{}             // signal to stop background refresh
	refreshWg    sync.WaitGroup            // wait for background refresh goroutine to finish
	refreshing   atomic.Bool               // guard against concurrent refreshToolCounts stacking up
	mu           sync.RWMutex
}

// NewExternalMCPManager creates an external MCP manager
func NewExternalMCPManager(logger *zap.Logger) *ExternalMCPManager {
	return NewExternalMCPManagerWithStorage(logger, nil)
}

// NewExternalMCPManagerWithStorage creates an external MCP manager with persistent storage
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
		toolCache:   make(map[string][]Tool),
		stopRefresh: make(chan struct{}),
	}
	// start background goroutine to refresh tool counts
	manager.startToolCountRefresh()
	return manager
}

// LoadConfigs loads configurations, replacing any existing set. Existing
// clients are closed before the config map is replaced so reloading does not
// abandon stdio child processes or HTTP sessions from the previous generation.
// Today this is only called once at startup (where m.clients is empty), but
// the Close() sweep keeps the function safe for callers that reload at runtime.
func (m *ExternalMCPManager) LoadConfigs(cfg *config.ExternalMCPConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if cfg == nil || cfg.Servers == nil {
		return
	}

	for name, client := range m.clients {
		client.Close()
		delete(m.clients, name)
	}

	m.configs = make(map[string]config.ExternalMCPServerConfig)
	for name, serverCfg := range cfg.Servers {
		m.configs[name] = serverCfg
	}
}

// GetConfigs returns all configurations
func (m *ExternalMCPManager) GetConfigs() map[string]config.ExternalMCPServerConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]config.ExternalMCPServerConfig)
	for k, v := range m.configs {
		result[k] = v
	}
	return result
}

// AddOrUpdateConfig adds or updates a configuration
func (m *ExternalMCPManager) AddOrUpdateConfig(name string, serverCfg config.ExternalMCPServerConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// if a client already exists, close it first
	if client, exists := m.clients[name]; exists {
		client.Close()
		delete(m.clients, name)
	}

	m.configs[name] = serverCfg

	// if enabled, connect automatically
	if m.isEnabled(serverCfg) {
		go m.connectClient(name, serverCfg)
	}

	return nil
}

// RemoveConfig removes a configuration
func (m *ExternalMCPManager) RemoveConfig(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// close the client
	if client, exists := m.clients[name]; exists {
		client.Close()
		delete(m.clients, name)
	}

	delete(m.configs, name)

	// clear tool count cache
	m.toolCountsMu.Lock()
	delete(m.toolCounts, name)
	m.toolCountsMu.Unlock()

	// clear tool list cache
	m.toolCacheMu.Lock()
	delete(m.toolCache, name)
	m.toolCacheMu.Unlock()

	return nil
}

// StartClient starts a client
func (m *ExternalMCPManager) StartClient(name string) error {
	m.mu.Lock()
	serverCfg, exists := m.configs[name]
	m.mu.Unlock()

	if !exists {
		return fmt.Errorf("configuration not found: %s", name)
	}

	// check if a connected client already exists
	m.mu.RLock()
	existingClient, hasClient := m.clients[name]
	m.mu.RUnlock()

	if hasClient {
		// check if the client is already connected
		if existingClient.IsConnected() {
			// client is already connected, return success directly (target state achieved)
			// update config to enabled (ensure config consistency)
			m.mu.Lock()
			serverCfg.ExternalMCPEnable = true
			m.configs[name] = serverCfg
			m.mu.Unlock()
			return nil
		}
		// if client exists but not connected, close it first
		existingClient.Close()
		m.mu.Lock()
		delete(m.clients, name)
		m.mu.Unlock()
	}

	// update config to enabled
	m.mu.Lock()
	serverCfg.ExternalMCPEnable = true
	m.configs[name] = serverCfg
	// clear previous error message (when restarting)
	delete(m.errors, name)
	m.mu.Unlock()

	// immediately create client and set to "connecting" state so the frontend can see the status right away
	client := m.createClient(serverCfg)
	if client == nil {
		return fmt.Errorf("failed to create client: unsupported transport mode")
	}

	// set status to connecting
	m.setClientStatus(client, "connecting")

	// immediately save the client so the frontend can see "connecting" status when querying
	m.mu.Lock()
	m.clients[name] = client
	m.mu.Unlock()

	// perform the actual connection asynchronously in the background
	go func() {
		if err := m.doConnect(name, serverCfg, client); err != nil {
			m.logger.Error("failed to connect external MCP client",
				zap.String("name", name),
				zap.Error(err),
			)
			// connection failed, set status to error and save error message
			m.setClientStatus(client, "error")
			m.mu.Lock()
			m.errors[name] = err.Error()
			m.mu.Unlock()
			// trigger tool count refresh (connection failed, tool count should be 0)
			m.triggerToolCountRefresh()
		} else {
			// connection succeeded, clear error message
			m.mu.Lock()
			delete(m.errors, name)
			m.mu.Unlock()
			// immediately refresh tool counts and tool list cache
			m.triggerToolCountRefresh()
			m.refreshToolCache(name, client)
			// refresh again after 2 seconds, to cover SSE/Streamable remotes that need a moment to be ready
			go func() {
				time.Sleep(2 * time.Second)
				m.triggerToolCountRefresh()
				m.refreshToolCache(name, client)
			}()
		}
	}()

	return nil
}

// StopClient stops a client
func (m *ExternalMCPManager) StopClient(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	serverCfg, exists := m.configs[name]
	if !exists {
		return fmt.Errorf("configuration not found: %s", name)
	}

	// close the client
	if client, exists := m.clients[name]; exists {
		client.Close()
		delete(m.clients, name)
	}

	// clear error message
	delete(m.errors, name)

	// update tool count cache (tool count is 0 after stopping)
	m.toolCountsMu.Lock()
	m.toolCounts[name] = 0
	m.toolCountsMu.Unlock()

	// update config to disabled
	serverCfg.ExternalMCPEnable = false
	m.configs[name] = serverCfg

	return nil
}

// GetClient returns a client
func (m *ExternalMCPManager) GetClient(name string) (ExternalMCPClient, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	client, exists := m.clients[name]
	return client, exists
}

// GetError returns the error message for a client
func (m *ExternalMCPManager) GetError(name string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.errors[name]
}

// GetAllTools returns all tools from all external MCPs.
// Prefers fetching from connected clients; falls back to cached tool list if disconnected.
// Strategy:
//   - error status: do not use cache, skip directly (config error or service unavailable)
//   - disconnected/connecting status: use cache (temporarily disconnected)
//   - connected status: fetch normally, fall back to cache on failure
func (m *ExternalMCPManager) GetAllTools(ctx context.Context) ([]Tool, error) {
	m.mu.RLock()
	clients := make(map[string]ExternalMCPClient)
	for k, v := range m.clients {
		clients[k] = v
	}
	m.mu.RUnlock()

	var allTools []Tool
	var hasError bool
	var lastError error

	// use a short timeout for quick checks (3 seconds) to avoid blocking
	quickCtx, quickCancel := context.WithTimeout(ctx, 3*time.Second)
	defer quickCancel()

	for name, client := range clients {
		tools, err := m.getToolsForClient(name, client, quickCtx)
		if err != nil {
			// record the error, but continue processing other clients
			hasError = true
			if lastError == nil {
				lastError = err
			}
			continue
		}

		// add a prefix to tool names to avoid conflicts
		for _, tool := range tools {
			tool.Name = fmt.Sprintf("%s::%s", name, tool.Name)
			allTools = append(allTools, tool)
		}
	}

	// if there were errors but at least some tools were returned, don't return an error (partial success)
	if hasError && len(allTools) == 0 {
		return nil, fmt.Errorf("failed to get external MCP tools: %w", lastError)
	}

	return allTools, nil
}

// getToolsForClient returns the tool list for a specific client.
// Returns the tool list and an error if the tools cannot be retrieved at all.
func (m *ExternalMCPManager) getToolsForClient(name string, client ExternalMCPClient, ctx context.Context) ([]Tool, error) {
	status := client.GetStatus()

	// error status: do not use cache, return error directly
	if status == "error" {
		m.logger.Debug("skipping failed external MCP (not using cache)",
			zap.String("name", name),
			zap.String("status", status),
		)
		return nil, fmt.Errorf("external MCP connection failed: %s", name)
	}

	// connected: try to get the latest tool list
	if client.IsConnected() {
		tools, err := client.ListTools(ctx)
		if err != nil {
			// fetch failed, try using cache
			return m.getCachedTools(name, "connected but fetch failed", err)
		}

		// fetch succeeded, update cache
		m.updateToolCache(name, tools)
		return tools, nil
	}

	// not connected: decide whether to use cache based on status
	if status == "disconnected" || status == "connecting" {
		return m.getCachedTools(name, fmt.Sprintf("client temporarily disconnected (status: %s)", status), nil)
	}

	// other unknown status, do not use cache
	m.logger.Debug("skipping external MCP (unknown status)",
		zap.String("name", name),
		zap.String("status", status),
	)
	return nil, fmt.Errorf("external MCP status unknown: %s (status: %s)", name, status)
}

// getCachedTools returns the cached tool list
func (m *ExternalMCPManager) getCachedTools(name, reason string, originalErr error) ([]Tool, error) {
	m.toolCacheMu.RLock()
	cachedTools, hasCache := m.toolCache[name]
	m.toolCacheMu.RUnlock()

	if hasCache && len(cachedTools) > 0 {
		m.logger.Debug("using cached tool list",
			zap.String("name", name),
			zap.String("reason", reason),
			zap.Int("count", len(cachedTools)),
			zap.Error(originalErr),
		)
		return cachedTools, nil
	}

	// no cache, return error
	if originalErr != nil {
		return nil, fmt.Errorf("failed to get external MCP tools and no cache available: %w", originalErr)
	}
	return nil, fmt.Errorf("no cached tools for external MCP: %s", name)
}

// updateToolCache updates the tool list cache
func (m *ExternalMCPManager) updateToolCache(name string, tools []Tool) {
	m.toolCacheMu.Lock()
	m.toolCache[name] = tools
	m.toolCacheMu.Unlock()

	// log a warning if the returned list is empty
	if len(tools) == 0 {
		m.logger.Warn("external MCP returned empty tool list",
			zap.String("name", name),
			zap.String("hint", "service may be temporarily unavailable, tool list is empty"),
		)
	} else {
		m.logger.Debug("tool list cache updated",
			zap.String("name", name),
			zap.Int("count", len(tools)),
		)
	}
}

// CallTool calls an external MCP tool (returns execution ID)
func (m *ExternalMCPManager) CallTool(ctx context.Context, toolName string, args map[string]interface{}) (*ToolResult, string, error) {
	// parse tool name: name::toolName
	var mcpName, actualToolName string
	if idx := strings.Index(toolName, "::"); idx > 0 {
		mcpName = toolName[:idx]
		actualToolName = toolName[idx+2:]
	} else {
		return nil, "", fmt.Errorf("invalid tool name format: %s", toolName)
	}

	client, exists := m.GetClient(mcpName)
	if !exists {
		return nil, "", fmt.Errorf("external MCP client not found: %s", mcpName)
	}

	// check connection status; if not connected or in error state, disallow calling
	if !client.IsConnected() {
		status := client.GetStatus()
		if status == "error" {
			// get error message (if any)
			errorMsg := m.GetError(mcpName)
			if errorMsg != "" {
				return nil, "", fmt.Errorf("external MCP connection failed: %s (error: %s)", mcpName, errorMsg)
			}
			return nil, "", fmt.Errorf("external MCP connection failed: %s", mcpName)
		}
		return nil, "", fmt.Errorf("external MCP client not connected: %s (status: %s)", mcpName, status)
	}

	// create execution record
	executionID := uuid.New().String()
	execution := &ToolExecution{
		ID:        executionID,
		ToolName:  toolName, // use full tool name (including MCP name)
		Arguments: args,
		Status:    "running",
		StartTime: time.Now(),
	}

	m.mu.Lock()
	m.executions[executionID] = execution
	// if execution records in memory exceed the limit, clean up the oldest records
	m.cleanupOldExecutions()
	m.mu.Unlock()

	if m.storage != nil {
		if err := m.storage.SaveToolExecution(execution); err != nil {
			m.logger.Warn("failed to save execution record to database", zap.Error(err))
		}
	}

	// call the tool
	result, err := client.CallTool(ctx, actualToolName, args)

	// update execution record
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
			execution.Error = "tool execution returned an error result"
		}
		execution.Result = result
	} else {
		execution.Status = "completed"
		if result == nil {
			result = &ToolResult{
				Content: []Content{
					{Type: "text", Text: "tool execution complete, but no result returned"},
				},
			}
		}
		execution.Result = result
	}
	m.mu.Unlock()

	if m.storage != nil {
		if err := m.storage.SaveToolExecution(execution); err != nil {
			m.logger.Warn("failed to save execution record to database", zap.Error(err))
		}
	}

	// update statistics
	failed := err != nil || (result != nil && result.IsError)
	m.updateStats(toolName, failed)

	// if using storage, remove from memory (already persisted)
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

// cleanupOldExecutions cleans up old execution records (keeps the count within the limit)
func (m *ExternalMCPManager) cleanupOldExecutions() {
	const maxExecutionsInMemory = 1000
	if len(m.executions) <= maxExecutionsInMemory {
		return
	}

	// sort by start time, delete the oldest records
	type execTime struct {
		id        string
		startTime time.Time
	}
	var execs []execTime
	for id, exec := range m.executions {
		execs = append(execs, execTime{id: id, startTime: exec.StartTime})
	}

	// sort by time (ascending - oldest first)
	sort.Slice(execs, func(i, j int) bool {
		return execs[i].startTime.Before(execs[j].startTime)
	})

	// delete the oldest records
	toDelete := len(m.executions) - maxExecutionsInMemory
	for i := 0; i < toDelete && i < len(execs); i++ {
		delete(m.executions, execs[i].id)
	}
}

// GetExecution returns an execution record (searches memory first, then database)
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

// updateStats updates statistics
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
			m.logger.Warn("failed to save statistics to database", zap.Error(err))
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

// GetStats returns MCP server statistics
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

// GetToolStats returns tool statistics (merged from memory and database).
// Only returns statistics for external MCP tools (tool names containing "::").
func (m *ExternalMCPManager) GetToolStats() map[string]*ToolStats {
	result := make(map[string]*ToolStats)

	// load statistics from database (if using database storage)
	if m.storage != nil {
		dbStats, err := m.storage.LoadToolStats()
		if err == nil {
			// only keep statistics for external MCP tools (tool names containing "::")
			for k, v := range dbStats {
				if strings.Index(k, "::") > 0 {
					result[k] = v
				}
			}
		} else {
			m.logger.Warn("failed to load statistics from database", zap.Error(err))
		}
	}

	// merge in-memory statistics
	m.mu.RLock()
	for k, v := range m.stats {
		// if the database already has stats for this tool, merge them
		if existing, exists := result[k]; exists {
			// create a new stats object to avoid modifying the shared object
			merged := &ToolStats{
				ToolName:     k,
				TotalCalls:   existing.TotalCalls + v.TotalCalls,
				SuccessCalls: existing.SuccessCalls + v.SuccessCalls,
				FailedCalls:  existing.FailedCalls + v.FailedCalls,
			}
			// use the most recent call time
			if v.LastCallTime != nil && (existing.LastCallTime == nil || v.LastCallTime.After(*existing.LastCallTime)) {
				merged.LastCallTime = v.LastCallTime
			} else if existing.LastCallTime != nil {
				timeCopy := *existing.LastCallTime
				merged.LastCallTime = &timeCopy
			}
			result[k] = merged
		} else {
			// if not in the database, use the in-memory statistics directly
			statCopy := *v
			result[k] = &statCopy
		}
	}
	m.mu.RUnlock()

	return result
}

// GetToolCount returns the tool count for a specific external MCP (reads from cache, non-blocking)
func (m *ExternalMCPManager) GetToolCount(name string) (int, error) {
	// read from cache first
	m.toolCountsMu.RLock()
	if count, exists := m.toolCounts[name]; exists {
		m.toolCountsMu.RUnlock()
		return count, nil
	}
	m.toolCountsMu.RUnlock()

	// if not in cache, check client status
	client, exists := m.GetClient(name)
	if !exists {
		return 0, fmt.Errorf("client not found: %s", name)
	}

	if !client.IsConnected() {
		// not connected, cache as 0
		m.toolCountsMu.Lock()
		m.toolCounts[name] = 0
		m.toolCountsMu.Unlock()
		return 0, nil
	}

	// connected but not in cache, trigger async refresh and return 0 (avoid blocking)
	m.triggerToolCountRefresh()
	return 0, nil
}

// GetToolCounts returns tool counts for all external MCPs (reads from cache, non-blocking)
func (m *ExternalMCPManager) GetToolCounts() map[string]int {
	m.toolCountsMu.RLock()
	defer m.toolCountsMu.RUnlock()

	// return a copy of the cache to prevent external modifications
	result := make(map[string]int)
	for k, v := range m.toolCounts {
		result[k] = v
	}
	return result
}

// refreshToolCounts refreshes the tool count cache (executed asynchronously in background).
// Guarded by an atomic flag: if a previous refresh hasn't finished, this invocation returns
// immediately instead of stacking up behind it. Slow MCP endpoints combined with the periodic
// ticker used to let goroutines queue and eventually exhaust heap/fd.
func (m *ExternalMCPManager) refreshToolCounts() {
	if !m.refreshing.CompareAndSwap(false, true) {
		return
	}
	defer m.refreshing.Store(false)

	m.mu.RLock()
	clients := make(map[string]ExternalMCPClient)
	for k, v := range m.clients {
		clients[k] = v
	}
	m.mu.RUnlock()

	newCounts := make(map[string]int)

	// use goroutines to concurrently fetch tool counts from each client, avoiding serial blocking
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

			// use a reasonable timeout (15 seconds) to handle network latency without blocking too long
			// since this is a background async refresh, timeouts don't affect frontend responses
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			tools, err := c.ListTools(ctx)
			cancel()

			if err != nil {
				errStr := err.Error()
				// SSE connection EOF: remote may have closed the stream or didn't push responses per spec, warn only on first occurrence
				if strings.Contains(errStr, "EOF") || strings.Contains(errStr, "client is closing") {
					m.logger.Warn("failed to get external MCP tool count (SSE stream closed or server did not return tools/list response on stream)",
						zap.String("name", n),
						zap.String("hint", "for SSE connections, ensure the server keeps the GET stream open and pushes JSON-RPC responses as event: message per MCP spec"),
						zap.Error(err),
					)
				} else {
					m.logger.Warn("failed to get external MCP tool count, check connection or server tools/list",
						zap.String("name", n),
						zap.Error(err),
					)
				}
				resultChan <- countResult{name: n, count: -1} // -1 means keep old value
				return
			}

			resultChan <- countResult{name: n, count: len(tools)}
		}(name, client)
	}

	// collect results
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
			// fetch failed, keep old value
			if oldCount, exists := oldCounts[result.name]; exists {
				newCounts[result.name] = oldCount
			} else {
				newCounts[result.name] = 0
			}
		}
	}

	// update cache
	m.toolCountsMu.Lock()
	// update all fetched values
	for name, count := range newCounts {
		m.toolCounts[name] = count
	}
	// set disconnected clients to 0
	for name, client := range clients {
		if !client.IsConnected() {
			m.toolCounts[name] = 0
		}
	}
	m.toolCountsMu.Unlock()
}

// refreshToolCache refreshes the tool list cache for a specific MCP
func (m *ExternalMCPManager) refreshToolCache(name string, client ExternalMCPClient) {
	if !client.IsConnected() {
		return
	}

	// check status; if error status, do not update cache
	status := client.GetStatus()
	if status == "error" {
		m.logger.Debug("skipping tool list cache refresh (connection failed)",
			zap.String("name", name),
			zap.String("status", status),
		)
		return
	}

	// use a shorter timeout (5 seconds)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tools, err := client.ListTools(ctx)
	if err != nil {
		m.logger.Debug("failed to refresh tool list cache",
			zap.String("name", name),
			zap.Error(err),
		)
		// on refresh failure, do not update cache, keep old cache (if any)
		return
	}

	// use the unified cache update method
	m.updateToolCache(name, tools)
}

// startToolCountRefresh starts a background goroutine to refresh tool counts
func (m *ExternalMCPManager) startToolCountRefresh() {
	m.refreshWg.Add(1)
	go func() {
		defer m.refreshWg.Done()
		ticker := time.NewTicker(10 * time.Second) // refresh every 10 seconds
		defer ticker.Stop()

		// run a refresh immediately
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

// triggerToolCountRefresh triggers an immediate tool count refresh (async).
// Tracked by refreshWg so StopAll can wait for in-flight refreshes before
// tearing down m.clients - otherwise a straggler goroutine races the teardown
// and repopulates caches against state that is already being cleared.
func (m *ExternalMCPManager) triggerToolCountRefresh() {
	m.refreshWg.Add(1)
	go func() {
		defer m.refreshWg.Done()
		m.refreshToolCounts()
	}()
}

// createClient creates a client (without connecting). Uses the official MCP Go SDK lazy client uniformly; connection is completed at Initialize time.
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
		// simple HTTP (one POST one response), for self-hosted MCPs etc.
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

// doConnect performs the actual connection
func (m *ExternalMCPManager) doConnect(name string, serverCfg config.ExternalMCPServerConfig, client ExternalMCPClient) error {
	timeout := time.Duration(serverCfg.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	// initialize connection
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := client.Initialize(ctx); err != nil {
		return err
	}

	m.logger.Info("external MCP client connected",
		zap.String("name", name),
	)

	return nil
}

// setClientStatus sets the client status (via type assertion)
func (m *ExternalMCPManager) setClientStatus(client ExternalMCPClient, status string) {
	if c, ok := client.(*lazySDKClient); ok {
		c.setStatus(status)
	}
}

// connectClient connects a client (async). Installs the client in the map in
// "connecting" state before initialization so a concurrent StartClient call for
// the same name sees the existing entry and yields to this goroutine instead
// of racing it. On Initialize failure, sets the client to "error" state and
// records the message in m.errors so the UI can surface it - matching the
// behavior of StartClient's failure path.
func (m *ExternalMCPManager) connectClient(name string, serverCfg config.ExternalMCPServerConfig) error {
	client := m.createClient(serverCfg)
	if client == nil {
		return fmt.Errorf("failed to create client: unsupported transport mode")
	}

	// Claim the slot before spending time on Initialize. If another goroutine
	// (typically StartClient, or a sibling connectClient from a rapid
	// AddOrUpdateConfig -> StartClient sequence) has already installed a
	// client for this name, release our unused client and exit - whoever won
	// the race owns the slot and we must not silently overwrite their session.
	m.mu.Lock()
	if existing, exists := m.clients[name]; exists && existing != nil {
		m.mu.Unlock()
		client.Close()
		m.logger.Debug("connectClient yielding: another goroutine already installed a client",
			zap.String("name", name),
		)
		return nil
	}
	m.clients[name] = client
	delete(m.errors, name) // clear any stale error from a previous attempt
	m.mu.Unlock()

	m.setClientStatus(client, "connecting")

	timeout := time.Duration(serverCfg.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := client.Initialize(ctx); err != nil {
		m.logger.Error("failed to initialize external MCP client",
			zap.String("name", name),
			zap.Error(err),
		)
		// Keep the client in the map so the UI sees "error" state instead of
		// the misleading "disconnected" (which would hide the error message).
		// m.errors[name] carries the detail the UI displays.
		m.setClientStatus(client, "error")
		m.mu.Lock()
		m.errors[name] = err.Error()
		m.mu.Unlock()
		m.triggerToolCountRefresh()
		return err
	}

	m.logger.Info("external MCP client connected",
		zap.String("name", name),
	)

	// connection succeeded, trigger tool count refresh and tool list cache refresh
	m.triggerToolCountRefresh()
	m.mu.RLock()
	if c, exists := m.clients[name]; exists {
		m.refreshToolCache(name, c)
	}
	m.mu.RUnlock()

	return nil
}

// isEnabled checks if a config is enabled
func (m *ExternalMCPManager) isEnabled(cfg config.ExternalMCPServerConfig) bool {
	// prefer ExternalMCPEnable field
	// if not set, check the old enabled/disabled fields (backward compatibility)
	if cfg.ExternalMCPEnable {
		return true
	}
	// backward compatibility: check old fields
	if cfg.Disabled {
		return false
	}
	if cfg.Enabled {
		return true
	}
	// neither set, default to enabled
	return true
}

// StartAllEnabled starts all enabled clients
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
					// check if this is a connection refused error (service may not have started yet)
					errStr := strings.ToLower(err.Error())
					isConnectionRefused := strings.Contains(errStr, "connection refused") ||
						strings.Contains(errStr, "dial tcp") ||
						strings.Contains(errStr, "connect: connection refused")

					if isConnectionRefused {
						// connection refused means the target service may not have started yet, which is normal
						// use Warn level to inform the user this is expected; they can reconnect manually or wait for auto-retry
						fields := []zap.Field{
							zap.String("name", n),
							zap.String("message", "target service may not have started yet, this is normal. You can connect manually via the UI once the service is up, or wait for auto-retry"),
							zap.Error(err),
						}

						// add transport-specific information
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

						m.logger.Warn("external MCP service not yet ready", fields...)
					} else {
						// other errors, use Error level
						m.logger.Error("failed to start external MCP client",
							zap.String("name", n),
							zap.Error(err),
						)
					}
				}
			}(name, cfg)
		}
	}
}

// StopAll stops all clients and waits for background refresh goroutines to drain.
//
// Ordering matters: we stop the ticker and drain refreshWg *before* taking the
// write lock on m.mu. If we waited while holding m.mu an in-flight
// refreshToolCounts goroutine blocked on m.mu.RLock would never finish, and
// refreshWg.Wait() would deadlock. Lock-free shutdown first, then clear state.
//
// Lock-order invariant for this package (see finding #6 of the MCP review):
// m.mu is acquired *before* m.toolCountsMu / m.toolCacheMu. Both
// refreshToolCounts and StopAll follow that order; any future modification
// that reverses it will deadlock.
func (m *ExternalMCPManager) StopAll() {
	// 1. Signal the background ticker to stop, then drain any in-flight
	//    refreshes (both ticker-driven and triggered). Use a select to avoid
	//    closing an already-closed channel when StopAll is invoked twice.
	select {
	case <-m.stopRefresh:
		// already closed
	default:
		close(m.stopRefresh)
	}
	m.refreshWg.Wait()

	// 2. Close clients and clear config-related state.
	m.mu.Lock()
	for name, client := range m.clients {
		client.Close()
		delete(m.clients, name)
	}
	m.mu.Unlock()

	// 3. Clear caches. These two locks are leaves of the lock order.
	m.toolCountsMu.Lock()
	m.toolCounts = make(map[string]int)
	m.toolCountsMu.Unlock()

	m.toolCacheMu.Lock()
	m.toolCache = make(map[string][]Tool)
	m.toolCacheMu.Unlock()
}
