package multiagent

import "testing"

func TestAggregatedReasoningFromTraceJSON(t *testing.T) {
	const j = `[
{"role":"user","content":"hi"},
{"role":"assistant","content":"c1","reasoning_content":"r1","tool_calls":[{"id":"1","type":"function","function":{"name":"f","arguments":"{}"}}]},
{"role":"tool","tool_call_id":"1","content":"out"},
{"role":"assistant","content":"c2","reasoning_content":"r2"}
]`
	got := AggregatedReasoningFromTraceJSON(j)
	want := "r1\nr2"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if AggregatedReasoningFromTraceJSON("") != "" || AggregatedReasoningFromTraceJSON("[]") != "" {
		t.Fatal("empty expected")
	}
}
