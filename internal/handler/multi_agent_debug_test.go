package handler

import (
	"testing"

	"cyberstrike-ai/internal/debug"
)

// fakeSink records the lifecycle method args so tests can assert the
// handler's session boundaries without spinning up a real orchestrator.
type fakeSink struct {
	enabled bool
	starts  []string
	ends    []endCall
}

type endCall struct {
	id      string
	outcome string
}

func (f *fakeSink) StartSession(id string)                      { f.starts = append(f.starts, id) }
func (f *fakeSink) EndSession(id, outcome string)               { f.ends = append(f.ends, endCall{id, outcome}) }
func (f *fakeSink) RecordLLMCall(string, string, debug.LLMCall) {}
func (f *fakeSink) RecordEvent(string, string, debug.Event)     {}
func (f *fakeSink) SetEnabled(v bool)                           { f.enabled = v }
func (f *fakeSink) Enabled() bool                               { return f.enabled }

type errTest string

func (e errTest) Error() string { return string(e) }

func TestWrapRunWithDebug_CompletedPath(t *testing.T) {
	fake := &fakeSink{enabled: true}
	outcome, err := wrapRunWithDebug(fake, "conv-x", func() (string, error) {
		return "completed", nil
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if outcome != "completed" {
		t.Fatalf("outcome: want completed, got %q", outcome)
	}
	if len(fake.starts) != 1 || fake.starts[0] != "conv-x" {
		t.Fatalf("StartSession not called once with conv-x: %v", fake.starts)
	}
	if len(fake.ends) != 1 || fake.ends[0].id != "conv-x" || fake.ends[0].outcome != "completed" {
		t.Fatalf("EndSession not called once with conv-x/completed: %v", fake.ends)
	}
}

func TestWrapRunWithDebug_FailedPath(t *testing.T) {
	fake := &fakeSink{enabled: true}
	outcome, err := wrapRunWithDebug(fake, "conv-err", func() (string, error) {
		return "failed", errTest("orchestrator boom")
	})
	if err == nil {
		t.Fatalf("want error, got nil")
	}
	if outcome != "failed" {
		t.Fatalf("outcome: want failed, got %q", outcome)
	}
	if len(fake.ends) != 1 || fake.ends[0].outcome != "failed" {
		t.Fatalf("EndSession outcome on error: want failed, got %v", fake.ends)
	}
}

func TestWrapRunWithDebug_DefaultsCompletedOnEmptyOutcome(t *testing.T) {
	// When the runFn returns "" outcome but no error, default to
	// "completed" so the session isn't stuck as NULL outcome.
	fake := &fakeSink{enabled: true}
	outcome, err := wrapRunWithDebug(fake, "conv-e", func() (string, error) {
		return "", nil
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if outcome != "completed" {
		t.Fatalf("default outcome: want completed, got %q", outcome)
	}
	if fake.ends[0].outcome != "completed" {
		t.Fatalf("EndSession outcome: want completed, got %q", fake.ends[0].outcome)
	}
}

func TestWrapRunWithDebug_DefaultsFailedOnEmptyOutcomeWithError(t *testing.T) {
	fake := &fakeSink{enabled: true}
	outcome, err := wrapRunWithDebug(fake, "conv-ef", func() (string, error) {
		return "", errTest("boom")
	})
	if err == nil {
		t.Fatalf("want error propagated")
	}
	if outcome != "failed" {
		t.Fatalf("default outcome on error: want failed, got %q", outcome)
	}
}

func TestWrapRunWithDebug_NilSinkSafe(t *testing.T) {
	// Handler constructs with sink=nil initially in some tests; must
	// not panic. Spec: noopSink is the real default, but defensive.
	outcome, err := wrapRunWithDebug(nil, "conv-n", func() (string, error) {
		return "completed", nil
	})
	if err != nil || outcome != "completed" {
		t.Fatalf("unexpected err=%v outcome=%q", err, outcome)
	}
}
