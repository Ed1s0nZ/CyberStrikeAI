package multiagent

import (
	"context"
	"testing"

	"cyberstrike-ai/internal/agent"
	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/einomcp"
	"cyberstrike-ai/internal/mcp"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

func TestEinoADKFilesystemToolMonitorBindsFinishesAndUpdatesDisplayResult(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	logger := zap.NewNop()
	server := mcp.NewServer(logger)
	ag := agent.NewAgent(&config.OpenAIConfig{}, &config.AgentConfig{}, server, nil, logger, 1)
	binder := NewMCPExecutionBinder()
	var recorded []string
	rec := einomcp.ExecutionRecorder(func(executionID, toolCallID string) {
		recorded = append(recorded, executionID+"|"+toolCallID)
	})

	beginEinoADKFilesystemToolMonitor(ctx, ag, rec, binder, "call-read", "read_file")
	execID := binder.ExecutionID("call-read")
	if execID == "" {
		t.Fatal("expected begin to bind execution id")
	}
	exec, ok := server.GetExecution(execID)
	if !ok || exec == nil || exec.Status != "running" || exec.ToolName != "eino_fs::read_file" {
		t.Fatalf("begin execution = %#v ok=%v", exec, ok)
	}
	if len(recorded) != 1 || recorded[0] != execID+"|call-read" {
		t.Fatalf("recorded begin ids = %#v", recorded)
	}

	runMessages := newEinoRunMessageAccumulator([]adk.Message{
		&schema.Message{
			Role: schema.Assistant,
			ToolCalls: []schema.ToolCall{{
				ID:   "call-read",
				Type: "function",
				Function: schema.FunctionCall{
					Name:      "read_file",
					Arguments: `{"path":"/tmp/secret.txt"}`,
				},
			}},
		},
	})
	emitter := newEinoToolResultProgressEmitter(einoToolResultProgressEmitterConfig{
		ConversationID:          "conv-1",
		RunMessages:             runMessages,
		FilesystemMonitorAgent:  ag,
		FilesystemMonitorRecord: rec,
		MCPExecutionBinder:      binder,
	})

	if !emitter.Emit(ctx, "read_file", "model-facing truncated body", "call-read", false, "lead") {
		t.Fatal("expected tool_result emit")
	}
	exec, ok = server.GetExecution(execID)
	if !ok || exec == nil {
		t.Fatalf("finished execution missing: ok=%v exec=%#v", ok, exec)
	}
	if exec.Status != "completed" || exec.ToolName != "eino_fs::read_file" {
		t.Fatalf("finished execution status/name = %#v", exec)
	}
	if got, _ := exec.Arguments["path"].(string); got != "/tmp/secret.txt" {
		t.Fatalf("execution args = %#v", exec.Arguments)
	}
	if exec.Result == nil || len(exec.Result.Content) != 1 || exec.Result.Content[0].Text != "model-facing truncated body" {
		t.Fatalf("execution display result = %#v", exec.Result)
	}
	if len(recorded) != 1 {
		t.Fatalf("finish should reuse existing execution without recording a second id, got %#v", recorded)
	}
}
