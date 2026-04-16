package security

import "testing"

func TestLookupRoutePermissionForSystemRoutes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		method     string
		path       string
		permission string
	}{
		{method: "GET", path: "/config", permission: PermissionSystemConfigSettingsRead},
		{method: "GET", path: "/config/tools", permission: PermissionSystemConfigSettingsRead},
		{method: "PUT", path: "/config", permission: PermissionSystemConfigSettingsUpdate},
		{method: "POST", path: "/config/apply", permission: PermissionSystemRuntimeConfigApply},
		{method: "POST", path: "/config/test-openai", permission: PermissionSystemModelConnectivityTest},
		{method: "GET", path: "/security/web-users", permission: PermissionSystemWebUserRead},
		{method: "POST", path: "/security/web-users", permission: PermissionSystemWebUserCreate},
		{method: "PUT", path: "/security/web-users/:id", permission: PermissionSystemWebUserUpdate},
		{method: "POST", path: "/security/web-users/:id/reset-password", permission: PermissionSystemWebUserCredentialReset},
		{method: "DELETE", path: "/security/web-users/:id", permission: PermissionSystemWebUserDelete},
		{method: "GET", path: "/security/web-access-roles", permission: PermissionSystemWebAccessRoleRead},
		{method: "POST", path: "/security/web-access-roles", permission: PermissionSystemWebAccessRoleCreate},
		{method: "PUT", path: "/security/web-access-roles/:id", permission: PermissionSystemWebAccessRoleUpdate},
		{method: "DELETE", path: "/security/web-access-roles/:id", permission: PermissionSystemWebAccessRoleDelete},
		{method: "GET", path: "/security/web-access-roles/permission-catalog", permission: PermissionSystemWebAccessRoleRead},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			t.Parallel()
			got, ok := LookupRoutePermission(tc.method, tc.path)
			if !ok {
				t.Fatalf("expected route permission for %s %s", tc.method, tc.path)
			}
			if got != tc.permission {
				t.Fatalf("LookupRoutePermission(%q, %q) = %q, want %q", tc.method, tc.path, got, tc.permission)
			}
		})
	}
}

