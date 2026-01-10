package builtin

// 内置工具名称常量
// 所有代码中使用内置工具名称的地方都应该使用这些常量，而不是硬编码字符串
const (
	// 漏洞管理工具
	ToolRecordVulnerability = "record_vulnerability"

	// 知识库工具
	ToolListKnowledgeRiskTypes = "list_knowledge_risk_types"
	ToolSearchKnowledgeBase    = "search_knowledge_base"
)

// IsBuiltinTool 检查工具名称是否是内置工具
func IsBuiltinTool(toolName string) bool {
	switch toolName {
	case ToolRecordVulnerability,
		ToolListKnowledgeRiskTypes,
		ToolSearchKnowledgeBase:
		return true
	default:
		return false
	}
}

// GetAllBuiltinTools 返回所有内置工具名称列表
func GetAllBuiltinTools() []string {
	return []string{
		ToolRecordVulnerability,
		ToolListKnowledgeRiskTypes,
		ToolSearchKnowledgeBase,
	}
}
