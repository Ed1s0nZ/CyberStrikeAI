package security

const (
	// Legacy coarse-grained permissions kept for deterministic normalization.
	PermissionSuperAdminLegacy          = "system.super_admin"
	PermissionSystemConfigReadLegacy    = "system.config.read"
	PermissionSystemConfigWriteLegacy   = "system.config.write"
	PermissionSecurityUsersManageLegacy = "security.users.manage"
	PermissionSecurityRolesManageLegacy = "security.roles.manage"

	// Backward-compatible aliases used by existing code paths.
	PermissionSuperAdmin          = PermissionSuperAdminLegacy
	PermissionSystemConfigRead    = PermissionSystemConfigReadLegacy
	PermissionSystemConfigWrite   = PermissionSystemConfigWriteLegacy
	PermissionSecurityUsersManage = PermissionSecurityUsersManageLegacy
	PermissionSecurityRolesManage = PermissionSecurityRolesManageLegacy

	PermissionIntelFofaQueryExecute = "intel.fofa_query.execute"

	PermissionTaskBatchQueueRead         = "task.batch_queue.read"
	PermissionTaskBatchQueueCreate       = "task.batch_queue.create"
	PermissionTaskBatchQueueUpdate       = "task.batch_queue.update"
	PermissionTaskBatchQueueDelete       = "task.batch_queue.delete"
	PermissionTaskBatchQueueStart        = "task.batch_queue.start"
	PermissionTaskBatchQueueStop         = "task.batch_queue.stop"
	PermissionTaskBatchTaskCreate        = "task.batch_task.create"
	PermissionTaskBatchTaskUpdate        = "task.batch_task.update"
	PermissionTaskBatchTaskDelete        = "task.batch_task.delete"
	PermissionTaskConversationRead       = "task.conversation.read"
	PermissionTaskConversationCreate     = "task.conversation.create"
	PermissionTaskConversationUpdate     = "task.conversation.update"
	PermissionTaskConversationDelete     = "task.conversation.delete"
	PermissionTaskGroupRead              = "task.group.read"
	PermissionTaskGroupCreate            = "task.group.create"
	PermissionTaskGroupUpdate            = "task.group.update"
	PermissionTaskGroupDelete            = "task.group.delete"
	PermissionTaskExecutionRead          = "task.execution.read"
	PermissionTaskExecutionDelete        = "task.execution.delete"
	PermissionTaskAttackChainRead        = "task.attack_chain.read"
	PermissionTaskAttackChainRegenerate  = "task.attack_chain.regenerate"
	PermissionTaskConversationResultRead = "task.conversation_result.read"

	PermissionVulnerabilityRecordRead   = "vulnerability.record.read"
	PermissionVulnerabilityRecordCreate = "vulnerability.record.create"
	PermissionVulnerabilityRecordUpdate = "vulnerability.record.update"
	PermissionVulnerabilityRecordDelete = "vulnerability.record.delete"
	PermissionVulnerabilityStatsRead    = "vulnerability.stats.read"

	PermissionWebshellConnectionRead   = "webshell.connection.read"
	PermissionWebshellConnectionCreate = "webshell.connection.create"
	PermissionWebshellConnectionUpdate = "webshell.connection.update"
	PermissionWebshellConnectionDelete = "webshell.connection.delete"
	PermissionWebshellConnectionTest   = "webshell.connection.test"
	PermissionWebshellSessionRead      = "webshell.session.read"
	PermissionWebshellSessionUpdate    = "webshell.session.update"
	PermissionWebshellCommandExecute   = "webshell.command.execute"
	PermissionWebshellFileExecute      = "webshell.file.execute"

	PermissionFileWorkspaceEntryRead     = "file.workspace_entry.read"
	PermissionFileWorkspaceEntryCreate   = "file.workspace_entry.create"
	PermissionFileWorkspaceEntryUpdate   = "file.workspace_entry.update"
	PermissionFileWorkspaceEntryDelete   = "file.workspace_entry.delete"
	PermissionFileWorkspaceContentRead   = "file.workspace_content.read"
	PermissionFileWorkspaceContentUpdate = "file.workspace_content.update"

	PermissionMCPGatewayExecute       = "mcp.gateway.execute"
	PermissionMCPExternalServerRead   = "mcp.external_server.read"
	PermissionMCPExternalServerCreate = "mcp.external_server.create"
	PermissionMCPExternalServerUpdate = "mcp.external_server.update"
	PermissionMCPExternalServerDelete = "mcp.external_server.delete"
	PermissionMCPExternalServerStart  = "mcp.external_server.start"
	PermissionMCPExternalServerStop   = "mcp.external_server.stop"

	PermissionKnowledgeCategoryRead       = "knowledge.category.read"
	PermissionKnowledgeItemRead           = "knowledge.item.read"
	PermissionKnowledgeItemCreate         = "knowledge.item.create"
	PermissionKnowledgeItemUpdate         = "knowledge.item.update"
	PermissionKnowledgeItemDelete         = "knowledge.item.delete"
	PermissionKnowledgeIndexRead          = "knowledge.index.read"
	PermissionKnowledgeIndexExecute       = "knowledge.index.execute"
	PermissionKnowledgeRetrievalLogRead   = "knowledge.retrieval_log.read"
	PermissionKnowledgeRetrievalLogDelete = "knowledge.retrieval_log.delete"
	PermissionKnowledgeSearchExecute      = "knowledge.search.execute"
	PermissionKnowledgeStatsRead          = "knowledge.stats.read"

	PermissionSkillDefinitionRead   = "skill.definition.read"
	PermissionSkillDefinitionCreate = "skill.definition.create"
	PermissionSkillDefinitionUpdate = "skill.definition.update"
	PermissionSkillDefinitionDelete = "skill.definition.delete"
	PermissionSkillBindingRead      = "skill.binding.read"
	PermissionSkillStatsRead        = "skill.stats.read"
	PermissionSkillStatsDelete      = "skill.stats.delete"

	PermissionAgentRunRead             = "agent.run.read"
	PermissionAgentRunExecute          = "agent.run.execute"
	PermissionAgentRunStop             = "agent.run.stop"
	PermissionAgentMultiRunRead        = "agent.multi_run.read"
	PermissionAgentMultiRunExecute     = "agent.multi_run.execute"
	PermissionAgentMultiRunStop        = "agent.multi_run.stop"
	PermissionAgentMarkdownAgentRead   = "agent.markdown_agent.read"
	PermissionAgentMarkdownAgentCreate = "agent.markdown_agent.create"
	PermissionAgentMarkdownAgentUpdate = "agent.markdown_agent.update"
	PermissionAgentMarkdownAgentDelete = "agent.markdown_agent.delete"
	PermissionAgentRobotTestExecute    = "agent.robot_test.execute"

	PermissionRoleAgentRoleRead   = "role.agent_role.read"
	PermissionRoleAgentRoleCreate = "role.agent_role.create"
	PermissionRoleAgentRoleUpdate = "role.agent_role.update"
	PermissionRoleAgentRoleDelete = "role.agent_role.delete"

	PermissionSystemConfigSettingsRead     = "system.config_settings.read"
	PermissionSystemConfigSettingsUpdate   = "system.config_settings.update"
	PermissionSystemRuntimeConfigApply     = "system.runtime_config.apply"
	PermissionSystemModelConnectivityTest  = "system.model_connectivity.test"
	PermissionSystemWebUserRead            = "system.web_user.read"
	PermissionSystemWebUserCreate          = "system.web_user.create"
	PermissionSystemWebUserUpdate          = "system.web_user.update"
	PermissionSystemWebUserDelete          = "system.web_user.delete"
	PermissionSystemWebUserCredentialReset = "system.web_user_credential.reset"
	PermissionSystemWebAccessRoleRead      = "system.web_access_role.read"
	PermissionSystemWebAccessRoleCreate    = "system.web_access_role.create"
	PermissionSystemWebAccessRoleUpdate    = "system.web_access_role.update"
	PermissionSystemWebAccessRoleDelete    = "system.web_access_role.delete"
	PermissionSystemTerminalExecute        = "system.terminal.execute"
	PermissionSystemAPISpecRead            = "system.api_spec.read"
	PermissionSuperAdminGrant              = "system.super_admin.grant"
)

