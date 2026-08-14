package tooloutput

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBoundWithSpillWritesFullFile(t *testing.T) {
	root := t.TempDir()
	full := strings.Repeat("A", 2000) + "TAIL"
	out := BoundWithSpill(full, 512, SpillOpts{
		RootDir:        root,
		ConversationID: "conv-1",
		ExecutionID:    "exec-1",
	})
	if len(out) > 512 {
		t.Fatalf("bounded output exceeds max: %d", len(out))
	}
	if !strings.Contains(out, "<persisted-output>") {
		t.Fatalf("expected persisted-output notice: %q", out)
	}
	path := filepath.Join(root, "conversations", "conv-1", "trunc", "exec-1")
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, abs) {
		t.Fatalf("expected absolute path %q in notice: %q", abs, out)
	}
	got, err := os.ReadFile(abs)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != full {
		t.Fatalf("spilled content mismatch: got %d want %d", len(got), len(full))
	}
}

func TestTeeThenFormatPersistedFromFile(t *testing.T) {
	root := t.TempDir()
	tee := NewTee(SpillOpts{RootDir: root, ConversationID: "c", ExecutionID: "e"})
	full := strings.Repeat("xy", 100)
	if _, err := tee.Write([]byte(full)); err != nil {
		t.Fatal(err)
	}
	if err := tee.Close(); err != nil {
		t.Fatal(err)
	}
	notice := FormatPersistedFromFile(tee.Path(), len(full), 512)
	if len(notice) > 512 {
		t.Fatalf("notice too long: %d", len(notice))
	}
	if !strings.Contains(notice, tee.Path()) {
		t.Fatalf("missing path in notice: %q", notice)
	}
}
