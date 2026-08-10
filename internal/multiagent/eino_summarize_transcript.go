package multiagent

import (
	"fmt"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"

	"cyberstrike-ai/internal/project"

	"github.com/bytedance/sonic"
)

const (
	transcriptFileHeader = `# CyberStrikeAI summarization transcript
# Pre-compaction session record for read_file after context compression.
# Omits static system/tool-index/skills boilerplate; full user/assistant/tool turns below.

`
	transcriptStaticSystemOmitNote = "[static system prompt omitted — unchanged in live context after compaction]"
	transcriptToolIndexStartMarker = "以下是当前会话绑定的工具名称索引"
	transcriptPersonaStartMarker   = "你是CyberStrikeAI"
	// ADK LanguageChinese injects skill middleware prompt with this header (see eino adk/middlewares/skill/prompt.go).
	transcriptSkillsSystemMarker        = "# Skill 系统"
	transcriptSkillsSystemMarkerEnglish = "# Skills System"
)

type transcriptToolCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// formatSummarizationTranscript renders pre-compaction messages for transcript.txt.
// Best practice: keep full user/assistant/tool turns; slim system to dynamic blocks only.
func formatSummarizationTranscript(msgs []adk.Message) string {
	var sb strings.Builder
	sb.WriteString(transcriptFileHeader)
	wrote := false
	for _, msg := range msgs {
		if msg == nil {
			continue
		}
		switch msg.Role {
		case schema.System:
			body := sanitizeSystemContentForTranscript(msg.Content)
			if strings.TrimSpace(body) == "" {
				continue
			}
			if wrote {
				sb.WriteString("\n")
			}
			appendTranscriptSection(&sb, schema.System, body)
			wrote = true
		default:
			if wrote {
				sb.WriteString("\n")
			}
			appendTranscriptMessage(&sb, msg)
			wrote = true
		}
	}
	return sb.String()
}

// formatSummarizationModelContext serializes conversation history as inert text
// for the summary model. Unlike formatSummarizationTranscript it omits the file
// header and never emits native assistant/tool protocol messages.
func formatSummarizationModelContext(msgs []adk.Message) string {
	var sb strings.Builder
	for _, msg := range msgs {
		if msg == nil || msg.Role == schema.System {
			continue
		}
		if sb.Len() > 0 {
			sb.WriteByte('\n')
		}
		appendTranscriptMessage(&sb, msg)
	}
	return sb.String()
}

func sanitizeSystemContentForTranscript(content string) string {
	content = stripToolNamesIndexFromSystem(content)
	content = stripSkillsSystemBoilerplate(content)
	blackboard := extractProjectBlackboardSection(content)

	var sb strings.Builder
	sb.WriteString(transcriptStaticSystemOmitNote)
	if bb := strings.TrimSpace(blackboard); bb != "" {
		sb.WriteString("\n\n")
		sb.WriteString(bb)
	}
	return sb.String()
}

func stripToolNamesIndexFromSystem(s string) string {
	if !strings.Contains(s, transcriptToolIndexStartMarker) {
		return s
	}
	idx := strings.Index(s, transcriptPersonaStartMarker)
	if idx < 0 {
		return s
	}
	return strings.TrimSpace(s[idx:])
}

