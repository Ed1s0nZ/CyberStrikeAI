package multiagent

import (
	"context"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"go.uber.org/zap"
)

type einoAgenticModelFactory func(context.Context) (model.AgenticModel, error)

type einoAgenticRuntimeSupport struct {
	TypedRunner           bool
	Streaming             bool
	CancelMonitoring      bool
	ModelRetry            bool
	ModelFailover         bool
	ToolResultObservation bool
	MCPExecutionAudit     bool
}

type einoAgenticModelGate struct {
	Ready   bool
	Reason  string
	Missing []string
}

// Eino v0.9.14 wires AgenticMessage through the same generic TypedRunner,
// stream cancel monitoring, model retry, and model failover wrappers used by
// schema.Message. Keep this matrix explicit so future upgrades are audited
// deliberately instead of flipping the AgenticModel path by accident.
func einoAgenticRuntimeSupportV0914() einoAgenticRuntimeSupport {
	return einoAgenticRuntimeSupport{
		TypedRunner:           true,
		Streaming:             true,
		CancelMonitoring:      true,
		ModelRetry:            true,
		ModelFailover:         true,
		ToolResultObservation: true,
		MCPExecutionAudit:     true,
	}
}

func evaluateEinoAgenticModelGate(factory einoAgenticModelFactory, support einoAgenticRuntimeSupport) einoAgenticModelGate {
	missing := make([]string, 0, 8)
	if factory == nil {
		missing = append(missing, "model.AgenticModel backend")
	} else {
		if m, err := factory(context.Background()); err != nil || m == nil {
			missing = append(missing, "model.AgenticModel backend")
		}
	}
	if !support.TypedRunner {
		missing = append(missing, "adk.TypedRunner[*schema.AgenticMessage]")
	}
	if !support.Streaming {
		missing = append(missing, "AgenticMessage streaming")
	}
	if !support.CancelMonitoring {
		missing = append(missing, "AgenticMessage model-stream cancel monitoring")
	}
	if !support.ModelRetry {
		missing = append(missing, "AgenticMessage ModelRetry")
	}
	if !support.ModelFailover {
		missing = append(missing, "AgenticMessage ModelFailover")
	}
	if !support.ToolResultObservation {
		missing = append(missing, "AgenticMessage tool-result observation")
	}
	if !support.MCPExecutionAudit {
		missing = append(missing, "AgenticMessage MCP execution audit")
	}
	if len(missing) == 0 {
		return einoAgenticModelGate{Ready: true, Reason: "ready"}
	}
	return einoAgenticModelGate{
		Reason:  "agentic_model_not_ready: " + strings.Join(missing, ", "),
		Missing: missing,
	}
}

func logEinoAgenticModelGate(logger *zap.Logger, scope, orchestration string, gate einoAgenticModelGate) {
	if logger == nil {
		return
	}
	fields := []zap.Field{
		zap.String("scope", scope),
		zap.String("orchestration", orchestration),
		zap.Bool("ready", gate.Ready),
		zap.String("reason", gate.Reason),
		zap.Strings("missing", gate.Missing),
	}
	if gate.Ready {
		logger.Info("eino agentic model gate ready", fields...)
		return
	}
	logger.Info("eino agentic model gate disabled", fields...)
}

func agenticTextModelFactory(m model.AgenticModel) einoAgenticModelFactory {
	if m == nil {
		return nil
	}
	return func(context.Context) (model.AgenticModel, error) {
		return m, nil
	}
}
