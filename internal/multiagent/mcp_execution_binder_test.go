package multiagent

import (
	"fmt"
	"sync"
	"testing"
)

func TestMCPExecutionBinder(t *testing.T) {
	b := NewMCPExecutionBinder()
	b.Bind("call-1", "exec-1")
	if got := b.ExecutionID("call-1"); got != "exec-1" {
		t.Fatalf("expected exec-1, got %q", got)
	}
	if got := b.ExecutionID("missing"); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

// TestMCPExecutionBinder_ConcurrentBind 回归并行 tool 回调不得 concurrent map panic。
func TestMCPExecutionBinder_ConcurrentBind(t *testing.T) {
	b := NewMCPExecutionBinder()
	const workers = 64
	var wg sync.WaitGroup
	wg.Add(workers * 2)
	for i := 0; i < workers; i++ {
		i := i
		toolCallID := fmt.Sprintf("call-%d", i)
		execID := fmt.Sprintf("exec-%d", i)
		go func() {
			defer wg.Done()
			b.Bind(toolCallID, execID)
		}()
		go func() {
			defer wg.Done()
			_ = b.ExecutionID(toolCallID)
		}()
	}
	wg.Wait()
	if got := b.ExecutionID("call-0"); got != "exec-0" {
		t.Fatalf("expected exec-0, got %q", got)
	}
}
