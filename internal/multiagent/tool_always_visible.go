package multiagent

import (
	"strings"
)

// expandAlwaysVisibleNameSet 将配置中的常驻工具名展开为可匹配运行时工具名的集合。
// 支持：内置短名 read_file；外部 mcp::tool；运行时 mcp__tool（OpenAI/Eino 命名）。
func expandAlwaysVisibleNameSet(names []string) map[string]struct{} {
	set := make(map[string]struct{}, len(names)*3)
	add := func(name string) {
		n := strings.TrimSpace(strings.ToLower(name))
		if n == "" {
			return
		}
		set[n] = struct{}{}
	}
	for _, raw := range names {
		n := strings.TrimSpace(strings.ToLower(raw))
		if n == "" {
			continue
		}
		add(n)
		if mcp, tool, ok := strings.Cut(n, "::"); ok && mcp != "" && tool != "" {
			// 外部工具用 mcp::tool 配置时只展开运行时 mcp__tool，避免短名误伤其它 MCP 同名工具。
			add(mcp + "__" + tool)
			continue
		}
		if idx := strings.LastIndex(n, "__"); idx > 0 {
			mcp, tool := n[:idx], n[idx+2:]
			if mcp != "" && tool != "" {
				add(mcp + "::" + tool)
			}
			continue
		}
	}
	return set
}

// toolMatchesAlwaysVisible 判断运行时工具名是否命中常驻白名单（含别名）。
func toolMatchesAlwaysVisible(runtimeName string, nameSet map[string]struct{}) bool {
	if len(nameSet) == 0 {
		return false
	}
	name := strings.TrimSpace(strings.ToLower(runtimeName))
	if name == "" {
		return false
	}
	if _, ok := nameSet[name]; ok {
		return true
	}
	if mcp, tool, ok := strings.Cut(name, "::"); ok && mcp != "" && tool != "" {
		if _, ok := nameSet[mcp+"__"+tool]; ok {
			return true
		}
		if _, ok := nameSet[tool]; ok {
			return true
		}
	}
	if idx := strings.LastIndex(name, "__"); idx > 0 {
		mcp, tool := name[:idx], name[idx+2:]
		if mcp != "" && tool != "" {
			if _, ok := nameSet[mcp+"::"+tool]; ok {
				return true
			}
			if _, ok := nameSet[tool]; ok {
				return true
			}
		}
	}
	return false
}
