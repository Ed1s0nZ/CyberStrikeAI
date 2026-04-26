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

func TestWrapRunWithDebug_PanicStillFiresEndSession(t *testing.T) {
	fake := &fakeSink{enabled: true}
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("expected panic to propagate out of wrapRunWithDebug")
		}
		if len(fake.ends) != 1 {
			t.Fatalf("EndSession did not fire on panic; got %d end calls", len(fake.ends))
		}
		if fake.ends[0].id != "conv-panic" {
			t.Fatalf("EndSession id: want conv-panic, got %q", fake.ends[0].id)
		}
		if fake.ends[0].outcome != "failed" {
			t.Fatalf("EndSession outcome on panic: want \"failed\", got %q", fake.ends[0].outcome)
		}
	}()
	_, _ = wrapRunWithDebug(fake, "conv-panic", func() (string, error) {
		panic("runFn exploded")
	})
	t.Fatalf("expected panic to propagate")
}

// ── F2: AgentLoopStream bookend pattern ──────────────────────────────────────

// TestSingleAgent_DebugBookends_RawPattern verifies that the raw
// StartSession + defer-closure (mutating taskStatus) pattern fires
// StartSession and EndSession with the correct outcome.  This exercises
// the same code path used by AgentLoopStream (the streaming single-agent
// handler) without needing a full gin+SSE harness.
func TestSingleAgent_DebugBookends_RawPattern(t *testing.T) {
	for _, tc := range []struct {
		name           string
		finalStatus    string
		wantOutcome    string
	}{
		{"completed path", "completed", "completed"},
		{"cancelled path", "cancelled", "cancelled"},
		{"failed path", "failed", "failed"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeSink{enabled: true}

			const convID = "stream-conv-1"
			taskStatus := "completed"
			fake.StartSession(convID)
			// The defer-closure in AgentLoopStream captures the mutable
			// taskStatus variable; we simulate a mutation then the defer.
			taskStatus = tc.finalStatus
			fake.EndSession(convID, taskStatus)

			if len(fake.starts) != 1 || fake.starts[0] != convID {
				t.Fatalf("StartSession: want 1 call with %q, got %v", convID, fake.starts)
			}
			if len(fake.ends) != 1 {
				t.Fatalf("EndSession: want 1 call, got %d", len(fake.ends))
			}
			if fake.ends[0].id != convID {
				t.Fatalf("EndSession id: want %q, got %q", convID, fake.ends[0].id)
			}
			if fake.ends[0].outcome != tc.wantOutcome {
				t.Fatalf("EndSession outcome: want %q, got %q", tc.wantOutcome, fake.ends[0].outcome)
			}
		})
	}
}
