package multiagent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"cyberstrike-ai/internal/einomcp"
	"cyberstrike-ai/internal/security"

	"github.com/cloudwego/eino/adk/filesystem"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// prependPythonUnbufferedEnv 为 /bin/sh -c 注入 PYTHONUNBUFFERED=1。
// eino-ext local 对流式 stdout 使用 bufio 按「行」推送；python3 写管道时默认块缓冲，print 长期留在用户态缓冲，
// 管道里收不到换行，表现为长时间无输出直至超时或退出。若命令里已出现 PYTHONUNBUFFERED 则不再覆盖。
func prependPythonUnbufferedEnv(shellCommand string) string {
	if strings.TrimSpace(shellCommand) == "" {
		return shellCommand
	}
	if strings.Contains(strings.ToUpper(shellCommand), "PYTHONUNBUFFERED") {
		return shellCommand
	}
	return "export PYTHONUNBUFFERED=1\n" + shellCommand
}

// einoExecuteTimeoutUserHint 与写入 ADK 工具消息（模型可见）及 SSE tool_result 尾标一致。
func einoExecuteTimeoutUserHint() string {
	return "已超时终止 · Timed out"
}

// einoStreamingShellWrap 包装 Eino filesystem 使用的 StreamingShell（cloudwego eino-ext local.Local）。
// 官方 execute 工具默认走 ExecuteStreaming 且不设 RunInBackendGround；末尾带 & 时子进程仍与管道相连，
// streamStdout 按行读取会在无换行输出时长时间阻塞（与 MCP 工具 exec 的独立实现不同）。
// 对「完全后台」命令自动开启 RunInBackendGround，与 local.runCmdInBackground 行为对齐。
//
// 使用 Pipe 将内层流转发给调用方：在 inner EOF 后、关闭 Pipe 前同步调用 ToolInvokeNotify.Fire，
// 保证 run loop 在模型开始下一轮输出前已记录 execute 结果（用于 UI 与「重复助手复述」去重）。
//
// 若 inner 在校验阶段直接返回 error（未建立 reader），不会进入下方 goroutine，也必须 Fire；
// 否则 pending tool_call 要等整轮 run 结束才被 force-close，与已展示的助手/工具软错误文案不同步。
type einoStreamingShellWrap struct {
	inner         filesystem.StreamingShell
	invokeNotify  *einomcp.ToolInvokeNotifyHolder
	einoAgentName string
	// outputChunk 可选；非 nil 时在收到内层 ExecuteResponse 片段时推送，与 MCP 工具的 tool_result_delta 一致（需有效 toolCallId）。
	outputChunk func(toolName, toolCallID, chunk string)
	// toolTimeoutMinutes 与 agent.tool_timeout_minutes 对齐；>0 时对单次 execute 套用 context 超时（与 MCP 工具经 executeToolViaMCP 行为一致）。0 表示仅依赖上层 ctx（如整任务 10h 上限）。
	toolTimeoutMinutes int
	// recordMonitor 在 execute 流结束后写入 tool_executions 并 recorder(executionId)，使「渗透测试详情」与常规 MCP 一致。
	recordMonitor func(toolCallID, command, stdout string, success bool, invokeErr error)
}

