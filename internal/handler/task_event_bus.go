package handler

import "sync"

// TaskEventBus 将主 SSE 连接上的事件镜像给后订阅的客户端（例如刷新页面后、HITL 审批通过需继续收事件）。
// 每个 payload 为完整 SSE 行： "data: {...}\n\n"
type TaskEventBus struct {
	mu   sync.RWMutex
	subs map[string]map[*taskEventSub]struct{}
}

type taskEventSub struct {
	mu     sync.Mutex
	ch     chan []byte
	closed bool
}

func (s *taskEventSub) sendNonBlocking(line []byte) bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return false
	}
	select {
	case s.ch <- line:
		return true
	default:
		return false
	}
}

func (s *taskEventSub) closeOnce() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	close(s.ch)
}

func NewTaskEventBus() *TaskEventBus {
	return &TaskEventBus{
		subs: make(map[string]map[*taskEventSub]struct{}),
	}
}

// Subscribe 注册订阅；cancel 时需调用 Unsubscribe。
func (b *TaskEventBus) Subscribe(conversationID string) (sub *taskEventSub, ch <-chan []byte) {
	chBuf := make(chan []byte, 256)
	sub = &taskEventSub{ch: chBuf}
	b.mu.Lock()
	if b.subs[conversationID] == nil {
		b.subs[conversationID] = make(map[*taskEventSub]struct{})
	}
	b.subs[conversationID][sub] = struct{}{}
	b.mu.Unlock()
	return sub, chBuf
}

func (b *TaskEventBus) Unsubscribe(conversationID string, sub *taskEventSub) {
	if sub == nil {
		return
	}
	b.mu.Lock()
	m, ok := b.subs[conversationID]
	if !ok {
		b.mu.Unlock()
		return
	}
	delete(m, sub)
	if len(m) == 0 {
		delete(b.subs, conversationID)
	}
	b.mu.Unlock()
	sub.closeOnce()
}

// Publish 非阻塞投递；慢消费者丢帧（HITL 场景以最新状态为准，丢帧可接受）。
func (b *TaskEventBus) Publish(conversationID string, line []byte) {
	if b == nil || conversationID == "" || len(line) == 0 {
		return
	}
	b.mu.RLock()
	m := b.subs[conversationID]
	subs := make([]*taskEventSub, 0, len(m))
	for s := range m {
		subs = append(subs, s)
	}
	b.mu.RUnlock()

	cp := append([]byte(nil), line...)
	for _, s := range subs {
		s.sendNonBlocking(cp)
	}
}

// CloseConversation 任务结束时关闭该会话所有订阅 channel。
func (b *TaskEventBus) CloseConversation(conversationID string) {
	if b == nil || conversationID == "" {
		return
	}
	b.mu.Lock()
	m := b.subs[conversationID]
	delete(b.subs, conversationID)
	b.mu.Unlock()
	for sub := range m {
		sub.closeOnce()
	}
}