func stripSkillsSystemBoilerplate(s string) string {
	idx := indexFirstSubstring(s, transcriptSkillsSystemMarker, transcriptSkillsSystemMarkerEnglish)
	if idx < 0 {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(s[:idx])
}

func indexFirstSubstring(s string, markers ...string) int {
	first := -1
	for _, m := range markers {
		if i := strings.Index(s, m); i >= 0 && (first < 0 || i < first) {
			first = i
		}
	}
	return first
}

func extractProjectBlackboardSection(s string) string {
	start := strings.Index(s, project.FactIndexSectionStartMarker)
	if start < 0 {
		return ""
	}
	section := s[start:]
	end := strings.Index(section, project.FactIndexSectionEndMarker)
	if end < 0 {
		return ""
	}
	section = section[:end+len(project.FactIndexSectionEndMarker)]
	return strings.TrimSpace(section)
}

func appendTranscriptSection(sb *strings.Builder, role schema.RoleType, body string) {
	sb.WriteString("--- [")
	sb.WriteString(string(role))
	sb.WriteString("] ---\n")
	sb.WriteString(body)
	if !strings.HasSuffix(body, "\n") {
		sb.WriteByte('\n')
	}
}

func appendTranscriptMessage(sb *strings.Builder, msg adk.Message) {
	sb.WriteString("--- [")
	sb.WriteString(string(msg.Role))
	sb.WriteString("] ---\n")
	if msg.Content != "" {
		sb.WriteString(msg.Content)
		if !strings.HasSuffix(msg.Content, "\n") {
			sb.WriteByte('\n')
		}
	}
	if msg.ReasoningContent != "" {
		sb.WriteString("[reasoning]\n")
		sb.WriteString(msg.ReasoningContent)
		if !strings.HasSuffix(msg.ReasoningContent, "\n") {
			sb.WriteByte('\n')
		}
	}
	for _, part := range msg.UserInputMultiContent {
		if part.Type == schema.ChatMessagePartTypeText && strings.TrimSpace(part.Text) != "" {
			sb.WriteString(part.Text)
			if !strings.HasSuffix(part.Text, "\n") {
				sb.WriteByte('\n')
			}
		}
	}
	if len(msg.ToolCalls) > 0 {
		if b, err := sonic.Marshal(formatTranscriptToolCalls(msg.ToolCalls)); err == nil {
			sb.WriteString("tool_calls: ")
			sb.Write(b)
			sb.WriteByte('\n')
		}
	}
}

func formatTranscriptToolCalls(calls []schema.ToolCall) []transcriptToolCall {
	out := make([]transcriptToolCall, 0, len(calls))
	for _, tc := range calls {
		out = append(out, transcriptToolCall{
			Name:      tc.Function.Name,
			Arguments: compactToolArguments(tc.Function.Arguments),
		})
	}
	return out
}
// compactToolArguments 对 tool_calls 的 arguments JSON 做"值级有损精简"，
// 降低 transcript 转录时的 token 占用（长路径/base64/长字符串噪声）。
// 保留参数名与 JSON 结构，仅将超长字符串值截断为可读前缀 + 省略计数；
// 整体长度也受 maxTotalRunes 上限约束。解析失败时退化为原始串安全截断。
func compactToolArguments(raw string) string {
	const maxValueRunes = 160 // 单个字符串值保留的最大 rune 数
	const maxTotalRunes = 4096 // 精简后整体最大 rune 数

	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed == "null" {
		return trimmed
	}
	var obj map[string]any
	if err := sonic.Unmarshal([]byte(trimmed), &obj); err != nil {
		return clipString(raw, maxValueRunes)
	}
	compactJSONValue(obj, maxValueRunes)
	b, err := sonic.Marshal(obj)
	if err != nil {
		return clipString(raw, maxValueRunes)
	}
	out := string(b)
	if r := []rune(out); len(r) > maxTotalRunes {
		out = string(r[:maxTotalRunes]) + "…[truncated]"
	}
	return out
}

// compactJSONValue 递归精简任意 JSON 值中的字符串字段，保持结构不变。
func compactJSONValue(v any, maxRunes int) {
	switch t := v.(type) {
	case map[string]any:
		for k, sub := range t {
			switch sv := sub.(type) {
			case string:
				t[k] = clipString(sv, maxRunes)
			case map[string]any:
				compactJSONValue(sv, maxRunes)
			case []any:
				compactJSONSlice(sv, maxRunes)
			}
		}
	case []any:
		compactJSONSlice(t, maxRunes)
	}
}

// compactJSONSlice 精简数组中的字符串元素，保持结构不变。
func compactJSONSlice(arr []any, maxRunes int) {
	for i, sub := range arr {
		switch sv := sub.(type) {
		case string:
			arr[i] = clipString(sv, maxRunes)
		case map[string]any:
			compactJSONValue(sv, maxRunes)
		case []any:
			compactJSONSlice(sv, maxRunes)
		}
	}
}

// clipString 将字符串截断为 maxRunes 的 UTF-8 安全前缀并带省略计数。
func clipString(s string, maxRunes int) string {
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return string(r[:maxRunes]) + fmt.Sprintf("…[+%d]", len(r)-maxRunes)
}
