package multiagent

import (
	"context"
	"sync/atomic"

	"github.com/cloudwego/eino/adk"
	"go.uber.org/zap"
)

type einoRunnerControl interface {
	Run(context.Context, []adk.Message, ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent]
	Resume(context.Context, string, ...adk.AgentRunOption) (*adk.AsyncIterator[*adk.AgentEvent], error)
}

type einoRunnerIteratorStarterConfig struct {
	Context                context.Context
	ConversationID         string
	OrchMode               string
	Logger                 *zap.Logger
	Runner                 einoRunnerControl
	CheckPointID           string
	NativeCancelCause      *atomic.Value
	UnregisterAgentCancel  *func()
	RuntimeCancelRegistrar AgentRuntimeCancelRegistrar
}

type einoRunnerIteratorStarter struct {
	cfg einoRunnerIteratorStarterConfig
}

func newEinoRunnerIteratorStarter(cfg einoRunnerIteratorStarterConfig) *einoRunnerIteratorStarter {
	return &einoRunnerIteratorStarter{cfg: cfg}
}

func (s *einoRunnerIteratorStarter) Start(runMsgs []adk.Message) *adk.AsyncIterator[*adk.AgentEvent] {
	if s == nil || s.cfg.Runner == nil {
		return nil
	}
	opts := s.newRunOptions()
	if s.cfg.CheckPointID != "" {
		opts = append(opts, adk.WithCheckPointID(s.cfg.CheckPointID))
	}
	return s.cfg.Runner.Run(s.cfg.Context, runMsgs, opts...)
}

func (s *einoRunnerIteratorStarter) Resume(checkPointID string) (*adk.AsyncIterator[*adk.AgentEvent], error) {
	if s == nil || s.cfg.Runner == nil {
		return nil, nil
	}
	return s.cfg.Runner.Resume(s.cfg.Context, checkPointID, s.newRunOptions()...)
}

func (s *einoRunnerIteratorStarter) newRunOptions() []adk.AgentRunOption {
	cancelOpt, cancelFn := adk.WithCancel()
	callAndClearUnregister(s.cfg.UnregisterAgentCancel)
	if s.cfg.RuntimeCancelRegistrar != nil && s.cfg.UnregisterAgentCancel != nil {
		*s.cfg.UnregisterAgentCancel = s.cfg.RuntimeCancelRegistrar(func(cause error) bool {
			s.storeNativeCancelCause(cause)
			waitErr, submitted, handled := requestEinoNativeAgentCancel(cancelFn, cause)
			s.logNativeCancelRequest(cause, waitErr, submitted, handled)
			return handled
		})
	}
	return []adk.AgentRunOption{cancelOpt}
}

func (s *einoRunnerIteratorStarter) storeNativeCancelCause(cause error) {
	if s == nil || s.cfg.NativeCancelCause == nil || cause == nil {
		return
	}
	s.cfg.NativeCancelCause.Store(cause)
}

func (s *einoRunnerIteratorStarter) logNativeCancelRequest(cause error, waitErr error, submitted bool, handled bool) {
	if s == nil || s.cfg.Logger == nil {
		return
	}
	fields := []zap.Field{
		zap.String("conversation_id", s.cfg.ConversationID),
		zap.String("orchestration", s.cfg.OrchMode),
		zap.Bool("submitted", submitted),
		zap.Bool("handled", handled),
	}
	if cause != nil {
		fields = append(fields, zap.Error(cause))
	}
	if waitErr != nil {
		fields = append(fields, zap.NamedError("cancel_wait_error", waitErr))
		s.cfg.Logger.Debug("eino native cancel requested", fields...)
	} else {
		s.cfg.Logger.Info("eino native cancel requested", fields...)
	}
}
