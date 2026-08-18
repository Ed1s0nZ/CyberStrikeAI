package multiagent

import "testing"

func TestUnwrapPlanExecuteUserText(t *testing.T) {
	raw := `{"response": "你好！很高兴见到你。"}`
	if got := UnwrapPlanExecuteUserText(raw); got != "你好！很高兴见到你。" {
		t.Fatalf("got %q", got)
	}
	if got := UnwrapPlanExecuteUserText("plain"); got != "plain" {
		t.Fatalf("got %q", got)
	}
	steps := `{"steps":["a","b"]}`
	if got := UnwrapPlanExecuteUserText(steps); got != steps {
		t.Fatalf("expected unchanged steps json, got %q", got)
	}
}
