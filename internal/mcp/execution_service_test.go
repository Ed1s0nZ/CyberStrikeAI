package mcp

import (
	"context"
	"testing"
)

func TestExecutionServiceBackgroundWaitResultCompletesWaitTool(t *testing.T) {
	service := NewExecutionService(nil, nil)
	handle, err := service.Submit(context.Background(), ExecutionRequest{
		ToolName: "wait_tool_execution",
		Run: func(context.Context) (*ToolResult, error) {
			return &ToolResult{
				Content: []Content{{Type: "text", Text: `{
  "execution_id": "3eaaa391-050b-4be1-a870-48a855923cb7",
  "tool": "exec",
  "status": "running"
}

本次等待已到达 timeout_seconds，上述 execution 仍未完成。可继续等待、取消，或采用其他步骤。`}},
				IsError: true,
			}, nil
		},
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	snap, err := service.Wait(context.Background(), handle.ID, 0)
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if snap == nil || snap.Execution == nil {
		t.Fatal("missing execution snapshot")
	}
	if snap.Execution.Status != ToolExecutionStatusCompleted {
		t.Fatalf("status = %q, want %q", snap.Execution.Status, ToolExecutionStatusCompleted)
	}
	if snap.Execution.Result == nil || !snap.Execution.Result.IsError {
		t.Fatal("model-facing result should remain IsError")
	}
}
