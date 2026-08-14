package c2

import (
	"sync"
	"sync/atomic"
	"time"
)

// Event 是 EventBus 内部传输的事件单元，是 database.C2Event 的"实时投影"。
// 区别在于：
//   - 数据库表保存全部历史，用于审计与列表分页；
//   - EventBus 只缓存最近 N 条，用于 SSE/WS 实时推送给在线订阅者。
type Event struct {
	ID        string                 `json:"id"`
	Level     string                 `json:"level"`
	Category  string                 `json:"category"`
	SessionID string                 `json:"sessionId,omitempty"`
	TaskID    string                 `json:"taskId,omitempty"`
	Message   string                 `json:"message"`
	Data      map[string]interface{} `json:"data,omitempty"`
	CreatedAt time.Time              `json:"createdAt"`
}

// EventBus 简单的内存广播总线。
// 设计要点：
//   - 多订阅者：每个订阅者有独立 buffered channel，慢消费者不会阻塞 publisher；
//   - 容量满即丢弃：发布端绝不阻塞，避免 listener accept loop / beacon handler 卡住；
//   - 全局过滤：订阅时可限定 SessionID/Category，前端按需订阅，省 CPU；
//   - 关闭安全：Close() 后所有订阅者 chan 关闭，防止 goroutine 泄漏。
type EventBus struct {
	mu          sync.RWMutex
	subscribers map[string]*Subscription
	closed      bool
}

// Subscription 订阅句柄
type Subscription struct {
	ID         string
	Ch         chan *Event
	SessionID  string // 空表示不限制
	Category   string // 空表示不限制
	Levels     map[string]struct{}
	dropCount  atomic.Int64
}

// NewEventBus 创建总线
func NewEventBus() *EventBus {
	return &EventBus{subscribers: make(map[string]*Subscription)}
}

// Subscribe 注册订阅者；返回 Subscription，调用方负责后续 Unsubscribe。
//   - bufferSize：单订阅者 channel 容量，建议 64~256；
//   - sessionFilter / categoryFilter：空字符串=不限；
//   - levelFilter：[]string{"warn","critical"} 这类，nil/空表示全收。
func (b *EventBus) Subscribe(id string, bufferSize int, sessionFilter, categoryFilter string, levelFilter []string) *Subscription {
	if bufferSize <= 0 {
		bufferSize = 128
	}
	sub := &Subscription{
		ID:        id,
		Ch:        make(chan *Event, bufferSize),
		SessionID: sessionFilter,
		Category:  categoryFilter,
	}
	if len(levelFilter) > 0 {
		sub.Levels = make(map[string]struct{}, len(levelFilter))
		for _, l := range levelFilter {
			sub.Levels[l] = struct{}{}
		}
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		close(sub.Ch)
		return sub
	}
	b.subscribers[id] = sub
	return sub
}

// Unsubscribe 注销订阅者并关闭 channel
func (b *EventBus) Unsubscribe(id string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if sub, ok := b.subscribers[id]; ok {
		delete(b.subscribers, id)
		close(sub.Ch)
	}
}

// Publish 广播事件给所有订阅者；非阻塞，channel 满时静默丢弃
func (b *EventBus) Publish(e *Event) {
	if e == nil {
		return
	}
	b.mu.RLock()
	subs := make([]*Subscription, 0, len(b.subscribers))
	for _, s := range b.subscribers {
		if s.matches(e) {
			subs = append(subs, s)
		}
	}
	closed := b.closed
	b.mu.RUnlock()
	if closed {
		return
	}
	for _, s := range subs {
		select {
		case s.Ch <- e:
		default:
			s.dropCount.Add(1)
		}
	}
}

// Close 关闭总线，停止所有订阅
func (b *EventBus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.closed = true
	for id, s := range b.subscribers {
		close(s.Ch)
		delete(b.subscribers, id)
	}
}

func (s *Subscription) matches(e *Event) bool {
	if s.SessionID != "" && e.SessionID != s.SessionID {
		return false
	}
	if s.Category != "" && e.Category != s.Category {
		return false
	}
	if len(s.Levels) > 0 {
		if _, ok := s.Levels[e.Level]; !ok {
			return false
		}
	}
	return true
}
