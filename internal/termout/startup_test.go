package termout

import (
	"strings"
	"testing"
)

func TestDisplayWidthEmoji(t *testing.T) {
	if got := displayWidth("🚀"); got != 2 {
		t.Fatalf("displayWidth(emoji) = %d, want 2", got)
	}
	if got := displayWidth("ab"); got != 2 {
		t.Fatalf("displayWidth(ab) = %d, want 2", got)
	}
}

func TestDisplayWidthIgnoresANSI(t *testing.T) {
	s := New(nil)
	colored := s.Bold("admin")
	if got := displayWidth(colored); got != 5 {
		t.Fatalf("displayWidth colored = %d, want 5", got)
	}
}

func TestPadRightDisplay(t *testing.T) {
	got := padRightDisplay("pwd", 10)
	if displayWidth(got) != 10 {
		t.Fatalf("padded width = %d, want 10", displayWidth(got))
	}
}

func TestColorDisabledWithoutTTY(t *testing.T) {
	s := New(nil)
	if s.enabled {
		t.Fatal("expected colors disabled for nil writer")
	}
	if got := s.Cyan("x"); got != "x" {
		t.Fatalf("Cyan without TTY = %q, want plain text", got)
	}
}

func TestPrintBootstrapAdminCredentialsEmpty(t *testing.T) {
	PrintBootstrapAdminCredentials("   ")
}

func TestPrintStartupWebUIOptions(t *testing.T) {
	PrintStartupWebUI(StartupWebUIOptions{
		Scheme:       "https",
		Port:         8080,
		SelfSigned:   true,
		HTTPRedirect: true,
	})
}

func TestBoxRowAlignedWidth(t *testing.T) {
	s := New(nil)
	rows := []string{
		s.Bold("CyberStrikeAI") + s.White(" is ready"),
		s.Dim("Web UI   ") + s.Bold("https://127.0.0.1:8080/"),
	}
	inner := maxDisplayWidth(rows...)
	for _, row := range rows {
		line := s.boxRow(inner, row)
		if !strings.Contains(line, "│") {
			t.Fatalf("box row missing border: %q", line)
		}
	}
}

func TestMaxDisplayWidth(t *testing.T) {
	short := "abc"
	long := "https://127.0.0.1:8080/"
	if got := maxDisplayWidth(short, long); got != displayWidth(long) {
		t.Fatalf("maxDisplayWidth = %d, want %d", got, displayWidth(long))
	}
}
