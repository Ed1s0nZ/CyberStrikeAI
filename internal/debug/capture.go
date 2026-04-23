package debug

import "context"

// captureCtxKey is the unexported context key for per-call capture
// coordinates. The orchestrator attaches these before invoking Agent
// LLM methods; the Agent's capture wrapper reads them back.
type captureCtxKey struct{}

// captureCtx carries the per-call coordinates.
type captureCtx struct {
	ConversationID string
	MessageID      string
	Iteration      int
	CallIndex      int
	AgentID        string
}

// WithCapture attaches capture metadata to ctx. The orchestrator sets
// this before invoking any Agent method that calls the LLM. When
// unset, the Agent's wrapper silently skips RecordLLMCall.
func WithCapture(ctx context.Context, conversationID, messageID string, iteration, callIndex int, agentID string) context.Context {
	return context.WithValue(ctx, captureCtxKey{}, captureCtx{
		ConversationID: conversationID,
		MessageID:      messageID,
		Iteration:      iteration,
		CallIndex:      callIndex,
		AgentID:        agentID,
	})
}

// CaptureCoords returns the capture metadata on ctx, or zero values
// if none is set. Exported so the agent package (different package)
// can read the fields without exposing the struct itself.
func CaptureCoords(ctx context.Context) (convID, msgID string, iteration, callIndex int, agentID string) {
	v, _ := ctx.Value(captureCtxKey{}).(captureCtx)
	return v.ConversationID, v.MessageID, v.Iteration, v.CallIndex, v.AgentID
}

// LLMCall is one LLM round-trip recorded to debug_llm_calls.
// Zero-valued fields serialize to NULL columns (first_token_at,
// prompt_tokens, completion_tokens) for backends that don't report
// them — see the plan's "Deviations from spec" note on streaming
// token usage.
type LLMCall struct {
	Iteration        int
	CallIndex        int
	AgentID          string
	SentAt           int64 // unix nanos
	FirstTokenAt     int64 // 0 means unknown
	FinishedAt       int64
	PromptTokens     int64 // 0 means unknown
	CompletionTokens int64 // 0 means unknown
	RequestJSON      string
	ResponseJSON     string
	Error            string
}

// Event is one orchestrator/agent progress event recorded to
// debug_events. Seq is assigned by the sink, not the caller.
type Event struct {
	EventType   string
	AgentID     string
	PayloadJSON string
	StartedAt   int64
	FinishedAt  int64 // 0 means instant / no duration
}
