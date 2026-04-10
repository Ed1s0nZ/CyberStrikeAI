package builtin

// 内置工具名称常量
// 所有代码中使用内置工具名称的地方都应该使用这些常量，而不是硬编码字符串
const (
	// 漏洞管理工具
	ToolRecordVulnerability = "record_vulnerability"

	// 知识库工具
	ToolListKnowledgeRiskTypes = "list_knowledge_risk_types"
	ToolSearchKnowledgeBase    = "search_knowledge_base"

	// Skills工具
	ToolListSkills    = "list_skills"
	ToolReadSkill     = "read_skill"

	// WebShell 助手工具（AI 在 WebShell 管理 - AI 助手 中使用）
	ToolWebshellExec       = "webshell_exec"
	ToolWebshellFileList   = "webshell_file_list"
	ToolWebshellFileRead   = "webshell_file_read"
	ToolWebshellFileWrite  = "webshell_file_write"

	// WebShell 连接管理工具（用于通过 MCP 管理 webshell 连接）
	ToolManageWebshellList   = "manage_webshell_list"
	ToolManageWebshellAdd    = "manage_webshell_add"
	ToolManageWebshellUpdate = "manage_webshell_update"
	ToolManageWebshellDelete = "manage_webshell_delete"
	ToolManageWebshellTest   = "manage_webshell_test"
)

// IsBuiltinTool 检查工具名称是否是内置工具
func IsBuiltinTool(toolName string) bool {
	switch toolName {
	case ToolRecordVulnerability,
		ToolListKnowledgeRiskTypes,
		ToolSearchKnowledgeBase,
		ToolListSkills,
		ToolReadSkill,
		ToolWebshellExec,
		ToolWebshellFileList,
		ToolWebshellFileRead,
		ToolWebshellFileWrite,
		ToolManageWebshellList,
		ToolManageWebshellAdd,
		ToolManageWebshellUpdate,
		ToolManageWebshellDelete,
		ToolManageWebshellTest:
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
		ToolListSkills,
		ToolReadSkill,
		ToolWebshellExec,
		ToolWebshellFileList,
		ToolWebshellFileRead,
		ToolWebshellFileWrite,
		ToolManageWebshellList,
		ToolManageWebshellAdd,
		ToolManageWebshellUpdate,
		ToolManageWebshellDelete,
		ToolManageWebshellTest,
	}
}
