package multiagent

import (
	"context"
	"errors"
	"time"

	"github.com/cloudwego/eino/adk"
)

const (
	einoNativeCancelImmediateWait = 1200 * time.Millisecond
	einoNativeCancelSafePointWait = 3500 * time.Millisecond
	einoNativeCancelSafePointTTL  = 3 * time.Second
)

type agentRuntimeCancelRegistrarKey struct{}
type agentTurnLoopInterruptRegistrarKey struct{}

// AgentRuntimeCancelRegistrar binds the currently active Eino ADK cancel hook
// into the host task manager. The hook returns true when Eino accepted and
// handled the cancel request, so the host can avoid canceling the parent context.
type AgentRuntimeCancelRegistrar func(cancel func(error) bool) (unregister func())

// WithAgentRuntimeCancelRegistrar lets the HTTP/task layer trigger Eino's native
// Agent Cancel before falling back to the existing context cancellation path.
func WithAgentRuntimeCancelRegistrar(ctx context.Context, registrar AgentRuntimeCancelRegistrar) context.Context {
	if ctx == nil || registrar == nil {
		return ctx
	}
	return context.WithValue(ctx, agentRuntimeCancelRegistrarKey{}, registrar)
}

func agentRuntimeCancelRegistrarFromContext(ctx context.Context) AgentRuntimeCancelRegistrar {
	if ctx == nil {
		return nil
	}
	if v, ok := ctx.Value(agentRuntimeCancelRegistrarKey{}).(AgentRuntimeCancelRegistrar); ok {
		return v
	}
	return nil
}

// AgentTurnLoopInterruptRegistrar binds a conversation-level TurnLoop interrupt
// pusher into the host task manager. The pusher receives the user supplied note
// and returns true when the note was accepted by the loop.
type AgentTurnLoopInterruptRegistrar func(push func(note string) bool) (unregister func())

// WithAgentTurnLoopInterruptRegistrar lets the HTTP/task layer enqueue a user
// supplement into an active Eino TurnLoop before falling back to cancellation.
func WithAgentTurnLoopInterruptRegistrar(ctx context.Context, registrar AgentTurnLoopInterruptRegistrar) context.Context {
	if ctx == nil || registrar == nil {
		return ctx
	}
	return context.WithValue(ctx, agentTurnLoopInterruptRegistrarKey{}, registrar)
}

func agentTurnLoopInterruptRegistrarFromContext(ctx context.Context) AgentTurnLoopInterruptRegistrar {
	if ctx == nil {
		return nil
	}
	if v, ok := ctx.Value(agentTurnLoopInterruptRegistrarKey{}).(AgentTurnLoopInterruptRegistrar); ok {
		return v
	}
	return nil
}

func requestEinoNativeAgentCancel(cancelFn adk.AgentCancelFunc, cause error) (waitErr error, submitted bool, handled bool) {
	if cancelFn == nil {
		return nil, false, false
	}
	opts, waitFor := einoNativeCancelOptions(cause)
	handle, submitted := cancelFn(opts...)
	if !submitted || handle == nil {
		return nil, submitted, false
	}
	waitCh := make(chan error, 1)
	go func() {
		waitCh <- handle.Wait()
	}()
	select {
	case err := <-waitCh:
		handled := err == nil || errors.Is(err, adk.ErrCancelTimeout) || errors.Is(err, adk.ErrExecutionEnded)
		return err, submitted, handled
	case <-time.After(waitFor):
		return context.DeadlineExceeded, submitted, false
	}
}

func einoNativeCancelOptions(cause error) ([]adk.AgentCancelOption, time.Duration) {
	if errors.Is(cause, ErrInterruptContinue) {
		return []adk.AgentCancelOption{
			adk.WithAgentCancelMode(adk.CancelAfterChatModel | adk.CancelAfterToolCalls),
			adk.WithAgentCancelTimeout(einoNativeCancelSafePointTTL),
			adk.WithRecursive(),
		}, einoNativeCancelSafePointWait
	}
	return []adk.AgentCancelOption{
		adk.WithAgentCancelMode(adk.CancelImmediate),
		adk.WithRecursive(),
	}, einoNativeCancelImmediateWait
}
