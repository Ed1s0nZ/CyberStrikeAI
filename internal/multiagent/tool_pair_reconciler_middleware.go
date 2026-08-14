package multiagent

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

const patchedMissingToolResult = "[Tool execution result was lost or interrupted; continue without relying on this call.]"

// toolPairReconcilerMiddleware is the final structural guard before a model call.
// It makes every assistant tool-call batch immediately followed by exactly one tool
// result per call ID, and drops tool messages that cannot belong to that batch.
//
// This intentionally runs after summarization/reduction/budget middleware: those
// middlewares rewrite history and can otherwise re-introduce a partial tool round
// after the ordinary patchtoolcalls middleware has already run.
type toolPairReconcilerMiddleware struct {
	adk.BaseChatModelAgentMiddleware
	logger *zap.Logger
	phase  string
}

func newToolPairReconcilerMiddleware(logger *zap.Logger, phase string) adk.ChatModelAgentMiddleware {
	return &toolPairReconcilerMiddleware{logger: logger, phase: phase}
}

func (m *toolPairReconcilerMiddleware) BeforeModelRewriteState(
	ctx context.Context,
	state *adk.ChatModelAgentState,
	mc *adk.ModelContext,
) (context.Context, *adk.ChatModelAgentState, error) {
	_ = mc
	if m == nil || state == nil || len(state.Messages) == 0 {
		return ctx, state, nil
	}

	usedIDs := make(map[string]struct{}, 16)
	changed := false
	patched := 0
	dropped := 0
	out := make([]adk.Message, 0, len(state.Messages))

	for i := 0; i < len(state.Messages); {
		msg := state.Messages[i]
		if msg == nil {
			changed = true
			i++
			continue
		}
		if msg.Role == schema.Tool {
			// Valid tool results are consumed with their immediately preceding assistant.
			changed = true
			dropped++
			i++
			continue
		}
		if msg.Role != schema.Assistant || len(msg.ToolCalls) == 0 {
			out = append(out, msg)
			i++
			continue
		}

		assistant := msg
		calls := append([]schema.ToolCall(nil), msg.ToolCalls...)
		expected := make(map[string]schema.ToolCall, len(calls))
		idsChanged := false
		for callIndex := range calls {
			id := calls[callIndex].ID
			_, duplicate := usedIDs[id]
			if id == "" || duplicate {
				base := fmt.Sprintf("patched_tool_call_%d_%d", i, callIndex)
				id = base
				for suffix := 1; ; suffix++ {
					if _, exists := usedIDs[id]; !exists {
						break
					}
					id = fmt.Sprintf("%s_%d", base, suffix)
				}
				calls[callIndex].ID = id
				idsChanged = true
				changed = true
			}
			usedIDs[id] = struct{}{}
			expected[id] = calls[callIndex]
		}
		if idsChanged {
			cloned := *msg
			cloned.ToolCalls = calls
			assistant = &cloned
		}
		out = append(out, assistant)

		results := make(map[string]adk.Message, len(calls))
		j := i + 1
		for j < len(state.Messages) {
			toolMsg := state.Messages[j]
			if toolMsg == nil {
				changed = true
				j++
				continue
			}
			if toolMsg.Role != schema.Tool {
				break
			}
			id := toolMsg.ToolCallID
			if _, wanted := expected[id]; !wanted {
				changed = true
				dropped++
				j++
				continue
			}
			if _, duplicate := results[id]; duplicate {
				changed = true
				dropped++
				j++
				continue
			}
			results[id] = toolMsg
			j++
		}
		for _, tc := range calls {
			if result, ok := results[tc.ID]; ok {
				out = append(out, result)
				continue
			}
			out = append(out, schema.ToolMessage(
				patchedMissingToolResult,
				tc.ID,
				schema.WithToolName(tc.Function.Name),
			))
			changed = true
			patched++
		}
		i = j
	}

	if !changed {
		return ctx, state, nil
	}
	if m.logger != nil {
		m.logger.Warn("eino tool-call/result pairs reconciled before model call",
			zap.String("phase", m.phase),
			zap.Int("patched_results", patched),
			zap.Int("dropped_results", dropped),
			zap.Int("messages_before", len(state.Messages)),
			zap.Int("messages_after", len(out)),
		)
	}
	ns := *state
	ns.Messages = out
	return ctx, &ns, nil
}
