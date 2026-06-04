package builtin

// 内置工具名称常量
// 所有代码中使用内置工具名称的地方都应该使用这些常量，而不是硬编码字符串
const (
	// 漏洞管理工具
	ToolRecordVulnerability = "record_vulnerability"
	ToolListVulnerabilities = "list_vulnerabilities"
	ToolGetVulnerability    = "get_vulnerability"

	// 项目黑板（事实）工具
	ToolUpsertProjectFact    = "upsert_project_fact"
	ToolGetProjectFact       = "get_project_fact"
	ToolListProjectFacts     = "list_project_facts"
	ToolSearchProjectFacts   = "search_project_facts"
	ToolDeprecateProjectFact = "deprecate_project_fact"
	ToolRestoreProjectFact   = "restore_project_fact"

	// 知识库工具
	ToolListKnowledgeRiskTypes = "list_knowledge_risk_types"
	ToolSearchKnowledgeBase    = "search_knowledge_base"

	// 视觉分析（本地图片 → VL 模型 → 文本摘要）
	ToolAnalyzeImage = "analyze_image"

	// WebShell 助手工具（AI 在 WebShell 管理 - AI 助手 中使用）
	ToolWebshellExec      = "webshell_exec"
	ToolWebshellFileList  = "webshell_file_list"
	ToolWebshellFileRead  = "webshell_file_read"
	ToolWebshellFileWrite = "webshell_file_write"
	ToolWebshellFileOp    = "webshell_file_op"

	// WebShell 连接管理工具（用于通过 MCP 管理 webshell 连接）
	ToolManageWebshellList   = "manage_webshell_list"
	ToolManageWebshellAdd    = "manage_webshell_add"
	ToolManageWebshellUpdate = "manage_webshell_update"
	ToolManageWebshellDelete = "manage_webshell_delete"
	ToolManageWebshellTest   = "manage_webshell_test"

	// 批量任务队列（与 Web 端批量任务一致，供模型创建/启停/查询队列）
	ToolBatchTaskList            = "batch_task_list"
	ToolBatchTaskGet             = "batch_task_get"
	ToolBatchTaskCreate          = "batch_task_create"
	ToolBatchTaskStart           = "batch_task_start"
	ToolBatchTaskRerun           = "batch_task_rerun"
	ToolBatchTaskPause           = "batch_task_pause"
	ToolBatchTaskDelete          = "batch_task_delete"
	ToolBatchTaskUpdateMetadata  = "batch_task_update_metadata"
	ToolBatchTaskUpdateSchedule  = "batch_task_update_schedule"
	ToolBatchTaskScheduleEnabled = "batch_task_schedule_enabled"
	ToolBatchTaskAdd             = "batch_task_add_task"
	ToolBatchTaskUpdate          = "batch_task_update_task"
	ToolBatchTaskRemove          = "batch_task_remove_task"

	// C2 工具集（合并同类项，8 个统一工具）
	ToolC2Listener   = "c2_listener"    // 监听器管理（create/start/stop/list/get/update/delete）
	ToolC2Session    = "c2_session"     // 会话管理（list/get/set_sleep/kill/delete）
	ToolC2Task       = "c2_task"        // 任务下发（统一 task_type 参数）
	ToolC2TaskManage = "c2_task_manage" // 任务管理（get_result/wait/list/cancel）
	ToolC2Payload    = "c2_payload"     // Payload 生成（oneliner/build）
	ToolC2Event      = "c2_event"       // 事件查询
	ToolC2Profile    = "c2_profile"     // Malleable Profile 管理（list/get/create/update/delete）
	ToolC2File       = "c2_file"        // 文件管理（list/get_result）
)

// IsBuiltinTool 检查工具名称是否是内置工具
func IsBuiltinTool(toolName string) bool {
	switch toolName {
	case ToolRecordVulnerability,
		ToolListVulnerabilities,
		ToolGetVulnerability,
		ToolUpsertProjectFact,
		ToolGetProjectFact,
		ToolListProjectFacts,
		ToolSearchProjectFacts,
		ToolDeprecateProjectFact,
		ToolRestoreProjectFact,
		ToolListKnowledgeRiskTypes,
		ToolSearchKnowledgeBase,
		ToolAnalyzeImage,
		ToolWebshellExec,
		ToolWebshellFileList,
		ToolWebshellFileRead,
		ToolWebshellFileWrite,
		ToolWebshellFileOp,
		ToolManageWebshellList,
		ToolManageWebshellAdd,
		ToolManageWebshellUpdate,
		ToolManageWebshellDelete,
		ToolManageWebshellTest,
		ToolBatchTaskList,
		ToolBatchTaskGet,
		ToolBatchTaskCreate,
		ToolBatchTaskStart,
		ToolBatchTaskRerun,
		ToolBatchTaskPause,
		ToolBatchTaskDelete,
		ToolBatchTaskUpdateMetadata,
		ToolBatchTaskUpdateSchedule,
		ToolBatchTaskScheduleEnabled,
		ToolBatchTaskAdd,
		ToolBatchTaskUpdate,
		ToolBatchTaskRemove,
		// C2 工具
		ToolC2Listener,
		ToolC2Session,
		ToolC2Task,
		ToolC2TaskManage,
		ToolC2Payload,
		ToolC2Event,
		ToolC2Profile,
		ToolC2File:
		return true
	default:
		return false
	}
}

// GetAllBuiltinTools 返回所有内置工具名称列表
func GetAllBuiltinTools() []string {
	return []string{
		ToolRecordVulnerability,
		ToolListVulnerabilities,
		ToolGetVulnerability,
		ToolUpsertProjectFact,
		ToolGetProjectFact,
		ToolListProjectFacts,
		ToolSearchProjectFacts,
		ToolDeprecateProjectFact,
		ToolRestoreProjectFact,
		ToolListKnowledgeRiskTypes,
		ToolSearchKnowledgeBase,
		ToolAnalyzeImage,
		ToolWebshellExec,
		ToolWebshellFileList,
		ToolWebshellFileRead,
		ToolWebshellFileWrite,
		ToolWebshellFileOp,
		ToolManageWebshellList,
		ToolManageWebshellAdd,
		ToolManageWebshellUpdate,
		ToolManageWebshellDelete,
		ToolManageWebshellTest,
		ToolBatchTaskList,
		ToolBatchTaskGet,
		ToolBatchTaskCreate,
		ToolBatchTaskStart,
		ToolBatchTaskRerun,
		ToolBatchTaskPause,
		ToolBatchTaskDelete,
		ToolBatchTaskUpdateMetadata,
		ToolBatchTaskUpdateSchedule,
		ToolBatchTaskScheduleEnabled,
		ToolBatchTaskAdd,
		ToolBatchTaskUpdate,
		ToolBatchTaskRemove,
		// C2 工具
		ToolC2Listener,
		ToolC2Session,
		ToolC2Task,
		ToolC2TaskManage,
		ToolC2Payload,
		ToolC2Event,
		ToolC2Profile,
		ToolC2File,
	}
}
