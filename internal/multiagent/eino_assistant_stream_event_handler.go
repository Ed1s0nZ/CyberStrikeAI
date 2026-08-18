package multiagent

import (
	"context"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

type einoAssistantStreamEventHandlerConfig struct {
	Context                   context.Context
	ConversationID            string
	OrchMode                  string
	Progress                  func(eventType, message string, data interface{})
	Logger                    *zap.Logger
	SnapshotMCPIDs            func() []string
	StreamsMainAssistant      func(agent string) bool
	EinoRoleTag               func(agent string) string
	RunProgress               *einoRunProgressTracker
	StdoutSuppressor          *einoExecuteStdoutSuppressor
	AssistantOutput           *einoAssistantOutputAccumulator
	RunMessages               *einoRunMessageAccumulator
	Usage                     *einoRunUsageAccumulator
	ToolCallCompletion        *einoStreamToolCallCompletionHandler
	NextMainStreamID          func() string
	NextReasoningStreamID     func() string
	NextSubAgentReplyStreamID func() string
}

type einoAssistantStreamEventHandler struct {
	ctx                       context.Context
	conversationID            string
	orchMode                  string
	progress                  func(eventType, message string, data interface{})
	logger                    *zap.Logger
	snapshotMCPIDs            func() []string
	streamsMainAssistant      func(agent string) bool
	einoRoleTag               func(agent string) string
	runProgress               *einoRunProgressTracker
	stdoutSuppressor          *einoExecuteStdoutSuppressor
	assistantOutput           *einoAssistantOutputAccumulator
	runMessages               *einoRunMessageAccumulator
	usage                     *einoRunUsageAccumulator
	toolCallCompletion        *einoStreamToolCallCompletionHandler
	nextMainStreamID          func() string
	nextReasoningStreamID     func() string
	nextSubAgentReplyStreamID func() string
}

func newEinoAssistantStreamEventHandler(cfg einoAssistantStreamEventHandlerConfig) *einoAssistantStreamEventHandler {
	if cfg.Context == nil {
		cfg.Context = context.Background()
	}
	if cfg.SnapshotMCPIDs == nil {
		cfg.SnapshotMCPIDs = func() []string { return nil }
	}
	if cfg.StreamsMainAssistant == nil {
		cfg.StreamsMainAssistant = func(string) bool { return true }
	}
	if cfg.EinoRoleTag == nil {
		cfg.EinoRoleTag = func(string) string { return "" }
	}
	if cfg.NextMainStreamID == nil {
		cfg.NextMainStreamID = func() string { return "eino-main" }
	}
	if cfg.NextReasoningStreamID == nil {
		cfg.NextReasoningStreamID = func() string { return "eino-reasoning" }
	}
	if cfg.NextSubAgentReplyStreamID == nil {
		cfg.NextSubAgentReplyStreamID = func() string { return "eino-sub-reply" }
	}
	return &einoAssistantStreamEventHandler{
		ctx:                       cfg.Context,
		conversationID:            cfg.ConversationID,
		orchMode:                  cfg.OrchMode,
		progress:                  cfg.Progress,
		logger:                    cfg.Logger,
		snapshotMCPIDs:            cfg.SnapshotMCPIDs,
		streamsMainAssistant:      cfg.StreamsMainAssistant,
		einoRoleTag:               cfg.EinoRoleTag,
		runProgress:               cfg.RunProgress,
		stdoutSuppressor:          cfg.StdoutSuppressor,
		assistantOutput:           cfg.AssistantOutput,
		runMessages:               cfg.RunMessages,
		usage:                     cfg.Usage,
		toolCallCompletion:        cfg.ToolCallCompletion,
		nextMainStreamID:          cfg.NextMainStreamID,
		nextReasoningStreamID:     cfg.NextReasoningStreamID,
		nextSubAgentReplyStreamID: cfg.NextSubAgentReplyStreamID,
	}
}

func (h *einoAssistantStreamEventHandler) Handle(mv *adk.MessageVariant, agentName string) (handled bool, recvErr error) {
	if h == nil || mv == nil || !mv.IsStreaming || mv.MessageStream == nil || mv.Role == schema.Tool {
		return false, nil
	}
	mainStreamID := h.nextMainStreamID()
	mainEmitter := newEinoMainResponseStreamEmitter(
		h.conversationID, h.orchMode, agentName, mainStreamID, h.mainIteration(agentName), h.progress, h.snapshotMCPIDs,
	)
	reasoningEmitter := newEinoReasoningStreamEmitter(
		h.conversationID,
		h.orchMode,
		agentName,
		h.einoRoleTag(agentName),
		h.progress,
		h.nextReasoningStreamID,
	)
	var toolStreamFragments []schema.ToolCall
	var streamUsage *schema.TokenUsage
	subReplyEmitter := newEinoSubAgentReplyEmitter(
		h.conversationID,
		agentName,
		h.progress,
		h.nextSubAgentReplyStreamID,
	)
	mainAssistantStream := newEinoMainAssistantStreamHandler(einoMainAssistantStreamHandlerConfig{
		AgentName:        agentName,
		Emitter:          mainEmitter,
		StdoutSuppressor: h.stdoutSuppressor,
		AssistantOutput:  h.assistantOutput,
		RunMessages:      h.runMessages,
	})
	recvErr = recvEinoSchemaMessageStreamWithContext(h.ctx, mv.MessageStream, 8, func(chunk *schema.Message) {
		reasoningEmitter.EmitDelta(chunk.ReasoningContent)
		if chunk.Content != "" {
			if h.streamsMainAssistant(agentName) {
				mainAssistantStream.EmitDelta(chunk.Content)
			} else if !h.streamsMainAssistant(agentName) {
				subReplyEmitter.EmitDelta(chunk.Content)
			}
		}
		if len(chunk.ToolCalls) > 0 {
			toolStreamFragments = append(toolStreamFragments, chunk.ToolCalls...)
		}
		if chunk.ResponseMeta != nil && chunk.ResponseMeta.Usage != nil {
			streamUsage = maxEinoTokenUsage(streamUsage, chunk.ResponseMeta.Usage)
		}
	})
	if recvErr != nil && !isEinoVoluntaryCancelErr(recvErr) && h.logger != nil {
		h.logger.Warn("eino stream recv error, flushing incomplete stream",
			zap.Error(recvErr),
			zap.String("agent", agentName),
			zap.Int("toolFragments", len(toolStreamFragments)))
	}
	reasoningEmitter.Finish()
	if h.streamsMainAssistant(agentName) {
		mainAssistantStream.Finish()
	}
	subReplyEmitter.Finish()
	if h.toolCallCompletion != nil {
		h.toolCallCompletion.Complete(toolStreamFragments, agentName)
	}
	if h.usage != nil {
		h.usage.AddUsage(streamUsage)
	}
	return true, recvErr
}

func (h *einoAssistantStreamEventHandler) mainIteration(agentName string) int {
	if h == nil || h.runProgress == nil {
		return 0
	}
	return h.runProgress.MainIteration(agentName)
}
