package multiagent

import (
	"context"
	"sync/atomic"

	"github.com/cloudwego/eino/adk"
	"go.uber.org/zap"
)

type einoRunRuntimeSessionConfig struct {
	Context        context.Context
	Args           *einoADKRunLoopArgs
	Drain          *einoRunEventDrain
	BaseMessages   []adk.Message
	EmptyHint      string
	SnapshotMCPIDs func() []string
	EinoRoleTag    func(agent string) string
}

type einoRunRuntimeErrorResult struct {
	Restarted bool
	Result    *RunResult
	Err       error
}

type einoRunRuntimeSession struct {
	ctx            context.Context
	args           *einoADKRunLoopArgs
	orchMode       string
	conversationID string
	progress       func(eventType, message string, data interface{})
	logger         *zap.Logger
	baseMsgs       []adk.Message
	msgs           []adk.Message
	drain          *einoRunEventDrain
	runMessages    *einoRunMessageAccumulator
	usage          *einoRunUsageAccumulator

	iter           *adk.AsyncIterator[*adk.AgentEvent]
	startFreshIter einoAgentEventIteratorStarter

	unregisterAgentCancel       func()
	unregisterTurnLoopInterrupt func()
	nativeCancelCause           atomic.Value

	transientRetry      *einoTransientRunRetryHandler
	runRecoveryHandler  *einoRunRecoveryHandler
	resultBuilder       *einoRunResultBuilder
	streamErrorHandler  *einoStreamErrorHandler
	completionHandler   *einoRunCompletionHandler
	cancellationHandler *einoRunCancellationHandler
}

func newEinoRunRuntimeSession(cfg einoRunRuntimeSessionConfig) *einoRunRuntimeSession {
	if cfg.Context == nil {
		cfg.Context = context.Background()
	}
	if cfg.Args == nil {
		cfg.Args = &einoADKRunLoopArgs{}
	}
	if cfg.SnapshotMCPIDs == nil {
		cfg.SnapshotMCPIDs = func() []string { return nil }
	}
	s := &einoRunRuntimeSession{
		ctx:            cfg.Context,
		args:           cfg.Args,
		orchMode:       cfg.Args.OrchMode,
		conversationID: cfg.Args.ConversationID,
		progress:       cfg.Args.Progress,
		logger:         cfg.Args.Logger,
		baseMsgs:       cfg.BaseMessages,
		msgs:           append([]adk.Message(nil), cfg.BaseMessages...),
		drain:          cfg.Drain,
	}
	if s.drain != nil {
		s.runMessages = s.drain.RunMessages()
		s.usage = s.drain.Usage()
	}
	if s.runMessages == nil {
		s.runMessages = newEinoRunMessageAccumulator(s.msgs)
	}
	s.initIteratorRuntime()
	s.initRecoveryRuntime()
	s.initResultRuntime(cfg.EmptyHint, cfg.SnapshotMCPIDs, cfg.EinoRoleTag)
	return s
}

func (s *einoRunRuntimeSession) Iterator() *adk.AsyncIterator[*adk.AgentEvent] {
	if s == nil {
		return nil
	}
	return s.iter
}

func (s *einoRunRuntimeSession) Close() {
	if s == nil {
		return
	}
	callAndClearUnregister(&s.unregisterAgentCancel)
	callAndClearUnregister(&s.unregisterTurnLoopInterrupt)
}

func (s *einoRunRuntimeSession) HandleIteratorContextError(err error) (*RunResult, error) {
	if s == nil || s.cancellationHandler == nil {
		return nil, err
	}
	return s.cancellationHandler.Handle(err)
}

func (s *einoRunRuntimeSession) HandleIteratorEnd() (completed bool, result *RunResult, err error) {
	if s == nil {
		return true, nil, nil
	}
	if ctxErr := s.ctx.Err(); ctxErr != nil {
		result, err = s.HandleIteratorContextError(ctxErr)
		return false, result, err
	}
	if s.completionHandler != nil {
		s.completionHandler.Complete()
	}
	return true, nil, nil
}

func (s *einoRunRuntimeSession) HandleRunError(runErr error) einoRunRuntimeErrorResult {
	if s == nil || runErr == nil {
		return einoRunRuntimeErrorResult{}
	}
	if isEinoTurnLoopPreemptErr(s.ctx, runErr) {
		return einoRunRuntimeErrorResult{}
	}
	restarted, fatal := s.maybeRestart(runErr)
	if fatal != nil {
		result, err := s.takePartial(fatal)
		return einoRunRuntimeErrorResult{Result: result, Err: err}
	}
	return einoRunRuntimeErrorResult{Restarted: restarted}
}

