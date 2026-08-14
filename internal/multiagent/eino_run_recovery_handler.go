package multiagent

import (
	"github.com/cloudwego/eino/adk"
	"go.uber.org/zap"
)

type einoRunRecoveryHandlerConfig struct {
	ConversationID  string
	OrchMode        string
	Args            *einoADKRunLoopArgs
	BaseMsgs        []adk.Message
	Progress        func(eventType, message string, data interface{})
	Logger          *zap.Logger
	RunError        *einoRunErrorHandler
	ContextOverflow *einoContextOverflowRetryHandler
	Transient       *einoTransientRunRetryHandler
}

type einoRunRecoveryResult struct {
	Handled     bool
	Restarted   bool
	RestartMsgs []adk.Message
	Fatal       error
}

type einoRunRecoveryHandler struct {
	cfg einoRunRecoveryHandlerConfig
}

func newEinoRunRecoveryHandler(cfg einoRunRecoveryHandlerConfig) *einoRunRecoveryHandler {
	if cfg.Args == nil {
		cfg.Args = &einoADKRunLoopArgs{}
	}
	return &einoRunRecoveryHandler{cfg: cfg}
}

func (h *einoRunRecoveryHandler) Handle(runErr error, accumulated []adk.Message, baseCount int) einoRunRecoveryResult {
	if h == nil || runErr == nil {
		return einoRunRecoveryResult{}
	}
	if willRetry, ok := isEinoNativeWillRetry(runErr); ok {
		emitEinoNativeModelRetryProgress(h.cfg.ConversationID, h.cfg.OrchMode, willRetry, h.cfg.Progress, h.cfg.Logger, runErr)
		return einoRunRecoveryResult{Handled: true}
	}
	if h.cfg.ContextOverflow != nil {
		if overflowRetry := h.cfg.ContextOverflow.Prepare(runErr, accumulated, baseCount); overflowRetry.Handled {
			return einoRunRecoveryResult{Handled: true, Restarted: true, RestartMsgs: overflowRetry.RestartMsgs}
		}
	}
	if h.cfg.Transient != nil {
		if runRetry := h.cfg.Transient.Prepare(runErr, accumulated, baseCount); runRetry.Handled {
			if runRetry.Fatal != nil {
				return einoRunRecoveryResult{Handled: true, Fatal: runRetry.Fatal}
			}
			if !runRetry.Restarted {
				return einoRunRecoveryResult{Handled: true}
			}
			return einoRunRecoveryResult{Handled: true, Restarted: true, RestartMsgs: runRetry.RestartMsgs}
		}
	}
	return einoRunRecoveryResult{Handled: true, Fatal: h.handleFatal(runErr)}
}

func (h *einoRunRecoveryHandler) handleFatal(runErr error) error {
	if h != nil && h.cfg.RunError != nil {
		return h.cfg.RunError.Handle(runErr)
	}
	return runErr
}
