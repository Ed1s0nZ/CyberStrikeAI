package multiagent

import (
	"os"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestNewEinoCheckpointRuntimeDisabledWithoutDir(t *testing.T) {
	if got := newEinoCheckpointRuntime(" ", "conv-1", "deep", nil); got != nil {
		t.Fatalf("runtime = %#v, want nil", got)
	}
}

func TestNewEinoCheckpointRuntimeCreatesStore(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	runtime := newEinoCheckpointRuntime(t.TempDir(), "conv/1", "deep", zap.New(core))
	if runtime == nil || runtime.Store == nil {
		t.Fatal("expected checkpoint runtime with store")
	}
	if runtime.CheckPointID != buildEinoCheckpointID("deep") {
		t.Fatalf("checkpoint id = %q", runtime.CheckPointID)
	}
	if !strings.Contains(runtime.Store.dir, sanitizeEinoPathSegment("conv/1")) {
		t.Fatalf("store dir = %q, want sanitized conversation segment", runtime.Store.dir)
	}
	if logs.FilterMessage("eino runner: checkpoint store enabled").Len() != 1 {
		t.Fatalf("expected enabled log, got %d", logs.Len())
	}
}

func TestNewEinoCheckpointRuntimeLogsCreateFailure(t *testing.T) {
	filePath := t.TempDir() + "/not-a-dir"
	if err := os.WriteFile(filePath, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	core, logs := observer.New(zap.WarnLevel)
	runtime := newEinoCheckpointRuntime(filePath, "conv-1", "deep", zap.New(core))
	if runtime != nil {
		t.Fatalf("runtime = %#v, want nil", runtime)
	}
	if logs.FilterMessage("eino checkpoint store disabled").Len() != 1 {
		t.Fatalf("expected disabled log, got %d", logs.Len())
	}
}
