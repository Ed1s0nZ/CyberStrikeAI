package multiagent

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/cloudwego/eino/compose"
)

func TestModelOutputExecutionGuardMiddlewareBlocksLegacyRecoveryMarker(t *testing.T) {
	called := false
	markerJSON := `{"` + modelOutputRecoveryKey + `":{"reason":"invalid_tool_arguments_json","repair_attempt":1}}`
	wrapped := modelOutputExecutionGuardMiddleware().Invokable(func(context.Context, *compose.ToolInput) (*compose.ToolOutput, error) {
		called = true
		return &compose.ToolOutput{Result: "executed"}, nil
	})

	out, err := wrapped(context.Background(), &compose.ToolInput{Name: "task", Arguments: markerJSON})
	if err != nil {
		t.Fatalf("guard returned error: %v", err)
	}
	if called {
		t.Fatal("legacy recovery marker should not reach the real tool endpoint")
	}
	if out == nil || !strings.HasPrefix(out.Result, modelOutputRejectedResultPrefix) {
		t.Fatalf("output = %#v, want legacy rejected result", out)
	}
}

func TestModelOutputExecutionGuardMiddlewarePassesNormalToolCall(t *testing.T) {
	called := false
	wrapped := modelOutputExecutionGuardMiddleware().Invokable(func(context.Context, *compose.ToolInput) (*compose.ToolOutput, error) {
		called = true
		return &compose.ToolOutput{Result: "executed"}, nil
	})

	out, err := wrapped(context.Background(), &compose.ToolInput{Name: "exec", Arguments: `{"command":"pwd"}`})
	if err != nil {
		t.Fatalf("guard returned error: %v", err)
	}
	if !called || out == nil || out.Result != "executed" {
		t.Fatalf("called=%v output=%#v, want normal execution", called, out)
	}
}

func TestModelOutputExecutionGuardMiddlewareBlocksLegacyRecoveryMarkerStream(t *testing.T) {
	markerJSON := `{"` + modelOutputRecoveryKey + `":{"reason":"shell_command_too_large","repair_attempt":1}}`
	wrapped := modelOutputExecutionGuardMiddleware().Streamable(func(context.Context, *compose.ToolInput) (*compose.StreamToolOutput, error) {
		t.Fatal("legacy recovery marker should not reach the stream endpoint")
		return nil, nil
	})

	out, err := wrapped(context.Background(), &compose.ToolInput{Name: "execute", Arguments: markerJSON})
	if err != nil {
		t.Fatalf("guard returned error: %v", err)
	}
	if out == nil || out.Result == nil {
		t.Fatal("expected stream output")
	}
	got, recvErr := out.Result.Recv()
	if recvErr != nil && recvErr != io.EOF {
		t.Fatalf("recv: %v", recvErr)
	}
	if !strings.HasPrefix(got, modelOutputRejectedResultPrefix) {
		t.Fatalf("stream output = %q, want legacy rejected result", got)
	}
}
