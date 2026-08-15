package multiagent

import (
	"context"
	"fmt"
	"sync/atomic"

	"cyberstrike-ai/internal/agent"
	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/einomcp"

	"github.com/cloudwego/eino/adk"
	"go.uber.org/zap"
)

type einoRunEventDrainConfig struct {
	Context              context.Context
	ConversationID       string
	OrchMode             string
	OrchestratorName     string
	Progress             func(eventType, message string, data interface{})
	Logger               *zap.Logger
	BaseMessages         []adk.Message
	SnapshotMCPIDs       func() []string
	StreamsMainAssistant func(agent string) bool
	EinoRoleTag          func(agent string) string
	MiddlewareConfig     *config.MultiAgentEinoMiddlewareConfig

	FilesystemMonitorAgent  *agent.Agent
	FilesystemMonitorRecord einomcp.ExecutionRecorder
	MCPExecutionBinder      *MCPExecutionBinder
}

type einoRunEventDrain struct {
	cfg einoRunEventDrainConfig

	runMessages       *einoRunMessageAccumulator
	assistantOutput   *einoAssistantOutputAccumulator
	runProgress       *einoRunProgressTracker
	pendingToolCalls  *einoPendingToolCalls
	stdoutSuppressor  *einoExecuteStdoutSuppressor
	toolResultEmitter *einoToolResultProgressEmitter
	usage             *einoRunUsageAccumulator

	reasoningStreamSeq    int64
	subReplyStreamSeq     int64
	mainResponseStreamSeq int64

	toolResultHandler          *einoToolResultEventHandler
	assistantStreamHandler     *einoAssistantStreamEventHandler
	materializedMessageHandler *einoMaterializedMessageEventHandler
}

func newEinoRunEventDrain(cfg einoRunEventDrainConfig) *einoRunEventDrain {
	if cfg.Context == nil {
		cfg.Context = context.Background()
	}
	if cfg.SnapshotMCPIDs == nil {
		cfg.SnapshotMCPIDs = func() []string { return nil }
	}
	if cfg.StreamsMainAssistant == nil {
		cfg.StreamsMainAssistant = func(agentName string) bool {
			return agentName == "" || agentName == cfg.OrchestratorName
		}
	}
	if cfg.EinoRoleTag == nil {
		cfg.EinoRoleTag = func(agentName string) string {
			if cfg.StreamsMainAssistant(agentName) {
				return "orchestrator"
			}
			return "sub"
		}
	}

	runMessages := newEinoRunMessageAccumulator(cfg.BaseMessages)
	assistantOutput := newEinoAssistantOutputAccumulator(cfg.OrchMode)
	runProgress := newEinoRunProgressTracker(
		cfg.OrchMode,
		cfg.OrchestratorName,
		cfg.ConversationID,
		cfg.Progress,
		cfg.StreamsMainAssistant,
		cfg.EinoRoleTag,
	)
	pendingToolCalls := newEinoPendingToolCalls(cfg.ConversationID, cfg.Progress)
	stdoutSuppressor := newEinoExecuteStdoutSuppressor()
	usage := newEinoRunUsageAccumulator()
	toolResultEmitter := newEinoToolResultProgressEmitter(einoToolResultProgressEmitterConfig{
		ConversationID:          cfg.ConversationID,
		OrchestratorName:        cfg.OrchestratorName,
		Progress:                cfg.Progress,
		EinoRoleTag:             cfg.EinoRoleTag,
		Pending:                 pendingToolCalls,
		ExecuteStdoutDup:        stdoutSuppressor,
		RunMessages:             runMessages,
		FilesystemMonitorAgent:  cfg.FilesystemMonitorAgent,
		FilesystemMonitorRecord: cfg.FilesystemMonitorRecord,
		MCPExecutionBinder:      cfg.MCPExecutionBinder,
	})

	return &einoRunEventDrain{
		cfg:               cfg,
		runMessages:       runMessages,
		assistantOutput:   assistantOutput,
		runProgress:       runProgress,
		pendingToolCalls:  pendingToolCalls,
		stdoutSuppressor:  stdoutSuppressor,
		toolResultEmitter: toolResultEmitter,
		usage:             usage,
	}
}

