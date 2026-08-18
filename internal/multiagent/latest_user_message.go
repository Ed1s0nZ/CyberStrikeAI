package multiagent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cyberstrike-ai/internal/config"

	"go.uber.org/zap"
)

func prepareLatestUserMessageForModel(userMessage string, appCfg *config.Config, mwCfg *config.MultiAgentEinoMiddlewareConfig, conversationID string, logger *zap.Logger) string {
	if strings.TrimSpace(userMessage) == "" {
		return strings.TrimSpace(userMessage)
	}
	if mwCfg == nil {
		var zero config.MultiAgentEinoMiddlewareConfig
		mwCfg = &zero
	}
	maxRunes := mwCfg.LatestUserMessageMaxRunesEffective()
	if appCfg != nil {
		maxRunes = minPositiveInt(maxRunes, modelFacingRuneBudget(appCfg.OpenAI.MaxTotalTokens, 0.20))
	}
	if maxRunes <= 0 || utf8RuneLen(userMessage) <= maxRunes {
		return userMessage
	}

	headRunes, tailRunes := normalizeLatestUserPreviewBudget(
		maxRunes,
		mwCfg.LatestUserMessageHeadRunesEffective(),
		mwCfg.LatestUserMessageTailRunesEffective(),
	)
	head, tail := splitHeadTailRunes(userMessage, headRunes, tailRunes)
	artifactPath, writeErr := persistLatestUserMessageArtifact(userMessage, appCfg, conversationID)
	if writeErr != nil && logger != nil {
		logger.Warn("latest user message artifact 写入失败，将仅使用预览",
			zap.String("conversationId", conversationID),
			zap.Error(writeErr),
		)
	}

	var sb strings.Builder
	sb.WriteString("【系统提示：本轮用户输入过长，已为模型上下文生成裁剪预览。】\n")
	sb.WriteString("用户原始输入已完整保存到数据库 messages.role=user；")
	if artifactPath != "" {
		sb.WriteString("同时已落盘为 artifact，可在需要全文时读取：\n")
		sb.WriteString("artifact_path: ")
		sb.WriteString(artifactPath)
		sb.WriteByte('\n')
	} else {
		sb.WriteString("artifact 写入失败时仍可从数据库原始消息恢复。\n")
	}
	sb.WriteString(fmt.Sprintf("original_runes: %d\n", utf8RuneLen(userMessage)))
	sb.WriteString(fmt.Sprintf("preview_head_runes: %d\n", utf8RuneLen(head)))
	sb.WriteString(fmt.Sprintf("preview_tail_runes: %d\n\n", utf8RuneLen(tail)))
	sb.WriteString("请优先基于以下预览理解用户目标；如必须查看全文，请读取 artifact 或数据库原始消息。\n\n")
	sb.WriteString("<latest_user_message_preview_head>\n")
	sb.WriteString(head)
	sb.WriteString("\n</latest_user_message_preview_head>\n")
	if tail != "" {
		sb.WriteString("\n<latest_user_message_preview_tail>\n")
		sb.WriteString(tail)
		sb.WriteString("\n</latest_user_message_preview_tail>\n")
	}
	return strings.TrimSpace(sb.String())
}

func modelFacingRuneBudget(maxTotalTokens int, ratio float64) int {
	if maxTotalTokens <= 0 {
		maxTotalTokens = 120000
	}
	if ratio <= 0 || ratio >= 1 {
		ratio = 0.20
	}
	budget := int(float64(maxTotalTokens) * ratio)
	if budget < 1024 {
		budget = 1024
	}
	return budget
}

func minPositiveInt(a, b int) int {
	if a <= 0 {
		return b
	}
	if b <= 0 || a < b {
		return a
	}
	return b
}

func normalizeLatestUserPreviewBudget(maxRunes, headRunes, tailRunes int) (int, int) {
	if maxRunes <= 0 {
		return headRunes, tailRunes
	}
	if headRunes <= 0 && tailRunes <= 0 {
		headRunes = maxRunes / 2
		tailRunes = maxRunes - headRunes
	}
	if headRunes < 0 {
		headRunes = 0
	}
	if tailRunes < 0 {
		tailRunes = 0
	}
	if headRunes+tailRunes <= maxRunes {
		return headRunes, tailRunes
	}
	if headRunes == 0 {
		return 0, maxRunes
	}
	if tailRunes == 0 {
		return maxRunes, 0
	}
	head := maxRunes / 2
	tail := maxRunes - head
	return head, tail
}

func splitHeadTailRunes(s string, headRunes, tailRunes int) (string, string) {
	runes := []rune(s)
	n := len(runes)
	if headRunes > n {
		headRunes = n
	}
	head := string(runes[:headRunes])
	if tailRunes <= 0 || headRunes >= n {
		return head, ""
	}
	if tailRunes > n-headRunes {
		tailRunes = n - headRunes
	}
	tail := string(runes[n-tailRunes:])
	return head, tail
}

func persistLatestUserMessageArtifact(content string, appCfg *config.Config, conversationID string) (string, error) {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		conversationID = "unknown"
	}
	baseRoot := filepath.Join(os.TempDir(), "cyberstrike-user-inputs")
	if appCfg != nil {
		if dbPath := strings.TrimSpace(appCfg.Database.Path); dbPath != "" {
			baseRoot = filepath.Join(filepath.Dir(dbPath), "conversation_artifacts")
		}
	}
	if abs, err := filepath.Abs(baseRoot); err == nil {
		baseRoot = abs
	}
	dir := filepath.Join(baseRoot, sanitizeEinoPathSegment(conversationID), "user_inputs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir user input artifact dir: %w", err)
	}
	name := "latest_user_" + time.Now().UTC().Format("20060102T150405.000000000Z") + ".txt"
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return "", fmt.Errorf("write user input artifact: %w", err)
	}
	return path, nil
}
