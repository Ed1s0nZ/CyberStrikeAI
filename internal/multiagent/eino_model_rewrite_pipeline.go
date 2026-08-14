package multiagent

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/adk"
)

func applyBeforeModelRewriteHandlers(
	ctx context.Context,
	msgs []adk.Message,
	handlers []adk.ChatModelAgentMiddleware,
) ([]adk.Message, error) {
	if len(msgs) == 0 || len(handlers) == 0 {
		return msgs, nil
	}
	state := &adk.ChatModelAgentState{Messages: msgs}
	modelCtx := &adk.ModelContext{}
	curCtx := ctx
	for _, h := range handlers {
		if h == nil {
			continue
		}
		nextCtx, nextState, err := h.BeforeModelRewriteState(curCtx, state, modelCtx)
		if err != nil {
			return nil, fmt.Errorf("before model rewrite: %w", err)
		}
		if nextCtx != nil {
			curCtx = nextCtx
		}
		if nextState != nil {
			state = nextState
		}
	}
	return state.Messages, nil
}

