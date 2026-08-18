package einomcp

import "sync"

// ToolInvokeNotifyHolder 由 Eino run loop 与 MCP/execute 桥共享；Fire 在工具原始返回时触发。
// UI 的 tool_result 须等 ADK schema.Tool 事件（reduction 后正文），不在此 holder 的回调里推送。
type ToolInvokeNotifyHolder struct {
	mu sync.RWMutex
	fn func(toolCallID, toolName, einoAgent string, success bool, content string, invokeErr error)
}

// NewToolInvokeNotifyHolder 创建可在 ToolsFromDefinitions 与 run loop 之间共享的 holder。
func NewToolInvokeNotifyHolder() *ToolInvokeNotifyHolder {
	return &ToolInvokeNotifyHolder{}
}

// Set 由 runEinoADKAgentLoop 在开始消费 iter 之前调用；可多次覆盖（通常仅一次）。
func (h *ToolInvokeNotifyHolder) Set(fn func(toolCallID, toolName, einoAgent string, success bool, content string, invokeErr error)) {
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.fn = fn
}

// Fire 由 mcpBridgeTool 在工具调用返回时调用；若尚未 Set 或 toolCallID 为空则忽略。
func (h *ToolInvokeNotifyHolder) Fire(toolCallID, toolName, einoAgent string, success bool, content string, invokeErr error) {
	if h == nil {
		return
	}
	h.mu.RLock()
	fn := h.fn
	h.mu.RUnlock()
	if fn == nil {
		return
	}
	fn(toolCallID, toolName, einoAgent, success, content, invokeErr)
}
