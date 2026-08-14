package multiagent

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

var (
	summarizationAnalysisBlockRegex = regexp.MustCompile(`(?is)<analysis>\s*.*?\s*</analysis>`)
	summarizationSummaryBlockRegex  = regexp.MustCompile(`(?is)<summary>\s*(.*?)\s*</summary>`)
	userIntentLedgerBlockRegex      = regexp.MustCompile(`(?is)<original_user_intent_ledger>\s*(.*?)\s*</original_user_intent_ledger>`)
	userIntentLedgerSectionRegex    = regexp.MustCompile(`(?is)\s*## 原始用户输入与约束账本（系统保真）\s*<original_user_intent_ledger>\s*.*?\s*</original_user_intent_ledger>\s*`)
)

const (
	userIntentLedgerStartMarker = "<original_user_intent_ledger>"
	userIntentLedgerEndMarker   = "</original_user_intent_ledger>"

	summarizationTranscriptPathInstructionZh = "如果你需要压缩之前的具体细节（如精确的代码片段、错误消息或你生成的内容），完整的对话记录位于：%s"
)

// stripAnalysisFromSummarizationMessage removes the <analysis> block from a post-processed
// Eino summary user message. Analysis helps one-shot generation quality but should not
// occupy continuation context after compaction.
func stripAnalysisFromSummarizationMessage(msg adk.Message) adk.Message {
	if msg == nil {
		return msg
	}
	cloned := *msg
	if cloned.Content != "" {
		cloned.Content = stripAnalysisFromSummarizationText(cloned.Content)
	}
	if len(cloned.UserInputMultiContent) > 0 {
		parts := make([]schema.MessageInputPart, len(cloned.UserInputMultiContent))
		copy(parts, cloned.UserInputMultiContent)
		// Only the first text part carries model output plus Eino preamble/transcript path.
		for i := range parts {
			if parts[i].Type != schema.ChatMessagePartTypeText || parts[i].Text == "" {
				continue
			}
			if i == 0 {
				parts[i].Text = stripAnalysisFromSummarizationText(parts[i].Text)
			}
			break
		}
		cloned.UserInputMultiContent = parts
	}
	return &cloned
}

func stripAnalysisFromSummarizationText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return text
	}
	stripped := strings.TrimSpace(summarizationAnalysisBlockRegex.ReplaceAllString(text, ""))
	if stripped == "" {
		return text
	}
	return stripped
}

func appendTranscriptPathToSummarizationMessage(msg adk.Message, transcriptPath string) adk.Message {
	transcriptPath = strings.TrimSpace(transcriptPath)
	if msg == nil || transcriptPath == "" {
		return msg
	}
	section := fmt.Sprintf(summarizationTranscriptPathInstructionZh, transcriptPath)
	cloned := *msg
	if cloned.Content != "" && !strings.Contains(cloned.Content, transcriptPath) {
		cloned.Content = appendSummarizationSection(cloned.Content, section)
	}
	if len(cloned.UserInputMultiContent) > 0 {
		parts := make([]schema.MessageInputPart, len(cloned.UserInputMultiContent))
		copy(parts, cloned.UserInputMultiContent)
		for i := range parts {
			if parts[i].Type != schema.ChatMessagePartTypeText {
				continue
			}
			if parts[i].Text != "" && !strings.Contains(parts[i].Text, transcriptPath) {
				parts[i].Text = appendSummarizationSection(parts[i].Text, section)
			}
			break
		}
		cloned.UserInputMultiContent = parts
	}
	return &cloned
}

func appendSummarizationSection(text, section string) string {
	text = strings.TrimSpace(text)
	section = strings.TrimSpace(section)
	if text == "" {
		return section
	}
	if section == "" {
		return text
	}
	return text + "\n\n" + section
}

// extractSummarizationSummaryBody returns the inner text of the last <summary> block when present.
// Used by tests and optional strict compaction paths.
func extractSummarizationSummaryBody(text string) (string, bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", false
	}
	all := summarizationSummaryBlockRegex.FindAllStringSubmatch(text, -1)
	if len(all) == 0 || len(all[len(all)-1]) < 2 {
		return "", false
	}
	body := strings.TrimSpace(all[len(all)-1][1])
	if body == "" {
		return "", false
	}
	return body, true
}

// buildOriginalUserIntentLedgerMessage returns a deterministic, host-generated
// context anchor for raw user inputs. Keep it separate from the model-generated
// working summary so compaction can rewrite the summary without rewriting user intent.
func buildOriginalUserIntentLedgerMessage(originalMessages []adk.Message, maxRunes, entryMaxRunes int) adk.Message {
	ledger := buildOriginalUserIntentLedger(originalMessages, maxRunes, entryMaxRunes)
	if strings.TrimSpace(ledger) == "" {
		return nil
	}
	return schema.SystemMessage(wrapUserIntentLedger(ledger))
}

func mergeMessageIntoLeadingSystem(msgs []adk.Message, msg adk.Message) []adk.Message {
	if msg == nil {
		return msgs
	}
	for i, existing := range msgs {
		if existing == nil || existing.Role != schema.System {
			continue
		}
		cloned := *existing
		cloned.Content = strings.TrimSpace(strings.TrimSpace(cloned.Content) + "\n\n" + strings.TrimSpace(msg.Content))
		out := make([]adk.Message, len(msgs))
		copy(out, msgs)
		out[i] = &cloned
		return out
	}
	out := make([]adk.Message, 0, len(msgs)+1)
	out = append(out, msg)
	out = append(out, msgs...)
	return out
}

