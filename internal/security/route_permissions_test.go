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
