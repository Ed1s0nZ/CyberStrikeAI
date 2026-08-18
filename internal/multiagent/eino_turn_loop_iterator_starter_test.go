package multiagent

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

type fakeTurnLoopRuntimeControl struct {
	mu          sync.Mutex
	runCalled   bool
	stopIdle    bool
	stopped     string
	pushedNotes []string
	pushOK      bool
}

func (f *fakeTurnLoopRuntimeControl) Run(context.Context) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.runCalled = true
}

func (f *fakeTurnLoopRuntimeControl) PushInterruptContinue(note string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pushedNotes = append(f.pushedNotes, note)
	return f.pushOK
}

func (f *fakeTurnLoopRuntimeControl) StopImmediate(cause string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stopped = cause
}

func (f *fakeTurnLoopRuntimeControl) StopWhenIdle() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stopIdle = true
}

func (f *fakeTurnLoopRuntimeControl) Wait() *adk.TurnLoopExitState[EinoTurnLoopItem, *schema.Message] {
	return nil
}

func (f *fakeTurnLoopRuntimeControl) snapshot() (runCalled bool, stopIdle bool, stopped string, pushed []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.runCalled, f.stopIdle, f.stopped, append([]string(nil), f.pushedNotes...)
}

func TestEinoTurnLoopIteratorStarterBindsRegistrarsAndProgress(t *testing.T) {
	fakeRuntime := &fakeTurnLoopRuntimeControl{pushOK: true}
	oldAgentCleared := false
	oldTurnCleared := false
	unregisterAgent := func() { oldAgentCleared = true }
	unregisterTurn := func() { oldTurnCleared = true }
	var interruptPush func(string) bool
	var cancelPush func(error) bool
	var createdCfg EinoTurnLoopRuntimeConfig
	var events []struct {
		eventType string
		message   string
		data      map[string]interface{}
	}

	iter := newEinoTurnLoopIteratorStarter(einoTurnLoopIteratorStarterConfig{
		Context:                     context.Background(),
		ConversationID:              "conv",
		OrchMode:                    "deep",
		CheckPointID:                "runner-checkpoint",
		UnregisterAgentCancel:       &unregisterAgent,
		UnregisterTurnLoopInterrupt: &unregisterTurn,
		RuntimeCancelRegistrar: func(push func(error) bool) func() {
			cancelPush = push
			return func() {}
		},
		TurnLoopInterruptRegistrar: func(push func(string) bool) func() {
			interruptPush = push
			return func() {}
		},
		RuntimeFactory: func(cfg EinoTurnLoopRuntimeConfig) einoTurnLoopRuntimeControl {
			createdCfg = cfg
			return fakeRuntime
		},
		Progress: func(eventType, message string, data interface{}) {
			item := struct {
				eventType string
				message   string
				data      map[string]interface{}
			}{eventType: eventType, message: message}
			if m, ok := data.(map[string]interface{}); ok {
				item.data = m
			}
			events = append(events, item)
		},
	}).Start([]adk.Message{})

	if iter == nil {
		t.Fatal("iterator should be created")
	}
	if !oldAgentCleared || !oldTurnCleared {
		t.Fatalf("oldAgentCleared=%v oldTurnCleared=%v, want both true", oldAgentCleared, oldTurnCleared)
	}
	if interruptPush == nil {
		t.Fatal("turn loop interrupt registrar was not bound")
	}
	if cancelPush == nil {
		t.Fatal("runtime cancel registrar was not bound")
	}
	if createdCfg.CheckpointID != buildEinoTurnLoopCheckpointID("deep") {
		t.Fatalf("checkpoint id = %q, want turn loop checkpoint id", createdCfg.CheckpointID)
	}
	if !interruptPush("  focus ssh  ") {
		t.Fatal("interrupt push should return runtime result")
	}

	runCalled, stopIdle, _, pushed := fakeRuntime.snapshot()
	if !runCalled || !stopIdle {
		t.Fatalf("runCalled=%v stopIdle=%v, want both true", runCalled, stopIdle)
	}
	if len(pushed) != 1 || pushed[0] != "  focus ssh  " {
		t.Fatalf("pushed notes = %#v", pushed)
	}
	if len(events) != 2 {
		t.Fatalf("events = %#v, want user interrupt and progress", events)
	}
	if events[0].eventType != "user_interrupt_continue" || events[0].data["rawReason"] != "focus ssh" {
		t.Fatalf("first event = %#v", events[0])
	}
	if events[1].eventType != "progress" {
		t.Fatalf("second event = %#v", events[1])
	}
}

func TestEinoTurnLoopIteratorStarterRuntimeCancel(t *testing.T) {
	fakeRuntime := &fakeTurnLoopRuntimeControl{pushOK: true}
	var nativeCancelCause atomic.Value
	var cancelPush func(error) bool
	var unregisterAgent func()

	newEinoTurnLoopIteratorStarter(einoTurnLoopIteratorStarterConfig{
		Context:               context.Background(),
		ConversationID:        "conv",
		OrchMode:              "eino_single",
		NativeCancelCause:     &nativeCancelCause,
		UnregisterAgentCancel: &unregisterAgent,
		RuntimeCancelRegistrar: func(push func(error) bool) func() {
			cancelPush = push
			return func() {}
		},
		RuntimeFactory: func(EinoTurnLoopRuntimeConfig) einoTurnLoopRuntimeControl {
			return fakeRuntime
		},
	}).Start(nil)
	if cancelPush == nil {
		t.Fatal("runtime cancel registrar was not bound")
	}

	if !cancelPush(ErrInterruptContinue) {
		t.Fatal("interrupt continue cancel should be handled by TurnLoop push")
	}
	_, _, stopped, pushed := fakeRuntime.snapshot()
	if stopped != "" {
		t.Fatalf("stopped = %q, want no immediate stop for interrupt continue", stopped)
	}
	if len(pushed) != 1 || pushed[0] != "" {
		t.Fatalf("pushed notes = %#v, want empty interrupt continue note", pushed)
	}

	stopErr := errors.New("stop now")
	if !cancelPush(stopErr) {
		t.Fatal("regular cancel should be handled")
	}
	_, _, stopped, _ = fakeRuntime.snapshot()
	if stopped != "task_cancelled" {
		t.Fatalf("stopped = %q, want task_cancelled", stopped)
	}
	if got, _ := nativeCancelCause.Load().(error); !errors.Is(got, stopErr) {
		t.Fatalf("native cancel cause = %v, want %v", got, stopErr)
	}
}
