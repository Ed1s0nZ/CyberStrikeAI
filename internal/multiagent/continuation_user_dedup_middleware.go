package multiagent

import (
	"context"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

// continuationSessionMarker matches Cursor / IDE session-resume user injections.
const continuationSessionMarker = "This session is being continued from a previous conversation"

// continuationUserDedupMiddleware keeps only the latest session-resume user message when
// multiple continuation injections were stacked (e.g. after repeated out-of-context resumes).
type continuationUserDedupMiddleware struct {
	adk.BaseChatModelAgentMiddleware
	logger *zap.Logger
	phase  string
}

func newContinuationUserDedupMiddleware(logger *zap.Logger, phase string) adk.ChatModelAgentMiddleware {
	return &continuationUserDedupMiddleware{logger: logger, phase: phase}
}

func (m *continuationUserDedupMiddleware) BeforeModelRewriteState(
	ctx context.Context,
	state *adk.ChatModelAgentState,
	mc *adk.ModelContext,
) (context.Context, *adk.ChatModelAgentState, error) {
	_ = mc
	if m == nil || state == nil || len(state.Messages) == 0 {
		return ctx, state, nil
	}
	deduped, dropped := dedupContinuationUserMessages(state.Messages)
	if dropped == 0 {
		return ctx, state, nil
	}
	if m.logger != nil {
		m.logger.Info("eino continuation user messages deduplicated",
			zap.String("phase", m.phase),
			zap.Int("dropped", dropped),
			zap.Int("messages_before", len(state.Messages)),
			zap.Int("messages_after", len(deduped)),
		)
	}
	out := *state
	out.Messages = deduped
	return ctx, &out, nil
}

func adkUserMessageText(msg adk.Message) string {
	if msg == nil {
		return ""
	}
	var b strings.Builder
	if s := strings.TrimSpace(msg.Content); s != "" {
		b.WriteString(s)
	}
	for _, part := range msg.UserInputMultiContent {
		if part.Type == schema.ChatMessagePartTypeText {
			if s := strings.TrimSpace(part.Text); s != "" {
				if b.Len() > 0 {
					b.WriteByte('\n')
				}
				b.WriteString(s)
			}
		}
	}
	return b.String()
}

func isContinuationUserMessage(msg adk.Message) bool {
	if msg == nil || msg.Role != schema.User {
		return false
	}
	return strings.Contains(adkUserMessageText(msg), continuationSessionMarker)
}

func dedupContinuationUserMessages(msgs []adk.Message) ([]adk.Message, int) {
	lastIdx := -1
	contCount := 0
	for i, msg := range msgs {
		if !isContinuationUserMessage(msg) {
			continue
		}
		contCount++
		lastIdx = i
	}
	if contCount <= 1 {
		return msgs, 0
	}
	out := make([]adk.Message, 0, len(msgs)-(contCount-1))
	dropped := 0
	for i, msg := range msgs {
		if isContinuationUserMessage(msg) && i != lastIdx {
			dropped++
			continue
		}
		out = append(out, msg)
	}
	return out, dropped
}
