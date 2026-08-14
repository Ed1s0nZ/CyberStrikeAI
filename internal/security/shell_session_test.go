package security

import (
	"context"
	"os/exec"
	"runtime"
	"testing"
	"time"
)

func TestShellSession_TerminateUsesCachedRootPID(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix process group kill")
	}

	cmd := exec.Command("sh", "-c", "sleep 300")
	ConfigureShellCmdForAgentExecute(cmd)

	session, err := StartShellSession(cmd)
	if err != nil {
		t.Fatalf("StartShellSession: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	session.Terminate()

	done := make(chan error, 1)
	go func() { done <- session.Wait() }()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("session did not finish within 5s after Terminate")
	}
}

func TestShellSession_TerminateAfterContextCancel(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix process group kill")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", "sleep 300")
	ConfigureShellCmdForAgentExecute(cmd)

	session, err := StartShellSession(cmd)
	if err != nil {
		t.Fatalf("StartShellSession: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	cancel()
	TerminateShellCmdSession(session)

	done := make(chan error, 1)
	go func() { done <- session.Wait() }()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("session did not finish within 5s after cancel+terminate")
	}
}
