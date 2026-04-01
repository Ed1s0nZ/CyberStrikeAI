package handler

// apiDocI18n provides x-i18n-* extension keys for OpenAPI docs, used by frontend for i18n.
// Frontend translates via apiDocs.tags.* / apiDocs.summary.* / apiDocs.response.* keys.
// Map keys = English tag/summary/description strings from OpenAPI spec handlers.

var apiDocI18nTagToKey = map[string]string{
	"Auth": "auth", "Conversation Management": "conversationManagement", "Conversation Interaction": "conversationInteraction",
	"Batch Tasks": "batchTasks", "Conversation Groups": "conversationGroups", "Vulnerability Management": "vulnerabilityManagement",
	"Role Management": "roleManagement", "Skills Management": "skillsManagement", "Monitoring": "monitoring",
	"Config Management": "configManagement", "External MCP Management": "externalMCPManagement", "Attack Chain": "attackChain",
	"Knowledge Base": "knowledgeBase", "MCP": "mcp",
}

var apiDocI18nSummaryToKey = map[string]string{
	"User Login": "login", "User Logout": "logout", "Change Password": "changePassword", "Validate Token": "validateToken",
	"Create Conversation": "createConversation", "List Conversations": "listConversations", "Get Conversation Detail": "getConversationDetail",
	"Update Conversation": "updateConversation", "Delete Conversation": "deleteConversation", "Get Conversation Result": "getConversationResult",
	"Send Message and Get AI Reply (Non-Stream)": "sendMessageNonStream", "Send Message and Get AI Reply (Stream)": "sendMessageStream",
	"Cancel Task": "cancelTask", "List Running Tasks": "listRunningTasks", "List Completed Tasks": "listCompletedTasks",
	"Create Batch Queue": "createBatchQueue", "List Batch Queues": "listBatchQueues", "Get Batch Queue": "getBatchQueue",
	"Delete Batch Queue": "deleteBatchQueue", "Start Batch Queue": "startBatchQueue", "Pause Batch Queue": "pauseBatchQueue",
	"Add Task to Queue": "addTaskToQueue", "SQL Injection Scan": "sqlInjectionScan", "Port Scan": "portScan",
	"Update Batch Task": "updateBatchTask", "Delete Batch Task": "deleteBatchTask",
	"Create Group": "createGroup", "List Groups": "listGroups", "Get Group": "getGroup", "Update Group": "updateGroup",
	"Delete Group": "deleteGroup", "Get Group Conversations": "getGroupConversations", "Add Conversation to Group": "addConversationToGroup",
	"Remove Conversation from Group": "removeConversationFromGroup",
	"List Vulnerabilities": "listVulnerabilities", "Create Vulnerability": "createVulnerability", "Get Vulnerability Stats": "getVulnerabilityStats",
	"Get Vulnerability": "getVulnerability", "Update Vulnerability": "updateVulnerability", "Delete Vulnerability": "deleteVulnerability",
	"List Roles": "listRoles", "Create Role": "createRole", "Get Role": "getRole", "Update Role": "updateRole", "Delete Role": "deleteRole",
	"Get Available Skills": "getAvailableSkills", "List Skills": "listSkills", "Create Skill": "createSkill",
	"Get Skill Stats": "getSkillStats", "Clear Skill Stats": "clearSkillStats", "Get Skill": "getSkill",
	"Update Skill": "updateSkill", "Delete Skill": "deleteSkill", "Get Bound Roles": "getBoundRoles",
	"Get Monitor Info": "getMonitorInfo", "Get Execution Records": "getExecutionRecords", "Delete Execution Record": "deleteExecutionRecord",
	"Batch Delete Execution Records": "batchDeleteExecutionRecords", "Get Stats": "getStats",
	"Get Config": "getConfig", "Update Config": "updateConfig", "Get Tool Config": "getToolConfig", "Apply Config": "applyConfig",
	"List External MCP": "listExternalMCP", "Get External MCP Stats": "getExternalMCPStats", "Get External MCP": "getExternalMCP",
	"Add or Update External MCP": "addOrUpdateExternalMCP", "Stdio Mode Config": "stdioModeConfig", "SSE Mode Config": "sseModeConfig",
	"Delete External MCP": "deleteExternalMCP", "Start External MCP": "startExternalMCP", "Stop External MCP": "stopExternalMCP",
	"Get Attack Chain": "getAttackChain", "Regenerate Attack Chain": "regenerateAttackChain",
	"Pin Conversation": "pinConversation", "Pin Group": "pinGroup", "Pin Group Conversation": "pinGroupConversation",
	"Get Categories": "getCategories", "List Knowledge Items": "listKnowledgeItems", "Create Knowledge Item": "createKnowledgeItem",
	"Get Knowledge Item": "getKnowledgeItem", "Update Knowledge Item": "updateKnowledgeItem", "Delete Knowledge Item": "deleteKnowledgeItem",
	"Get Index Status": "getIndexStatus", "Rebuild Index": "rebuildIndex", "Scan Knowledge Base": "scanKnowledgeBase",
	"Search Knowledge Base": "searchKnowledgeBase", "Basic Search": "basicSearch", "Search by Risk Type": "searchByRiskType",
	"Get Retrieval Logs": "getRetrievalLogs", "Delete Retrieval Log": "deleteRetrievalLog",
	"MCP Endpoint": "mcpEndpoint", "List All Tools": "listAllTools", "Invoke Tool": "invokeTool", "Init Connection": "initConnection",
	"Success Response": "successResponse", "Error Response": "errorResponse",
}

