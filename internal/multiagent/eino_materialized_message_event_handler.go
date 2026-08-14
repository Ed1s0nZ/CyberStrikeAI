package multiagent

import (
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

type einoMaterializedMessageEventHandlerConfig struct {
	ConversationID       string
	OrchMode             string
	Progress             func(eventType, message string, data interface{})
	SnapshotMCPIDs       func() []string
	StreamsMainAssistant func(agent string) bool
	EinoRoleTag          func(agent string) string
	RunProgress          *einoRunProgressTracker
	StdoutSuppressor     *einoExecuteStdoutSuppressor
	AssistantOutput      *einoAssistantOutputAccumulator
	RunMessages          *einoRunMessageAccumulator
	Usage                *einoRunUsageAccumulator
	ToolResultHandler    *einoToolResultEventHandler
	MarkPending          func(toolCallPendingInfo)
	NextMainStreamID     func() string
}

type einoMaterializedMessageEventHandler struct {
	conversationID       string
	orchMode             string
	progress             func(eventType, message string, data interface{})
	snapshotMCPIDs       func() []string
	streamsMainAssistant func(agent string) bool
	einoRoleTag          func(agent string) string
	runProgress          *einoRunProgressTracker
	stdoutSuppressor     *einoExecuteStdoutSuppressor
	assistantOutput      *einoAssistantOutputAccumulator
	runMessages          *einoRunMessageAccumulator
	usage                *einoRunUsageAccumulator
	toolResultHandler    *einoToolResultEventHandler
	markPending          func(toolCallPendingInfo)
	nextMainStreamID     func() string
}

func newEinoMaterializedMessageEventHandler(cfg einoMaterializedMessageEventHandlerConfig) *einoMaterializedMessageEventHandler {
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
	return &einoMaterializedMessageEventHandler{
		conversationID:       cfg.ConversationID,
		orchMode:             cfg.OrchMode,
		progress:             cfg.Progress,
		snapshotMCPIDs:       cfg.SnapshotMCPIDs,
		streamsMainAssistant: cfg.StreamsMainAssistant,
		einoRoleTag:          cfg.EinoRoleTag,
		runProgress:          cfg.RunProgress,
		stdoutSuppressor:     cfg.StdoutSuppressor,
		assistantOutput:      cfg.AssistantOutput,
		runMessages:          cfg.RunMessages,
		usage:                cfg.Usage,
		toolResultHandler:    cfg.ToolResultHandler,
		markPending:          cfg.MarkPending,
		nextMainStreamID:     cfg.NextMainStreamID,
	}
}

func (h *einoMaterializedMessageEventHandler) Handle(mv *adk.MessageVariant, msg adk.Message, agentName string) bool {
	if h == nil || mv == nil || msg == nil {
		return false
	}
	if h.runMessages != nil {
		h.runMessages.Append(msg)
	}
	if msg.Role == schema.Assistant && h.usage != nil {
		h.usage.AddMessage(msg)
	}
	if h.runProgress != nil {
		h.runProgress.EmitToolCalls(mergeMessageToolCalls(msg), agentName, h.markPending)
	}
	if mv.Role == schema.Assistant {
		newEinoReasoningStreamEmitter(h.conversationID, h.orchMode, agentName, h.einoRoleTag(agentName), h.progress, nil).EmitComplete(msg.ReasoningContent)
		body := strings.TrimSpace(msg.Content)
		if body != "" {
			if h.streamsMainAssistant(agentName) {
				newEinoMainAssistantCompleteHandler(einoMainAssistantCompleteHandlerConfig{
					AgentName:        agentName,
					Emitter:          newEinoMainResponseStreamEmitter(h.conversationID, h.orchMode, agentName, h.nextMainStreamID(), h.mainIteration(agentName), h.progress, h.snapshotMCPIDs),
					StdoutSuppressor: h.stdoutSuppressor,
					AssistantOutput:  h.assistantOutput,
				}).EmitComplete(body)
			} else {
				newEinoSubAgentReplyEmitter(h.conversationID, agentName, h.progress, nil).EmitComplete(body)
			}
		}
	}
	if h.toolResultHandler != nil {
		h.toolResultHandler.HandleMaterialized(mv, msg, agentName)
	}
	return true
}

func (h *einoMaterializedMessageEventHandler) mainIteration(agentName string) int {
	if h == nil || h.runProgress == nil {
		return 0
	}
	return h.runProgress.MainIteration(agentName)
}