func TestLookupRoutePermissionForTaskAndAgentRoutes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		method     string
		path       string
		permission string
	}{
		{method: "POST", path: "/robot/test", permission: PermissionAgentRobotTestExecute},
		{method: "POST", path: "/agent-loop", permission: PermissionAgentRunExecute},
		{method: "POST", path: "/agent-loop/stream", permission: PermissionAgentRunExecute},
		{method: "POST", path: "/agent-loop/cancel", permission: PermissionAgentRunStop},
		{method: "GET", path: "/agent-loop/tasks", permission: PermissionAgentRunRead},
		{method: "GET", path: "/agent-loop/tasks/completed", permission: PermissionAgentRunRead},
		{method: "POST", path: "/multi-agent", permission: PermissionAgentMultiRunExecute},
		{method: "POST", path: "/multi-agent/stream", permission: PermissionAgentMultiRunExecute},
		{method: "GET", path: "/multi-agent/markdown-agents", permission: PermissionAgentMarkdownAgentRead},
		{method: "GET", path: "/multi-agent/markdown-agents/:filename", permission: PermissionAgentMarkdownAgentRead},
		{method: "POST", path: "/multi-agent/markdown-agents", permission: PermissionAgentMarkdownAgentCreate},
		{method: "PUT", path: "/multi-agent/markdown-agents/:filename", permission: PermissionAgentMarkdownAgentUpdate},
		{method: "DELETE", path: "/multi-agent/markdown-agents/:filename", permission: PermissionAgentMarkdownAgentDelete},
		{method: "POST", path: "/fofa/search", permission: PermissionIntelFofaQueryExecute},
		{method: "POST", path: "/fofa/parse", permission: PermissionIntelFofaQueryExecute},
		{method: "POST", path: "/batch-tasks", permission: PermissionTaskBatchQueueCreate},
		{method: "GET", path: "/batch-tasks", permission: PermissionTaskBatchQueueRead},
		{method: "GET", path: "/batch-tasks/:queueId", permission: PermissionTaskBatchQueueRead},
		{method: "POST", path: "/batch-tasks/:queueId/start", permission: PermissionTaskBatchQueueStart},
		{method: "POST", path: "/batch-tasks/:queueId/pause", permission: PermissionTaskBatchQueueStop},
		{method: "PUT", path: "/batch-tasks/:queueId/schedule-enabled", permission: PermissionTaskBatchQueueUpdate},
		{method: "DELETE", path: "/batch-tasks/:queueId", permission: PermissionTaskBatchQueueDelete},
		{method: "PUT", path: "/batch-tasks/:queueId/tasks/:taskId", permission: PermissionTaskBatchTaskUpdate},
		{method: "POST", path: "/batch-tasks/:queueId/tasks", permission: PermissionTaskBatchTaskCreate},
		{method: "DELETE", path: "/batch-tasks/:queueId/tasks/:taskId", permission: PermissionTaskBatchTaskDelete},
		{method: "POST", path: "/conversations", permission: PermissionTaskConversationCreate},
		{method: "GET", path: "/conversations", permission: PermissionTaskConversationRead},
		{method: "GET", path: "/conversations/:id", permission: PermissionTaskConversationRead},
		{method: "GET", path: "/messages/:id/process-details", permission: PermissionTaskConversationRead},
		{method: "PUT", path: "/conversations/:id", permission: PermissionTaskConversationUpdate},
		{method: "DELETE", path: "/conversations/:id", permission: PermissionTaskConversationDelete},
		{method: "POST", path: "/conversations/:id/delete-turn", permission: PermissionTaskConversationUpdate},
		{method: "PUT", path: "/conversations/:id/pinned", permission: PermissionTaskConversationUpdate},
		{method: "POST", path: "/groups", permission: PermissionTaskGroupCreate},
		{method: "GET", path: "/groups", permission: PermissionTaskGroupRead},
		{method: "GET", path: "/groups/:id", permission: PermissionTaskGroupRead},
		{method: "PUT", path: "/groups/:id", permission: PermissionTaskGroupUpdate},
		{method: "DELETE", path: "/groups/:id", permission: PermissionTaskGroupDelete},
		{method: "PUT", path: "/groups/:id/pinned", permission: PermissionTaskGroupUpdate},
		{method: "GET", path: "/groups/:id/conversations", permission: PermissionTaskGroupRead},
		{method: "GET", path: "/groups/mappings", permission: PermissionTaskGroupRead},
		{method: "POST", path: "/groups/conversations", permission: PermissionTaskGroupUpdate},
		{method: "DELETE", path: "/groups/:id/conversations/:conversationId", permission: PermissionTaskGroupUpdate},
		{method: "PUT", path: "/groups/:id/conversations/:conversationId/pinned", permission: PermissionTaskGroupUpdate},
		{method: "GET", path: "/monitor", permission: PermissionTaskExecutionRead},
		{method: "GET", path: "/monitor/execution/:id", permission: PermissionTaskExecutionRead},
		{method: "POST", path: "/monitor/executions/names", permission: PermissionTaskExecutionRead},
		{method: "DELETE", path: "/monitor/execution/:id", permission: PermissionTaskExecutionDelete},
		{method: "DELETE", path: "/monitor/executions", permission: PermissionTaskExecutionDelete},
		{method: "GET", path: "/monitor/stats", permission: PermissionTaskExecutionRead},
		{method: "GET", path: "/attack-chain/:conversationId", permission: PermissionTaskAttackChainRead},
		{method: "POST", path: "/attack-chain/:conversationId/regenerate", permission: PermissionTaskAttackChainRegenerate},
		{method: "GET", path: "/conversations/:id/results", permission: PermissionTaskConversationResultRead},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			t.Parallel()
			got, ok := LookupRoutePermission(tc.method, tc.path)
			if !ok {
				t.Fatalf("expected route permission for %s %s", tc.method, tc.path)
			}
			if got != tc.permission {
				t.Fatalf("LookupRoutePermission(%q, %q) = %q, want %q", tc.method, tc.path, got, tc.permission)
			}
		})
	}
}

