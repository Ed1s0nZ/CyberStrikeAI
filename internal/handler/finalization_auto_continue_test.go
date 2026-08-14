package handler

import (
	"testing"

	"cyberstrike-ai/internal/agentfinalizer"
)

func TestShouldAutoContinueAfterFinalization(t *testing.T) {
	missingEvidence := agentfinalizer.Decision{
		Status:           agentfinalizer.StatusBlocked,
		CompletionReason: agentfinalizer.ReasonMissingEvidence,
	}
	if !shouldAutoContinueAfterFinalization(missingEvidence, 0) {
		t.Fatal("missing execution evidence should trigger auto-continue")
	}
	if shouldAutoContinueAfterFinalization(missingEvidence, finalizationAutoContinueMaxAttempts) {
		t.Fatal("auto-continue should stop at max attempts")
	}

	finalized := agentfinalizer.Decision{
		Status:           agentfinalizer.StatusCompleted,
		CompletionReason: agentfinalizer.ReasonVerified,
		Finalizable:      true,
		Finalized:        true,
	}
	if shouldAutoContinueAfterFinalization(finalized, 0) {
		t.Fatal("finalized decision should not auto-continue")
	}

	awaitingHITL := agentfinalizer.Decision{
		Status:           agentfinalizer.StatusAwaitingHITL,
		CompletionReason: agentfinalizer.ReasonAwaitingHITL,
	}
	if shouldAutoContinueAfterFinalization(awaitingHITL, 0) {
		t.Fatal("awaiting HITL should not auto-continue without approval")
	}
}

func TestRequestRequiresExecutionEvidenceUsesExplicitPolicyOnly(t *testing.T) {
	if requestRequiresExecutionEvidence(nil) {
		t.Fatal("nil request should not require execution evidence")
	}
	if requestRequiresExecutionEvidence(&ChatRequest{}) {
		t.Fatal("missing finalization policy should not require execution evidence")
	}
	require := true
	if !requestRequiresExecutionEvidence(&ChatRequest{
		Finalization: ChatFinalizationRequest{RequireExecutionEvidence: &require},
	}) {
		t.Fatal("explicit true policy should require execution evidence")
	}
	require = false
	if requestRequiresExecutionEvidence(&ChatRequest{
		Finalization: ChatFinalizationRequest{RequireExecutionEvidence: &require},
	}) {
		t.Fatal("explicit false policy should not require execution evidence")
	}
}
