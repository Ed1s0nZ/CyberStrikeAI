package multiagent

import (
	"context"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

type einoToolResultEventHandlerConfig struct {
	Context         context.Context
	Logger          *zap.Logger
	RunMessages     *einoRunMessageAccumulator
	Emitter         *einoToolResultProgressEmitter
	ConfirmRecovery func()
}

type einoToolResultEventHandler struct {
	ctx             context.Context
	logger          *zap.Logger
	runMessages     *einoRunMessageAccumulator
	emitter         *einoToolResultProgressEmitter
	confirmRecovery func()
}

func newEinoToolResultEventHandler(cfg einoToolResultEventHandlerConfig) *einoToolResultEventHandler {
	if cfg.Context == nil {
		cfg.Context = context.Background()
	}
	return &einoToolResultEventHandler{
		ctx:             cfg.Context,
		logger:          cfg.Logger,
		runMessages:     cfg.RunMessages,
		emitter:         cfg.Emitter,
		confirmRecovery: cfg.ConfirmRecovery,
	}
}

func (h *einoToolResultEventHandler) HandleStreaming(mv *adk.MessageVariant, agentName string) bool {
	if h == nil || mv == nil || !mv.IsStreaming || mv.MessageStream == nil || mv.Role != schema.Tool {
		return false
	}
	toolName := strings.TrimSpace(mv.ToolName)
	content, streamToolCallID, streamToolName, recvErr := recvSchemaMessageStream(h.ctx, mv.MessageStream)
	if toolName == "" {
		toolName = streamToolName
	}
	isErr := einoToolResultIsError(toolName, content)
	content = einoToolResultBody(content)
	if streamToolCallID != "" && h.runMessages != nil {
		h.runMessages.AppendToolMessage(content, streamToolCallID, schema.WithToolName(toolName))
	}
	if h.emitter != nil {
		h.emitter.Emit(h.ctx, toolName, content, streamToolCallID, isErr, agentName)
	}
	if recvErr != nil && h.logger != nil {
		h.logger.Warn("eino tool result stream recv error",
			zap.Error(recvErr),
			zap.String("agent", agentName),
			zap.String("tool", toolName))
	}
	if recvErr == nil && h.confirmRecovery != nil {
		h.confirmRecovery()
	}
	return true
}

func (h *einoToolResultEventHandler) HandleMaterialized(mv *adk.MessageVariant, msg adk.Message, agentName string) bool {
	if h == nil || mv == nil || msg == nil || (mv.Role != schema.Tool && msg.Role != schema.Tool) {
		return false
	}
	toolName := msg.ToolName
	if toolName == "" {
		toolName = mv.ToolName
	}
	content := msg.Content
	isErr := einoToolResultIsError(toolName, content)
	content = einoToolResultBody(content)
	toolCallID := strings.TrimSpace(msg.ToolCallID)
	if h.emitter != nil {
		h.emitter.Emit(h.ctx, toolName, content, toolCallID, isErr, agentName)
	}
	return true
}