func TestLookupRoutePermissionForKnowledgeAndOpsRoutes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		method     string
		path       string
		permission string
	}{
		{method: "GET", path: "/knowledge/categories", permission: PermissionKnowledgeCategoryRead},
		{method: "GET", path: "/knowledge/items", permission: PermissionKnowledgeItemRead},
		{method: "GET", path: "/knowledge/items/:id", permission: PermissionKnowledgeItemRead},
		{method: "POST", path: "/knowledge/items", permission: PermissionKnowledgeItemCreate},
		{method: "PUT", path: "/knowledge/items/:id", permission: PermissionKnowledgeItemUpdate},
		{method: "DELETE", path: "/knowledge/items/:id", permission: PermissionKnowledgeItemDelete},
		{method: "GET", path: "/knowledge/index-status", permission: PermissionKnowledgeIndexRead},
		{method: "POST", path: "/knowledge/index", permission: PermissionKnowledgeIndexExecute},
		{method: "POST", path: "/knowledge/scan", permission: PermissionKnowledgeIndexExecute},
		{method: "GET", path: "/knowledge/retrieval-logs", permission: PermissionKnowledgeRetrievalLogRead},
		{method: "DELETE", path: "/knowledge/retrieval-logs/:id", permission: PermissionKnowledgeRetrievalLogDelete},
		{method: "POST", path: "/knowledge/search", permission: PermissionKnowledgeSearchExecute},
		{method: "GET", path: "/knowledge/stats", permission: PermissionKnowledgeStatsRead},
		{method: "GET", path: "/vulnerabilities", permission: PermissionVulnerabilityRecordRead},
		{method: "GET", path: "/vulnerabilities/stats", permission: PermissionVulnerabilityStatsRead},
		{method: "GET", path: "/vulnerabilities/:id", permission: PermissionVulnerabilityRecordRead},
		{method: "POST", path: "/vulnerabilities", permission: PermissionVulnerabilityRecordCreate},
		{method: "PUT", path: "/vulnerabilities/:id", permission: PermissionVulnerabilityRecordUpdate},
		{method: "DELETE", path: "/vulnerabilities/:id", permission: PermissionVulnerabilityRecordDelete},
		{method: "GET", path: "/webshell/connections", permission: PermissionWebshellConnectionRead},
		{method: "POST", path: "/webshell/connections", permission: PermissionWebshellConnectionCreate},
		{method: "GET", path: "/webshell/connections/:id/ai-history", permission: PermissionWebshellSessionRead},
		{method: "GET", path: "/webshell/connections/:id/ai-conversations", permission: PermissionWebshellSessionRead},
		{method: "GET", path: "/webshell/connections/:id/state", permission: PermissionWebshellSessionRead},
		{method: "PUT", path: "/webshell/connections/:id", permission: PermissionWebshellConnectionUpdate},
		{method: "PUT", path: "/webshell/connections/:id/state", permission: PermissionWebshellSessionUpdate},
		{method: "DELETE", path: "/webshell/connections/:id", permission: PermissionWebshellConnectionDelete},
		{method: "POST", path: "/webshell/exec", permission: PermissionWebshellCommandExecute},
		{method: "POST", path: "/webshell/file", permission: PermissionWebshellFileExecute},
		{method: "GET", path: "/chat-uploads", permission: PermissionFileWorkspaceEntryRead},
		{method: "GET", path: "/chat-uploads/download", permission: PermissionFileWorkspaceContentRead},
		{method: "GET", path: "/chat-uploads/content", permission: PermissionFileWorkspaceContentRead},
		{method: "POST", path: "/chat-uploads", permission: PermissionFileWorkspaceEntryCreate},
		{method: "POST", path: "/chat-uploads/mkdir", permission: PermissionFileWorkspaceEntryCreate},
		{method: "DELETE", path: "/chat-uploads", permission: PermissionFileWorkspaceEntryDelete},
		{method: "PUT", path: "/chat-uploads/rename", permission: PermissionFileWorkspaceEntryUpdate},
		{method: "PUT", path: "/chat-uploads/content", permission: PermissionFileWorkspaceContentUpdate},
		{method: "GET", path: "/roles", permission: PermissionRoleAgentRoleRead},
		{method: "GET", path: "/roles/:name", permission: PermissionRoleAgentRoleRead},
		{method: "GET", path: "/roles/skills/list", permission: PermissionRoleAgentRoleRead},
		{method: "POST", path: "/roles", permission: PermissionRoleAgentRoleCreate},
		{method: "PUT", path: "/roles/:name", permission: PermissionRoleAgentRoleUpdate},
		{method: "DELETE", path: "/roles/:name", permission: PermissionRoleAgentRoleDelete},
		{method: "GET", path: "/skills", permission: PermissionSkillDefinitionRead},
		{method: "GET", path: "/skills/stats", permission: PermissionSkillStatsRead},
		{method: "DELETE", path: "/skills/stats", permission: PermissionSkillStatsDelete},
		{method: "GET", path: "/skills/:name", permission: PermissionSkillDefinitionRead},
		{method: "GET", path: "/skills/:name/bound-roles", permission: PermissionSkillBindingRead},
		{method: "POST", path: "/skills", permission: PermissionSkillDefinitionCreate},
		{method: "PUT", path: "/skills/:name", permission: PermissionSkillDefinitionUpdate},
		{method: "DELETE", path: "/skills/:name", permission: PermissionSkillDefinitionDelete},
		{method: "DELETE", path: "/skills/:name/stats", permission: PermissionSkillStatsDelete},
		{method: "GET", path: "/external-mcp", permission: PermissionMCPExternalServerRead},
		{method: "GET", path: "/external-mcp/stats", permission: PermissionMCPExternalServerRead},
		{method: "GET", path: "/external-mcp/:name", permission: PermissionMCPExternalServerRead},
		{method: "PUT", path: "/external-mcp/:name", permission: PermissionMCPExternalServerUpdate},
		{method: "DELETE", path: "/external-mcp/:name", permission: PermissionMCPExternalServerDelete},
		{method: "POST", path: "/external-mcp/:name/start", permission: PermissionMCPExternalServerStart},
		{method: "POST", path: "/external-mcp/:name/stop", permission: PermissionMCPExternalServerStop},
		{method: "POST", path: "/mcp", permission: PermissionMCPGatewayExecute},
		{method: "POST", path: "/terminal/run", permission: PermissionSystemTerminalExecute},
		{method: "POST", path: "/terminal/run/stream", permission: PermissionSystemTerminalExecute},
		{method: "GET", path: "/terminal/ws", permission: PermissionSystemTerminalExecute},
		{method: "GET", path: "/openapi/spec", permission: PermissionSystemAPISpecRead},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			t.Parallel()
			got, ok := LookupRoutePermission(tc.method, tc.path)
			if !ok {
				t.Fatalf("expected route permission for %s %s", tc.method, tc.path)
			}
			if got != tc.permission {
				t.Fatalf("LookupRoutePermission(%q, %q) = %q, want %q", tc.method, tc.path, got, tc.permission)
			}
		})
	}
}
