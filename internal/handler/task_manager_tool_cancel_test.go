package handler

import (
	"context"
	"errors"
	"testing"

	"cyberstrike-ai/internal/multiagent"
)

func TestCancelTaskInvokesToolCancelerOnFullStop(t *testing.T) {
	tm := NewAgentTaskManager()
	called := false
	tm.SetToolCanceler(func(conversationID string) {
		if conversationID == "conv-1" {
			called = true
		}
	})

	_, cancel := context.WithCancelCause(context.Background())
	_, err := tm.StartTask("conv-1", "hello", cancel)
	if err != nil {
		t.Fatalf("StartTask: %v", err)
	}

	ok, err := tm.CancelTask("conv-1", ErrTaskCancelled)
	if err != nil || !ok {
		t.Fatalf("CancelTask: ok=%v err=%v", ok, err)
	}
	if !called {
		t.Fatal("expected tool canceler to be invoked on full task cancel")
	}
}

func TestCancelTaskFullStopCancelsRuntimeAndParentContext(t *testing.T) {
	tm := NewAgentTaskManager()
	var order []string
	tm.SetToolCanceler(func(conversationID string) {
		if conversationID == "conv-native" {
			order = append(order, "tool")
		}
	})

	_, cancel := context.WithCancelCause(context.Background())
	if _, err := tm.StartTask("conv-native", "hello", func(err error) {
		order = append(order, "context")
		cancel(err)
	}); err != nil {
		t.Fatalf("StartTask: %v", err)
	}
	unregister := tm.BindAgentRuntimeCancel("conv-native", func(err error) bool {
		if !errors.Is(err, ErrTaskCancelled) {
			t.Fatalf("runtime cancel got %v", err)
		}
		order = append(order, "runtime")
		return true
	})
	defer unregister()

	ok, err := tm.CancelTask("conv-native", ErrTaskCancelled)
	if err != nil || !ok {
		t.Fatalf("CancelTask: ok=%v err=%v", ok, err)
	}
	want := []string{"runtime", "context", "tool"}
	if len(order) != len(want) {
		t.Fatalf("order length got %d want %d: %#v", len(order), len(want), order)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order[%d] got %q want %q; full=%#v", i, order[i], want[i], order)
		}
	}
}

func TestCancelTaskInterruptContinueKeepsParentWhenRuntimeHandlesIt(t *testing.T) {
	tm := NewAgentTaskManager()
	ctx, cancel := context.WithCancelCause(context.Background())
	if _, err := tm.StartTask("conv-interrupt-native", "hello", cancel); err != nil {
		t.Fatalf("StartTask: %v", err)
	}
	unregister := tm.BindAgentRuntimeCancel("conv-interrupt-native", func(err error) bool {
		if !errors.Is(err, multiagent.ErrInterruptContinue) {
			t.Fatalf("runtime cancel got %v", err)
		}
		return true
	})
	defer unregister()

	ok, err := tm.CancelTask("conv-interrupt-native", multiagent.ErrInterruptContinue)
	if err != nil || !ok {
		t.Fatalf("CancelTask: ok=%v err=%v", ok, err)
	}
	if cause := context.Cause(ctx); cause != nil {
		t.Fatalf("interrupt-continue parent context cause = %v, want nil", cause)
	}
}

func TestCancelTaskFallsBackToContextWhenAgentRuntimeCancelMisses(t *testing.T) {
	tm := NewAgentTaskManager()
	var order []string

	_, cancel := context.WithCancelCause(context.Background())
	if _, err := tm.StartTask("conv-fallback", "hello", func(err error) {
		order = append(order, "context")
		cancel(err)
	}); err != nil {
		t.Fatalf("StartTask: %v", err)
	}
	unregister := tm.BindAgentRuntimeCancel("conv-fallback", func(err error) bool {
		order = append(order, "runtime")
		return false
	})
	defer unregister()

	ok, err := tm.CancelTask("conv-fallback", ErrTaskCancelled)
	if err != nil || !ok {
		t.Fatalf("CancelTask: ok=%v err=%v", ok, err)
	}
	want := []string{"runtime", "context"}
	if len(order) != len(want) {
		t.Fatalf("order length got %d want %d: %#v", len(order), len(want), order)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order[%d] got %q want %q; full=%#v", i, order[i], want[i], order)
		}
	}
}

func TestCancelTaskSkipsToolCancelerOnInterruptContinue(t *testing.T) {
	tm := NewAgentTaskManager()
	called := false
	tm.SetToolCanceler(func(conversationID string) {
		called = true
	})

	_, cancel := context.WithCancelCause(context.Background())
	_, err := tm.StartTask("conv-1", "hello", cancel)
	if err != nil {
		t.Fatalf("StartTask: %v", err)
	}

	ok, err := tm.CancelTask("conv-1", multiagent.ErrInterruptContinue)
	if err != nil || !ok {
		t.Fatalf("CancelTask: ok=%v err=%v", ok, err)
	}
	if called {
		t.Fatal("tool canceler must not run for interrupt-continue")
	}
}

