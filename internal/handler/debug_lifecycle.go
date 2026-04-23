package handler

import (
	"cyberstrike-ai/internal/debug"
)

// wrapRunWithDebug calls sink.StartSession before runFn and
// sink.EndSession after (with the outcome runFn returned).
// Centralizes the boundary logic so MultiAgentLoop uses a single
// code path for the capture bookends.
//
// runFn returns (outcome, err) where outcome is one of
// "completed"|"cancelled"|"failed"|"interrupted"|"". Empty-string
// outcome is defaulted to "failed" on non-nil err, "completed" on
// nil err — so a caller that doesn't bother classifying still
// produces a terminal debug_sessions row.
//
// A nil sink is safe — both methods are skipped. Callers that want
// on-state behavior should pass a real Sink.
//
// EndSession is deferred so it fires even when runFn panics.
// StartSession is not deferred (it must run before runFn), so a
// panic inside StartSession itself would skip EndSession — that edge
// case is accepted as negligible given StartSession's trivial body.
func wrapRunWithDebug(sink debug.Sink, conversationID string, runFn func() (string, error)) (outcome string, err error) {
	if sink != nil {
		sink.StartSession(conversationID)
	}
	defer func() {
		// Apply defaults before EndSession fires so the deferred
		// call always receives a non-empty outcome string.
		if outcome == "" {
			if err != nil {
				outcome = "failed"
			} else {
				outcome = "completed"
			}
		}
		if sink != nil {
			sink.EndSession(conversationID, outcome)
		}
	}()
	outcome, err = runFn()
	return outcome, err
}
