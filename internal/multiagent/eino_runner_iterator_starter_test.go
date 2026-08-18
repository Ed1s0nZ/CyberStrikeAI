package multiagent

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/cloudwego/eino/adk"
)

type fakeRunnerControl struct {
	runMessages []adk.Message
	runOpts     int
	resumeID    string
	resumeOpts  int
	resumeErr   error
}

func (f *fakeRunnerControl) Run(_ context.Context, messages []adk.Message, opts ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	f.runMessages = messages
	f.runOpts = len(opts)
	iter, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	gen.Close()
	return iter
}

func (f *fakeRunnerControl) Resume(_ context.Context, checkPointID string, opts ...adk.AgentRunOption) (*adk.AsyncIterator[*adk.AgentEvent], error) {
	f.resumeID = checkPointID
	f.resumeOpts = len(opts)
	iter, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	gen.Close()
	return iter, f.resumeErr
}

func TestEinoRunnerIteratorStarterStartAddsCancelAndCheckpoint(t *testing.T) {
	runner := &fakeRunnerControl{}
	var cancelPush func(error) bool
	var nativeCancelCause atomic.Value
	oldUnregistered := false
	newUnregistered := false
	unregister := func() { oldUnregistered = true }

	iter := newEinoRunnerIteratorStarter(einoRunnerIteratorStarterConfig{
		Context:               context.Background(),
		Runner:                runner,
		CheckPointID:          "cp-1",
		NativeCancelCause:     &nativeCancelCause,
		UnregisterAgentCancel: &unregister,
		RuntimeCancelRegistrar: func(push func(error) bool) func() {
			cancelPush = push
			return func() { newUnregistered = true }
		},
	}).Start([]adk.Message{})

	if iter == nil {
		t.Fatal("iterator should be created")
	}
	if runner.runOpts != 2 {
		t.Fatalf("run opts = %d, want cancel + checkpoint", runner.runOpts)
	}
	if !oldUnregistered {
		t.Fatal("old unregister should be called before binding a new cancel hook")
	}
	if cancelPush == nil {
		t.Fatal("cancel hook should be registered")
	}
	stopErr := errors.New("stop")
	if cancelPush(stopErr) {
		t.Fatal("unbound fake runner cancel should not report handled")
	}
	if got, _ := nativeCancelCause.Load().(error); !errors.Is(got, stopErr) {
		t.Fatalf("native cancel cause = %v, want %v", got, stopErr)
	}
	unregister()
	if !newUnregistered {
		t.Fatal("new unregister should replace old unregister")
	}
}

func TestEinoRunnerIteratorStarterResumeUsesCancelOnly(t *testing.T) {
	runner := &fakeRunnerControl{}

	iter, err := newEinoRunnerIteratorStarter(einoRunnerIteratorStarterConfig{
		Context:      context.Background(),
		Runner:       runner,
		CheckPointID: "fresh-run-checkpoint",
	}).Resume("resume-cp")

	if err != nil {
		t.Fatalf("resume err = %v", err)
	}
	if iter == nil {
		t.Fatal("iterator should be created")
	}
	if runner.resumeID != "resume-cp" {
		t.Fatalf("resume id = %q, want resume-cp", runner.resumeID)
	}
	if runner.resumeOpts != 1 {
		t.Fatalf("resume opts = %d, want cancel only", runner.resumeOpts)
	}
}

func TestEinoRunnerIteratorStarterResumePropagatesError(t *testing.T) {
	resumeErr := errors.New("resume failed")
	runner := &fakeRunnerControl{resumeErr: resumeErr}

	_, err := newEinoRunnerIteratorStarter(einoRunnerIteratorStarterConfig{
		Context: context.Background(),
		Runner:  runner,
	}).Resume("resume-cp")

	if !errors.Is(err, resumeErr) {
		t.Fatalf("resume err = %v, want %v", err, resumeErr)
	}
}
