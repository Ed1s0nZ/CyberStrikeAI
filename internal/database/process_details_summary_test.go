package database

import (
	"path/filepath"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestProcessDetailsSummaryDoesNotGuessIDLessResultsOntoDifferentTool(t *testing.T) {
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
		{"toolName": "other-tool", "success": true},
		{"toolName": "other-tool", "success": true},
	}
	var resultIDs []string
	for _, result := range results {
		resultID, err := db.AddProcessDetailWithID(messageID, conversationID, "tool_result", "result", result)
		if err != nil {
			t.Fatalf("AddProcessDetail(tool_result): %v", err)
		}
		resultIDs = append(resultIDs, resultID)
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
		if execution.ResultDetailID != resultIDs[i] {
			t.Fatalf("execution %d result detail id = %q, want %q", i, execution.ResultDetailID, resultIDs[i])
		}
	}
	for i, execution := range summary.ToolExecutions[2:4] {
		if execution.Status != "result_missing" {
			t.Fatalf("unmatched call %d status = %q, want result_missing", i, execution.Status)
		}
	}
	for i, execution := range summary.ToolExecutions[4:] {
		if execution.Status != "completed" || execution.ToolCallID != "" || execution.ToolName != "other-tool" {
			t.Fatalf("idless result %d = %#v, want separate completed result without toolCallId", i, execution)
		}
	}
}

func TestProcessDetailsSummaryPairsIDLessResultsWithSameToolName(t *testing.T) {
	db, conversationID, messageID := setupProcessDetailsSummaryTest(t)
	for i, id := range []string{"call-1", "call-2"} {
		if err := db.AddProcessDetail(messageID, conversationID, "tool_call", "call", map[string]interface{}{
			"toolName": "nmap", "toolCallId": id, "index": i + 1, "total": 2,
		}); err != nil {
			t.Fatalf("AddProcessDetail(tool_call): %v", err)
		}
	}
	var resultIDs []string
	for i := 0; i < 2; i++ {
		resultID, err := db.AddProcessDetailWithID(messageID, conversationID, "tool_result", "result", map[string]interface{}{
			"toolName": "nmap", "success": true,
		})
		if err != nil {
			t.Fatalf("AddProcessDetail(tool_result): %v", err)
		}
		resultIDs = append(resultIDs, resultID)
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
		if execution.ResultDetailID != resultIDs[i] {
			t.Fatalf("execution %d result detail id = %q, want %q", i, execution.ResultDetailID, resultIDs[i])
		}
	}
}

func TestProcessDetailsSummaryPairedResultWithoutSuccessIsCompleted(t *testing.T) {
	db, conversationID, messageID := setupProcessDetailsSummaryTest(t)
	if err := db.AddProcessDetail(messageID, conversationID, "tool_call", "call", map[string]interface{}{
		"toolName": "nmap", "toolCallId": "call-1",
	}); err != nil {
		t.Fatalf("AddProcessDetail(tool_call): %v", err)
	}
	if err := db.AddProcessDetail(messageID, conversationID, "tool_result", "result", map[string]interface{}{
		"toolName": "nmap", "toolCallId": "call-1", "resultPreview": "open 22",
	}); err != nil {
		t.Fatalf("AddProcessDetail(tool_result): %v", err)
	}

	summary, err := db.GetProcessDetailsSummary(messageID)
	if err != nil {
		t.Fatalf("GetProcessDetailsSummary: %v", err)
	}
	if len(summary.ToolExecutions) != 1 || summary.ToolExecutions[0].Status != "completed" {
		t.Fatalf("tool executions = %#v, want completed", summary.ToolExecutions)
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

func TestProcessDetailsSummaryReportsUnmatchedToolCallAsRunningForActiveTurn(t *testing.T) {
	db, conversationID, messageID := setupProcessDetailsSummaryTest(t)
	if _, err := db.Exec(
		"UPDATE messages SET content = ?, updated_at = ? WHERE id = ?",
		"处理中...", "2026-08-10T08:00:00Z", messageID,
	); err != nil {
		t.Fatalf("update running message: %v", err)
	}
	if err := db.AddProcessDetail(messageID, conversationID, "tool_call", "call", map[string]interface{}{
		"toolName": "execute", "toolCallId": "pending",
	}); err != nil {
		t.Fatalf("AddProcessDetail(tool_call): %v", err)
	}

	summary, err := db.GetProcessDetailsSummary(messageID)
	if err != nil {
		t.Fatalf("GetProcessDetailsSummary: %v", err)
	}
	if summary.Status != "running" {
		t.Fatalf("summary status = %q, want running", summary.Status)
	}
	if len(summary.ToolExecutions) != 1 || summary.ToolExecutions[0].Status != "running" {
		t.Fatalf("tool executions = %#v, want running", summary.ToolExecutions)
	}
}

func TestProcessDetailsSummaryIncludesPersistedTurnTiming(t *testing.T) {
	db, _, messageID := setupProcessDetailsSummaryTest(t)
	startedAt := "2026-08-10T08:00:00Z"
	completedAt := "2026-08-10T08:12:59Z"
	if _, err := db.Exec(
		"UPDATE messages SET content = ?, created_at = ?, updated_at = ? WHERE id = ?",
		"done", startedAt, completedAt, messageID,
	); err != nil {
		t.Fatalf("update message timing: %v", err)
	}

	summary, err := db.GetProcessDetailsSummary(messageID)
	if err != nil {
		t.Fatalf("GetProcessDetailsSummary: %v", err)
	}
	if summary.Status != "completed" {
		t.Fatalf("status = %q, want completed", summary.Status)
	}
	if summary.StartedAt == nil || summary.CompletedAt == nil {
		t.Fatalf("timing missing: %#v", summary)
	}
	if want := int64((12*time.Minute + 59*time.Second) / time.Millisecond); summary.DurationMs != want {
		t.Fatalf("durationMs = %d, want %d", summary.DurationMs, want)
	}
}

func TestProcessDetailsSummaryTreatsCancelledPlaceholderAsTerminal(t *testing.T) {
	db, conversationID, messageID := setupProcessDetailsSummaryTest(t)
	startedAt := "2026-08-10T08:00:00Z"
	if _, err := db.Exec(
		"UPDATE messages SET content = ?, created_at = ?, updated_at = ? WHERE id = ?",
		"处理中...", startedAt, startedAt, messageID,
	); err != nil {
		t.Fatalf("update running placeholder: %v", err)
	}
	if _, err := db.Exec(`
INSERT INTO process_details (id, message_id, conversation_id, event_type, message, data, created_at)
VALUES ('cancelled-detail', ?, ?, 'cancelled', 'interrupted', '{}', '2026-08-10T08:02:05Z')`,
		messageID, conversationID); err != nil {
		t.Fatalf("insert cancelled detail: %v", err)
	}

	summary, err := db.GetProcessDetailsSummary(messageID)
	if err != nil {
		t.Fatalf("GetProcessDetailsSummary: %v", err)
	}
	if summary.Status != "cancelled" {
		t.Fatalf("status = %q, want cancelled", summary.Status)
	}
	if summary.CompletedAt == nil {
		t.Fatal("cancelled summary should expose a fixed completion time")
	}
	if want := int64((2*time.Minute + 5*time.Second) / time.Millisecond); summary.DurationMs != want {
		t.Fatalf("durationMs = %d, want %d", summary.DurationMs, want)
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