func (d *einoRunEventDrain) BindHandlers(confirmRecovery func()) {
	if d == nil {
		return
	}
	d.toolResultHandler = newEinoToolResultEventHandler(einoToolResultEventHandlerConfig{
		Context:         d.cfg.Context,
		Logger:          d.cfg.Logger,
		RunMessages:     d.runMessages,
		Emitter:         d.toolResultEmitter,
		ConfirmRecovery: confirmRecovery,
	})
	streamToolCallCompletion := newEinoStreamToolCallCompletionHandler(einoStreamToolCallCompletionHandlerConfig{
		ConversationID: d.cfg.ConversationID,
		OrchMode:       d.cfg.OrchMode,
		Progress:       d.cfg.Progress,
		RunProgress:    d.runProgress,
		RunMessages:    d.runMessages,
		MarkPending:    d.markPendingWithMonitor,
	})
	d.assistantStreamHandler = newEinoAssistantStreamEventHandler(einoAssistantStreamEventHandlerConfig{
		Context:                   d.cfg.Context,
		ConversationID:            d.cfg.ConversationID,
		OrchMode:                  d.cfg.OrchMode,
		Progress:                  d.cfg.Progress,
		Logger:                    d.cfg.Logger,
		SnapshotMCPIDs:            d.cfg.SnapshotMCPIDs,
		StreamsMainAssistant:      d.cfg.StreamsMainAssistant,
		EinoRoleTag:               d.cfg.EinoRoleTag,
		RunProgress:               d.runProgress,
		StdoutSuppressor:          d.stdoutSuppressor,
		AssistantOutput:           d.assistantOutput,
		RunMessages:               d.runMessages,
		Usage:                     d.usage,
		ToolCallCompletion:        streamToolCallCompletion,
		NextMainStreamID:          d.nextMainStreamID,
		NextReasoningStreamID:     d.nextReasoningStreamID,
		NextSubAgentReplyStreamID: d.nextSubAgentReplyStreamID,
	})
	d.materializedMessageHandler = newEinoMaterializedMessageEventHandler(einoMaterializedMessageEventHandlerConfig{
		ConversationID:       d.cfg.ConversationID,
		OrchMode:             d.cfg.OrchMode,
		Progress:             d.cfg.Progress,
		SnapshotMCPIDs:       d.cfg.SnapshotMCPIDs,
		StreamsMainAssistant: d.cfg.StreamsMainAssistant,
		EinoRoleTag:          d.cfg.EinoRoleTag,
		RunProgress:          d.runProgress,
		StdoutSuppressor:     d.stdoutSuppressor,
		AssistantOutput:      d.assistantOutput,
		RunMessages:          d.runMessages,
		Usage:                d.usage,
		ToolResultHandler:    d.toolResultHandler,
		MarkPending:          d.markPendingWithMonitor,
		NextMainStreamID:     d.nextMainStreamID,
	})
}

func (d *einoRunEventDrain) RunMessages() *einoRunMessageAccumulator {
	if d == nil {
		return nil
	}
	return d.runMessages
}

func (d *einoRunEventDrain) AssistantOutput() *einoAssistantOutputAccumulator {
	if d == nil {
		return nil
	}
	return d.assistantOutput
}

func (d *einoRunEventDrain) PendingToolCalls() *einoPendingToolCalls {
	if d == nil {
		return nil
	}
	return d.pendingToolCalls
}

func (d *einoRunEventDrain) Usage() *einoRunUsageAccumulator {
	if d == nil {
		return nil
	}
	return d.usage
}

func (d *einoRunEventDrain) ObserveAgent(agentName string) {
	if d == nil || d.runProgress == nil {
		return
	}
	d.runProgress.ObserveAgent(agentName)
}

func (d *einoRunEventDrain) HandleToolResultStreaming(mv *adk.MessageVariant, agentName string) bool {
	return d != nil && d.toolResultHandler != nil && d.toolResultHandler.HandleStreaming(mv, agentName)
}

func (d *einoRunEventDrain) HandleAssistantStream(mv *adk.MessageVariant, agentName string) (bool, error) {
	if d == nil || d.assistantStreamHandler == nil {
		return false, nil
	}
	return d.assistantStreamHandler.Handle(mv, agentName)
}

func (d *einoRunEventDrain) HandleMaterialized(mv *adk.MessageVariant, msg adk.Message, agentName string) bool {
	return d != nil && d.materializedMessageHandler != nil && d.materializedMessageHandler.Handle(mv, msg, agentName)
}

func (d *einoRunEventDrain) markPendingWithMonitor(tc toolCallPendingInfo) {
	if d == nil || d.pendingToolCalls == nil {
		return
	}
	d.pendingToolCalls.Mark(tc)
	beginEinoADKFilesystemToolMonitor(
		d.cfg.Context,
		d.cfg.FilesystemMonitorAgent,
		d.cfg.FilesystemMonitorRecord,
		d.cfg.MCPExecutionBinder,
		tc.ToolCallID,
		tc.ToolName,
		tc.Arguments,
	)
}

func (d *einoRunEventDrain) nextMainStreamID() string {
	return fmt.Sprintf("eino-main-%s-%d", d.cfg.ConversationID, atomic.AddInt64(&d.mainResponseStreamSeq, 1))
}

func (d *einoRunEventDrain) nextReasoningStreamID() string {
	return fmt.Sprintf("eino-reasoning-%s-%d", d.cfg.ConversationID, atomic.AddInt64(&d.reasoningStreamSeq, 1))
}

func (d *einoRunEventDrain) nextSubAgentReplyStreamID() string {
	return fmt.Sprintf("eino-sub-reply-%s-%d", d.cfg.ConversationID, atomic.AddInt64(&d.subReplyStreamSeq, 1))
}
