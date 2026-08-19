package multiagent

import (
	"context"
	"errors"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

func TestEinoTurnLoopEventBridgeSwallowsPreemptCancel(t *testing.T) {
	iter, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	outIter, outGen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()

	preempted := make(chan struct{})
	close(preempted)
	var eventTypes []string
	bridge := newEinoTurnLoopEventBridge("conv", "eino_single", func(eventType, _ string, _ interface{}) {
		eventTypes = append(eventTypes, eventType)
	}, outGen)

	gen.Send(&adk.AgentEvent{Err: &adk.CancelError{Info: &adk.AgentCancelInfo{}}})
	gen.Close()

	err := bridge.OnAgentEvents(context.Background(), &adk.TurnContext[EinoTurnLoopItem, *schema.Message]{
		Preempted: preempted,
	}, iter)
	if err != nil {
		t.Fatalf("preempt cancel should be swallowed, got %v", err)
	}
	if bridge.ForwardedError() {
		t.Fatal("preempt cancel should not be marked as forwarded")
	}
	if !containsString(eventTypes, "progress") {
		t.Fatalf("events = %#v, want progress", eventTypes)
	}
	outGen.Close()
	if ev, ok := outIter.Next(); ok || ev != nil {
		t.Fatalf("preempt cancel should not be forwarded, got ok=%v ev=%#v", ok, ev)
	}
}

func TestEinoTurnLoopEventBridgeNeverForwardsStreamCanceled(t *testing.T) {
	iter, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	outIter, outGen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	bridge := newEinoTurnLoopEventBridge("conv", "eino_single", nil, outGen)

	gen.Send(&adk.AgentEvent{Err: adk.ErrStreamCanceled})
	gen.Close()

	err := bridge.OnAgentEvents(context.Background(), &adk.TurnContext[EinoTurnLoopItem, *schema.Message]{
		Preempted: make(chan struct{}),
	}, iter)
	if err != nil {
		t.Fatalf("ErrStreamCanceled must be owned by TurnLoop, got %v", err)
	}
	if bridge.ForwardedError() {
		t.Fatal("ErrStreamCanceled must not be marked as forwarded")
	}
	outGen.Close()
	if ev, ok := outIter.Next(); ok || ev != nil {
		t.Fatalf("ErrStreamCanceled must not be forwarded, got ok=%v ev=%#v", ev, ok)
	}
}

func TestEinoTurnLoopEventBridgeNeverForwardsCancelError(t *testing.T) {
	iter, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	outIter, outGen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	bridge := newEinoTurnLoopEventBridge("conv", "eino_single", nil, outGen)

	gen.Send(&adk.AgentEvent{Err: &adk.CancelError{Info: &adk.AgentCancelInfo{}}})
	gen.Close()

	err := bridge.OnAgentEvents(context.Background(), &adk.TurnContext[EinoTurnLoopItem, *schema.Message]{
		Preempted: make(chan struct{}),
	}, iter)
	if err != nil {
		t.Fatalf("CancelError must be owned by TurnLoop, got %v", err)
	}
	if bridge.ForwardedError() {
		t.Fatal("CancelError must not be marked as forwarded")
	}
	outGen.Close()
	if ev, ok := outIter.Next(); ok || ev != nil {
		t.Fatalf("CancelError must not be forwarded, got ok=%v ev=%#v", ok, ev)
	}
}

func TestEinoTurnLoopEventBridgeForwardsRegularError(t *testing.T) {
	iter, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	outIter, outGen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()

	want := errors.New("model failed")
	bridge := newEinoTurnLoopEventBridge("conv", "eino_single", nil, outGen)
	gen.Send(&adk.AgentEvent{Err: want})
	gen.Close()

	err := bridge.OnAgentEvents(context.Background(), &adk.TurnContext[EinoTurnLoopItem, *schema.Message]{
		Preempted: make(chan struct{}),
	}, iter)
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
	if !bridge.ForwardedError() {
		t.Fatal("regular error should be marked as forwarded")
	}
	outGen.Close()
	ev, ok := outIter.Next()
	if !ok || ev == nil || !errors.Is(ev.Err, want) {
		t.Fatalf("forwarded event = %#v ok=%v", ev, ok)
	}
}

func TestEinoTurnLoopEventBridgeForwardsNormalEvents(t *testing.T) {
	iter, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	outIter, outGen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()

	bridge := newEinoTurnLoopEventBridge("conv", "eino_single", nil, outGen)
	gen.Send(&adk.AgentEvent{
		AgentName: "agent",
		Output: &adk.AgentOutput{MessageOutput: &adk.MessageVariant{
			Message: schema.AssistantMessage("ok", nil),
			Role:    schema.Assistant,
		}},
	})
	gen.Close()

	if err := bridge.OnAgentEvents(context.Background(), &adk.TurnContext[EinoTurnLoopItem, *schema.Message]{
		Preempted: make(chan struct{}),
	}, iter); err != nil {
		t.Fatalf("OnAgentEvents: %v", err)
	}
	outGen.Close()
	ev, ok := outIter.Next()
	if !ok || ev == nil || ev.AgentName != "agent" {
		t.Fatalf("forwarded event = %#v ok=%v", ev, ok)
	}
}
