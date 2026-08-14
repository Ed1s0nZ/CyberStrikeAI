package multiagent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

const (
	modelOutputRecoveryKey          = "_cyberstrike_model_output_recovery"
	modelOutputRejectedResultPrefix = "[Model Output Rejected]"
)

type modelOutputRecoveryMarker struct {
	Reason        string `json:"reason"`
	RepairAttempt int    `json:"repair_attempt"`
}

// modelOutputExecutionGuardMiddleware is a compatibility shim for old persisted
// recovery-marker tool calls. New runs should let the tool layer return normal
// soft errors to the model instead of pre-rewriting model output.
func modelOutputExecutionGuardMiddleware() compose.ToolMiddleware {
	messageFor := func(input *compose.ToolInput) (string, bool) {
		if input == nil {
			return "", false
		}
		var envelope map[string]json.RawMessage
		if json.Unmarshal([]byte(input.Arguments), &envelope) != nil {
			return "", false
		}
		raw, ok := envelope[modelOutputRecoveryKey]
		if !ok {
			return "", false
		}
		var marker modelOutputRecoveryMarker
		_ = json.Unmarshal(raw, &marker)
		return fmt.Sprintf("%s Tool call '%s' was not executed because it is a legacy model-output recovery marker (%s). Repair attempt %d.",
			modelOutputRejectedResultPrefix, input.Name, marker.Reason, marker.RepairAttempt), true
	}
	return compose.ToolMiddleware{
		Invokable: func(next compose.InvokableToolEndpoint) compose.InvokableToolEndpoint {
			return func(ctx context.Context, input *compose.ToolInput) (*compose.ToolOutput, error) {
				if msg, reject := messageFor(input); reject {
					return &compose.ToolOutput{Result: msg}, nil
				}
				return next(ctx, input)
			}
		},
		Streamable: func(next compose.StreamableToolEndpoint) compose.StreamableToolEndpoint {
			return func(ctx context.Context, input *compose.ToolInput) (*compose.StreamToolOutput, error) {
				if msg, reject := messageFor(input); reject {
					return &compose.StreamToolOutput{Result: schema.StreamReaderFromArray([]string{msg})}, nil
				}
				return next(ctx, input)
			}
		},
	}
}

func modelOutputRecoveryFromToolCall(tc schema.ToolCall) (modelOutputRecoveryMarker, bool) {
	var envelope map[string]json.RawMessage
	if json.Unmarshal([]byte(tc.Function.Arguments), &envelope) != nil {
		return modelOutputRecoveryMarker{}, false
	}
	raw, ok := envelope[modelOutputRecoveryKey]
	if !ok {
		return modelOutputRecoveryMarker{}, false
	}
	var marker modelOutputRecoveryMarker
	if json.Unmarshal(raw, &marker) != nil {
		return modelOutputRecoveryMarker{}, false
	}
	return marker, strings.TrimSpace(marker.Reason) != "" || marker.RepairAttempt > 0
}
