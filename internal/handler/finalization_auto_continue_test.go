package handler

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	agentpkg "cyberstrike-ai/internal/agent"
	"cyberstrike-ai/internal/agentfinalizer"
	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/database"
	"cyberstrike-ai/internal/mcp"

	"go.uber.org/zap"
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

func TestCleanupPendingToolExecutionsAfterIterationAllowsFinalization(t *testing.T) {
	logger := zap.NewNop()
	db, err := database.NewDB(filepath.Join(t.TempDir(), "cleanup-finalization.db"), logger)
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	server := mcp.NewServerWithStorage(logger, db)
	server.ConfigureToolWaitTimeoutSeconds(1)
	server.RegisterTool(mcp.Tool{Name: "block", InputSchema: map[string]interface{}{"type": "object"}}, func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})
	ag := agentpkg.NewAgent(&config.OpenAIConfig{}, &config.AgentConfig{}, server, nil, logger, 10)
	h := &AgentHandler{agent: ag, db: db, logger: logger}

	callCtx := mcp.WithMCPConversationID(context.Background(), "conv-cleanup")
	result, execID, err := server.CallTool(callCtx, "block", nil)
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if result == nil || !result.IsError || execID == "" {
		t.Fatalf("expected background wait result, result=%#v execID=%q", result, execID)
	}

	decision := agentfinalizer.Decide(db, agentfinalizer.Input{
		Response:        "基于已完成信息的阶段性总结。",
		MCPExecutionIDs: []string{execID},
	})
	if decision.CompletionReason != agentfinalizer.ReasonPendingTools {
		t.Fatalf("decision reason = %s, want pending tools: %+v", decision.CompletionReason, decision)
	}

	var eventType string
	cancelled := h.cleanupPendingToolExecutionsAfterIteration(context.Background(), "conv-cleanup", decision, func(et, _ string, _ interface{}) {
		eventType = et
	})
	if len(cancelled) != 1 || cancelled[0] != execID {
		t.Fatalf("cancelled = %#v, want [%s]", cancelled, execID)
	}
	if eventType != "finalization_pending_tools_cancelled" {
		t.Fatalf("event type = %q", eventType)
	}

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		exec, err := db.GetToolExecution(execID)
		if err == nil && exec != nil && exec.Status == mcp.ToolExecutionStatusCancelled {
			after := agentfinalizer.Decide(db, agentfinalizer.Input{
				Response:        "基于已完成信息的阶段性总结。",
				MCPExecutionIDs: []string{execID},
			})
			if !after.Finalizable || !after.Finalized {
				t.Fatalf("decision should finalize after cleanup: %+v", after)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("execution did not become cancelled")
}
