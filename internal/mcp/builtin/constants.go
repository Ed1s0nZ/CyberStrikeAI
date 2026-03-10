package builtin

// Built-in tool name constants.
// All code that references built-in tool names should use these constants rather than hardcoded strings.
const (
	// Vulnerability management tool
	ToolRecordVulnerability = "record_vulnerability"

	// Knowledge base tools
	ToolListKnowledgeRiskTypes = "list_knowledge_risk_types"
	ToolSearchKnowledgeBase    = "search_knowledge_base"

	// Skills tools
	ToolListSkills = "list_skills"
	ToolReadSkill  = "read_skill"

	// Time awareness tool
	ToolGetCurrentTime = "get_current_time"

	// Persistent memory tools
	ToolStoreMemory        = "store_memory"
	ToolRetrieveMemory     = "retrieve_memory"
	ToolListMemories       = "list_memories"
	ToolDeleteMemory       = "delete_memory"
	ToolUpdateMemoryStatus = "update_memory_status"

	// File manager tools
	ToolRegisterFile     = "register_file"
	ToolUpdateFile       = "update_file"
	ToolListFiles        = "list_files"
	ToolGetFile          = "get_file"
	ToolAppendFileLog    = "append_file_log"
	ToolAppendFindings   = "append_file_findings"

	// Cuttlefish (Android VM) tools
	ToolCuttlefishLaunch   = "cuttlefish_launch"
	ToolCuttlefishStop     = "cuttlefish_stop"
	ToolCuttlefishStatus   = "cuttlefish_status"
	ToolCuttlefishInstall  = "cuttlefish_install_apk"
	ToolCuttlefishHotswap  = "cuttlefish_hotswap"
	ToolCuttlefishShell    = "cuttlefish_shell"
	ToolCuttlefishPush     = "cuttlefish_push"
	ToolCuttlefishPull     = "cuttlefish_pull"
	ToolCuttlefishScreenshot = "cuttlefish_screenshot"
	ToolCuttlefishLogcat   = "cuttlefish_logcat"
	ToolCuttlefishFrida    = "cuttlefish_frida_setup"
	ToolCuttlefishProxy    = "cuttlefish_proxy"
	ToolCuttlefishCert     = "cuttlefish_install_cert"
	ToolCuttlefishSnapshot = "cuttlefish_snapshot"
	ToolCuttlefishPackages = "cuttlefish_packages"
	ToolCuttlefishDroidRun = "cuttlefish_droidrun"

	// Tool discovery — lets the model request full descriptions on demand
	ToolGetToolDetails = "get_tool_details"

	// DroidRun proxy tools — high-level LLM-friendly device interaction
	// These are easier for the LLM than raw ADB: indexed elements, formatted state, vision support
	ToolDroidRunConnect    = "droidrun_connect"
	ToolDroidRunState      = "droidrun_state"
	ToolDroidRunScreenshot = "droidrun_screenshot"
	ToolDroidRunClick      = "droidrun_click"
	ToolDroidRunType       = "droidrun_type"
	ToolDroidRunSwipe      = "droidrun_swipe"
	ToolDroidRunScroll     = "droidrun_scroll"
	ToolDroidRunButton     = "droidrun_button"
	ToolDroidRunOpenApp    = "droidrun_open_app"
	ToolDroidRunListApps   = "droidrun_list_apps"
	ToolDroidRunInstall    = "droidrun_install"
	ToolDroidRunWait       = "droidrun_wait"
)

// IsBuiltinTool reports whether the given tool name is a built-in tool.
func IsBuiltinTool(toolName string) bool {
	switch toolName {
	case ToolRecordVulnerability,
		ToolListKnowledgeRiskTypes,
		ToolSearchKnowledgeBase,
		ToolListSkills,
		ToolReadSkill,
		ToolGetCurrentTime,
		ToolStoreMemory,
		ToolRetrieveMemory,
		ToolListMemories,
		ToolDeleteMemory,
		ToolUpdateMemoryStatus,
		ToolRegisterFile,
		ToolUpdateFile,
		ToolListFiles,
		ToolGetFile,
		ToolAppendFileLog,
		ToolAppendFindings,
		ToolCuttlefishLaunch,
		ToolCuttlefishStop,
		ToolCuttlefishStatus,
		ToolCuttlefishInstall,
		ToolCuttlefishHotswap,
		ToolCuttlefishShell,
		ToolCuttlefishPush,
		ToolCuttlefishPull,
		ToolCuttlefishScreenshot,
		ToolCuttlefishLogcat,
		ToolCuttlefishFrida,
		ToolCuttlefishProxy,
		ToolCuttlefishCert,
		ToolCuttlefishSnapshot,
		ToolCuttlefishPackages,
		ToolCuttlefishDroidRun,
		ToolDroidRunConnect,
		ToolDroidRunState,
		ToolDroidRunScreenshot,
		ToolDroidRunClick,
		ToolDroidRunType,
		ToolDroidRunSwipe,
		ToolDroidRunScroll,
		ToolDroidRunButton,
		ToolDroidRunOpenApp,
		ToolDroidRunListApps,
		ToolDroidRunInstall,
		ToolDroidRunWait,
		ToolGetToolDetails:
		return true
	default:
		return false
	}
}

// GetAllBuiltinTools returns the list of all built-in tool names.
func GetAllBuiltinTools() []string {
	return []string{
		ToolRecordVulnerability,
		ToolListKnowledgeRiskTypes,
		ToolSearchKnowledgeBase,
		ToolListSkills,
		ToolReadSkill,
		ToolGetCurrentTime,
		ToolStoreMemory,
		ToolRetrieveMemory,
		ToolListMemories,
		ToolDeleteMemory,
		ToolUpdateMemoryStatus,
		ToolRegisterFile,
		ToolUpdateFile,
		ToolListFiles,
		ToolGetFile,
		ToolAppendFileLog,
		ToolAppendFindings,
		ToolCuttlefishLaunch,
		ToolCuttlefishStop,
		ToolCuttlefishStatus,
		ToolCuttlefishInstall,
		ToolCuttlefishHotswap,
		ToolCuttlefishShell,
		ToolCuttlefishPush,
		ToolCuttlefishPull,
		ToolCuttlefishScreenshot,
		ToolCuttlefishLogcat,
		ToolCuttlefishFrida,
		ToolCuttlefishProxy,
		ToolCuttlefishCert,
		ToolCuttlefishSnapshot,
		ToolCuttlefishPackages,
		ToolCuttlefishDroidRun,
		ToolDroidRunConnect,
		ToolDroidRunState,
		ToolDroidRunScreenshot,
		ToolDroidRunClick,
		ToolDroidRunType,
		ToolDroidRunSwipe,
		ToolDroidRunScroll,
		ToolDroidRunButton,
		ToolDroidRunOpenApp,
		ToolDroidRunListApps,
		ToolDroidRunInstall,
		ToolDroidRunWait,
		ToolGetToolDetails,
	}
}