func stripOriginalUserIntentLedgerFromMessages(msgs []adk.Message) []adk.Message {
	if len(msgs) == 0 {
		return msgs
	}
	out := make([]adk.Message, 0, len(msgs))
	for _, msg := range msgs {
		if msg == nil {
			continue
		}
		cloned := *msg
		cloned.Content = stripOriginalUserIntentLedgerFromText(cloned.Content)
		if len(cloned.UserInputMultiContent) > 0 {
			parts := make([]schema.MessageInputPart, len(cloned.UserInputMultiContent))
			copy(parts, cloned.UserInputMultiContent)
			for i := range parts {
				if parts[i].Type == schema.ChatMessagePartTypeText {
					parts[i].Text = stripOriginalUserIntentLedgerFromText(parts[i].Text)
				}
			}
			cloned.UserInputMultiContent = parts
		}
		if strings.TrimSpace(cloned.Content) == "" && len(cloned.UserInputMultiContent) == 0 && len(cloned.ToolCalls) == 0 && cloned.ReasoningContent == "" {
			continue
		}
		out = append(out, &cloned)
	}
	return out
}

func buildOriginalUserIntentLedger(msgs []adk.Message, maxRunes, entryMaxRunes int) string {
	entries := collectOriginalUserIntentEntries(msgs)
	if len(entries) == 0 {
		return ""
	}
	var sb strings.Builder
	for i, entry := range entries {
		line := fmt.Sprintf("- [U%03d] %s\n", i+1, sanitizeUserIntentLedgerEntry(entry, entryMaxRunes))
		if maxRunes > 0 && utf8RuneLen(sb.String())+utf8RuneLen(line) > maxRunes {
			sb.WriteString("- [...truncated] 用户原始输入账本超过预算；完整压缩前记录见 summarization transcript。\n")
			break
		}
		sb.WriteString(line)
	}
	return strings.TrimSpace(sb.String())
}

func collectOriginalUserIntentEntries(msgs []adk.Message) []string {
	seen := make(map[string]struct{})
	entries := make([]string, 0, 16)
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" || isSyntheticContinuationUserText(s) {
			return
		}
		key := normalizeUserIntentLedgerText(s)
		if key == "" {
			return
		}
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		entries = append(entries, s)
	}
	for _, msg := range msgs {
		if msg == nil {
			continue
		}
		for _, entry := range extractExistingUserIntentLedgerEntries(messageTextForLedger(msg)) {
			add(entry)
		}
		if msg.Role == schema.User {
			add(adkUserMessageText(msg))
		}
	}
	return entries
}

func messageTextForLedger(msg adk.Message) string {
	if msg == nil {
		return ""
	}
	var b strings.Builder
	if strings.TrimSpace(msg.Content) != "" {
		b.WriteString(msg.Content)
	}
	for _, part := range msg.UserInputMultiContent {
		if part.Type != schema.ChatMessagePartTypeText || strings.TrimSpace(part.Text) == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(part.Text)
	}
	return b.String()
}

func extractExistingUserIntentLedgerEntries(text string) []string {
	matches := userIntentLedgerBlockRegex.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil
	}
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		for _, line := range strings.Split(match[1], "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "- [...truncated]") {
				continue
			}
			if strings.HasPrefix(line, "- [U") {
				if idx := strings.Index(line, "]"); idx >= 0 && idx+1 < len(line) {
					line = strings.TrimSpace(line[idx+1:])
				}
			}
			if line != "" {
				out = append(out, line)
			}
		}
	}
	return out
}

func stripOriginalUserIntentLedgerFromText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	text = userIntentLedgerSectionRegex.ReplaceAllString(text, "\n")
	text = userIntentLedgerBlockRegex.ReplaceAllString(text, "\n")
	return strings.TrimSpace(text)
}

func wrapUserIntentLedger(ledger string) string {
	return strings.TrimSpace("## 原始用户输入与约束账本（系统保真）\n" +
		userIntentLedgerStartMarker + "\n" +
		strings.TrimSpace(ledger) + "\n" +
		userIntentLedgerEndMarker)
}

func sanitizeUserIntentLedgerEntry(s string, maxRunes int) string {
	s = strings.ReplaceAll(strings.TrimSpace(s), userIntentLedgerStartMarker, "[ledger-start]")
	s = strings.ReplaceAll(s, userIntentLedgerEndMarker, "[ledger-end]")
	s = strings.Join(strings.Fields(s), " ")
	if maxRunes > 0 && utf8RuneLen(s) > maxRunes {
		return truncateUserIntentLedgerRunes(s, maxRunes) + "..."
	}
	return s
}

func normalizeUserIntentLedgerText(s string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}

func isSyntheticContinuationUserText(s string) bool {
	return strings.Contains(s, continuationSessionMarker) ||
		strings.Contains(s, "【系统自动续跑 / Auto resume】")
}

func utf8RuneLen(s string) int {
	return len([]rune(s))
}

func truncateUserIntentLedgerRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}
