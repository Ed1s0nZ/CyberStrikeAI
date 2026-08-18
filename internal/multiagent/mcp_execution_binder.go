package multiagent

import (
	"strings"
	"sync"
)

// MCPExecutionBinder maps ADK toolCallID → MCP monitor execution ID for a single agent run.
type MCPExecutionBinder struct {
	mu             sync.RWMutex
	byToolCall     map[string]string
	argsByToolCall map[string]map[string]interface{}
}

func NewMCPExecutionBinder() *MCPExecutionBinder {
	return &MCPExecutionBinder{
		byToolCall:     make(map[string]string),
		argsByToolCall: make(map[string]map[string]interface{}),
	}
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

func (b *MCPExecutionBinder) BindArguments(toolCallID string, args map[string]interface{}) {
	if b == nil || len(args) == 0 {
		return
	}
	tid := strings.TrimSpace(toolCallID)
	if tid == "" {
		return
	}
	b.mu.Lock()
	b.argsByToolCall[tid] = cloneToolArgs(args)
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

func (b *MCPExecutionBinder) Arguments(toolCallID string) map[string]interface{} {
	if b == nil {
		return nil
	}
	tid := strings.TrimSpace(toolCallID)
	b.mu.RLock()
	defer b.mu.RUnlock()
	return cloneToolArgs(b.argsByToolCall[tid])
}

func cloneToolArgs(args map[string]interface{}) map[string]interface{} {
	if len(args) == 0 {
		return nil
	}
	out := make(map[string]interface{}, len(args))
	for k, v := range args {
		out[k] = v
	}
	return out
}
