package multiagent

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

type einoAgenticMessageAgentAdapter struct {
	inner adk.TypedAgent[*schema.AgenticMessage]
}

func newEinoAgenticMessageAgentAdapter(inner adk.TypedAgent[*schema.AgenticMessage]) adk.Agent {
	if inner == nil {
		return nil
	}
	return &einoAgenticMessageAgentAdapter{inner: inner}
}

func (a *einoAgenticMessageAgentAdapter) Name(ctx context.Context) string {
	if a == nil || a.inner == nil {
		return ""
	}
	return a.inner.Name(ctx)
}

func (a *einoAgenticMessageAgentAdapter) Description(ctx context.Context) string {
	if a == nil || a.inner == nil {
		return ""
	}
	return a.inner.Description(ctx)
}

func (a *einoAgenticMessageAgentAdapter) Run(ctx context.Context, input *adk.AgentInput, opts ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	return a.runTyped(ctx, input, nil, opts...)
}

func (a *einoAgenticMessageAgentAdapter) Resume(ctx context.Context, info *adk.ResumeInfo, opts ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	return a.runTyped(ctx, nil, info, opts...)
}

func (a *einoAgenticMessageAgentAdapter) runTyped(ctx context.Context, input *adk.AgentInput, resumeInfo *adk.ResumeInfo, opts ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	iter, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	go func() {
		defer gen.Close()
		if a == nil || a.inner == nil {
			gen.Send(&adk.AgentEvent{Err: fmt.Errorf("agentic adapter: inner agent is nil")})
			return
		}
		var agenticIter *adk.AsyncIterator[*adk.TypedAgentEvent[*schema.AgenticMessage]]
		if resumeInfo != nil {
			resumable, ok := a.inner.(adk.TypedResumableAgent[*schema.AgenticMessage])
			if !ok {
				gen.Send(&adk.AgentEvent{Err: fmt.Errorf("agentic adapter: inner agent does not support resume")})
				return
			}
			agenticIter = resumable.Resume(ctx, resumeInfo, opts...)
		} else {
			agenticInput := &adk.TypedAgentInput[*schema.AgenticMessage]{}
			if input != nil {
				agenticInput.EnableStreaming = input.EnableStreaming
				agenticInput.Messages = EinoMessagesToAgentic(input.Messages)
			}
			agenticIter = a.inner.Run(ctx, agenticInput, opts...)
		}
		for {
			ev, ok := agenticIter.Next()
			if !ok {
				return
			}
			for _, adapted := range adaptAgenticEventToEinoEvents(ev) {
				if adapted != nil {
					gen.Send(adapted)
				}
			}
		}
	}()
	return iter
}
