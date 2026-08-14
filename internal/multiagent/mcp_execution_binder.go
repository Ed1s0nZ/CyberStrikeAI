package multiagent

import (
	"strings"
	"sync"
)

// MCPExecutionBinder maps ADK toolCallID → MCP monitor execution ID for a single agent run.
type MCPExecutionBinder struct {
	mu         sync.RWMutex
	byToolCall map[string]string
}

func NewMCPExecutionBinder() *MCPExecutionBinder {
	return &MCPExecutionBinder{byToolCall: make(map[string]string)}
}

func (b *MCPExecutionBinder) Bind(toolCallID, executionID string) {
	if b == nil {
		return
	}
	tid := strings.TrimSpace(toolCallID)
	eid := strings.TrimSpace(executionID)
	if tid == "" || eid == "" {
		return
	}
	b.mu.Lock()
	b.byToolCall[tid] = eid
	b.mu.Unlock()
}

func (b *MCPExecutionBinder) ExecutionID(toolCallID string) string {
	if b == nil {
		return ""
	}
	tid := strings.TrimSpace(toolCallID)
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.byToolCall[tid]
}
