package multiagent

import (
	"context"
	"errors"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

type fakeRuntimeSessionAgent struct {
	runMessages []adk.Message
	runOpts     int
}

func (a *fakeRuntimeSessionAgent) Name(context.Context) string {
	return "lead"
}

func (a *fakeRuntimeSessionAgent) Description(context.Context) string {
	return "fake runtime session agent"
}

func (a *fakeRuntimeSessionAgent) Run(_ context.Context, input *adk.AgentInput, opts ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	if input != nil {
		a.runMessages = input.Messages
	}
	a.runOpts = len(opts)
	iter, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	gen.Close()
	return iter
}

func TestEinoRunRuntimeSessionStartsRunner(t *testing.T) {
	agent := &fakeRuntimeSessionAgent{}
	drain := newEinoRunEventDrain(einoRunEventDrainConfig{
		ConversationID:   "conv-1",
		OrchMode:         "deep",
		OrchestratorName: "lead",
		BaseMessages:     []adk.Message{schema.UserMessage("base")},
	})
	session := newEinoRunRuntimeSession(einoRunRuntimeSessionConfig{
		Context: context.Background(),
		Args: &einoADKRunLoopArgs{
			ConversationID:   "conv-1",
			OrchMode:         "deep",
			OrchestratorName: "lead",
			DA:               agent,
		},
		Drain:        drain,
		BaseMessages: []adk.Message{schema.UserMessage("base")},
		EmptyHint:    "empty",
	})
	defer session.Close()

	if session.Iterator() == nil {
		t.Fatal("session should start an iterator")
	}
	if len(agent.runMessages) != 1 || agent.runMessages[0].Content != "base" {
		t.Fatalf("run messages = %#v", agent.runMessages)
	}
	if agent.runOpts != 1 {
		t.Fatalf("run opts = %d, want native cancel option", agent.runOpts)
	}
}

func TestEinoRunRuntimeSessionCompletionFlushesPending(t *testing.T) {
	agent := &fakeRuntimeSessionAgent{}
	var events []string
	drain := newEinoRunEventDrain(einoRunEventDrainConfig{
		ConversationID:   "conv-1",
		OrchMode:         "deep",
		OrchestratorName: "lead",
		Progress: func(eventType, _ string, _ interface{}) {
			events = append(events, eventType)
		},
		BaseMessages: []adk.Message{schema.UserMessage("base")},
	})
	session := newEinoRunRuntimeSession(einoRunRuntimeSessionConfig{
		Context: context.Background(),
		Args: &einoADKRunLoopArgs{
			ConversationID:   "conv-1",
			OrchMode:         "deep",
			OrchestratorName: "lead",
			Progress: func(eventType, _ string, _ interface{}) {
				events = append(events, eventType)
			},
			DA: agent,
		},
		Drain:        drain,
		BaseMessages: []adk.Message{schema.UserMessage("base")},
		EmptyHint:    "empty",
	})
	defer session.Close()

	drain.PendingToolCalls().Mark(toolCallPendingInfo{
		ToolCallID: "call-1",
		ToolName:   "execute",
		EinoAgent:  "lead",
		EinoRole:   "orchestrator",
	})
	completed, result, err := session.HandleIteratorEnd()

	if !completed || result != nil || err != nil {
		t.Fatalf("completed=%v result=%#v err=%v", completed, result, err)
	}
	if !containsString(events, "tool_result") || !containsString(events, "eino_pending_orphaned") {
		t.Fatalf("events = %#v, want orphan pending flush", events)
	}
}

func TestEinoRunRuntimeSessionCancellationReturnsPartialError(t *testing.T) {
	agent := &fakeRuntimeSessionAgent{}
	var events []string
	drain := newEinoRunEventDrain(einoRunEventDrainConfig{
		ConversationID:   "conv-1",
		OrchMode:         "deep",
		OrchestratorName: "lead",
		Progress: func(eventType, _ string, _ interface{}) {
			events = append(events, eventType)
		},
		BaseMessages: []adk.Message{schema.UserMessage("base")},
	})
	session := newEinoRunRuntimeSession(einoRunRuntimeSessionConfig{
		Context: context.Background(),
		Args: &einoADKRunLoopArgs{
			ConversationID:   "conv-1",
			OrchMode:         "deep",
			OrchestratorName: "lead",
			Progress: func(eventType, _ string, _ interface{}) {
				events = append(events, eventType)
			},
			DA: agent,
		},
		Drain:        drain,
		BaseMessages: []adk.Message{schema.UserMessage("base")},
		EmptyHint:    "empty",
	})
	defer session.Close()

	stopErr := errors.New("stop")
	result, err := session.HandleIteratorContextError(stopErr)

	if result != nil {
		t.Fatalf("result = %#v, want nil without new messages", result)
	}
	if !errors.Is(err, stopErr) {
		t.Fatalf("err = %v, want %v", err, stopErr)
	}
	if !containsString(events, "error") {
		t.Fatalf("events = %#v, want cancellation error event", events)
	}
}

func TestEinoRunRuntimeSessionHandleRunErrorSwallowsTurnLoopPreempt(t *testing.T) {
	agent := &fakeRuntimeSessionAgent{}
	var events []string
	drain := newEinoRunEventDrain(einoRunEventDrainConfig{
		ConversationID:   "conv-1",
		OrchMode:         "deep",
		OrchestratorName: "lead",
		Progress: func(eventType, _ string, _ interface{}) {
			events = append(events, eventType)
		},
		BaseMessages: []adk.Message{schema.UserMessage("base")},
	})
	session := newEinoRunRuntimeSession(einoRunRuntimeSessionConfig{
		Context: context.Background(),
		Args: &einoADKRunLoopArgs{
			ConversationID:   "conv-1",
			OrchMode:         "deep",
			OrchestratorName: "lead",
			Progress: func(eventType, _ string, _ interface{}) {
				events = append(events, eventType)
			},
			DA: agent,
		},
		Drain:        drain,
		BaseMessages: []adk.Message{schema.UserMessage("base")},
		EmptyHint:    "empty",
	})
	defer session.Close()

	got := session.HandleRunError(adk.ErrStreamCanceled)
	if got.Restarted || got.Result != nil || got.Err != nil {
		t.Fatalf("result = %+v, want swallowed preempt", got)
	}
	if containsString(events, "error") || containsString(events, "eino_usage_summary") {
		t.Fatalf("events = %#v, want no fatal/partial events", events)
	}
}

func TestEinoRunRuntimeSessionBuildFinalEmitsUsageSummary(t *testing.T) {
	agent := &fakeRuntimeSessionAgent{}
	var usageEvent map[string]interface{}
	progress := func(eventType, _ string, data interface{}) {
		if eventType != "eino_usage_summary" {
			return
		}
		usageEvent, _ = data.(map[string]interface{})
	}
	drain := newEinoRunEventDrain(einoRunEventDrainConfig{
		ConversationID:   "conv-1",
		OrchMode:         "deep",
		OrchestratorName: "lead",
		Progress:         progress,
		BaseMessages:     []adk.Message{schema.UserMessage("base")},
	})
	session := newEinoRunRuntimeSession(einoRunRuntimeSessionConfig{
		Context: context.Background(),
		Args: &einoADKRunLoopArgs{
			ConversationID:   "conv-1",
			OrchMode:         "deep",
			OrchestratorName: "lead",
			Progress:         progress,
			DA:               agent,
		},
		Drain:        drain,
		BaseMessages: []adk.Message{schema.UserMessage("base")},
		EmptyHint:    "empty",
	})
	defer session.Close()

	drain.Usage().AddUsage(&schema.TokenUsage{PromptTokens: 3, CompletionTokens: 4, TotalTokens: 7})
	_ = session.BuildFinalResult()

	if usageEvent == nil {
		t.Fatal("usage summary event was not emitted")
	}
	if usageEvent["conversationId"] != "conv-1" || usageEvent["orchestration"] != "deep" || usageEvent["reason"] != "final" || usageEvent["totalTokens"] != 7 {
		t.Fatalf("usage event = %#v", usageEvent)
	}
}
