package agent

import (
	"context"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
)

type fakeCompletionClient struct {
	resp string
	err  error
}

func (f *fakeCompletionClient) Complete(_ context.Context, _ string, _ string, _ time.Duration) (string, error) {
	return f.resp, f.err
}

func TestMemoryCompressorSummarizeChunk_PreservesCriticalFindings(t *testing.T) {
	mc := &MemoryCompressor{
		summaryModel:     "test-model",
		timeout:          time.Second,
		completionClient: &fakeCompletionClient{resp: "[TASK_STATUS]\n- recon completed"},
		logger:           zap.NewNop(),
	}

	chunk := []ChatMessage{
		{Role: "user", Content: "Continue test for https://target.example and prioritize high impact findings"},
		{Role: "assistant", Content: "Found CVE-2023-1719 in Bitrix main module with potential RCE chain"},
		{Role: "tool", Content: "Error: 403 Forbidden for url: https://api.zoomeye.ai/v2/search"},
		{Role: "tool", Content: "Potential credential leak: api_key=abcdef123456"},
	}

	summary, err := mc.summarizeChunk(context.Background(), chunk)
	if err != nil {
		t.Fatalf("summarizeChunk failed: %v", err)
	}

	if !strings.Contains(summary.Content, "<context_summary message_count='4'>") {
		t.Fatalf("expected context_summary wrapper with message count")
	}
	if !strings.Contains(summary.Content, "[PRESERVED_KEY_FINDINGS]") {
		t.Fatalf("expected preserved findings section")
	}
	if !strings.Contains(strings.ToLower(summary.Content), "cve-2023-1719") {
		t.Fatalf("expected CVE finding to be preserved")
	}
	if !strings.Contains(strings.ToLower(summary.Content), "403 forbidden") {
		t.Fatalf("expected decisive error to be preserved")
	}
}

func TestMemoryCompressorExtractMessageText_StripsSummaryTags(t *testing.T) {
	mc := &MemoryCompressor{}
	msg := ChatMessage{
		Role:    "assistant",
		Content: "<context_summary message_count='7'>\n[TASK_STATUS]\n- Step 1 done\n</context_summary>",
	}
	out := mc.extractMessageText(msg)
	if strings.Contains(out, "<context_summary") || strings.Contains(out, "</context_summary>") {
		t.Fatalf("expected context_summary tags to be removed")
	}
	if !strings.Contains(out, "[TASK_STATUS]") {
		t.Fatalf("expected content to remain after stripping tags")
	}
}
