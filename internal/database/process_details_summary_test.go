package database

import (
	"path/filepath"
	"testing"

	"go.uber.org/zap"
)

func TestProcessDetailsSummaryDoesNotGuessIDLessResultsByOrder(t *testing.T) {
	db, conversationID, messageID := setupProcessDetailsSummaryTest(t)
	for _, id := range []string{"call-1", "call-2", "call-3", "call-4"} {
		if err := db.AddProcessDetail(messageID, conversationID, "tool_call", "call", map[string]interface{}{
			"toolName": "http-framework-test", "toolCallId": id,
		}); err != nil {
			t.Fatalf("AddProcessDetail(tool_call): %v", err)
		}
	}
	results := []map[string]interface{}{
		{"toolName": "http-framework-test", "toolCallId": "call-1", "success": true},
		{"toolName": "http-framework-test", "toolCallId": "call-2", "success": true},
		{"toolName": "http-framework-test", "success": true},
		{"toolName": "http-framework-test", "success": true},
	}
	for _, result := range results {
		if err := db.AddProcessDetail(messageID, conversationID, "tool_result", "result", result); err != nil {
			t.Fatalf("AddProcessDetail(tool_result): %v", err)
		}
	}

	summary, err := db.GetProcessDetailsSummary(messageID)
	if err != nil {
		t.Fatalf("GetProcessDetailsSummary: %v", err)
	}
	if len(summary.ToolExecutions) != 6 {
		t.Fatalf("tool executions = %d, want 6", len(summary.ToolExecutions))
	}
	for i, execution := range summary.ToolExecutions[:2] {
		if execution.Status != "completed" {
			t.Fatalf("execution %d status = %q, want completed", i, execution.Status)
		}
	}
	for i, execution := range summary.ToolExecutions[2:4] {
		if execution.Status != "result_missing" {
			t.Fatalf("unmatched call %d status = %q, want result_missing", i, execution.Status)
		}
	}
	for i, execution := range summary.ToolExecutions[4:] {
		if execution.Status != "completed" || execution.ToolCallID != "" {
			t.Fatalf("idless result %d = %#v, want separate completed result without toolCallId", i, execution)
		}
	}
}

func TestProcessDetailsSummaryPairsRepeatedToolCallIDsFIFO(t *testing.T) {
	db, conversationID, messageID := setupProcessDetailsSummaryTest(t)
	for i := 0; i < 2; i++ {
		if err := db.AddProcessDetail(messageID, conversationID, "tool_call", "call", map[string]interface{}{
			"toolName": "execute", "toolCallId": "legacy-reused-id",
		}); err != nil {
			t.Fatalf("AddProcessDetail(tool_call): %v", err)
		}
	}
	for i := 0; i < 2; i++ {
		if err := db.AddProcessDetail(messageID, conversationID, "tool_result", "result", map[string]interface{}{
			"toolName": "execute", "toolCallId": "legacy-reused-id", "success": true,
		}); err != nil {
			t.Fatalf("AddProcessDetail(tool_result): %v", err)
		}
	}

	summary, err := db.GetProcessDetailsSummary(messageID)
	if err != nil {
		t.Fatalf("GetProcessDetailsSummary: %v", err)
	}
	if len(summary.ToolExecutions) != 2 {
		t.Fatalf("tool executions = %d, want 2", len(summary.ToolExecutions))
	}
	for i, execution := range summary.ToolExecutions {
		if execution.Status != "completed" {
			t.Fatalf("execution %d status = %q, want completed", i, execution.Status)
		}
	}
}

func TestProcessDetailsSummaryDoesNotReportPersistedOrphanAsRunning(t *testing.T) {
	db, conversationID, messageID := setupProcessDetailsSummaryTest(t)
	if err := db.AddProcessDetail(messageID, conversationID, "tool_call", "call", map[string]interface{}{
		"toolName": "execute", "toolCallId": "orphan",
	}); err != nil {
		t.Fatalf("AddProcessDetail(tool_call): %v", err)
	}
	summary, err := db.GetProcessDetailsSummary(messageID)
	if err != nil {
		t.Fatalf("GetProcessDetailsSummary: %v", err)
	}
	if len(summary.ToolExecutions) != 1 || summary.ToolExecutions[0].Status != "result_missing" {
		t.Fatalf("tool executions = %#v, want result_missing", summary.ToolExecutions)
	}
}

func setupProcessDetailsSummaryTest(t *testing.T) (*DB, string, string) {
	t.Helper()
	db, err := NewDB(filepath.Join(t.TempDir(), "process-details.db"), zap.NewNop())
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	conversation, err := db.CreateConversation("process details", ConversationCreateMeta{})
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	message, err := db.AddMessage(conversation.ID, "assistant", "done", nil)
	if err != nil {
		t.Fatalf("AddMessage: %v", err)
	}
	return db, conversation.ID, message.ID
}
