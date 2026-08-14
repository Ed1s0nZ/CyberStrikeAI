package multiagent

import (
	"context"
	"errors"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type fakeAgenticGateModel struct{}

func (m *fakeAgenticGateModel) Generate(context.Context, []*schema.AgenticMessage, ...model.Option) (*schema.AgenticMessage, error) {
	return &schema.AgenticMessage{Role: schema.AgenticRoleTypeAssistant}, nil
}

func (m *fakeAgenticGateModel) Stream(context.Context, []*schema.AgenticMessage, ...model.Option) (*schema.StreamReader[*schema.AgenticMessage], error) {
	return schema.StreamReaderFromArray([]*schema.AgenticMessage{{Role: schema.AgenticRoleTypeAssistant}}), nil
}

func TestEinoAgenticModelGateV0914WaitsOnlyForBackend(t *testing.T) {
	gate := evaluateEinoAgenticModelGate(nil, einoAgenticRuntimeSupportV0914())

	if gate.Ready {
		t.Fatal("v0.9.14 gate should stay disabled without an AgenticModel backend")
	}
	if !containsString(gate.Missing, "model.AgenticModel backend") {
		t.Fatalf("missing = %#v, want backend reason", gate.Missing)
	}
	for _, unexpected := range []string{
		"AgenticMessage model-stream cancel monitoring",
		"AgenticMessage ModelRetry",
		"AgenticMessage ModelFailover",
		"AgenticMessage tool-result observation",
		"AgenticMessage MCP execution audit",
	} {
		if containsString(gate.Missing, unexpected) {
			t.Fatalf("missing = %#v, should not include %q for v0.9.14 runtime support", gate.Missing, unexpected)
		}
	}
}

func TestEinoAgenticModelGateV0914ReadyWithBackend(t *testing.T) {
	gate := evaluateEinoAgenticModelGate(agenticTextModelFactory(&fakeAgenticGateModel{}), einoAgenticRuntimeSupportV0914())

	if !gate.Ready {
		t.Fatalf("gate = %#v, want ready when v0.9.14 runtime support has a backend", gate)
	}
	if gate.Reason != "ready" || len(gate.Missing) != 0 {
		t.Fatalf("gate details = %#v", gate)
	}
}

func TestEinoAgenticModelGateReadyWhenBackendAndRuntimeParityExist(t *testing.T) {
	gate := evaluateEinoAgenticModelGate(agenticTextModelFactory(&fakeAgenticGateModel{}), einoAgenticRuntimeSupport{
		TypedRunner:           true,
		Streaming:             true,
		CancelMonitoring:      true,
		ModelRetry:            true,
		ModelFailover:         true,
		ToolResultObservation: true,
		MCPExecutionAudit:     true,
	})

	if !gate.Ready {
		t.Fatalf("gate = %#v, want ready", gate)
	}
	if gate.Reason != "ready" || len(gate.Missing) != 0 {
		t.Fatalf("gate details = %#v", gate)
	}
}

func TestEinoAgenticModelGateTreatsFactoryErrorAsMissingBackend(t *testing.T) {
	gate := evaluateEinoAgenticModelGate(func(context.Context) (model.AgenticModel, error) {
		return nil, errors.New("not implemented")
	}, einoAgenticRuntimeSupport{
		TypedRunner:           true,
		Streaming:             true,
		CancelMonitoring:      true,
		ModelRetry:            true,
		ModelFailover:         true,
		ToolResultObservation: true,
		MCPExecutionAudit:     true,
	})

	if gate.Ready {
		t.Fatal("factory error should disable gate")
	}
	if !containsString(gate.Missing, "model.AgenticModel backend") {
		t.Fatalf("missing = %#v, want backend reason", gate.Missing)
	}
}
