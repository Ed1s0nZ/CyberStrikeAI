package multiagent

import (
	"context"
	"fmt"
	"strings"

	"cyberstrike-ai/internal/agent"
	"cyberstrike-ai/internal/config"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/middlewares/summarization"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

// einoSummarizeUserInstruction 与单 Agent MemoryCompressor 目标一致：压缩时保留渗透关键信息。
const einoSummarizeUserInstruction = `在保持所有关键安全测试信息完整的前提下压缩对话历史。

必须保留：已确认漏洞与攻击路径、工具输出中的核心发现、凭证与认证细节、架构与薄弱点、当前进度、失败尝试与死路、策略决策。
保留精确技术细节（URL、路径、参数、Payload、版本号、报错原文可摘要但要点不丢）。
将冗长扫描输出概括为结论；重复发现合并表述。

输出须使后续代理能无缝继续同一授权测试任务。`

// newEinoSummarizationMiddleware 使用 Eino ADK Summarization 中间件（见 https://www.cloudwego.io/zh/docs/eino/core_modules/eino_adk/eino_adk_chatmodelagentmiddleware/middleware_summarization/）。
// 触发阈值与单 Agent MemoryCompressor 一致：当估算 token 超过 openai.max_total_tokens 的 90% 时摘要。
func newEinoSummarizationMiddleware(
	ctx context.Context,
	summaryModel model.BaseChatModel,
	appCfg *config.Config,
	logger *zap.Logger,
) (adk.ChatModelAgentMiddleware, error) {
	if summaryModel == nil || appCfg == nil {
		return nil, fmt.Errorf("multiagent: summarization 需要 model 与配置")
	}
	maxTotal := appCfg.OpenAI.MaxTotalTokens
	if maxTotal <= 0 {
		maxTotal = 120000
	}
	trigger := int(float64(maxTotal) * 0.9)
	if trigger < 4096 {
		trigger = maxTotal
		if trigger < 4096 {
			trigger = 4096
		}
	}
	preserveMax := trigger / 3
	if preserveMax < 2048 {
		preserveMax = 2048
	}

	modelName := strings.TrimSpace(appCfg.OpenAI.Model)
	if modelName == "" {
		modelName = "gpt-4o"
	}

	mw, err := summarization.New(ctx, &summarization.Config{
		Model: summaryModel,
		Trigger: &summarization.TriggerCondition{
			ContextTokens: trigger,
		},
		TokenCounter:       einoSummarizationTokenCounter(modelName),
		UserInstruction:    einoSummarizeUserInstruction,
		EmitInternalEvents: false,
		PreserveUserMessages: &summarization.PreserveUserMessages{
			Enabled:   true,
			MaxTokens: preserveMax,
		},
		Callback: func(ctx context.Context, before, after adk.ChatModelAgentState) error {
			if logger == nil {
				return nil
			}
			logger.Info("eino summarization 已压缩上下文",
				zap.Int("messages_before", len(before.Messages)),
				zap.Int("messages_after", len(after.Messages)),
				zap.Int("max_total_tokens", maxTotal),
				zap.Int("trigger_context_tokens", trigger),
			)
			return nil
		},
	})
	if err != nil {
		return nil, fmt.Errorf("summarization.New: %w", err)
	}
	return mw, nil
}

func einoSummarizationTokenCounter(openAIModel string) summarization.TokenCounterFunc {
	tc := agent.NewTikTokenCounter()
	return func(ctx context.Context, input *summarization.TokenCounterInput) (int, error) {
		var sb strings.Builder
		for _, msg := range input.Messages {
			if msg == nil {
				continue
			}
			sb.WriteString(string(msg.Role))
			sb.WriteByte('\n')
			if msg.Content != "" {
				sb.WriteString(msg.Content)
				sb.WriteByte('\n')
			}
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
			for _, part := range msg.UserInputMultiContent {
				if part.Type == schema.ChatMessagePartTypeText && part.Text != "" {
					sb.WriteString(part.Text)
					sb.WriteByte('\n')
				}
			}
		}
		for _, tl := range input.Tools {
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
		n, err := tc.Count(openAIModel, text)
		if err != nil {
			return (len(text) + 3) / 4, nil
		}
		return n, nil
	}
}