func (s *einoRunRuntimeSession) HandleStreamError(streamErr error, agentName string) einoRunRuntimeErrorResult {
	if s == nil || s.streamErrorHandler == nil || streamErr == nil {
		return einoRunRuntimeErrorResult{}
	}
	handled := s.streamErrorHandler.Handle(streamErr, agentName)
	return einoRunRuntimeErrorResult{
		Restarted: handled.Restarted,
		Result:    handled.Result,
		Err:       handled.Err,
	}
}

func (s *einoRunRuntimeSession) ConfirmRecovery() {
	if s != nil && s.transientRetry != nil {
		s.transientRetry.ConfirmRecovery()
	}
}

func (s *einoRunRuntimeSession) BuildFinalResult() *RunResult {
	if s == nil || s.resultBuilder == nil {
		return &RunResult{}
	}
	s.emitUsageSummary("final")
	return s.resultBuilder.BuildFinal()
}

func (s *einoRunRuntimeSession) takePartial(err error) (*RunResult, error) {
	if s == nil || s.resultBuilder == nil {
		return nil, err
	}
	s.emitUsageSummary("partial")
	return s.resultBuilder.BuildPartial(err)
}

func (s *einoRunRuntimeSession) maybeRestart(runErr error) (restarted bool, fatal error) {
	if s == nil || s.runRecoveryHandler == nil {
		return false, runErr
	}
	recovery := s.runRecoveryHandler.Handle(runErr, s.runMessages.Messages(), s.runMessages.BaseCount())
	if recovery.Fatal != nil {
		return false, recovery.Fatal
	}
	if !recovery.Restarted {
		return false, nil
	}
	s.msgs = recovery.RestartMsgs
	s.iter = s.startFreshIter(s.msgs)
	return true, nil
}

func (s *einoRunRuntimeSession) initIteratorRuntime() {
	if s == nil || s.args == nil {
		return
	}
	runnerCfg := adk.RunnerConfig{
		Agent: s.args.DA,
		// 启用 ADK 流式事件：plan_execute 也需要输出 reasoning/response 流，
		// 与 deep/supervisor/eino_single 的前端体验保持一致。
		EnableStreaming: true,
	}
	var cpStore *fileCheckPointStore
	var checkPointID string
	if checkpoint := newEinoCheckpointRuntime(s.args.CheckpointDir, s.conversationID, s.orchMode, s.logger); checkpoint != nil {
		cpStore = checkpoint.Store
		checkPointID = checkpoint.CheckPointID
		runnerCfg.CheckPointStore = checkpoint.Store
	}
	runner := adk.NewRunner(s.ctx, runnerCfg)
	runtimeCancelRegistrar := agentRuntimeCancelRegistrarFromContext(s.ctx)
	turnLoopInterruptRegistrar := agentTurnLoopInterruptRegistrarFromContext(s.ctx)
	runnerStarter := newEinoRunnerIteratorStarter(einoRunnerIteratorStarterConfig{
		Context:                s.ctx,
		ConversationID:         s.conversationID,
		OrchMode:               s.orchMode,
		Logger:                 s.logger,
		Runner:                 runner,
		CheckPointID:           checkPointID,
		NativeCancelCause:      &s.nativeCancelCause,
		UnregisterAgentCancel:  &s.unregisterAgentCancel,
		RuntimeCancelRegistrar: runtimeCancelRegistrar,
	})
	turnLoopStarter := newEinoTurnLoopIteratorStarter(einoTurnLoopIteratorStarterConfig{
		Context:                     s.ctx,
		Agent:                       s.args.DA,
		ConversationID:              s.conversationID,
		OrchMode:                    s.orchMode,
		Progress:                    s.progress,
		Logger:                      s.logger,
		Store:                       cpStore,
		CheckPointID:                checkPointID,
		InterruptTimeout:            s.args.TurnLoopInterruptTimeout,
		NativeCancelCause:           &s.nativeCancelCause,
		UnregisterAgentCancel:       &s.unregisterAgentCancel,
		UnregisterTurnLoopInterrupt: &s.unregisterTurnLoopInterrupt,
		RuntimeCancelRegistrar:      runtimeCancelRegistrar,
		TurnLoopInterruptRegistrar:  turnLoopInterruptRegistrar,
	})
	useTurnLoop := turnLoopInterruptRegistrar != nil
	s.startFreshIter = runnerStarter.Start
	if useTurnLoop {
		s.startFreshIter = turnLoopStarter.Start
	}
	if !useTurnLoop && cpStore != nil && checkPointID != "" {
		s.iter = newEinoCheckpointResumeHandler(einoCheckpointResumeHandlerConfig{
			Context:        s.ctx,
			ConversationID: s.conversationID,
			OrchMode:       s.orchMode,
			Progress:       s.progress,
			Logger:         s.logger,
			Store:          cpStore,
			CheckPointID:   checkPointID,
			Resume:         runnerStarter.Resume,
		}).TryResume()
	}
	s.iter = newEinoInitialIteratorStartHandler(einoInitialIteratorStartHandlerConfig{
		ConversationID: s.conversationID,
		OrchMode:       s.orchMode,
		Progress:       s.progress,
		UseTurnLoop:    useTurnLoop,
		StartRunner:    runnerStarter.Start,
		StartTurnLoop:  turnLoopStarter.Start,
	}).StartIfNeeded(s.iter, s.msgs)

	pending := s.pending()
	s.completionHandler = newEinoRunCompletionHandler(einoRunCompletionHandlerConfig{
		ConversationID: s.conversationID,
		OrchMode:       s.orchMode,
		Progress:       s.progress,
		Logger:         s.logger,
		Pending:        pending,
		Checkpoint:     cpStore,
		CheckpointID:   checkPointID,
	})
	s.cancellationHandler = newEinoRunCancellationHandler(einoRunCancellationHandlerConfig{
		Context:        s.ctx,
		ConversationID: s.conversationID,
		Progress:       s.progress,
		Pending:        pending,
		TakePartial:    s.takePartial,
	})
}

