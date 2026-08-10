package security

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestPrependNonInteractiveShellExports(t *testing.T) {
	out := PrependNonInteractiveShellExports("echo hi")
	if !strings.Contains(out, "GIT_PAGER=cat") || !strings.Contains(out, "PAGER=cat") {
		t.Fatalf("missing pager exports: %q", out)
	}
	if !strings.HasSuffix(strings.TrimSpace(out), "echo hi") {
		t.Fatalf("command suffix lost: %q", out)
	}
	skip := PrependNonInteractiveShellExports("GIT_PAGER=less echo hi")
	if strings.Contains(skip, "export GIT_PAGER=cat") {
		t.Fatalf("should not override existing GIT_PAGER: %q", skip)
	}
}

func TestPrependNonInteractiveStdinRedirect(t *testing.T) {
	out := PrependNonInteractiveStdinRedirect("echo hi")
	if !strings.HasPrefix(out, "exec </dev/null") {
		t.Fatalf("missing stdin redirect: %q", out)
	}
	if !strings.HasSuffix(strings.TrimSpace(out), "echo hi") {
		t.Fatalf("command suffix lost: %q", out)
	}
	skip := PrependNonInteractiveStdinRedirect("cmd </dev/null")
	if strings.HasPrefix(skip, "exec </dev/null") {
		t.Fatalf("should not double redirect: %q", skip)
	}
}

func TestPrepareNonInteractiveShellCommand(t *testing.T) {
	out := PrepareNonInteractiveShellCommand("echo hi")
	if !strings.Contains(out, "exec </dev/null") {
		t.Fatalf("missing stdin redirect: %q", out)
	}
	if !strings.Contains(out, "GIT_PAGER=cat") {
		t.Fatalf("missing pager export: %q", out)
	}
}

func TestNewShellInactivityWatch(t *testing.T) {
	w := NewShellInactivityWatch(1)
	if w == nil {
		t.Fatal("expected watch")
	}
	w.Bump()
	select {
	case <-w.Expired:
	case <-time.After(3 * time.Second):
		t.Fatal("expected inactivity fire within 3s")
	}
}

func TestResolveShellNoOutputTimeoutSeconds(t *testing.T) {
	if ResolveShellNoOutputTimeoutSeconds(0) != 300 {
		t.Fatal("zero should default to 300")
	}
	if ResolveShellNoOutputTimeoutSeconds(-1) != 0 {
		t.Fatal("-1 should disable")
	}
	if ResolveShellNoOutputTimeoutSeconds(30) != 30 {
		t.Fatal("explicit value")
	}
}

// TestNonInteractiveStdinReadExitsQuickly 验证 exec </dev/null + attachNonInteractiveStdin 时 read 立即 EOF，不挂起。
func TestNonInteractiveStdinReadExitsQuickly(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping shell integration in -short")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Unix 走 sh + exec </dev/null 包装；Windows 无 sh，靠 Go 层 attachNonInteractiveStdin（NUL）关闭 stdin，
	// PowerShell [Console]::In.ReadLine() 读到 EOF 立即返回 $null，与 Unix read 语义一致。
	shell, arg, cmdStr := shellExecPair(
		PrepareNonInteractiveShellCommand(`read x; echo "x=<$x>"`),
		`$x = [Console]::In.ReadLine(); Write-Output "x=<$x>"`,
	)
	cmd := exec.CommandContext(ctx, shell, arg, cmdStr)
	attachNonInteractiveStdin(cmd)

	start := time.Now()
	out, err := cmd.CombinedOutput()
	elapsed := time.Since(start)
	limit := 2 * time.Second
	if runtime.GOOS == "windows" {
		// PowerShell 5.1 句柄重定向下启动约 1.8s（满载更慢），与 Unix 的亚秒级基准不同；
		// 若 stdin 未关闭，ReadLine 会阻塞到 ctx 超时（5s），测试仍能捕获该回归
		limit = 4 * time.Second
	}
	if elapsed > limit {
		t.Fatalf("read with closed stdin took %v, want <%v", elapsed, limit)
	}
	if err != nil {
		t.Fatalf("unexpected error: %v output=%q", err, out)
	}
	if !strings.Contains(string(out), "x=<>") {
		t.Fatalf("unexpected output: %q", out)
	}
}

// TestNonInteractiveStdinReadBlocksWithoutRedirect 对照：stdin 为永不写入的管道时 read 会挂起。
func TestNonInteractiveStdinReadBlocksWithoutRedirect(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping shell integration in -short")
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	// 保持 w 打开且不写数据，模拟「等待用户输入」

	shell, arg, cmdStr := shellExecPair(
		`read x; echo done`,
		`[Console]::In.ReadLine() | Out-Null; Write-Output done`,
	)
	cmd := exec.Command(shell, arg, cmdStr)
	cmd.Stdin = r

	done := make(chan error, 1)
	go func() { done <- cmd.Run() }()

	select {
	case err := <-done:
		t.Fatalf("expected hang, but command finished: %v", err)
	case <-time.After(500 * time.Millisecond):
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = w.Close()
		<-done // 等待 goroutine 退出
	}
}
