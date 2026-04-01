package builtin

// built-in tool name constants
// all code that uses built-in tool names should use these constants instead of hardcoded strings
const (
	// vulnerability management tool
	ToolRecordVulnerability = "record_vulnerability"

	// knowledge base tools
	ToolListKnowledgeRiskTypes = "list_knowledge_risk_types"
	ToolSearchKnowledgeBase    = "search_knowledge_base"

	// Skills tools
	ToolListSkills    = "list_skills"
	ToolReadSkill     = "read_skill"

	// WebShell assistant tools (used by AI in WebShell management - AI assistant)
	ToolWebshellExec       = "webshell_exec"
	ToolWebshellFileList   = "webshell_file_list"
	ToolWebshellFileRead   = "webshell_file_read"
	ToolWebshellFileWrite  = "webshell_file_write"

	// WebShell connection management tools (for managing webshell connections via MCP)
	ToolManageWebshellList   = "manage_webshell_list"
	ToolManageWebshellAdd    = "manage_webshell_add"
	ToolManageWebshellUpdate = "manage_webshell_update"
	ToolManageWebshellDelete = "manage_webshell_delete"
	ToolManageWebshellTest   = "manage_webshell_test"
)

// IsBuiltinTool checks if tool name is a built-in tool
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

// GetAllBuiltinTools returns list of all built-in tool names
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
