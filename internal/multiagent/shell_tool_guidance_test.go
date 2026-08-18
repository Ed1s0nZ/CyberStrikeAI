package multiagent

import (
	"strings"
	"testing"
)

func TestInjectShellToolGuidance(t *testing.T) {
	got := injectShellToolGuidance("base", []string{"nmap"})
	if got != "base" {
		t.Fatalf("expected unchanged, got %q", got)
	}
	got = injectShellToolGuidance("base", []string{"exec", "nmap"})
	if !strings.Contains(got, "exec/execute") || !strings.Contains(got, "base") {
		t.Fatalf("expected shell guidance appended, got %q", got)
	}
}