func TestCancelTaskPushesInterruptContinueToTurnLoopFirst(t *testing.T) {
	tm := NewAgentTaskManager()
	ctx, cancel := context.WithCancelCause(context.Background())
	if _, err := tm.StartTask("conv-turn", "hello", cancel); err != nil {
		t.Fatalf("StartTask: %v", err)
	}
	tm.SetInterruptContinueNote("conv-turn", "focus ssh")

	var gotNote string
	unregister := tm.BindAgentTurnLoopInterrupt("conv-turn", func(note string) bool {
		gotNote = note
		return true
	})
	defer unregister()

	ok, err := tm.CancelTask("conv-turn", multiagent.ErrInterruptContinue)
	if err != nil || !ok {
		t.Fatalf("CancelTask: ok=%v err=%v", ok, err)
	}
	if gotNote != "focus ssh" {
		t.Fatalf("turn loop note = %q, want focus ssh", gotNote)
	}
	if cause := context.Cause(ctx); cause != nil {
		t.Fatalf("context should not be cancelled when turn loop accepted interrupt, got %v", cause)
	}
	if note := tm.TakeInterruptContinueNote("conv-turn"); note != "" {
		t.Fatalf("interrupt note should be consumed after turn loop push, got %q", note)
	}
}

func TestCancelTaskFallsBackWhenTurnLoopInterruptRejects(t *testing.T) {
	tm := NewAgentTaskManager()
	var order []string

	_, cancel := context.WithCancelCause(context.Background())
	if _, err := tm.StartTask("conv-turn-fallback", "hello", func(err error) {
		order = append(order, "context")
		cancel(err)
	}); err != nil {
		t.Fatalf("StartTask: %v", err)
	}
	tm.SetInterruptContinueNote("conv-turn-fallback", "fallback note")
	unregisterTurn := tm.BindAgentTurnLoopInterrupt("conv-turn-fallback", func(note string) bool {
		order = append(order, "turn")
		if note != "fallback note" {
			t.Fatalf("turn loop note = %q, want fallback note", note)
		}
		return false
	})
	defer unregisterTurn()
	unregisterRuntime := tm.BindAgentRuntimeCancel("conv-turn-fallback", func(err error) bool {
		order = append(order, "runtime")
		return false
	})
	defer unregisterRuntime()

	ok, err := tm.CancelTask("conv-turn-fallback", multiagent.ErrInterruptContinue)
	if err != nil || !ok {
		t.Fatalf("CancelTask: ok=%v err=%v", ok, err)
	}
	want := []string{"turn", "runtime", "context"}
	if len(order) != len(want) {
		t.Fatalf("order length got %d want %d: %#v", len(order), len(want), order)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order[%d] got %q want %q; full=%#v", i, order[i], want[i], order)
		}
	}
	if note := tm.TakeInterruptContinueNote("conv-turn-fallback"); note != "fallback note" {
		t.Fatalf("interrupt note should remain for fallback rerun, got %q", note)
	}
}

func TestCancelTaskDefaultCauseIsTaskCancelled(t *testing.T) {
	tm := NewAgentTaskManager()
	var gotCause error
	tm.SetToolCanceler(func(conversationID string) {
		if conversationID == "conv-2" {
			gotCause = ErrTaskCancelled
		}
	})

	ctx, cancel := context.WithCancelCause(context.Background())
	if _, err := tm.StartTask("conv-2", "hello", cancel); err != nil {
		t.Fatalf("StartTask: %v", err)
	}

	if _, err := tm.CancelTask("conv-2", nil); err != nil {
		t.Fatalf("CancelTask: %v", err)
	}
	if !errors.Is(context.Cause(ctx), ErrTaskCancelled) {
		t.Fatalf("expected ErrTaskCancelled cause, got %v", context.Cause(ctx))
	}
	if gotCause != ErrTaskCancelled {
		t.Fatalf("expected tool canceler path for default cancel cause")
	}
}

func TestFinishTaskInvokesToolCancelerOnSessionEnd(t *testing.T) {
	tm := NewAgentTaskManager()
	calls := 0
	tm.SetToolCanceler(func(conversationID string) {
		if conversationID == "conv-3" {
			calls++
		}
	})

	_, cancel := context.WithCancelCause(context.Background())
	if _, err := tm.StartTask("conv-3", "hello", cancel); err != nil {
		t.Fatalf("StartTask: %v", err)
	}

	tm.FinishTask("conv-3", "completed")
	if calls != 1 {
		t.Fatalf("expected one tool cleanup on FinishTask, got %d", calls)
	}
}
