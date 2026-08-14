package multiagent

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"cyberstrike-ai/internal/einomcp"
	"cyberstrike-ai/internal/mcp"

	"github.com/cloudwego/eino/adk/filesystem"
	"github.com/cloudwego/eino/schema"
)

type mockStreamingShell struct {
	immediateErr error
	recvErr      error
	output       string
	called       bool
	lastCommand  string
}

func (m *mockStreamingShell) ExecuteStreaming(ctx context.Context, input *filesystem.ExecuteRequest) (*schema.StreamReader[*filesystem.ExecuteResponse], error) {
	m.called = true
	if input != nil {
		m.lastCommand = input.Command
	}
	if m.immediateErr != nil {
		return nil, m.immediateErr
	}
	outR, outW := schema.Pipe[*filesystem.ExecuteResponse](4)
	go func() {
		defer outW.Close()
		if strings.TrimSpace(m.output) != "" {
			_ = outW.Send(&filesystem.ExecuteResponse{Output: m.output}, nil)
		}
		if m.recvErr != nil {
			_ = outW.Send(nil, m.recvErr)
		}
	}()
	return outR, nil
}

func TestEinoStreamingShellWrap_PreparesNonInteractiveCommand(t *testing.T) {
	inner := &mockStreamingShell{output: "ok\n"}
	wrap := &einoStreamingShellWrap{inner: inner}
	sr, err := wrap.ExecuteStreaming(context.Background(), &filesystem.ExecuteRequest{Command: "echo ok"})
	if err != nil {
		t.Fatalf("ExecuteStreaming: %v", err)
	}
	defer sr.Close()
	for {
		_, rerr := sr.Recv()
		if errors.Is(rerr, io.EOF) {
			break
		}
		if rerr != nil {
			t.Fatalf("recv: %v", rerr)
		}
	}
	if !strings.Contains(inner.lastCommand, "PYTHONUNBUFFERED=1") {
		t.Fatalf("missing python unbuffer in inner command: %q", inner.lastCommand)
	}
}

func TestEinoStreamingShellWrap_NoOutputTimeout(t *testing.T) {
	inner := &mockStreamingShellHanging{}
	notify := einomcp.NewToolInvokeNotifyHolder()
	var fired string
	notify.Set(func(toolCallID, toolName, einoAgent string, success bool, content string, invokeErr error) {
		fired = content
	})
	wrap := &einoStreamingShellWrap{
		inner:                   inner,
		invokeNotify:            notify,
		shellNoOutputTimeoutSec: 1,
	}
	sr, err := wrap.ExecuteStreaming(context.Background(), &filesystem.ExecuteRequest{Command: "sudo whoami"})
	if err != nil {
		t.Fatalf("ExecuteStreaming: %v", err)
	}
	defer sr.Close()
	var got strings.Builder
	for {
		resp, rerr := sr.Recv()
		if errors.Is(rerr, io.EOF) {
			break
		}
		if rerr != nil {
			t.Fatalf("recv: %v", rerr)
		}
		if resp != nil {
			got.WriteString(resp.Output)
		}
	}
	if !inner.called {
		t.Fatal("inner shell should run (no command blacklist)")
	}
	out := got.String()
	if !strings.Contains(out, "没有新的输出") && !strings.Contains(out, "no new output") {
		t.Fatalf("expected inactivity timeout message, got: %q notify=%q", out, fired)
	}
}

type mockStreamingShellPartialThenHang struct {
	called bool
}

func (m *mockStreamingShellPartialThenHang) ExecuteStreaming(ctx context.Context, input *filesystem.ExecuteRequest) (*schema.StreamReader[*filesystem.ExecuteResponse], error) {
	m.called = true
	outR, outW := schema.Pipe[*filesystem.ExecuteResponse](4)
	go func() {
		_ = outW.Send(&filesystem.ExecuteResponse{Output: "[sudo] password:\n"}, nil)
		<-ctx.Done()
		outW.Close()
	}()
	return outR, nil
}

func TestEinoStreamingShellWrap_InactivityAfterPartialOutput(t *testing.T) {
	inner := &mockStreamingShellPartialThenHang{}
	wrap := &einoStreamingShellWrap{
		inner:                   inner,
		shellNoOutputTimeoutSec: 1,
	}
	start := time.Now()
	sr, err := wrap.ExecuteStreaming(context.Background(), &filesystem.ExecuteRequest{Command: "sudo whoami"})
	if err != nil {
		t.Fatalf("ExecuteStreaming: %v", err)
	}
	defer sr.Close()
	var got strings.Builder
	for {
		resp, rerr := sr.Recv()
		if errors.Is(rerr, io.EOF) {
			break
		}
		if rerr != nil {
			t.Fatalf("recv: %v", rerr)
		}
		if resp != nil {
			got.WriteString(resp.Output)
		}
	}
	if time.Since(start) > 5*time.Second {
		t.Fatalf("expected inactivity timeout ~1s, took %v", time.Since(start))
	}
	if !strings.Contains(got.String(), "没有新的输出") && !strings.Contains(got.String(), "no new output") {
		t.Fatalf("expected inactivity message, got: %q", got.String())
	}
}

