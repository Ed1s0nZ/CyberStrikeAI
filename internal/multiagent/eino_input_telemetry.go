package multiagent

import (
	"context"
	"strings"

	"cyberstrike-ai/internal/agent"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

type einoModelInputTelemetryMiddleware struct {
	adk.BaseChatModelAgentMiddleware
	logger         *zap.Logger
	modelName      string
	conversationID string
	phase          string
}

func newEinoModelInputTelemetryMiddleware(
	logger *zap.Logger,
	modelName string,
	conversationID string,
	phase string,
) adk.ChatModelAgentMiddleware {
	if logger == nil {
		return nil
	}
	return &einoModelInputTelemetryMiddleware{
		logger:         logger,
		modelName:      strings.TrimSpace(modelName),
		conversationID: strings.TrimSpace(conversationID),
		phase:          strings.TrimSpace(phase),
	}
}

func (m *einoModelInputTelemetryMiddleware) BeforeModelRewriteState(
	ctx context.Context,
	state *adk.ChatModelAgentState,
	mc *adk.ModelContext,
) (context.Context, *adk.ChatModelAgentState, error) {
	if m == nil || m.logger == nil || state == nil {
		return ctx, state, nil
	}
	tokens := estimateTokensForMessagesAndTools(ctx, m.modelName, state.Messages, mcTools(mc))
	m.logger.Info("eino model input estimated",
		zap.String("phase", m.phase),
		zap.String("conversation_id", m.conversationID),
		zap.Int("messages", len(state.Messages)),
		zap.Int("tools", len(mcTools(mc))),
		zap.Int("input_tokens_estimated", tokens),
	)
	return ctx, state, nil
}

func mcTools(mc *adk.ModelContext) []*schema.ToolInfo {
	if mc == nil || len(mc.Tools) == 0 {
		return nil
	}
	return mc.Tools
}

func estimateTokensForMessagesAndTools(
	_ context.Context,
	modelName string,
	messages []adk.Message,
	tools []*schema.ToolInfo,
) int {
	var sb strings.Builder
	for _, msg := range messages {
		if msg == nil {
			continue
		}
		sb.WriteString(string(msg.Role))
		sb.WriteByte('\n')
		sb.WriteString(msg.Content)
		sb.WriteByte('\n')
		if msg.ReasoningContent != "" {
			sb.WriteString(msg.ReasoningContent)
			sb.WriteByte('\n')
		}
		if len(msg.ToolCalls) > 0 {
			if b, err := sonic.Marshal(msg.ToolCalls); err == nil {
				sb.Write(b)
				sb.WriteByte('\n')
			}
		}
	}
	for _, tl := range tools {
		if tl == nil {
			continue
		}
		cp := *tl
		cp.Extra = nil
		if text, err := sonic.MarshalString(cp); err == nil {
			sb.WriteString(text)
			sb.WriteByte('\n')
		}
	}
	text := sb.String()
	if text == "" {
		return 0
	}
	tc := agent.NewTikTokenCounter()
	if n, err := tc.Count(modelName, text); err == nil {
		return n
	}
	return (len(text) + 3) / 4
}

func logPlanExecuteModelInputEstimate(
	logger *zap.Logger,
	modelName string,
	conversationID string,
	phase string,
	msgs []adk.Message,
) {
	if logger == nil {
		return
	}
	tokens := estimateTokensForMessagesAndTools(context.Background(), modelName, msgs, nil)
	logger.Info("eino model input estimated",
		zap.String("phase", phase),
		zap.String("conversation_id", strings.TrimSpace(conversationID)),
		zap.Int("messages", len(msgs)),
		zap.Int("tools", 0),
		zap.Int("input_tokens_estimated", tokens),
	)
}

