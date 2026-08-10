package workflow

import (
	"context"

	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/database"
	"cyberstrike-ai/internal/einoobserve"

	"go.uber.org/zap"
)

func attachWorkflowCallbacks(ctx context.Context, cfg *config.Config, args RunArgs, workflowName string) context.Context {
	if cfg == nil {
		return ctx
	}
	cbCfg := &cfg.MultiAgent.EinoCallbacks
	var usageRecorder func(einoobserve.UsageRecord)
	if args.DB != nil {
		usageRecorder = func(rec einoobserve.UsageRecord) {
			if err := args.DB.RecordLLMUsage(database.LLMUsage{
				ConversationID:   rec.ConversationID,
				Phase:            rec.Phase,
				Model:            rec.Model,
				PromptTokens:     rec.PromptTokens,
				CompletionTokens: rec.CompletionTokens,
				TotalTokens:      rec.TotalTokens,
				CachedTokens:     rec.CachedTokens,
				ReasoningTokens:  rec.ReasoningTokens,
				CreatedAt:        rec.CreatedAt,
			}); err != nil && args.Logger != nil {
				args.Logger.Warn("记录 LLM usage 失败",
					zap.String("conversation_id", rec.ConversationID),
					zap.String("phase", rec.Phase),
					zap.Error(err))
			}
		}
	}
	return einoobserve.AttachAgentRunCallbacks(ctx, cbCfg, einoobserve.Params{
		Logger:           args.Logger,
		Progress:         args.Progress,
		ConversationID:   args.ConversationID,
		OrchMode:         "workflow",
		OrchestratorName: workflowName,
		ModelName:        cfg.OpenAI.Model,
		UsageRecorder:    usageRecorder,
	})
}
