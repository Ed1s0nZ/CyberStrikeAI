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
	logger           *zap.Logger
	modelName        string
	conversationID   string
	phase            string
	dynamicToolNames map[string]struct{}
}

func newEinoModelInputTelemetryMiddleware(
	logger *zap.Logger,
	modelName string,
	conversationID string,
	phase string,
	dynamicToolNames map[string]struct{},
) adk.ChatModelAgentMiddleware {
	if logger == nil {
		return nil
	}
	return &einoModelInputTelemetryMiddleware{
		logger:           logger,
		modelName:        strings.TrimSpace(modelName),
		conversationID:   strings.TrimSpace(conversationID),
		phase:            strings.TrimSpace(phase),
		dynamicToolNames: dynamicToolNames,
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
	// tool_search 中间件在模型调用前会把未解锁的动态工具从实际请求中移除，
	// 这里按同一规则过滤后再估算，避免把全量工具定义计入日志。
	tools := modelFacingTools(mcTools(mc), m.dynamicToolNames, state.Messages)
	tokens := estimateTokensForMessagesAndTools(ctx, m.modelName, state.Messages, tools)
	m.logger.Info("eino model input estimated",
		zap.String("phase", m.phase),
		zap.String("conversation_id", m.conversationID),
		zap.Int("messages", len(state.Messages)),
		zap.Int("tools", len(tools)),
		zap.Int("input_tokens_estimated", tokens),
	)
	return ctx, state, nil
}

// modelFacingTools 返回 tool_search 实际会随请求发送的工具：
// 静态工具 + tool_search 本身 + 已通过 tool_search 解锁的动态工具。
func modelFacingTools(all []*schema.ToolInfo, dynamicNames map[string]struct{}, msgs []adk.Message) []*schema.ToolInfo {
	if len(dynamicNames) == 0 || len(all) == 0 {
		return all
	}
	selected := selectedDynamicToolNames(msgs)
	ret := make([]*schema.ToolInfo, 0, len(all))
	for _, info := range all {
		if info == nil {
			continue
		}
		if _, dyn := dynamicNames[strings.ToLower(strings.TrimSpace(info.Name))]; dyn {
			if _, ok := selected[info.Name]; !ok {
				continue
			}
		}
		ret = append(ret, info)
	}
	return ret
}

// selectedDynamicToolNames 从历史消息中提取 tool_search 工具返回的已选工具名。
func selectedDynamicToolNames(msgs []adk.Message) map[string]struct{} {
	selected := make(map[string]struct{})
	for _, msg := range msgs {
		if msg.Role != schema.Tool {
			continue
		}
		var r struct {
			SelectedTools []string `json:"selectedTools"`
		}
		if err := sonic.Unmarshal([]byte(msg.Content), &r); err != nil || len(r.SelectedTools) == 0 {
			continue
		}
		for _, n := range r.SelectedTools {
			if t := strings.TrimSpace(n); t != "" {
				selected[t] = struct{}{}
			}
		}
	}
	return selected
}

// toolNameSet 将工具名列表转为小写集合（供 telemetry 过滤动态工具用）。
func toolNameSet(names []string) map[string]struct{} {
	if len(names) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(names))
	for _, n := range names {
		if t := strings.ToLower(strings.TrimSpace(n)); t != "" {
			set[t] = struct{}{}
		}
	}
	return set
}

// mcTools 返回模型上下文中的工具列表。
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

