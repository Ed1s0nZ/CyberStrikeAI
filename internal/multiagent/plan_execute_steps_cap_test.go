package multiagent

import (
	"strings"
	"testing"

	"github.com/cloudwego/eino/adk/prebuilt/planexecute"
)

func TestCapPlanExecuteExecutedSteps_TruncatesLongResult(t *testing.T) {
	long := strings.Repeat("x", planExecuteMaxStepResultRunes+500)
	steps := []planexecute.ExecutedStep{{Step: "s1", Result: long}}
	out := capPlanExecuteExecutedSteps(steps)
	if len(out) != 1 {
		t.Fatalf("len=%d", len(out))
	}
	if !strings.Contains(out[0].Result, "truncated") {
		t.Fatalf("expected truncation marker in %q", out[0].Result[:80])
	}
}

func TestCapPlanExecuteExecutedSteps_FoldsEarlySteps(t *testing.T) {
	var steps []planexecute.ExecutedStep
	for i := 0; i < planExecuteKeepLastSteps+5; i++ {
		steps = append(steps, planexecute.ExecutedStep{Step: "step", Result: "ok"})
	}
	out := capPlanExecuteExecutedSteps(steps)
	if len(out) != planExecuteKeepLastSteps+1 {
		t.Fatalf("want %d entries, got %d", planExecuteKeepLastSteps+1, len(out))
	}
	if out[0].Step != "[Earlier steps — titles only]" {
		t.Fatalf("first entry: %#v", out[0])
	}
}
