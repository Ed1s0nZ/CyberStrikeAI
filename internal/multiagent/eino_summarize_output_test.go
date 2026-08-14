package multiagent

import (
	"strings"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

func TestStripAnalysisFromSummarizationText(t *testing.T) {
	in := "<analysis>internal notes</analysis>\n\n<summary>\n## 1. 授权\n- example.com\n</summary>"
	got := stripAnalysisFromSummarizationText(in)
	if strings.Contains(got, "<analysis>") {
		t.Fatalf("analysis block should be removed: %q", got)
	}
	if !strings.Contains(got, "## 1. 授权") {
		t.Fatalf("summary body should remain: %q", got)
	}
}

func TestStripAnalysisFromSummarizationMessage_UserInputMultiContent(t *testing.T) {
	msg := &schema.Message{
		Role: schema.User,
		UserInputMultiContent: []schema.MessageInputPart{
			{
				Type: schema.ChatMessagePartTypeText,
				Text: "此会话延续自此前一段因上下文耗尽而终止的对话。\n\n<analysis>draft</analysis>\n<summary>body</summary>\n\n完整记录位于：/tmp/transcript.txt",
			},
			{
				Type: schema.ChatMessagePartTypeText,
				Text: "请从我们中断的地方继续对话，无需向用户提出任何进一步的问题。",
			},
		},
	}
	out := stripAnalysisFromSummarizationMessage(msg)
	if len(out.UserInputMultiContent) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(out.UserInputMultiContent))
	}
	if strings.Contains(out.UserInputMultiContent[0].Text, "<analysis>") {
		t.Fatalf("part 0 should drop analysis: %q", out.UserInputMultiContent[0].Text)
	}
	if !strings.Contains(out.UserInputMultiContent[0].Text, "<summary>body</summary>") {
		t.Fatalf("part 0 should keep summary: %q", out.UserInputMultiContent[0].Text)
	}
	if out.UserInputMultiContent[1].Text != "请从我们中断的地方继续对话，无需向用户提出任何进一步的问题。" {
		t.Fatalf("continue instruction part should be unchanged: %q", out.UserInputMultiContent[1].Text)
	}
}

func TestExtractSummarizationSummaryBody(t *testing.T) {
	body, ok := extractSummarizationSummaryBody("<analysis>x</analysis><summary>  kept  </summary>")
	if !ok || body != "kept" {
		t.Fatalf("extract summary body: ok=%v body=%q", ok, body)
	}
	_, ok = extractSummarizationSummaryBody("plain text only")
	if ok {
		t.Fatal("expected false for plain text")
	}
}

func TestStripAnalysisFromSummarizationText_NoAnalysisUnchanged(t *testing.T) {
	in := "<summary>only summary</summary>"
	got := stripAnalysisFromSummarizationText(in)
	if got != in {
		t.Fatalf("expected unchanged text, got %q", got)
	}
}

func TestBuildOriginalUserIntentLedgerMessage_AppendsRawUserMessages(t *testing.T) {
	original := []adk.Message{
		schema.UserMessage("第一轮：只测 staging，不要碰 prod。"),
		schema.AssistantMessage("ok", nil),
		schema.UserMessage("第二轮：优先验证 /api/login 的 SQL 注入。"),
		schema.UserMessage(FormatEmptyResponseContinueUserMessage()),
	}

	out := buildOriginalUserIntentLedgerMessage(original, 96000, 16000)
	if out == nil {
		t.Fatal("ledger message should be non-nil")
	}
	if out.Role != schema.System {
		t.Fatalf("ledger should be a system anchor, got %s", out.Role)
	}
	body := out.Content
	if !strings.Contains(body, userIntentLedgerStartMarker) || !strings.Contains(body, userIntentLedgerEndMarker) {
		t.Fatalf("ledger markers missing: %q", body)
	}
	if !strings.Contains(body, "只测 staging，不要碰 prod") {
		t.Fatalf("first user constraint missing: %q", body)
	}
	if !strings.Contains(body, "优先验证 /api/login") {
		t.Fatalf("second user request missing: %q", body)
	}
	if strings.Contains(body, "系统自动续跑") {
		t.Fatalf("synthetic auto-resume user message should be skipped: %q", body)
	}
}