func (s *einoRunRuntimeSession) initRecoveryRuntime() {
	if s == nil || s.args == nil {
		return
	}
	pending := s.pending()
	contextOverflowRetry := newEinoContextOverflowRetryHandler(einoContextOverflowRetryConfig{
		Context:        s.ctx,
		ConversationID: s.conversationID,
		OrchMode:       s.orchMode,
		Args:           s.args,
		BaseMsgs:       s.baseMsgs,
		Progress:       s.progress,
		Logger:         s.logger,
	})
	s.transientRetry = newEinoTransientRunRetryHandler(einoTransientRunRetryHandlerConfig{
		Context:        s.ctx,
		ConversationID: s.conversationID,
		OrchMode:       s.orchMode,
		Args:           s.args,
		BaseMsgs:       s.baseMsgs,
		Progress:       s.progress,
		Logger:         s.logger,
		Pending:        pending,
	})
	runErrorHandler := newEinoRunErrorHandler(einoRunErrorHandlerConfig{
		ConversationID:       s.conversationID,
		OrchMode:             s.orchMode,
		Progress:             s.progress,
		Pending:              pending,
		NativeCancelFallback: s.nativeCancelCauseOrCanceled,
	})
	s.runRecoveryHandler = newEinoRunRecoveryHandler(einoRunRecoveryHandlerConfig{
		ConversationID:  s.conversationID,
		OrchMode:        s.orchMode,
		Args:            s.args,
		BaseMsgs:        s.baseMsgs,
		Progress:        s.progress,
		Logger:          s.logger,
		RunError:        runErrorHandler,
		ContextOverflow: contextOverflowRetry,
		Transient:       s.transientRetry,
	})
}

func (s *einoRunRuntimeSession) initResultRuntime(emptyHint string, snapshotMCPIDs func() []string, einoRoleTag func(agent string) string) {
	if s == nil {
		return
	}
	var assistantOutput *einoAssistantOutputAccumulator
	if s.drain != nil {
		assistantOutput = s.drain.AssistantOutput()
	}
	s.resultBuilder = newEinoRunResultBuilder(einoRunResultBuilderConfig{
		OrchMode:         s.orchMode,
		EmptyHint:        emptyHint,
		RunMessages:      s.runMessages,
		AssistantOutput:  assistantOutput,
		SnapshotMCPIDs:   snapshotMCPIDs,
		ModelFacingTrace: func() []adk.Message { return modelFacingTraceSnapshot(s.args) },
	})
	s.streamErrorHandler = newEinoStreamErrorHandler(
		s.ctx,
		s.conversationID,
		s.progress,
		einoRoleTag,
		s.maybeRestart,
		s.takePartial,
	)
}

func (s *einoRunRuntimeSession) pending() *einoPendingToolCalls {
	if s == nil || s.drain == nil {
		return nil
	}
	return s.drain.PendingToolCalls()
}

func (s *einoRunRuntimeSession) nativeCancelCauseOrCanceled() error {
	if s != nil {
		if v := s.nativeCancelCause.Load(); v != nil {
			if err, ok := v.(error); ok && err != nil {
				return err
			}
		}
	}
	return context.Canceled
}

func (s *einoRunRuntimeSession) emitUsageSummary(reason string) bool {
	if s == nil || s.usage == nil {
		return false
	}
	modelName := ""
	if s.args != nil {
		modelName = s.args.ModelName
	}
	return s.usage.EmitOnce(s.conversationID, s.orchMode, reason, modelName, s.progress, s.logger)
}