func (w *einoStreamingShellWrap) ExecuteStreaming(ctx context.Context, input *filesystem.ExecuteRequest) (*schema.StreamReader[*filesystem.ExecuteResponse], error) {
	if input == nil {
		if w.inner == nil {
			return nil, fmt.Errorf("einoStreamingShellWrap: inner shell is nil")
		}
		return w.inner.ExecuteStreaming(ctx, nil)
	}
	req := *input
	userCmd := strings.TrimSpace(req.Command)
	if security.IsBackgroundShellCommand(req.Command) && !req.RunInBackendGround {
		req.RunInBackendGround = true
	}
	req.Command = prependPythonUnbufferedEnv(req.Command)
	tid := strings.TrimSpace(compose.GetToolCallID(ctx))
	agentTag := strings.TrimSpace(w.einoAgentName)

	execCtx := ctx
	var execCancel context.CancelFunc
	if w.toolTimeoutMinutes > 0 {
		execCtx, execCancel = context.WithTimeout(ctx, time.Duration(w.toolTimeoutMinutes)*time.Minute)
	}

	sr, err := executeManagedStreamingShell(execCtx, &req)
	if err != nil {
		if execCancel != nil {
			execCancel()
		}
		if w.recordMonitor != nil {
			w.recordMonitor(tid, userCmd, "", false, err)
		}
		if w.invokeNotify != nil && tid != "" {
			w.invokeNotify.Fire(tid, "execute", agentTag, false, "", err)
		}
		return nil, err
	}
	if sr == nil || w.invokeNotify == nil || tid == "" {
		if execCancel != nil {
			execCancel()
		}
		return sr, nil
	}

	outR, outW := schema.Pipe[*filesystem.ExecuteResponse](32)

	go func(inner *schema.StreamReader[*filesystem.ExecuteResponse], command string, cancel context.CancelFunc, tctx context.Context) {
		defer inner.Close()
		if cancel != nil {
			defer cancel()
		}

		var sb strings.Builder
		success := true
		var invokeErr error
		exitCode := 0
		hasExitCode := false

		for {
			resp, rerr := inner.Recv()
			if errors.Is(rerr, io.EOF) {
				break
			}
			if rerr != nil {
				success = false
				invokeErr = rerr
				_ = outW.Send(nil, rerr)
				break
			}
			if resp != nil {
				if resp.ExitCode != nil {
					hasExitCode = true
					exitCode = *resp.ExitCode
				}
				var appended string
				if resp.Output != "" {
					sb.WriteString(resp.Output)
					appended = resp.Output
				}
				if w.outputChunk != nil && strings.TrimSpace(appended) != "" {
					w.outputChunk("execute", tid, appended)
				}
				if outW.Send(resp, nil) {
					success = false
					invokeErr = fmt.Errorf("execute stream closed by consumer")
					break
				}
			}
		}

		if success && hasExitCode && exitCode != 0 {
			success = false
			invokeErr = fmt.Errorf("execute exited with code %d", exitCode)
		}
		// WithTimeout 触发后，子进程常被信号结束，local 侧多报 exit -1 / canceled，错误链里不一定带 DeadlineExceeded。
		// 用执行所用 ctx 归一化，便于 UI 展示「超时」而非含糊的 -1。
		if tctx != nil && errors.Is(tctx.Err(), context.DeadlineExceeded) {
			success = false
			invokeErr = context.DeadlineExceeded
		}
		// ADK 从本 Pipe 拼出 tool 消息正文；仅 Notify 尾标不会进入模型上下文。超时句写入流，与 UI 一致。
		if invokeErr != nil && errors.Is(invokeErr, context.DeadlineExceeded) {
			hint := "\n\n" + einoExecuteTimeoutUserHint() + "\n"
			_ = outW.Send(&filesystem.ExecuteResponse{Output: hint}, nil)
			if w.outputChunk != nil && tid != "" {
				w.outputChunk("execute", tid, hint)
			}
			sb.WriteString(hint)
		}
		if w.recordMonitor != nil {
			w.recordMonitor(tid, command, sb.String(), success, invokeErr)
		}
		w.invokeNotify.Fire(tid, "execute", agentTag, success, sb.String(), invokeErr)
		outW.Close()
	}(sr, userCmd, execCancel, execCtx)

	return outR, nil
}

