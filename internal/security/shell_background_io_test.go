package security

import (
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
	out := PrepareShellCommandForExecute("java -jar x & echo hi")
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
