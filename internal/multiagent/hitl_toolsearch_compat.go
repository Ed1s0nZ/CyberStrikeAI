package multiagent

import (
	"encoding/json"
	"fmt"
	"strings"
)

const toolSearchToolName = "tool_search"

// HitlExemptMetaTools 为编排/元工具及限定在本地工作区的低风险工具。
// tool_search 必须免审批，否则其 HITL 拒绝结果与 Eino toolsearch 中间件不兼容（会硬崩 ChatModel）。
var HitlExemptMetaTools = []string{
	toolSearchToolName,
	"write_file",
	"skill",
	"task",
	"write_todos",
	"transfer_to_agent",
	"exit",
	"TaskCreate",
	"TaskGet",
	"TaskUpdate",
	"TaskList",
	"upsert_project_fact",
}

// IsToolSearchTool reports whether name is the Eino dynamictool tool_search meta-tool.
func IsToolSearchTool(name string) bool {
	return strings.EqualFold(strings.TrimSpace(name), toolSearchToolName)
}

// MergeHitlExemptMetaTools unions configured whitelist with built-in meta-tool exemptions.
func MergeHitlExemptMetaTools(configured []string) []string {
	merged := make([]string, 0, len(configured)+len(HitlExemptMetaTools))
	seen := make(map[string]struct{}, len(configured)+len(HitlExemptMetaTools))
	add := func(name string) {
		n := strings.ToLower(strings.TrimSpace(name))
		if n == "" {
			return
		}
		if _, ok := seen[n]; ok {
			return
		}
		seen[n] = struct{}{}
		merged = append(merged, strings.TrimSpace(name))
	}
	for _, t := range configured {
		add(t)
	}
	for _, t := range HitlExemptMetaTools {
		add(t)
	}
	return merged
}

type toolSearchHitlRejectPayload struct {
	SelectedTools []string `json:"selectedTools"`
	HitlRejected  bool     `json:"_hitlRejected"`
	Reason        string   `json:"reason"`
}

// HitlRejectToolResult returns a tool result body safe for downstream consumers.
// tool_search must stay JSON-shaped so toolsearch.extractSelectedTools does not terminate the graph.
func HitlRejectToolResult(toolName, reason string) string {
	reason = strings.TrimSpace(reason)
	if !IsToolSearchTool(toolName) {
		if reason == "" {
			reason = "rejected by reviewer"
		}
		return fmt.Sprintf("[HITL Reject] Tool '%s' was rejected by reviewer. Reason: %s\nPlease adjust parameters/plan and continue without this call.",
			strings.TrimSpace(toolName), reason)
	}
	payload := toolSearchHitlRejectPayload{
		SelectedTools: []string{},
		HitlRejected:  true,
		Reason:        reason,
	}
	if payload.Reason == "" {
		payload.Reason = "tool_search rejected by reviewer; no dynamic tools unlocked"
	}
	out, err := json.Marshal(payload)
	if err != nil {
		return `{"selectedTools":[],"_hitlRejected":true,"reason":"tool_search rejected by reviewer"}`
	}
	return string(out)
}
