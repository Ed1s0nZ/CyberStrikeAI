package security

import (
	"slices"
	"strings"
	"testing"
)

func TestCanonicalPermissionCatalogIncludesPlatformDomains(t *testing.T) {
	permissions := CanonicalWebPermissions()
	expected := []string{
		"agent.markdown_agent.create",
		"agent.markdown_agent.delete",
		"agent.markdown_agent.read",
		"agent.markdown_agent.update",
		"agent.multi_run.execute",
		"agent.multi_run.read",
		"agent.multi_run.stop",
		"agent.robot_test.execute",
		"agent.run.execute",
		"agent.run.read",
		"agent.run.stop",
		"file.workspace_content.read",
		"file.workspace_content.update",
		"file.workspace_entry.create",
		"file.workspace_entry.delete",
		"file.workspace_entry.read",
		"file.workspace_entry.update",
		"intel.fofa_query.execute",
		"knowledge.category.read",
		"knowledge.index.execute",
		"knowledge.index.read",
		"knowledge.item.create",
		"knowledge.item.delete",
		"knowledge.item.read",
		"knowledge.item.update",
		"knowledge.retrieval_log.delete",
		"knowledge.retrieval_log.read",
		"knowledge.search.execute",
		"knowledge.stats.read",
		"mcp.external_server.create",
		"mcp.external_server.delete",
		"mcp.external_server.read",
		"mcp.external_server.start",
		"mcp.external_server.stop",
		"mcp.external_server.update",
		"mcp.gateway.execute",
		"role.agent_role.create",
		"role.agent_role.delete",
		"role.agent_role.read",
		"role.agent_role.update",
		"skill.binding.read",
		"skill.definition.create",
		"skill.definition.delete",
		"skill.definition.read",
		"skill.definition.update",
		"skill.stats.delete",
		"skill.stats.read",
		"system.api_spec.read",
		"system.config_settings.read",
		"system.config_settings.update",
		"system.model_connectivity.test",
		"system.runtime_config.apply",
		"system.super_admin.grant",
		"system.terminal.execute",
		"system.web_access_role.create",
		"system.web_access_role.delete",
		"system.web_access_role.read",
		"system.web_access_role.update",
		"system.web_user.create",
		"system.web_user.delete",
		"system.web_user.read",
		"system.web_user.update",
		"system.web_user_credential.reset",
		"task.attack_chain.read",
		"task.attack_chain.regenerate",
		"task.batch_queue.create",
		"task.batch_queue.delete",
		"task.batch_queue.read",
		"task.batch_queue.start",
		"task.batch_queue.stop",
		"task.batch_queue.update",
		"task.batch_task.create",
		"task.batch_task.delete",
		"task.batch_task.update",
		"task.conversation.create",
		"task.conversation.delete",
		"task.conversation.read",
		"task.conversation.update",
		"task.conversation_result.read",
		"task.execution.delete",
		"task.execution.read",
		"task.group.create",
		"task.group.delete",
		"task.group.read",
		"task.group.update",
		"vulnerability.record.create",
		"vulnerability.record.delete",
		"vulnerability.record.read",
		"vulnerability.record.update",
		"vulnerability.stats.read",
		"webshell.command.execute",
		"webshell.connection.create",
		"webshell.connection.delete",
		"webshell.connection.read",
		"webshell.connection.test",
		"webshell.connection.update",
		"webshell.file.execute",
		"webshell.session.read",
		"webshell.session.update",
	}

	if !slices.Equal(permissions, expected) {
		t.Fatalf("CanonicalWebPermissions() = %#v, want %#v", permissions, expected)
	}

	expectedActionSubsets := map[string][]string{
		"task.batch_queue":       {"create", "delete", "read", "start", "stop", "update"},
		"task.batch_task":        {"create", "delete", "update"},
		"task.execution":         {"delete", "read"},
		"task.attack_chain":      {"read", "regenerate"},
		"webshell.connection":    {"create", "delete", "read", "test", "update"},
		"webshell.session":       {"read", "update"},
		"file.workspace_content": {"read", "update"},
		"mcp.external_server":    {"create", "delete", "read", "start", "stop", "update"},
		"knowledge.index":        {"execute", "read"},
		"skill.stats":            {"delete", "read"},
		"agent.run":              {"execute", "read", "stop"},
		"agent.multi_run":        {"execute", "read", "stop"},
	}

	actualActionSubsets := make(map[string][]string)
	for _, permission := range permissions {
		parts := strings.Split(permission, ".")
		if len(parts) != 3 {
			t.Fatalf("permission %q should have domain.resource.action shape", permission)
		}
		resourceKey := parts[0] + "." + parts[1]
		actualActionSubsets[resourceKey] = append(actualActionSubsets[resourceKey], parts[2])
	}

	for resourceKey, expectedActions := range expectedActionSubsets {
		gotActions, ok := actualActionSubsets[resourceKey]
		if !ok {
			t.Fatalf("expected resource %q to exist in canonical catalog", resourceKey)
		}
		slices.Sort(gotActions)
		if !slices.Equal(gotActions, expectedActions) {
			t.Fatalf("actions for %s = %#v, want %#v", resourceKey, gotActions, expectedActions)
		}
	}

	rejected := []string{
		"task.batch_task.read",
		"task.execution.start",
		"webshell.session.create",
		"knowledge.index.create",
		"agent.run.create",
	}

	for _, permission := range rejected {
		if slices.Contains(permissions, permission) {
			t.Fatalf("expected canonical catalog to exclude %q", permission)
		}
		if IsCanonicalWebPermission(permission) {
			t.Fatalf("expected IsCanonicalWebPermission(%q) to be false", permission)
		}
	}

	canonicalConstants := []string{
		PermissionAgentMarkdownAgentCreate,
		PermissionAgentMarkdownAgentDelete,
		PermissionAgentMarkdownAgentRead,
		PermissionAgentMarkdownAgentUpdate,
		PermissionAgentMultiRunExecute,
		PermissionAgentMultiRunRead,
		PermissionAgentMultiRunStop,
		PermissionAgentRobotTestExecute,
		PermissionAgentRunExecute,
		PermissionAgentRunRead,
		PermissionAgentRunStop,
		PermissionFileWorkspaceContentRead,
		PermissionFileWorkspaceContentUpdate,
		PermissionFileWorkspaceEntryCreate,
		PermissionFileWorkspaceEntryDelete,
		PermissionFileWorkspaceEntryRead,
		PermissionFileWorkspaceEntryUpdate,
		PermissionIntelFofaQueryExecute,
		PermissionKnowledgeCategoryRead,
		PermissionKnowledgeIndexExecute,
		PermissionKnowledgeIndexRead,
		PermissionKnowledgeItemCreate,
		PermissionKnowledgeItemDelete,
		PermissionKnowledgeItemRead,
		PermissionKnowledgeItemUpdate,
		PermissionKnowledgeRetrievalLogDelete,
		PermissionKnowledgeRetrievalLogRead,
		PermissionKnowledgeSearchExecute,
		PermissionKnowledgeStatsRead,
		PermissionMCPExternalServerCreate,
		PermissionMCPExternalServerDelete,
		PermissionMCPExternalServerRead,
		PermissionMCPExternalServerStart,
		PermissionMCPExternalServerStop,
		PermissionMCPExternalServerUpdate,
		PermissionMCPGatewayExecute,
		PermissionRoleAgentRoleCreate,
		PermissionRoleAgentRoleDelete,
		PermissionRoleAgentRoleRead,
		PermissionRoleAgentRoleUpdate,
		PermissionSkillBindingRead,
		PermissionSkillDefinitionCreate,
		PermissionSkillDefinitionDelete,
		PermissionSkillDefinitionRead,
		PermissionSkillDefinitionUpdate,
		PermissionSkillStatsDelete,
		PermissionSkillStatsRead,
		PermissionSuperAdminGrant,
		PermissionSystemAPISpecRead,
		PermissionSystemConfigSettingsRead,
		PermissionSystemConfigSettingsUpdate,
		PermissionSystemModelConnectivityTest,
		PermissionSystemRuntimeConfigApply,
		PermissionSystemTerminalExecute,
		PermissionSystemWebAccessRoleCreate,
		PermissionSystemWebAccessRoleDelete,
		PermissionSystemWebAccessRoleRead,
		PermissionSystemWebAccessRoleUpdate,
		PermissionSystemWebUserCreate,
		PermissionSystemWebUserCredentialReset,
		PermissionSystemWebUserDelete,
		PermissionSystemWebUserRead,
		PermissionSystemWebUserUpdate,
		PermissionTaskAttackChainRead,
		PermissionTaskAttackChainRegenerate,
		PermissionTaskBatchQueueCreate,
		PermissionTaskBatchQueueDelete,
		PermissionTaskBatchQueueRead,
		PermissionTaskBatchQueueStart,
		PermissionTaskBatchQueueStop,
		PermissionTaskBatchQueueUpdate,
		PermissionTaskBatchTaskCreate,
		PermissionTaskBatchTaskDelete,
		PermissionTaskBatchTaskUpdate,
		PermissionTaskConversationCreate,
		PermissionTaskConversationDelete,
		PermissionTaskConversationRead,
		PermissionTaskConversationResultRead,
		PermissionTaskConversationUpdate,
		PermissionTaskExecutionDelete,
		PermissionTaskExecutionRead,
		PermissionTaskGroupCreate,
		PermissionTaskGroupDelete,
		PermissionTaskGroupRead,
		PermissionTaskGroupUpdate,
		PermissionVulnerabilityRecordCreate,
		PermissionVulnerabilityRecordDelete,
		PermissionVulnerabilityRecordRead,
		PermissionVulnerabilityRecordUpdate,
		PermissionVulnerabilityStatsRead,
		PermissionWebshellCommandExecute,
		PermissionWebshellConnectionCreate,
		PermissionWebshellConnectionDelete,
		PermissionWebshellConnectionRead,
		PermissionWebshellConnectionTest,
		PermissionWebshellConnectionUpdate,
		PermissionWebshellFileExecute,
		PermissionWebshellSessionRead,
		PermissionWebshellSessionUpdate,
	}

	slices.Sort(canonicalConstants)
	if !slices.Equal(canonicalConstants, permissions) {
		t.Fatalf("canonical permission constants = %#v, want %#v", canonicalConstants, permissions)
	}
}

