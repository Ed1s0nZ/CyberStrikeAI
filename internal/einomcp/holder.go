package einomcp

import "sync"

// ConversationHolder 在每次 DeepAgent 运行前写入会话 ID，供 MCP 工具桥接使用。
type ConversationHolder struct {
	mu sync.RWMutex
	id string
}

func (h *ConversationHolder) Set(id string) {
	h.mu.Lock()
	h.id = id
	h.mu.Unlock()
}

func (h *ConversationHolder) Get() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.id
}
