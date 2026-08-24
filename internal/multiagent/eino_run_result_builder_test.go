package multiagent

import (
	"errors"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

func TestEinoRunResultBuilderPartialWithoutNewMessagesReturnsOriginalError(t *testing.T) {
	runMessages := newEinoRunMessageAccumulator([]adk.Message{schema.UserMessage("base")})
	wantErr := errors.New("stream failed")

	got, err := newEinoRunResultBuilder(einoRunResultBuilderConfig{
		RunMessages: runMessages,
		EmptyHint:   "empty",
	}).BuildPartial(wantErr)

	if got != nil {
		t.Fatalf("partial result = %#v, want nil", got)
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
}

func TestEinoRunResultBuilderFinalUsesSnapshots(t *testing.T) {
	runMessages := newEinoRunMessageAccumulator([]adk.Message{schema.UserMessage("base")})
	runMessages.Append(schema.AssistantMessage("assistant done", nil))
	assistantOutput := newEinoAssistantOutputAccumulator("deep")
	assistantOutput.RecordMainAssistant("orchestrator", "assistant done")

	got := newEinoRunResultBuilder(einoRunResultBuilderConfig{
		OrchMode:        "deep",
		EmptyHint:       "empty",
		RunMessages:     runMessages,
		AssistantOutput: assistantOutput,
		SnapshotMCPIDs: func() []string {
			return []string{"exec-1"}
		},
		ModelFacingTrace: func() []adk.Message {
			return []adk.Message{schema.UserMessage("model-facing")}
		},
	}).BuildFinal()

	if got.Response != "assistant done" {
		t.Fatalf("response = %q, want assistant done", got.Response)
	}
	if len(got.MCPExecutionIDs) != 1 || got.MCPExecutionIDs[0] != "exec-1" {
		t.Fatalf("mcp ids = %#v", got.MCPExecutionIDs)
	}
	if got.LastAgentTraceInput == "" {
		t.Fatal("model-facing trace should be persisted")
	}
}

func TestEinoRunResultBuilderFallbackIgnoresBaseHistory(t *testing.T) {
	runMessages := newEinoRunMessageAccumulator([]adk.Message{
		schema.UserMessage("previous request"),
		schema.AssistantMessage("previous answer", nil),
		schema.UserMessage("new request"),
	})

	got := newEinoRunResultBuilder(einoRunResultBuilderConfig{
		OrchMode:    "deep",
		EmptyHint:   "empty",
		RunMessages: runMessages,
	}).BuildFinal()

	if got.Response != "empty" {
		t.Fatalf("response = %q, want empty hint", got.Response)
	}
}

func TestEinoRunResultBuilderPlanExecutePrefersExecutorOutput(t *testing.T) {
	runMessages := newEinoRunMessageAccumulator(nil)
	runMessages.Append(schema.AssistantMessage(`{"response":"planner text"}`, nil))
	assistantOutput := newEinoAssistantOutputAccumulator("plan_execute")
	assistantOutput.RecordMainAssistant("planner", `{"response":"planner text"}`)
	assistantOutput.RecordMainAssistant("executor", `{"response":"executor text"}`)

	got := newEinoRunResultBuilder(einoRunResultBuilderConfig{
		OrchMode:        "plan_execute",
		EmptyHint:       "empty",
		RunMessages:     runMessages,
		AssistantOutput: assistantOutput,
	}).BuildFinal()

	if got.Response != "executor text" {
		t.Fatalf("response = %q, want executor text", got.Response)
	}
}

func TestEinoRunResultBuilderPlanExecuteUnwrapsFallbackAssistant(t *testing.T) {
	runMessages := newEinoRunMessageAccumulator(nil)
	runMessages.Append(schema.AssistantMessage(`{"response":"fallback executor text"}`, nil))

	got := newEinoRunResultBuilder(einoRunResultBuilderConfig{
		OrchMode:    "plan_execute",
		EmptyHint:   "empty",
		RunMessages: runMessages,
	}).BuildFinal()

	if got.Response != "fallback executor text" {
		t.Fatalf("response = %q, want fallback executor text", got.Response)
	}
}