var legacyPermissionMap = map[string][]string{
	PermissionSystemConfigReadLegacy: {
		PermissionSystemConfigSettingsRead,
	},
	PermissionSystemConfigWriteLegacy: {
		PermissionSystemConfigSettingsUpdate,
		PermissionSystemRuntimeConfigApply,
		PermissionSystemModelConnectivityTest,
	},
	PermissionSecurityUsersManageLegacy: {
		PermissionSystemWebUserRead,
		PermissionSystemWebUserCreate,
		PermissionSystemWebUserUpdate,
		PermissionSystemWebUserDelete,
		PermissionSystemWebUserCredentialReset,
	},
	PermissionSecurityRolesManageLegacy: {
		PermissionSystemWebAccessRoleRead,
		PermissionSystemWebAccessRoleCreate,
		PermissionSystemWebAccessRoleUpdate,
		PermissionSystemWebAccessRoleDelete,
	},
	PermissionSuperAdminLegacy: {
		PermissionSuperAdminGrant,
	},
}

var canonicalToLegacyPermissionMap = buildCanonicalToLegacyPermissionMap()

// HasPermission returns true when the required permission is present.
// Either legacy or canonical super-admin permissions bypass all required permission checks.
func HasPermission(permissionSet map[string]struct{}, required string) bool {
	if hasSuperAdminPermission(permissionSet) {
		return true
	}

	required = normalizePermissionToken(required)
	if required == "" {
		return false
	}

	if _, ok := permissionSet[required]; ok {
		return true
	}

	if expandedLegacyRequired, ok := legacyPermissionMap[required]; ok {
		return hasAllPermissions(permissionSet, expandedLegacyRequired)
	}

	for _, legacy := range canonicalToLegacyPermissionMap[required] {
		if _, ok := permissionSet[legacy]; ok {
			return true
		}
	}

	return false
}

func hasSuperAdminPermission(permissionSet map[string]struct{}) bool {
	if _, ok := permissionSet[PermissionSuperAdminLegacy]; ok {
		return true
	}
	if _, ok := permissionSet[PermissionSuperAdminGrant]; ok {
		return true
	}
	return false
}

func hasAllPermissions(permissionSet map[string]struct{}, permissions []string) bool {
	for _, permission := range permissions {
		if _, ok := permissionSet[permission]; !ok {
			return false
		}
	}
	return true
}

func buildCanonicalToLegacyPermissionMap() map[string][]string {
	mapping := make(map[string][]string, len(canonicalWebPermissions))
	for legacy, canonicalPermissions := range legacyPermissionMap {
		for _, canonicalPermission := range canonicalPermissions {
			mapping[canonicalPermission] = append(mapping[canonicalPermission], legacy)
		}
	}

	return mapping
}
