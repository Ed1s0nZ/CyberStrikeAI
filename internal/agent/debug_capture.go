package agent

import (
	"context"
	"encoding/json"
	"time"

	"cyberstrike-ai/internal/debug"
)

// captureLLMCall wraps one LLM round-trip and records it to the
// debug sink if capture coordinates are attached to ctx (via
// debug.WithCapture). requestPayload serializes into the exact
// request body we'd send the provider. callFn runs the actual call
// and returns (response, promptTokens, completionTokens, error).
// Zero token counts indicate the backend didn't report usage; the
// sink writes them as SQL NULL so aggregates stay correct.
//
// When the sink is a noopSink, RecordLLMCall returns immediately —
// the only per-call overhead is the time.Now() calls and a JSON
// marshal of the request payload. Acceptable: capture cost is paid
// only when debug is on.
func (a *Agent) captureLLMCall(
	ctx context.Context,
	requestPayload interface{},
	callFn func() (response interface{}, promptTokens, completionTokens int64, err error),
) (interface{}, error) {
	sentAt := time.Now().UnixNano()
	resp, pt, ct, err := callFn()
	finishedAt := time.Now().UnixNano()

	convID, msgID, iteration, callIndex, agentID := debug.CaptureCoords(ctx)
	if convID == "" {
		// No capture coordinates on ctx — skip recording. Happens
		// in unit tests that exercise the raw LLM path without
		// orchestrator, and in any non-captured call.
		return resp, err
	}
	reqJSON, _ := json.Marshal(requestPayload)
	var respJSON []byte
	if resp != nil {
		respJSON, _ = json.Marshal(resp)
	}
	errStr := ""
	if err != nil {
		errStr = err.Error()
	}
	a.debugSink.RecordLLMCall(convID, msgID, debug.LLMCall{
		Iteration:        iteration,
		CallIndex:        callIndex,
		AgentID:          agentID,
		SentAt:           sentAt,
		FinishedAt:       finishedAt,
		PromptTokens:     pt,
		CompletionTokens: ct,
		RequestJSON:      string(reqJSON),
		ResponseJSON:     string(respJSON),
		Error:            errStr,
	})
	return resp, err
}