var apiDocI18nResponseDescToKey = map[string]string{
	"Get Success": "getSuccess", "Unauthorized": "unauthorized", "Unauthorized, valid Token required": "unauthorizedToken",
	"Create Success": "createSuccess", "Bad Request": "badRequest", "Conversation Not Found": "conversationNotFound",
	"Conversation or Result Not Found": "conversationOrResultNotFound", "Bad Request (task is empty)": "badRequestTaskEmpty",
	"Bad Request or Group Name Exists": "badRequestGroupNameExists", "Group Not Found": "groupNotFound",
	"Bad Request (invalid config format or missing fields)": "badRequestConfig",
	"Bad Request (query is empty)": "badRequestQueryEmpty", "Method Not Allowed (POST only)": "methodNotAllowed",
	"Login Success": "loginSuccess", "Invalid Password": "invalidPassword", "Logout Success": "logoutSuccess",
	"Password Changed": "passwordChanged", "Token Valid": "tokenValid", "Token Invalid or Expired": "tokenInvalid",
	"Conversation Created": "conversationCreated", "Internal Server Error": "internalError", "Update Success": "updateSuccess",
	"Delete Success": "deleteSuccess", "Queue Not Found": "queueNotFound", "Start Success": "startSuccess",
	"Pause Success": "pauseSuccess", "Add Success": "addSuccess",
	"Task Not Found": "taskNotFound", "Conversation or Group Not Found": "conversationOrGroupNotFound",
	"Cancel Request Submitted": "cancelSubmitted", "No Running Task Found": "noRunningTask",
	"Message Sent, AI Reply Returned": "messageSent", "Stream Response (Server-Sent Events)": "streamResponse",
}

// enrichSpecWithI18nKeys writes x-i18n-tags and x-i18n-summary on each operation,
// and x-i18n-description on each response, for frontend i18n lookup.
func enrichSpecWithI18nKeys(spec map[string]interface{}) {
	paths, _ := spec["paths"].(map[string]interface{})
	if paths == nil {
		return
	}
	for _, pathItem := range paths {
		pm, _ := pathItem.(map[string]interface{})
		if pm == nil {
			continue
		}
		for _, method := range []string{"get", "post", "put", "delete", "patch"} {
			opVal, ok := pm[method]
			if !ok {
				continue
			}
			op, _ := opVal.(map[string]interface{})
			if op == nil {
				continue
			}
			// x-i18n-tags: i18n key array corresponding to tags
			switch tags := op["tags"].(type) {
			case []string:
				if len(tags) > 0 {
					keys := make([]string, 0, len(tags))
					for _, s := range tags {
						if k := apiDocI18nTagToKey[s]; k != "" {
							keys = append(keys, k)
						} else {
							keys = append(keys, s)
						}
					}
					op["x-i18n-tags"] = keys
				}
			case []interface{}:
				if len(tags) > 0 {
					keys := make([]interface{}, 0, len(tags))
					for _, t := range tags {
						if s, ok := t.(string); ok {
							if k := apiDocI18nTagToKey[s]; k != "" {
								keys = append(keys, k)
							} else {
								keys = append(keys, s)
							}
						}
					}
					if len(keys) > 0 {
						op["x-i18n-tags"] = keys
					}
				}
			}
			// x-i18n-summary
			if summary, _ := op["summary"].(string); summary != "" {
				if k := apiDocI18nSummaryToKey[summary]; k != "" {
					op["x-i18n-summary"] = k
				}
			}
			// responses -> each status -> x-i18n-description
			if respMap, _ := op["responses"].(map[string]interface{}); respMap != nil {
				for _, rv := range respMap {
					if r, _ := rv.(map[string]interface{}); r != nil {
						if desc, _ := r["description"].(string); desc != "" {
							if k := apiDocI18nResponseDescToKey[desc]; k != "" {
								r["x-i18n-description"] = k
							}
						}
					}
				}
			}
		}
	}
}