func TestBuildOriginalUserIntentLedgerMessage_CarriesPreviousLedgerAndDedups(t *testing.T) {
	prevSummary := schema.AssistantMessage(wrapUserIntentLedger("- [U001] 原始目标：example.com\n- [U002] 禁止高危破坏性操作"), nil)
	original := []adk.Message{
		prevSummary,
		schema.UserMessage("禁止高危破坏性操作"),
		schema.UserMessage("新增约束：只输出中文报告"),
	}

	out := buildOriginalUserIntentLedgerMessage(original, 96000, 16000)
	body := out.Content
	if !strings.Contains(body, "原始目标：example.com") || !strings.Contains(body, "新增约束：只输出中文报告") {
		t.Fatalf("ledger did not carry old and new entries: %q", body)
	}
	if strings.Count(body, "禁止高危破坏性操作") != 1 {
		t.Fatalf("duplicate ledger entry was not deduped: %q", body)
	}
	if strings.Count(body, userIntentLedgerStartMarker) != 1 {
		t.Fatalf("expected one ledger block: %q", body)
	}
}

func TestStripOriginalUserIntentLedgerFromMessages_RemovesOldLedgerBeforeRebuild(t *testing.T) {
	msgs := []adk.Message{
		schema.SystemMessage("sys\n\n" + wrapUserIntentLedger("- [U001] old goal")),
		schema.AssistantMessage("summary\n\n"+wrapUserIntentLedger("- [U001] old goal"), nil),
	}

	out := stripOriginalUserIntentLedgerFromMessages(msgs)
	if len(out) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(out))
	}
	for _, msg := range out {
		if strings.Contains(msg.Content, userIntentLedgerStartMarker) || strings.Contains(msg.Content, "old goal") {
			t.Fatalf("old ledger leaked after strip: %q", msg.Content)
		}
	}
	if out[0].Content != "sys" {
		t.Fatalf("non-ledger system content should remain: %q", out[0].Content)
	}
	if out[1].Content != "summary" {
		t.Fatalf("non-ledger summary content should remain: %q", out[1].Content)
	}
}

func TestMergeMessageIntoLeadingSystem_KeepsLedgerSeparateFromSummary(t *testing.T) {
	sys := schema.SystemMessage("sys")
	ledger := schema.SystemMessage(wrapUserIntentLedger("- [U001] goal"))
	summary := schema.AssistantMessage("<summary>work state</summary>", nil)

	out := mergeMessageIntoLeadingSystem([]adk.Message{sys, summary}, ledger)
	if len(out) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(out))
	}
	if out[0].Role != schema.System || !strings.Contains(out[0].Content, "sys") || !strings.Contains(out[0].Content, userIntentLedgerStartMarker) {
		t.Fatalf("ledger should be merged into leading system: %+v", out[0])
	}
	if out[1] != summary {
		t.Fatalf("summary should remain second message")
	}
	if strings.Contains(out[1].Content, userIntentLedgerStartMarker) {
		t.Fatalf("summary should not carry ledger: %q", out[1].Content)
	}
}

func TestMergeMessageIntoLeadingSystem_NoSystemPrependsLedger(t *testing.T) {
	ledger := schema.SystemMessage(wrapUserIntentLedger("- [U001] goal"))
	summary := schema.AssistantMessage("<summary>work state</summary>", nil)

	out := mergeMessageIntoLeadingSystem([]adk.Message{summary}, ledger)
	if len(out) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(out))
	}
	if out[0] != ledger || out[1] != summary {
		t.Fatalf("ledger should be prepended when no system exists: %+v", out)
	}
}