func executeManagedStreamingShell(ctx context.Context, input *filesystem.ExecuteRequest) (*schema.StreamReader[*filesystem.ExecuteResponse], error) {
	if input == nil || strings.TrimSpace(input.Command) == "" {
		return nil, fmt.Errorf("command is required")
	}

	cmd := managedShellCommand(ctx, input.Command)
	if err := security.PrepareShellCmdSession(cmd); err != nil {
		return nil, err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		_ = stdout.Close()
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	sr, w := schema.Pipe[*filesystem.ExecuteResponse](100)
	if err := cmd.Start(); err != nil {
		_ = stdout.Close()
		_ = stderr.Close()
		go sendManagedShellError(w, fmt.Errorf("failed to start command: %w", err))
		return sr, nil
	}

	if input.RunInBackendGround {
		runManagedShellInBackground(ctx, cmd, stdout, stderr, w)
		return sr, nil
	}

	go streamManagedShellOutput(ctx, cmd, stdout, stderr, w)
	return sr, nil
}

func managedShellCommand(ctx context.Context, command string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.CommandContext(ctx, "cmd", "/C", command)
	}
	return exec.CommandContext(ctx, "/bin/sh", "-c", command)
}

func sendManagedShellError(w *schema.StreamWriter[*filesystem.ExecuteResponse], err error) {
	defer w.Close()
	w.Send(nil, err)
}

func runManagedShellInBackground(ctx context.Context, cmd *exec.Cmd, stdout, stderr io.ReadCloser, w *schema.StreamWriter[*filesystem.ExecuteResponse]) {
	go func() {
		defer func() {
			_ = stdout.Close()
			_ = stderr.Close()
		}()

		done := make(chan struct{})
		go func() {
			drainReaders(stdout, stderr)
			_ = cmd.Wait()
			close(done)
		}()

		select {
		case <-ctx.Done():
			security.TerminateCmdTree(cmd)
		case <-done:
		}
	}()

	go func() {
		defer w.Close()
		exitCode := 0
		w.Send(&filesystem.ExecuteResponse{Output: "command started in background\n", ExitCode: &exitCode}, nil)
	}()
}

func streamManagedShellOutput(ctx context.Context, cmd *exec.Cmd, stdout, stderr io.ReadCloser, w *schema.StreamWriter[*filesystem.ExecuteResponse]) {
	defer w.Close()
	defer func() {
		_ = stdout.Close()
		_ = stderr.Close()
	}()

	stopWatch := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			security.TerminateCmdTree(cmd)
		case <-stopWatch:
		}
	}()
	defer close(stopWatch)

	chunks := make(chan string, 64)
	var wg sync.WaitGroup
	readFn := func(r io.Reader) {
		defer wg.Done()
		buf := make([]byte, 8192)
		for {
			n, readErr := r.Read(buf)
			if n > 0 {
				chunks <- string(buf[:n])
			}
			if readErr != nil {
				return
			}
		}
	}
	wg.Add(2)
	go readFn(stdout)
	go readFn(stderr)

	go func() {
		wg.Wait()
		close(chunks)
	}()

	hasOutput := false
	for chunk := range chunks {
		hasOutput = true
		if w.Send(&filesystem.ExecuteResponse{Output: chunk}, nil) {
			security.TerminateCmdTree(cmd)
			return
		}
	}

	waitErr := cmd.Wait()
	if ctxErr := ctx.Err(); ctxErr != nil {
		w.Send(nil, ctxErr)
		return
	}
	if waitErr != nil {
		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) {
			exitCode := exitErr.ExitCode()
			w.Send(&filesystem.ExecuteResponse{
				Output:   fmt.Sprintf("command exited with non-zero code %d", exitCode),
				ExitCode: &exitCode,
			}, nil)
			return
		}
		w.Send(nil, fmt.Errorf("command failed: %w", waitErr))
		return
	}
	if !hasOutput {
		exitCode := 0
		w.Send(&filesystem.ExecuteResponse{ExitCode: &exitCode}, nil)
	}
}

func drainReaders(readers ...io.Reader) {
	var wg sync.WaitGroup
	for _, r := range readers {
		if r == nil {
			continue
		}
		wg.Add(1)
		go func(reader io.Reader) {
			defer wg.Done()
			_, _ = io.Copy(io.Discard, reader)
		}(r)
	}
	wg.Wait()
}
