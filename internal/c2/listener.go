package c2

import (
	"strings"
	"sync"

	"cyberstrike-ai/internal/database"

	"go.uber.org/zap"
)

// Listener 监听器抽象：每种传输方式（TCP/HTTP/HTTPS/WS/DNS）都实现此接口；
// Manager 不感知具体实现细节，通过 ListenerRegistry 工厂创建。
type Listener interface {
	// Type 返回当前 listener 的类型字符串（如 "tcp_reverse"）
	Type() string
	// Start 启动监听；如果端口被占用应返回 ErrPortInUse
	Start() error
	// Stop 停止监听并释放所有相关 goroutine（不应抛 panic）
	Stop() error
}

// ListenerCreationCtx 工厂初始化 listener 时收到的上下文
type ListenerCreationCtx struct {
	Listener *database.C2Listener
	Config   *ListenerConfig
	Manager  *Manager
	Logger   *zap.Logger
}

// ListenerFactory 创建 listener 实例的工厂；返回的实例尚未 Start
type ListenerFactory func(ctx ListenerCreationCtx) (Listener, error)

// ListenerRegistry 类型 → 工厂 的注册表，由 internal/app 启动时注册具体实现，
// 测试中也可注入 mock 工厂来覆盖。
type ListenerRegistry struct {
	mu        sync.RWMutex
	factories map[string]ListenerFactory
}

// NewListenerRegistry 创建空注册表
func NewListenerRegistry() *ListenerRegistry {
	return &ListenerRegistry{factories: make(map[string]ListenerFactory)}
}

// Register 注册一种 listener 工厂
func (r *ListenerRegistry) Register(typeName string, f ListenerFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[strings.ToLower(strings.TrimSpace(typeName))] = f
}

// Get 取工厂；nil 表示未注册
func (r *ListenerRegistry) Get(typeName string) ListenerFactory {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.factories[strings.ToLower(strings.TrimSpace(typeName))]
}

// RegisteredTypes 列出已注册的类型，给前端枚举用
func (r *ListenerRegistry) RegisteredTypes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.factories))
	for k := range r.factories {
		out = append(out, k)
	}
	return out
}
