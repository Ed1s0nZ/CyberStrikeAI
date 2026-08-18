package multiagent

import (
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestEinoRunProgressTrackerMainToolCallAdvancesResponseIteration(t *testing.T) {
	var events []string
	var iterations []int
	progress := func(eventType, _ string, raw interface{}) {
		events = append(events, eventType)
		data, _ := raw.(map[string]interface{})
		if eventType == "iteration" {
			if n, ok := data["iteration"].(int); ok {
				iterations = append(iterations, n)
			}
		}
	}
	tracker := newEinoRunProgressTracker(
		"eino_single", "main", "conv-1", progress,
		func(agent string) bool { return agent == "" || agent == "main" },
		nil,
	)

	tracker.ObserveAgent("main")
	if got := tracker.MainIteration("main"); got != 1 {
		t.Fatalf("initial main iteration = %d, want 1", got)
	}
	tracker.EmitToolCalls(&schema.Message{ToolCalls: []schema.ToolCall{{
		ID:   "call-1",
		Type: "function",
		Function: schema.FunctionCall{
			Name:      "execute",
			Arguments: `{"command":"pwd"}`,
		},
	}}}, "main", nil)
	if got := tracker.MainIteration("main"); got != 2 {
		t.Fatalf("post-tool main iteration = %d, want 2", got)
	}
	if len(iterations) != 2 || iterations[0] != 1 || iterations[1] != 2 {
		t.Fatalf("iteration events = %#v, want [1 2]; events=%#v", iterations, events)
	}
}

func TestEinoRunProgressTrackerMainAgentSwitchAdvancesIteration(t *testing.T) {
	var iterations []int
	progress := func(eventType, _ string, raw interface{}) {
		if eventType != "iteration" {
			return
		}
		data, _ := raw.(map[string]interface{})
		if n, ok := data["iteration"].(int); ok {
			iterations = append(iterations, n)
		}
	}
	tracker := newEinoRunProgressTracker(
		"supervisor", "lead", "conv-1", progress,
		func(agent string) bool { return agent == "" || agent == "lead" },
		nil,
	)

	tracker.ObserveAgent("lead")
	tracker.ObserveAgent("sub")
	tracker.ObserveAgent("lead")

	if got := tracker.MainIteration("lead"); got != 2 {
		t.Fatalf("main iteration after sub->main = %d, want 2", got)
	}
	if len(iterations) != 2 || iterations[0] != 1 || iterations[1] != 2 {
		t.Fatalf("iteration events = %#v, want [1 2]", iterations)
	}
}

func TestEinoRunProgressTrackerDedupesToolCalls(t *testing.T) {
	var toolCalls int
	progress := func(eventType, _ string, _ interface{}) {
		if eventType == "tool_call" {
			toolCalls++
		}
	}
	tracker := newEinoRunProgressTracker("deep", "lead", "conv-1", progress, nil, nil)
	msg := &schema.Message{ToolCalls: []schema.ToolCall{{
		ID:   "call-1",
		Type: "function",
		Function: schema.FunctionCall{
			Name:      "search",
			Arguments: `{"q":"x"}`,
		},
	}}}

	tracker.EmitToolCalls(msg, "lead", nil)
	tracker.EmitToolCalls(msg, "lead", nil)

	if toolCalls != 1 {
		t.Fatalf("tool call events = %d, want 1", toolCalls)
	}
}

func TestEinoRunProgressTrackerDedupesSameToolCallIDsWithDifferentArgs(t *testing.T) {
	var toolCalls int
	progress := func(eventType, _ string, _ interface{}) {
		if eventType == "tool_call" {
			toolCalls++
		}
	}
	tracker := newEinoRunProgressTracker("deep", "lead", "conv-1", progress, nil, nil)
	first := &schema.Message{ToolCalls: []schema.ToolCall{{
		ID:   "call-1",
		Type: "function",
		Function: schema.FunctionCall{
			Name:      "nmap",
			Arguments: `{"host":"10.0.0.1"}`,
		},
	}}}
	second := &schema.Message{ToolCalls: []schema.ToolCall{{
		ID:   "call-1",
		Type: "function",
		Function: schema.FunctionCall{
			Name:      "nmap",
			Arguments: `{"host":"10.0.0.1","ports":"1-1024"}`,
		},
	}}}

	tracker.EmitToolCalls(first, "lead", nil)
	tracker.EmitToolCalls(second, "lead", nil)

	if toolCalls != 1 {
		t.Fatalf("tool call events = %d, want 1", toolCalls)
	}
}

func TestEinoRunProgressTrackerHidesModelOutputRecoveryToolCalls(t *testing.T) {
	var eventTypes []string
	var marked []toolCallPendingInfo
	progress := func(eventType, _ string, _ interface{}) {
		eventTypes = append(eventTypes, eventType)
	}
	tracker := newEinoRunProgressTracker("deep", "lead", "conv-1", progress, nil, nil)
	msg := &schema.Message{ToolCalls: []schema.ToolCall{{
		ID:   "call-recovery",
		Type: "function",
		Function: schema.FunctionCall{
			Name:      "task",
			Arguments: `{"_cyberstrike_model_output_recovery":{"reason":"invalid_tool_arguments_json","repair_attempt":1}}`,
		},
	}}}

	tracker.EmitToolCalls(msg, "lead", func(info toolCallPendingInfo) {
		marked = append(marked, info)
	})

	if containsString(eventTypes, "tool_calls_detected") || containsString(eventTypes, "tool_call") {
		t.Fatalf("event types = %#v, want no visible recovery tool call events", eventTypes)
	}
	if len(marked) != 0 {
		t.Fatalf("marked pending = %#v, want none", marked)
	}
}

func TestEinoRunProgressTrackerHidesAnonymousToolCallFragments(t *testing.T) {
	var eventTypes []string
	var marked []toolCallPendingInfo
	progress := func(eventType, _ string, _ interface{}) {
		eventTypes = append(eventTypes, eventType)
	}
	tracker := newEinoRunProgressTracker("eino_single", "lead", "conv-1", progress, nil, nil)
	idx := 0
	msg := &schema.Message{ToolCalls: []schema.ToolCall{{
		Type:  "function",
		Index: &idx,
		Function: schema.FunctionCall{
			Arguments: `"`,
		},
	}}}

	tracker.EmitToolCalls(msg, "lead", func(info toolCallPendingInfo) {
		marked = append(marked, info)
	})

	if containsString(eventTypes, "tool_calls_detected") || containsString(eventTypes, "tool_call") {
		t.Fatalf("event types = %#v, want no visible anonymous fragment tool call events", eventTypes)
	}
	if len(marked) != 0 {
		t.Fatalf("marked pending = %#v, want none", marked)
	}
}

func TestEinoRunProgressTrackerKeepsNamedInvalidToolCallsVisible(t *testing.T) {
	var toolCalls int
	var marked []toolCallPendingInfo
	progress := func(eventType, _ string, _ interface{}) {
		if eventType == "tool_call" {
			toolCalls++
		}
	}
	tracker := newEinoRunProgressTracker("eino_single", "lead", "conv-1", progress, nil, nil)
	msg := &schema.Message{ToolCalls: []schema.ToolCall{{
		ID:   "call-bad-args",
		Type: "function",
		Function: schema.FunctionCall{
			Name:      "exec",
			Arguments: `command`,
		},
	}}}

	tracker.EmitToolCalls(msg, "lead", func(info toolCallPendingInfo) {
		marked = append(marked, info)
	})

	if toolCalls != 1 {
		t.Fatalf("tool call events = %d, want 1", toolCalls)
	}
	if len(marked) != 1 || marked[0].ToolName != "exec" {
		t.Fatalf("marked pending = %#v, want one exec call", marked)
	}
}
