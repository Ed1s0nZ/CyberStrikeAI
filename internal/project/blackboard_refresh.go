package project

import "strings"

// FactIndexSectionHeading 黑板索引可读标题行前缀（块内保留，供 Agent 阅读）。
const FactIndexSectionHeading = "## 项目黑板索引"

// FactIndexSectionStartMarker / EndMarker：HTML 注释边界，供程序化替换；对模型无指令语义。
const (
	FactIndexSectionStartMarker = "<!-- fact-index-start -->"
	FactIndexSectionEndMarker   = "<!-- fact-index-end -->"
)

// ReplaceFactIndexSection 用 freshIndex 替换 content 中已有的项目黑板索引段。
// freshIndex 须为 BuildFactIndexBlock 的完整输出。起止 HTML 注释缺失时返回 (_, false)。
func ReplaceFactIndexSection(content, freshIndex string) (string, bool) {
	freshIndex = strings.TrimSpace(freshIndex)
	if freshIndex == "" {
		return content, false
	}
	start, ok := factIndexSectionStart(content)
	if !ok {
		return content, false
	}
	end, ok := factIndexSectionEnd(content, start)
	if !ok || end <= start {
		return content, false
	}
	return content[:start] + freshIndex + content[end:], true
}

// wrapFactIndexBlock 为 BuildFactIndexBlock 正文加上统一起止 HTML 注释边界。
func wrapFactIndexBlock(content string) string {
	content = strings.TrimSpace(content)
	return FactIndexSectionStartMarker + "\n" + content + "\n" + FactIndexSectionEndMarker + "\n"
}

func factIndexSectionStart(content string) (int, bool) {
	idx := strings.Index(content, FactIndexSectionStartMarker)
	if idx < 0 {
		return 0, false
	}
	return idx, true
}

func factIndexSectionEnd(content string, start int) (int, bool) {
	if start < 0 || start >= len(content) {
		return 0, false
	}
	tail := content[start:]
	idx := strings.LastIndex(tail, FactIndexSectionEndMarker)
	if idx < 0 {
		return 0, false
	}
	return start + idx + len(FactIndexSectionEndMarker), true
}
