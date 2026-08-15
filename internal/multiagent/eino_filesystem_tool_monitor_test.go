package multiagent

import (
	"context"
	"strings"
	"testing"

	"cyberstrike-ai/internal/agent"
	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/einomcp"
	"cyberstrike-ai/internal/mcp"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
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

	beginEinoADKFilesystemToolMonitor(ctx, ag, rec, binder, "call-read", "read_file", map[string]interface{}{"path": "/tmp/secret.txt"})
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
	if got, _ := exec.Arguments["path"].(string); got != "/tmp/secret.txt" {
		t.Fatalf("begin execution args = %#v", exec.Arguments)
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

func TestEinoADKFilesystemToolMonitorSpillsLargeReadFileResultForProgress(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	logger := zap.NewNop()
	server := mcp.NewServer(logger)
	server.ConfigureToolResultMaxBytes(400)
	server.ConfigureToolResultSpillRoot(t.TempDir())
	ag := agent.NewAgent(&config.OpenAIConfig{}, &config.AgentConfig{}, server, nil, logger, 1)
	binder := NewMCPExecutionBinder()
	rec := einomcp.ExecutionRecorder(func(executionID, toolCallID string) {})
	var event map[string]interface{}

	runMessages := newEinoRunMessageAccumulator([]adk.Message{
		&schema.Message{
			Role: schema.Assistant,
			ToolCalls: []schema.ToolCall{{
				ID:   "call-read",
				Type: "function",
				Function: schema.FunctionCall{
					Name:      "read_file",
					Arguments: `{"path":"/tmp/large.txt"}`,
				},
			}},
		},
	})
	beginEinoADKFilesystemToolMonitor(ctx, ag, rec, binder, "call-read", "read_file", map[string]interface{}{"path": "/tmp/large.txt"})
	emitter := newEinoToolResultProgressEmitter(einoToolResultProgressEmitterConfig{
		ConversationID:          "conv-1",
		RunMessages:             runMessages,
		FilesystemMonitorAgent:  ag,
		FilesystemMonitorRecord: rec,
		MCPExecutionBinder:      binder,
		Progress: func(eventType, _ string, data interface{}) {
			if eventType == "tool_result" {
				event, _ = data.(map[string]interface{})
			}
		},
	})

	if !emitter.Emit(ctx, "read_file", strings.Repeat("0123456789", 100), "call-read", false, "lead") {
		t.Fatal("expected tool result emit")
	}
	result, _ := event["result"].(string)
	if !strings.Contains(result, "<persisted-output>") || !strings.Contains(result, "Full output saved to:") {
		t.Fatalf("large read_file result was not spilled in progress event: %q", result)
	}
	if len(result) > 400 {
		t.Fatalf("progress result exceeded configured max: len=%d text=%q", len(result), result)
	}
}

func TestEinoAgenticFilesystemWrapperCapturesArgumentsAndSpillsResult(t *testing.T) {
	t.Parallel()
	binder := NewMCPExecutionBinder()
	mw := &einoAgenticFilesystemToolMiddleware{
		TypedChatModelAgentMiddleware: &adk.TypedBaseChatModelAgentMiddleware[*schema.AgenticMessage]{},
		conversationID:                "conv-1",
		toolMaxBytes:                  400,
		reductionRootDir:              t.TempDir(),
		binder:                        binder,
	}
	endpoint, err := mw.WrapInvokableToolCall(context.Background(), func(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
		return strings.Repeat("0123456789", 100), nil
	}, &adk.ToolContext{Name: "read_file", CallID: "call-read"})
	if err != nil {
		t.Fatalf("WrapInvokableToolCall: %v", err)
	}
	result, err := endpoint(context.Background(), `{"file_path":"/tmp/requirements.txt","limit":2000}`)
	if err != nil {
		t.Fatalf("endpoint: %v", err)
	}
	args := binder.Arguments("call-read")
	if args["file_path"] != "/tmp/requirements.txt" {
		t.Fatalf("captured args = %#v", args)
	}
	if !strings.Contains(result, "<persisted-output>") || !strings.Contains(result, "Full output saved to:") {
		t.Fatalf("expected persisted-output summary, got %q", result)
	}
	if len(result) > 400 {
		t.Fatalf("summary exceeded max bytes: len=%d", len(result))
	}
}
