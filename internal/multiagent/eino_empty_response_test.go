package multiagent

import "testing"

func TestIsEinoEmptyResponseResult(t *testing.T) {
	empty := &RunResult{
		Response: "(Eino ADK single-agent session completed but no assistant text was captured. Check process details or logs.) " +
			"（Eino ADK 单代理会话已完成，但未捕获到助手文本输出。请查看过程详情或日志。）",
	}
	if !IsEinoEmptyResponseResult(empty) {
		t.Fatal("expected empty placeholder response")
	}
	ok := &RunResult{Response: "扫描完成，发现 2 个开放端口。"}
	if IsEinoEmptyResponseResult(ok) {
		t.Fatalf("expected real response, got placeholder match")
	}
	if IsEinoEmptyResponseResult(nil) {
		t.Fatal("nil result should be false")
	}
}

func TestHasEinoResumeTrace(t *testing.T) {
	if HasEinoResumeTrace(nil) {
		t.Fatal("nil")
	}
	if HasEinoResumeTrace(&RunResult{LastAgentTraceInput: "[]"}) {
		t.Fatal("enable resume on empty trace")
	}
	if !HasEinoResumeTrace(&RunResult{LastAgentTraceInput: `[{"role":"user","content":"hi"}]`}) {
		t.Fatal("expected resume trace")
	}
}

func TestEmptyResponseContinueMaxAttemptsFromConfig(t *testing.T) {
	if got := EmptyResponseContinueMaxAttemptsFromConfig(nil); got != defaultEmptyResponseContinueMaxAttempts {
		t.Fatalf("default: got %d want %d", got, defaultEmptyResponseContinueMaxAttempts)
	}
}
