package multiagent

import (
	"context"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

// systemMessageNormalizerMiddleware merges duplicate role=system messages into a single
// leading system message before summarization and each ChatModel call.
type systemMessageNormalizerMiddleware struct {
	adk.BaseChatModelAgentMiddleware
	logger *zap.Logger
	phase  string
}

func newSystemMessageNormalizerMiddleware(logger *zap.Logger, phase string) adk.ChatModelAgentMiddleware {
	return &systemMessageNormalizerMiddleware{logger: logger, phase: phase}
}

func (m *systemMessageNormalizerMiddleware) BeforeModelRewriteState(
	ctx context.Context,
	state *adk.ChatModelAgentState,
	mc *adk.ModelContext,
) (context.Context, *adk.ChatModelAgentState, error) {
	_ = mc
	if m == nil || state == nil || len(state.Messages) == 0 {
		return ctx, state, nil
	}
	before := countADKSystemMessages(state.Messages)
	if before <= 1 {
		return ctx, state, nil
	}
	normalized := normalizeSingleLeadingSystemMessage(state.Messages, "")
	if len(normalized) == len(state.Messages) && countADKSystemMessages(normalized) >= before {
		return ctx, state, nil
	}
	if m.logger != nil {
		m.logger.Info("eino system messages merged",
			zap.String("phase", m.phase),
			zap.Int("system_before", before),
			zap.Int("system_after", countADKSystemMessages(normalized)),
			zap.Int("messages_before", len(state.Messages)),
			zap.Int("messages_after", len(normalized)),
		)
	}
	out := *state
	out.Messages = normalized
	return ctx, &out, nil
}

func countADKSystemMessages(msgs []adk.Message) int {
	n := 0
	for _, msg := range msgs {
		if msg != nil && msg.Role == schema.System {
			n++
		}
	}
	return n
}

// stripADKSystemMessages removes all system messages. Use before runner.Run restart when
// genModelInput will prepend a fresh Instruction.
func stripADKSystemMessages(msgs []adk.Message) []adk.Message {
	if len(msgs) == 0 {
		return msgs
	}
	out := make([]adk.Message, 0, len(msgs))
	for _, msg := range msgs {
		if msg == nil || msg.Role == schema.System {
			continue
		}
		out = append(out, msg)
	}
	return out
}

// mergeCollectedSystemMessages collapses multiple system messages into one (or none).
func mergeCollectedSystemMessages(systemMsgs []adk.Message) []adk.Message {
	if len(systemMsgs) == 0 {
		return nil
	}
	return normalizeSingleLeadingSystemMessage(systemMsgs, "")
}
