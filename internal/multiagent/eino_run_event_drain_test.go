package multiagent

import (
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

func TestEinoRunEventDrainDefaultsAndStreamIDs(t *testing.T) {
	drain := newEinoRunEventDrain(einoRunEventDrainConfig{
		ConversationID:   "conv-1",
		OrchestratorName: "lead",
		BaseMessages:     []adk.Message{schema.UserMessage("base")},
	})

	if drain.RunMessages().BaseCount() != 1 {
		t.Fatalf("base count = %d, want 1", drain.RunMessages().BaseCount())
	}
	if !drain.cfg.StreamsMainAssistant("lead") || drain.cfg.StreamsMainAssistant("worker") {
		t.Fatal("default main-assistant predicate should match only orchestrator")
	}
	if got := drain.cfg.EinoRoleTag("lead"); got != "orchestrator" {
		t.Fatalf("lead role = %q, want orchestrator", got)
	}
	if got := drain.cfg.EinoRoleTag("worker"); got != "sub" {
		t.Fatalf("worker role = %q, want sub", got)
	}
	if got := drain.nextMainStreamID(); got != "eino-main-conv-1-1" {
		t.Fatalf("first main stream id = %q", got)
	}
	if got := drain.nextMainStreamID(); got != "eino-main-conv-1-2" {
		t.Fatalf("second main stream id = %q", got)
	}
}

func TestEinoRunEventDrainBindsHandlersAndRecordsEvents(t *testing.T) {
	var events []string
	recovered := false
	drain := newEinoRunEventDrain(einoRunEventDrainConfig{
		ConversationID:   "conv-1",
		OrchMode:         "deep",
		OrchestratorName: "lead",
		Progress: func(eventType, _ string, _ interface{}) {
			events = append(events, eventType)
		},
		BaseMessages: []adk.Message{schema.UserMessage("base")},
	})
	drain.BindHandlers(func() { recovered = true })

	drain.ObserveAgent("lead")
	if !drain.HandleMaterialized(&adk.MessageVariant{Role: schema.Assistant}, schema.AssistantMessage("done", nil), "lead") {
		t.Fatal("materialized assistant should be handled")
	}
	if got := drain.AssistantOutput().LastAssistant(); got != "done" {
		t.Fatalf("last assistant = %q, want done", got)
	}

	stream := schema.StreamReaderFromArray([]*schema.Message{
		{Role: schema.Tool, Content: "ok", ToolCallID: "call-1"},
	})
	if !drain.HandleToolResultStreaming(&adk.MessageVariant{
		IsStreaming:   true,
		Role:          schema.Tool,
		ToolName:      "execute",
		MessageStream: stream,
	}, "lead") {
		t.Fatal("streaming tool result should be handled")
	}
	if !recovered {
		t.Fatal("tool stream completion should confirm recovery")
	}
	if len(drain.RunMessages().Messages()) != 3 {
		t.Fatalf("run messages = %#v, want base + assistant + tool", drain.RunMessages().Messages())
	}
	if !containsString(events, "iteration") || !containsString(events, "response_start") || !containsString(events, "tool_result") {
		t.Fatalf("events = %#v, want iteration, response and tool_result", events)
	}
}