func TestNormalizeWebPermissionsExpandsLegacyValues(t *testing.T) {
	input := []string{
		" system.config.write ",
		"security.users.manage",
		"system.super_admin",
		"",
		"knowledge.search.execute",
		"knowledge.search.execute",
	}

	got := NormalizeWebPermissions(input)
	want := []string{
		"knowledge.search.execute",
		"system.config_settings.update",
		"system.model_connectivity.test",
		"system.runtime_config.apply",
		"system.super_admin.grant",
		"system.web_user.create",
		"system.web_user.delete",
		"system.web_user.read",
		"system.web_user.update",
		"system.web_user_credential.reset",
	}

	if !slices.Equal(got, want) {
		t.Fatalf("NormalizeWebPermissions() = %#v, want %#v", got, want)
	}
}

func TestIsCanonicalWebPermissionRejectsRetiredPermission(t *testing.T) {
	if IsCanonicalWebPermission("system.super_admin") {
		t.Fatal("expected retired legacy permission to be rejected")
	}

	if IsCanonicalWebPermission("security.users.manage") {
		t.Fatal("expected retired coarse-grained permission to be rejected")
	}

	if !IsCanonicalWebPermission("system.super_admin.grant") {
		t.Fatal("expected canonical privileged permission to be accepted")
	}
}
