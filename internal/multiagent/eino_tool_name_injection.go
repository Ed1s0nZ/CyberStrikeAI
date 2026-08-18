package multiagent

import (
	"context"
	"strings"

	"github.com/cloudwego/eino/components/tool"
)

// injectToolNamesOnlyInstruction prepends a compact tool-name-only section into
// the system instruction so the model can reference current callable names.
// toolSearchMiddlewareActive must be true when prependEinoMiddlewares mounted toolsearch (dynamic tools); do not infer this
// by scanning tool names — tool_search is injected by middleware and is usually absent from the pre-split tools list.
func injectToolNamesOnlyInstruction(ctx context.Context, instruction string, tools []tool.BaseTool, toolSearchMiddlewareActive bool) string {
	names := collectToolNames(ctx, tools)
	if len(names) == 0 {
		return strings.TrimSpace(instruction)
	}
	hasToolSearch := toolSearchMiddlewareActive
	if !hasToolSearch {
		for _, n := range names {
			if strings.EqualFold(strings.TrimSpace(n), "tool_search") {
				hasToolSearch = true
				break
			}
		}
	}

	var sb strings.Builder
	sb.WriteString("以下是当前会话绑定的工具名称索引（仅名称，无参数 JSON Schema）。\n")
	sb.WriteString("说明：若启用了 tool_search，则列表里可能含「非常驻」工具——它们不一定出现在当前轮次下发给模型的工具定义中；在未看到该工具的完整 schema 前，禁止凭名称臆测参数。\n")
	for _, name := range names {
		sb.WriteString("- ")
		sb.WriteString(name)
		sb.WriteByte('\n')
	}
	sb.WriteString("\n使用规则：\n")
	sb.WriteString("1) 上表仅为名称索引，不含参数定义。禁止猜测参数名、类型、枚举取值或是否必填。\n")
	if hasToolSearch {
		sb.WriteString("【强制 / 最高优先级】本会话已启用 tool_search（动态工具池）。凡名称索引里出现、但你在「当前请求所附 tools 定义」中看不到其完整参数 schema 的工具，一律必须先调用 tool_search；为省 token 或赶进度而跳过 tool_search、直接调用业务工具，属于明确禁止的错误流程。\n")
		sb.WriteString("2) 默认策略：只要对目标工具的参数定义有任何不确定，就先 tool_search；宁可多一次 tool_search，也不要在未见 schema 时盲调业务工具。\n")
		sb.WriteString("3) 调用顺序：先 tool_search（唯一必填参数 regex_pattern：按工具名匹配的正则，如子串 nuclei 或 ^exact_tool_name$）→ 在后续轮次确认目标工具已出现在 tools 列表且已阅读其 schema → 再发起对该工具的真实调用。\n")
		sb.WriteString("4) tool_search 的返回仅为匹配到的工具名列表；schema 在解锁后的下一轮才会下发。禁止在 schema 未出现时编造 JSON 参数。\n")
		sb.WriteString("5) 不要臆造不存在的工具名。\n\n")
	} else {
		sb.WriteString("2) 调用具体工具前，请先确认该工具的参数要求（以当前请求中的工具定义为准）；不确定时先澄清再调用。\n")
		sb.WriteString("3) 不要臆造不存在的工具名。\n\n")
	}
	if s := strings.TrimSpace(injectShellToolGuidance("", names)); s != "" {
		sb.WriteString(s)
		sb.WriteString("\n\n")
	}
	if s := strings.TrimSpace(instruction); s != "" {
		sb.WriteString(s)
	}
	return sb.String()
}

func collectToolNames(ctx context.Context, tools []tool.BaseTool) []string {
	if len(tools) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(tools))
	out := make([]string, 0, len(tools))
	for _, t := range tools {
		if t == nil {
			continue
		}
		info, err := t.Info(ctx)
		if err != nil || info == nil {
			continue
		}
		name := strings.TrimSpace(info.Name)
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, name)
	}
	return out
}

