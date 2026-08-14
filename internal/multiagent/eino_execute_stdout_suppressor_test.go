package multiagent

import "testing"

func TestEinoExecuteStdoutSuppressorRecordsOnlySuccessfulExecute(t *testing.T) {
	s := newEinoExecuteStdoutSuppressor()
	s.Record("read_file", "file body", false)
	if got := s.Peek(); got != "" {
		t.Fatalf("non-execute should not be recorded, got %q", got)
	}
	s.Record("execute", "failed", true)
	if got := s.Peek(); got != "" {
		t.Fatalf("failed execute should not be recorded, got %q", got)
	}
	s.Record(" execute ", "  hello\n", false)
	if got := s.Peek(); got != "hello" {
		t.Fatalf("Peek = %q, want hello", got)
	}
}

func TestEinoExecuteStdoutSuppressorConsumeAndClear(t *testing.T) {
	s := newEinoExecuteStdoutSuppressor()
	s.Record("execute", "stdout", false)
	if got := s.Peek(); got != "stdout" {
		t.Fatalf("Peek = %q, want stdout", got)
	}
	if got := s.Peek(); got != "stdout" {
		t.Fatalf("Peek should not clear, got %q", got)
	}
	if got := s.Consume(); got != "stdout" {
		t.Fatalf("Consume = %q, want stdout", got)
	}
	if got := s.Peek(); got != "" {
		t.Fatalf("Consume should clear, got %q", got)
	}

	s.Record("execute", "again", false)
	s.Clear()
	if got := s.Consume(); got != "" {
		t.Fatalf("Clear should remove pending value, got %q", got)
	}
}
