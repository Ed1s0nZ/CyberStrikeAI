package multiagent

import (
	"context"
	"errors"
	"testing"

	"github.com/cloudwego/eino/adk"
)

func TestEinoStreamErrorHandlerEmitsProgressAndRestarts(t *testing.T) {
	streamErr := errors.New("stream broken")
	var progressEvents []map[string]interface{}
	handler := newEinoStreamErrorHandler(
		context.Background(),
		"conv-1",
		func(eventType, _ string, data interface{}) {
			if eventType != "eino_stream_error" {
				return
			}
			m, _ := data.(map[string]interface{})
			progressEvents = append(progressEvents, m)
		},
		func(agent string) string {
			if agent == "worker" {
				return "sub"
			}
			return "orchestrator"
		},
		func(err error) (bool, error) {
			if !errors.Is(err, streamErr) {
				t.Fatalf("retry err = %v", err)
			}
			return true, nil
		},
		nil,
	)

	got := handler.Handle(streamErr, "worker")
	if !got.Handled || !got.Restarted || got.Result != nil || got.Err != nil {
		t.Fatalf("result = %+v", got)
	}
	if len(progressEvents) != 1 {
		t.Fatalf("progress events = %#v", progressEvents)
	}
	if progressEvents[0]["conversationId"] != "conv-1" || progressEvents[0]["einoAgent"] != "worker" || progressEvents[0]["einoRole"] != "sub" {
		t.Fatalf("progress data = %#v", progressEvents[0])
	}
}

func TestEinoStreamErrorHandlerRetryFatalUsesPartial(t *testing.T) {
	streamErr := errors.New("stream broken")
	fatalErr := errors.New("retry exhausted")
	wantResult := &RunResult{Response: "partial"}
	handler := newEinoStreamErrorHandler(
		context.Background(),
		"conv-1",
		nil,
		nil,
		func(error) (bool, error) { return false, fatalErr },
		func(err error) (*RunResult, error) {
			if !errors.Is(err, fatalErr) {
				t.Fatalf("partial err = %v", err)
			}
			return wantResult, err
		},
	)

	got := handler.Handle(streamErr, "lead")
	if !got.Handled || got.Restarted || got.Result != wantResult || !errors.Is(got.Err, fatalErr) {
		t.Fatalf("result = %+v", got)
	}
}

func TestEinoStreamErrorHandlerTurnLoopPreemptSwallowsStreamCanceled(t *testing.T) {
	var progressCalled bool
	var retryCalled bool
	var partialCalled bool
	handler := newEinoStreamErrorHandler(
		context.Background(),
		"conv-1",
		func(string, string, interface{}) { progressCalled = true },
		nil,
		func(error) (bool, error) {
			retryCalled = true
			return false, errors.New("should not retry")
		},
		func(error) (*RunResult, error) {
			partialCalled = true
			return nil, errors.New("should not take partial")
		},
	)

	got := handler.Handle(adk.ErrStreamCanceled, "lead")
	if !got.Handled || got.Restarted || got.Result != nil || got.Err != nil {
		t.Fatalf("result = %+v, want swallowed preempt", got)
	}
	if progressCalled || retryCalled || partialCalled {
		t.Fatalf("progressCalled=%v retryCalled=%v partialCalled=%v, want all false", progressCalled, retryCalled, partialCalled)
	}
}

func TestEinoStreamErrorHandlerInterruptContinueUsesPartialWithoutProgress(t *testing.T) {
	base := context.Background()
	ctx, cancel := context.WithCancelCause(base)
	cancel(ErrInterruptContinue)
	streamErr := errors.New("context canceled while streaming")
	var progressCalled bool
	var retryCalled bool
	handler := newEinoStreamErrorHandler(
		ctx,
		"conv-1",
		func(string, string, interface{}) { progressCalled = true },
		nil,
		func(error) (bool, error) {
			retryCalled = true
			return false, nil
		},
		func(err error) (*RunResult, error) {
			if !errors.Is(err, streamErr) {
				t.Fatalf("partial err = %v", err)
			}
			return nil, err
		},
	)

	got := handler.Handle(streamErr, "lead")
	if !got.Handled || got.Result != nil || !errors.Is(got.Err, streamErr) {
		t.Fatalf("result = %+v", got)
	}
	if progressCalled || retryCalled {
		t.Fatalf("progressCalled=%v retryCalled=%v, want both false", progressCalled, retryCalled)
	}
}

func TestIsEinoTurnLoopPreemptErr(t *testing.T) {
	if !isEinoTurnLoopPreemptErr(context.Background(), adk.ErrStreamCanceled) {
		t.Fatal("alive host + stream canceled should be treated as TurnLoop preempt")
	}
	if !isEinoTurnLoopPreemptErr(context.Background(), context.Canceled) {
		t.Fatal("alive host + context.Canceled should be treated as TurnLoop preempt")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if isEinoTurnLoopPreemptErr(ctx, adk.ErrStreamCanceled) {
		t.Fatal("canceled host should not be treated as TurnLoop preempt")
	}
	if isEinoTurnLoopPreemptErr(context.Background(), errors.New("boom")) {
		t.Fatal("regular errors must stay fatal")
	}
}

func TestEinoStreamErrorHandlerNilError(t *testing.T) {
	got := newEinoStreamErrorHandler(context.Background(), "conv", nil, nil, nil, nil).Handle(nil, "lead")
	if got.Handled || got.Restarted || got.Result != nil || got.Err != nil {
		t.Fatalf("nil error result = %+v", got)
	}
}