func TestEinoStreamingShellWrap_SoftWaitTimeoutReturnsExecutionIDAndKeepsRunning(t *testing.T) {
	inner := &mockStreamingShellPartialThenHang{}
	partialCh := make(chan string, 4)
	cancelCh := make(chan context.CancelFunc, 1)
	unregistered := make(chan string, 1)
	wrap := &einoStreamingShellWrap{
		inner:                  inner,
		toolWaitTimeoutSeconds: 1,
		beginMonitor: func(toolCallID, command string) string {
			return "exec-soft-wait"
		},
		appendPartialMonitor: func(executionID, toolCallID, chunk string) {
			partialCh <- chunk
		},
		registerCancelMonitor: func(executionID string, cancel context.CancelFunc) {
			if executionID == "exec-soft-wait" {
				cancelCh <- cancel
			}
		},
		unregisterCancelMonitor: func(executionID string) {
			unregistered <- executionID
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sr, err := wrap.ExecuteStreaming(ctx, &filesystem.ExecuteRequest{Command: "sudo whoami"})
	if err != nil {
		t.Fatalf("ExecuteStreaming: %v", err)
	}
	defer sr.Close()

	var got strings.Builder
	for {
		resp, rerr := sr.Recv()
		if errors.Is(rerr, io.EOF) {
			break
		}
		if rerr != nil {
			t.Fatalf("recv: %v", rerr)
		}
		if resp != nil {
			got.WriteString(resp.Output)
		}
	}
	body := got.String()
	if !strings.Contains(body, "execution_id: exec-soft-wait") || !strings.Contains(body, "status: running") {
		t.Fatalf("expected background execution marker, got: %q", body)
	}
	select {
	case chunk := <-partialCh:
		if !strings.Contains(chunk, "[sudo] password") {
			t.Fatalf("unexpected partial chunk: %q", chunk)
		}
	default:
		t.Fatal("expected streamed partial output before soft wait return")
	}
	if !inner.called {
		t.Fatal("inner shell did not run")
	}
	select {
	case registeredCancel := <-cancelCh:
		registeredCancel()
	case <-time.After(time.Second):
		t.Fatal("expected execution cancel registration")
	}
	select {
	case id := <-unregistered:
		if id != "exec-soft-wait" {
			t.Fatalf("unexpected unregistered id: %q", id)
		}
	case <-time.After(time.Second):
		t.Fatal("expected execution cancel unregister")
	}
}

type mockStreamingShellHanging struct {
	called bool
}

func (m *mockStreamingShellHanging) ExecuteStreaming(ctx context.Context, input *filesystem.ExecuteRequest) (*schema.StreamReader[*filesystem.ExecuteResponse], error) {
	m.called = true
	outR, outW := schema.Pipe[*filesystem.ExecuteResponse](4)
	go func() {
		<-ctx.Done()
		outW.Close()
	}()
	return outR, nil
}

func TestEinoExecuteRecvErrIsToolTimeout(t *testing.T) {
	tctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	time.Sleep(2 * time.Millisecond)
	<-tctx.Done()

	if !einoExecuteRecvErrIsToolTimeout(context.Canceled, tctx) {
		t.Fatal("expected canceled recv with deadline exec ctx to count as tool timeout")
	}
	if !einoExecuteRecvErrIsToolTimeout(context.DeadlineExceeded, nil) {
		t.Fatal("expected DeadlineExceeded recv without tctx")
	}
	if einoExecuteRecvErrIsToolTimeout(errors.New("exit status 1"), context.Background()) {
		t.Fatal("unexpected timeout for generic error")
	}
}

func TestEinoStreamingShellWrap_ToolTimeoutImmediateErrIsSoft(t *testing.T) {
	inner := &mockStreamingShell{immediateErr: context.DeadlineExceeded}
	wrap := &einoStreamingShellWrap{
		inner:              inner,
		toolTimeoutMinutes: 60,
	}
	sr, err := wrap.ExecuteStreaming(context.Background(), &filesystem.ExecuteRequest{Command: "true"})
	if err != nil {
		t.Fatalf("immediate tool timeout must return soft stream, got err: %v", err)
	}
	defer sr.Close()

	var got strings.Builder
	for {
		resp, rerr := sr.Recv()
		if errors.Is(rerr, io.EOF) {
			break
		}
		if rerr != nil {
			t.Fatalf("outer stream must not hard-fail, got: %v", rerr)
		}
		if resp != nil && resp.Output != "" {
			got.WriteString(resp.Output)
		}
	}
	if !strings.Contains(got.String(), einoExecuteTimeoutUserHint()) {
		t.Fatalf("expected timeout hint, got: %q", got.String())
	}
}

func TestEinoStreamingShellWrap_ToolTimeoutRecvErrIsSoft(t *testing.T) {
	inner := &mockStreamingShell{recvErr: context.DeadlineExceeded}
	notify := einomcp.NewToolInvokeNotifyHolder()
	wrap := &einoStreamingShellWrap{
		inner:              inner,
		invokeNotify:       notify,
		toolTimeoutMinutes: 60,
	}
	// 生产路径由 Eino compose 注入 toolCallID；单测通过已过期 execCtx 识别 tool_timeout 软错误。
	tctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	time.Sleep(2 * time.Millisecond)
	<-tctx.Done()

	sr, err := wrap.ExecuteStreaming(tctx, &filesystem.ExecuteRequest{Command: "sleep 999"})
	if err != nil {
		t.Fatalf("ExecuteStreaming: %v", err)
	}
	defer sr.Close()

	var got strings.Builder
	for {
		resp, rerr := sr.Recv()
		if errors.Is(rerr, io.EOF) {
			break
		}
		if rerr != nil {
			t.Fatalf("outer stream must not hard-fail on tool timeout, got: %v", rerr)
		}
		if resp != nil && resp.Output != "" {
			got.WriteString(resp.Output)
		}
	}
	if !strings.Contains(got.String(), einoExecuteTimeoutUserHint()) {
		t.Fatalf("expected timeout hint in stream, got: %q", got.String())
	}
}

func TestEinoStreamingShellWrap_CapturesOutputWithToolTimeout(t *testing.T) {
	inner := &mockStreamingShell{output: "100\n"}
	notify := einomcp.NewToolInvokeNotifyHolder()
	var firedContent string
	notify.Set(func(toolCallID, toolName, einoAgent string, success bool, content string, invokeErr error) {
		firedContent = content
	})
	wrap := &einoStreamingShellWrap{
		inner:              inner,
		invokeNotify:       notify,
		toolTimeoutMinutes: 60,
	}
	sr, err := wrap.ExecuteStreaming(context.Background(), &filesystem.ExecuteRequest{Command: "echo 100"})
	if err != nil {
		t.Fatalf("ExecuteStreaming: %v", err)
	}
	defer sr.Close()

	var got strings.Builder
	for {
		resp, rerr := sr.Recv()
		if errors.Is(rerr, io.EOF) {
			break
		}
		if rerr != nil {
			t.Fatalf("unexpected stream error: %v", rerr)
		}
		if resp != nil && resp.Output != "" {
			got.WriteString(resp.Output)
		}
	}
	if !strings.Contains(got.String(), "100") {
		t.Fatalf("stream output = %q, want contains 100", got.String())
	}
	if !strings.Contains(firedContent, "100") {
		t.Fatalf("notify content = %q, want contains 100", firedContent)
	}
}

func TestEinoStreamingShellWrap_AbortNoteDoesNotDuplicateStreamedOutput(t *testing.T) {
	inner := &mockStreamingShell{output: "line1\nline2\n", recvErr: context.Canceled}
	notify := einomcp.NewToolInvokeNotifyHolder()
	wrap := &einoStreamingShellWrap{
		inner:        inner,
		invokeNotify: notify,
	}
	reg := &abortNoteTestRegistry{note: "改成20次"}
	ctx := mcp.WithEinoExecuteRunRegistry(
		mcp.WithMCPConversationID(context.Background(), "conv-abort-dup"),
		reg,
	)
	sr, err := wrap.ExecuteStreaming(ctx, &filesystem.ExecuteRequest{Command: "ping -c 10 baidu.com"})
	if err != nil {
		t.Fatalf("ExecuteStreaming: %v", err)
	}
	defer sr.Close()

	var got strings.Builder
	for {
		resp, rerr := sr.Recv()
		if errors.Is(rerr, io.EOF) {
			break
		}
		if rerr != nil {
			t.Fatalf("unexpected stream error: %v", rerr)
		}
		if resp != nil && resp.Output != "" {
			got.WriteString(resp.Output)
		}
	}
	out := got.String()
	if strings.Count(out, "line1") != 1 || strings.Count(out, "line2") != 1 {
		t.Fatalf("stream duplicated stdout: %q", out)
	}
	if !strings.Contains(out, "改成20次") {
		t.Fatalf("stream missing abort note: %q", out)
	}
}

type abortNoteTestRegistry struct {
	note string
}

func (r *abortNoteTestRegistry) RegisterActiveEinoExecute(string, context.CancelFunc) {}
func (r *abortNoteTestRegistry) UnregisterActiveEinoExecute(string)                   {}
func (r *abortNoteTestRegistry) AbortActiveEinoExecute(string, string) bool           { return false }
func (r *abortNoteTestRegistry) TakeEinoExecuteAbortNote(string) string               { return r.note }

func TestEinoStreamingShellWrap_NonTimeoutRecvErrStillHard(t *testing.T) {
	inner := &mockStreamingShell{recvErr: errors.New("broken pipe")}
	wrap := &einoStreamingShellWrap{inner: inner}
	sr, err := wrap.ExecuteStreaming(context.Background(), &filesystem.ExecuteRequest{Command: "true"})
	if err != nil {
		t.Fatalf("ExecuteStreaming: %v", err)
	}
	defer sr.Close()

	_, rerr := sr.Recv()
	if rerr == nil || errors.Is(rerr, io.EOF) {
		t.Fatal("expected hard stream error for non-timeout failure")
	}
}
