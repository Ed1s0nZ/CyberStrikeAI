package multiagent

import (
	"strings"
	"sync"
)

type einoExecuteStdoutSuppressor struct {
	mu      sync.Mutex
	pending string
}

func newEinoExecuteStdoutSuppressor() *einoExecuteStdoutSuppressor {
	return &einoExecuteStdoutSuppressor{}
}

func (s *einoExecuteStdoutSuppressor) Record(toolName, stdout string, isErr bool) {
	if s == nil || isErr || !strings.EqualFold(strings.TrimSpace(toolName), "execute") {
		return
	}
	t := strings.TrimSpace(stdout)
	if t == "" {
		return
	}
	s.mu.Lock()
	s.pending = t
	s.mu.Unlock()
}

func (s *einoExecuteStdoutSuppressor) Peek() string {
	if s == nil {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pending
}

func (s *einoExecuteStdoutSuppressor) Consume() string {
	if s == nil {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.pending
	s.pending = ""
	return out
}

func (s *einoExecuteStdoutSuppressor) Clear() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.pending = ""
	s.mu.Unlock()
}
