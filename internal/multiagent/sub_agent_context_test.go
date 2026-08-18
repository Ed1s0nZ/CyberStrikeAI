package multiagent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"cyberstrike-ai/internal/agent"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
)

// --- buildUserContextSupplement tests ---

func TestBuildUserContextSupplement_SingleMessage(t *testing.T) {
	result := buildUserContextSupplement("http://8.163.32.73:8081 测试命令执行", nil, 0)
	if result == "" {
		t.Fatal("expected non-empty supplement")
	}
	if !strings.Contains(result, "http://8.163.32.73:8081") {
		t.Error("expected URL in supplement")
	}
}

func TestBuildUserContextSupplement_MultiTurn(t *testing.T) {
	history := []agent.ChatMessage{
		{Role: "user", Content: "http://8.163.32.73:8081 这是一个pikachu靶场，尝试测试命令执行"},
		{Role: "assistant", Content: "好的，我来测试..."},
		{Role: "user", Content: "继续，并持久化webshell"},
		{Role: "assistant", Content: "正在处理..."},
	}
	result := buildUserContextSupplement("你好", history, 0)
	if !strings.Contains(result, "http://8.163.32.73:8081") {
		t.Error("expected first turn URL to be preserved")
	}
	if !strings.Contains(result, "你好") {
		t.Error("expected current message")
	}
}

func TestBuildUserContextSupplement_Empty(t *testing.T) {
	if result := buildUserContextSupplement("", nil, 0); result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestBuildUserContextSupplement_Deduplicate(t *testing.T) {
	history := []agent.ChatMessage{{Role: "user", Content: "你好"}}
	result := buildUserContextSupplement("你好", history, 0)
	if strings.Count(result, "你好") != 1 {
		t.Errorf("expected '你好' once, got: %s", result)
	}
}

func TestBuildUserContextSupplement_SkipsNonUser(t *testing.T) {
	history := []agent.ChatMessage{
		{Role: "user", Content: "目标是 10.0.0.1"},
		{Role: "assistant", Content: "不应该出现"},
	}
	result := buildUserContextSupplement("确认", history, 0)
	if strings.Contains(result, "不应该出现") {
		t.Error("assistant message should not be included")
	}
}

func TestBuildUserContextSupplement_DisabledByNegative(t *testing.T) {
	if result := buildUserContextSupplement("test", nil, -1); result != "" {
		t.Errorf("expected empty when disabled, got %q", result)
	}
}

func TestBuildUserContextSupplement_CustomMaxRunes(t *testing.T) {
	msg := strings.Repeat("A", 200)
	result := buildUserContextSupplement(msg, nil, 50)
	header := userContextSupplementHeader
	body := strings.TrimPrefix(result, header)
	if len([]rune(body)) > 50 {
		t.Errorf("body should be capped at 50 runes, got %d", len([]rune(body)))
	}
}

func TestBuildUserContextSupplement_TruncateKeepsFirstAndLast(t *testing.T) {
	first := "http://target.com " + strings.Repeat("A", 500)
	var history []agent.ChatMessage
	history = append(history, agent.ChatMessage{Role: "user", Content: first})
	for i := 0; i < 10; i++ {
		history = append(history, agent.ChatMessage{Role: "user", Content: strings.Repeat("B", 500)})
	}
	last := "最后一条指令"
	result := buildUserContextSupplement(last, history, 800)
	if !strings.Contains(result, "http://target.com") {
		t.Error("first message (target URL) should survive truncation")
	}
	if !strings.Contains(result, last) {
		t.Error("last message should survive truncation")
	}
}

// --- middleware integration tests ---

func TestTaskContextEnrichMiddleware_EnrichesTaskDescription(t *testing.T) {
	mw := newTaskContextEnrichMiddleware(
		"继续测试",
		[]agent.ChatMessage{{Role: "user", Content: "http://8.163.32.73:8081 pikachu靶场"}},
		0,
		"",
	)
	if mw == nil {
		t.Fatal("expected non-nil middleware")
	}

	called := false
	var capturedArgs string
	fakeEndpoint := func(ctx context.Context, args string, opts ...tool.Option) (string, error) {
		called = true
		capturedArgs = args
		return "ok", nil
	}

	wrapped, err := mw.(interface {
		WrapInvokableToolCall(context.Context, adk.InvokableToolCallEndpoint, *adk.ToolContext) (adk.InvokableToolCallEndpoint, error)
	}).WrapInvokableToolCall(context.Background(), fakeEndpoint, &adk.ToolContext{Name: "task"})
	if err != nil {
		t.Fatal(err)
	}

	taskArgs := `{"subagent_type":"recon","description":"扫描目标端口"}`
	wrapped(context.Background(), taskArgs)

	if !called {
		t.Fatal("endpoint was not called")
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(capturedArgs), &parsed); err != nil {
		t.Fatalf("enriched args not valid JSON: %v", err)
	}
	desc := parsed["description"].(string)
	if !strings.Contains(desc, "扫描目标端口") {
		t.Error("original description should be preserved")
	}
	if !strings.Contains(desc, "http://8.163.32.73:8081") {
		t.Error("user context should be appended to description")
	}
	if !strings.Contains(desc, "继续测试") {
		t.Error("current user message should be in description")
	}
}

func TestTaskContextEnrichMiddleware_IgnoresNonTaskTools(t *testing.T) {
	mw := newTaskContextEnrichMiddleware("test", nil, 0, "")
	if mw == nil {
		t.Fatal("expected non-nil middleware")
	}

	original := `{"command":"nmap -sV target"}`
	var capturedArgs string
	fakeEndpoint := func(ctx context.Context, args string, opts ...tool.Option) (string, error) {
		capturedArgs = args
		return "ok", nil
	}

	wrapped, err := mw.(interface {
		WrapInvokableToolCall(context.Context, adk.InvokableToolCallEndpoint, *adk.ToolContext) (adk.InvokableToolCallEndpoint, error)
	}).WrapInvokableToolCall(context.Background(), fakeEndpoint, &adk.ToolContext{Name: "nmap_scan"})
	if err != nil {
		t.Fatal(err)
	}

	wrapped(context.Background(), original)
	if capturedArgs != original {
		t.Errorf("non-task tool args should not be modified, got %q", capturedArgs)
	}
}

func TestTaskContextEnrichMiddleware_NilWhenDisabled(t *testing.T) {
	mw := newTaskContextEnrichMiddleware("test", nil, -1, "")
	if mw != nil {
		t.Error("middleware should be nil when disabled")
	}
}
