package multiagent

import "testing"

func TestEinoAssistantOutputAccumulatorRecordsMainAssistant(t *testing.T) {
	acc := newEinoAssistantOutputAccumulator("deep")
	if acc.RecordMainAssistant("lead", "  hello  ") != true {
		t.Fatal("expected record")
	}
	if got := acc.LastAssistant(); got != "hello" {
		t.Fatalf("last assistant = %q, want hello", got)
	}
	if got := acc.LastPlanExecuteExecutor(); got != "" {
		t.Fatalf("plan execute executor = %q, want empty", got)
	}
	if acc.RecordMainAssistant("lead", "   ") {
		t.Fatal("blank content should not record")
	}
	if got := acc.LastAssistant(); got != "hello" {
		t.Fatalf("blank content changed last assistant to %q", got)
	}
}

func TestEinoAssistantOutputAccumulatorPlanExecuteExecutor(t *testing.T) {
	acc := newEinoAssistantOutputAccumulator("plan_execute")
	raw := `{"response":"给用户看的正文","scratchpad":"internal"}`
	acc.RecordMainAssistant("executor", raw)

	if got := acc.LastAssistant(); got != raw {
		t.Fatalf("last assistant = %q, want raw", got)
	}
	if got := acc.LastPlanExecuteExecutor(); got != "给用户看的正文" {
		t.Fatalf("executor output = %q", got)
	}
	acc.RecordMainAssistant("planner", "planner note")
	if got := acc.LastAssistant(); got != "planner note" {
		t.Fatalf("last assistant after planner = %q", got)
	}
	if got := acc.LastPlanExecuteExecutor(); got != "给用户看的正文" {
		t.Fatalf("planner should not overwrite executor output, got %q", got)
	}
}

func TestEinoAssistantOutputAccumulatorNilSafe(t *testing.T) {
	var acc *einoAssistantOutputAccumulator
	if acc.RecordMainAssistant("agent", "hello") {
		t.Fatal("nil accumulator should not record")
	}
	if acc.LastAssistant() != "" || acc.LastPlanExecuteExecutor() != "" {
		t.Fatal("nil accumulator should return empty values")
	}
}
