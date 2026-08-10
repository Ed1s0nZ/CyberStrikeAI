package security

import (
	"runtime"
	"strings"
	"testing"
)

func TestRedirectBackgroundJobStdio_mixedCommand(t *testing.T) {
	in := "java -jar app.jar & JRMP_PID=$!; echo started"
	out := RedirectBackgroundJobStdio(in)
	if !strings.Contains(out, "java -jar app.jar </dev/null >/dev/null 2>&1 &") {
		t.Fatalf("expected redirect before &: %q", out)
	}
	if !strings.Contains(out, "echo started") {
		t.Fatalf("foreground tail preserved: %q", out)
	}
}

func TestRedirectBackgroundJobStdio_trailingOnly(t *testing.T) {
	in := "sleep 120 &"
	out := RedirectBackgroundJobStdio(in)
	want := "sleep 120 </dev/null >/dev/null 2>&1 &"
	if strings.TrimSpace(out) != want {
		t.Fatalf("got %q want %q", out, want)
	}
}

func TestRedirectBackgroundJobStdio_skipsAlreadyRedirected(t *testing.T) {
	in := "sleep 1 >/dev/null 2>&1 & echo ok"
	out := RedirectBackgroundJobStdio(in)
	if out != in {
		t.Fatalf("should not double-redirect: %q", out)
	}
}

func TestRedirectBackgroundJobStdio_skipsAndAnd(t *testing.T) {
	in := "test -f /etc/passwd && echo ok"
	out := RedirectBackgroundJobStdio(in)
	if out != in {
		t.Fatalf("&& must not be treated as background &: %q", out)
	}
}

func TestPrepareShellCommandForExecute(t *testing.T) {
	cmd := "java -jar x & echo hi"
	out := PrepareShellCommandForExecute(cmd)
	if runtime.GOOS == "windows" {
		// Windows 分支必须原样返回，不得再注入 bash 语法（跨平台注入修复目标）
		if out != cmd {
			t.Fatalf("windows branch must return command unchanged, got %q", out)
		}
		if strings.Contains(out, "exec </dev/null") || strings.Contains(out, "GIT_PAGER=cat") || strings.Contains(out, "export ") {
			t.Fatalf("windows branch must not inject unix syntax: %q", out)
		}
		return
	}
	if !strings.Contains(out, "exec </dev/null") {
		t.Fatalf("missing stdin redirect: %q", out)
	}
	if !strings.Contains(out, "GIT_PAGER=cat") {
		t.Fatalf("missing pager export: %q", out)
	}
	if !strings.Contains(out, "java -jar x </dev/null >/dev/null 2>&1 &") {
		t.Fatalf("missing background redirect: %q", out)
	}
}

func TestIsBackgroundShellCommand_usesSharedParser(t *testing.T) {
	if !IsBackgroundShellCommand("sleep 1 &") {
		t.Fatal("trailing & should be background")
	}
	if IsBackgroundShellCommand("sleep 1 & echo hi") {
		t.Fatal("mixed should not be fully background")
	}
}
