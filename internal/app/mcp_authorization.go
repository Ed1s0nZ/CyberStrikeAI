package app

import (
	"context"
	"fmt"
	"strings"

	"cyberstrike-ai/internal/agent"
	"cyberstrike-ai/internal/authctx"
	"cyberstrike-ai/internal/database"
	"cyberstrike-ai/internal/mcp"
	"cyberstrike-ai/internal/mcp/builtin"
)

func mcpToolAuthorizer(db *database.DB) func(context.Context, string, map[string]interface{}) error {
	return func(ctx context.Context, toolName string, args map[string]interface{}) error {
		principal, ok := authctx.PrincipalFromContext(ctx)
		if !ok {
			return fmt.Errorf("missing authenticated principal")
		}
		require := func(permission string) error {
			if !principal.HasPermission(permission) {
				return fmt.Errorf("missing permission %s", permission)
			}
			return nil
		}
		resource := func(permission, resourceType, argument string) error {
			if err := require(permission); err != nil {
				return err
			}
			id := mcpAuthorizationString(args, argument)
			if id == "" || db == nil || !db.UserCanAccessResource(principal.UserID, principal.ScopeFor(permission), resourceType, id) {
				return fmt.Errorf("no access to %s %s", resourceType, id)
			}
			return nil
		}
		toolExecutionResource := func(permission string) error {
			if err := require(permission); err != nil {
				return err
			}
			id := mcpAuthorizationString(args, "execution_id")
			if id == "" || db == nil || !db.UserCanAccessToolExecution(principal.UserID, principal.ScopeFor(permission), id) {
				return fmt.Errorf("no access to tool execution %s", id)
			}
			return nil
		}

		switch toolName {
		case builtin.ToolWebshellExec, builtin.ToolWebshellFileWrite:
			return resource("webshell:write", "webshell", "connection_id")
		case builtin.ToolWebshellFileList, builtin.ToolWebshellFileRead:
			return resource("webshell:read", "webshell", "connection_id")
		case builtin.ToolManageWebshellList:
			return require("webshell:read")
		case builtin.ToolManageWebshellAdd:
			return require("webshell:write")
		case builtin.ToolManageWebshellUpdate, builtin.ToolManageWebshellTest:
			return resource("webshell:write", "webshell", "connection_id")
		case builtin.ToolManageWebshellDelete:
			return resource("webshell:delete", "webshell", "connection_id")
		case builtin.ToolRecordVulnerability:
			if err := require("vulnerability:write"); err != nil {
				return err
			}
			conversationID := mcpAuthorizationString(args, "conversation_id")
			if conversationID == "" {
				conversationID = mcpAuthorizationConversationID(ctx)
			}
			if conversationID == "" || db == nil || !db.UserCanAccessResource(principal.UserID, principal.ScopeFor("vulnerability:write"), "conversation", conversationID) {
				return fmt.Errorf("no access to conversation %s", conversationID)
			}
			return nil
		case builtin.ToolListVulnerabilities:
			if err := require("vulnerability:read"); err != nil {
				return err
			}
			conversationID := mcpAuthorizationConversationID(ctx)
			if conversationID == "" || db == nil || !db.UserCanAccessResource(principal.UserID, principal.ScopeFor("vulnerability:read"), "conversation", conversationID) {
				return fmt.Errorf("no access to conversation %s", conversationID)
			}
			return nil
		case builtin.ToolGetVulnerability:
			return resource("vulnerability:read", "vulnerability", "id")
		case builtin.ToolQueryAssets:
			return require("asset:read")
		case builtin.ToolGetAsset:
			return resource("asset:read", "asset", "id")
		case builtin.ToolCreateAsset:
			if err := require("asset:write"); err != nil {
				return err
			}
			if projectID := mcpAuthorizationString(args, "project_id"); projectID != "" && (db == nil || !db.UserCanAccessResource(principal.UserID, principal.ScopeFor("asset:write"), "project", projectID)) {
				return fmt.Errorf("no access to project %s", projectID)
			}
			return nil
		case builtin.ToolUpdateAsset, builtin.ToolCompleteAssetScan:
			if err := resource("asset:write", "asset", "id"); err != nil {
				return err
			}
			if toolName == builtin.ToolCompleteAssetScan {
				conversationID := mcpAuthorizationConversationID(ctx)
				if conversationID == "" || db == nil || !db.UserCanAccessResource(principal.UserID, principal.ScopeFor("asset:write"), "conversation", conversationID) {
					return fmt.Errorf("no access to conversation %s", conversationID)
				}
				return nil
			}
			if projectID := mcpAuthorizationString(args, "project_id"); projectID != "" && (db == nil || !db.UserCanAccessResource(principal.UserID, principal.ScopeFor("asset:write"), "project", projectID)) {
				return fmt.Errorf("no access to project %s", projectID)
			}
			return nil
		case builtin.ToolDeleteAsset:
			return resource("asset:delete", "asset", "id")
		case builtin.ToolUpsertProjectFact, builtin.ToolDeprecateProjectFact, builtin.ToolRestoreProjectFact:
			return authorizeProjectTool(ctx, principal, db, "project:write")
		case builtin.ToolGetProjectFact, builtin.ToolListProjectFacts, builtin.ToolSearchProjectFacts:
			return authorizeProjectTool(ctx, principal, db, "project:read")
		case builtin.ToolListKnowledgeRiskTypes, builtin.ToolSearchKnowledgeBase:
			return require("knowledge:read")
		case builtin.ToolAnalyzeImage:
			return require("agent:execute")
		case builtin.ToolGetToolExecution, builtin.ToolWaitToolExecution:
			return toolExecutionResource("monitor:read")
		case builtin.ToolCancelToolExecution:
			return toolExecutionResource("monitor:write")
		case builtin.ToolBatchTaskList:
			return require("tasks:read")
		case builtin.ToolBatchTaskGet:
			return resource("tasks:read", "batch_task", "queue_id")
		case builtin.ToolBatchTaskCreate:
			if err := require("tasks:write"); err != nil {
				return err
			}
			if projectID := mcpAuthorizationString(args, "project_id"); projectID != "" && (db == nil || !db.UserCanAccessResource(principal.UserID, principal.ScopeFor("tasks:write"), "project", projectID)) {
				return fmt.Errorf("no access to project %s", projectID)
			}
			return nil
		case builtin.ToolBatchTaskDelete, builtin.ToolBatchTaskRemove:
			return resource("tasks:delete", "batch_task", "queue_id")
		case builtin.ToolBatchTaskStart, builtin.ToolBatchTaskRerun, builtin.ToolBatchTaskPause,
			builtin.ToolBatchTaskUpdateMetadata, builtin.ToolBatchTaskUpdateSchedule,
			builtin.ToolBatchTaskScheduleEnabled, builtin.ToolBatchTaskAdd, builtin.ToolBatchTaskUpdate:
			return resource("tasks:write", "batch_task", "queue_id")
		case builtin.ToolC2Listener:
			return authorizeC2Action(principal, db, args, "c2_listener", "listener_id")
		case builtin.ToolC2Session, builtin.ToolC2Task, builtin.ToolC2File:
			if toolName == builtin.ToolC2File && mcpAuthorizationString(args, "action") == "get_result" {
				return authorizeC2Action(principal, db, args, "c2_task", "task_id")
			}
			return authorizeC2Action(principal, db, args, "c2_session", "session_id")
		case builtin.ToolC2TaskManage:
			return authorizeC2Action(principal, db, args, "c2_task", "task_id")
		case builtin.ToolC2Payload:
			return resource("c2:write", "c2_listener", "listener_id")
		case builtin.ToolC2Event:
			if id := mcpAuthorizationString(args, "session_id"); id != "" {
				return resource("c2:read", "c2_session", "session_id")
			}
			if principal.ScopeFor("c2:read") != database.RBACScopeAll {
				return fmt.Errorf("unfiltered C2 event list requires global scope")
			}
			return require("c2:read")
		case builtin.ToolC2Profile:
			// Profiles are process-global and do not yet have an owner. Writes are
			// therefore reserved for global scope; reads require c2:read.
			if mcpAuthorizationString(args, "action") == "list" || mcpAuthorizationString(args, "action") == "get" {
				return require("c2:read")
			}
			permission := "c2:write"
			if mcpAuthorizationString(args, "action") == "delete" {
				permission = "c2:delete"
			}
			if principal.ScopeFor(permission) != database.RBACScopeAll {
				return fmt.Errorf("C2 profile mutation requires global scope")
			}
			if mcpAuthorizationString(args, "action") == "delete" {
				return require("c2:delete")
			}
			return require("c2:write")
		default:
			if builtin.IsBuiltinTool(toolName) {
				return fmt.Errorf("no authorization policy registered for builtin tool %s", toolName)
			}
			if principal.HasPermission("agent:local-execute") {
				return nil
			}
			return fmt.Errorf("missing agent:local-execute")
		}
	}
}

func externalMCPToolAuthorizer() func(context.Context, string, map[string]interface{}) error {
	return func(ctx context.Context, toolName string, _ map[string]interface{}) error {
		principal, ok := authctx.PrincipalFromContext(ctx)
		if !ok {
			return fmt.Errorf("missing authenticated principal")
		}
		if !principal.HasPermission("mcp:external:execute") {
			return fmt.Errorf("missing permission mcp:external:execute")
		}
		if principal.ScopeFor("mcp:external:execute") != database.RBACScopeAll {
			return fmt.Errorf("external MCP invocation requires global scope")
		}
		if strings.TrimSpace(toolName) == "" {
			return fmt.Errorf("missing external tool name")
		}
		return nil
	}
}

func authorizeC2Action(principal authctx.Principal, db *database.DB, args map[string]interface{}, resourceType, argument string) error {
	action := mcpAuthorizationString(args, "action")
	permission := "c2:write"
	if action == "list" || action == "get" || action == "get_result" || action == "wait" {
		permission = "c2:read"
	} else if action == "delete" || action == "delete_batch" {
		permission = "c2:delete"
	}
	if !principal.HasPermission(permission) {
		return fmt.Errorf("missing permission %s", permission)
	}
	id := mcpAuthorizationString(args, argument)
	if action == "delete_batch" {
		ids := mcpAuthorizationStrings(args, argument+"s")
		if len(ids) == 0 {
			return fmt.Errorf("missing resource identifiers %ss", argument)
		}
		for _, candidate := range ids {
			if db == nil || !db.UserCanAccessResource(principal.UserID, principal.ScopeFor(permission), resourceType, candidate) {
				return fmt.Errorf("no access to %s %s", resourceType, candidate)
			}
		}
		return nil
	}
	if id == "" {
		if action == "create" || action == "list" {
			return nil
		}
		return fmt.Errorf("missing resource identifier %s", argument)
	}
	if db == nil || !db.UserCanAccessResource(principal.UserID, principal.ScopeFor(permission), resourceType, id) {
		return fmt.Errorf("no access to %s %s", resourceType, id)
	}
	return nil
}

func mcpAuthorizationStrings(args map[string]interface{}, key string) []string {
	values := []string{}
	switch raw := args[key].(type) {
	case []string:
		for _, value := range raw {
			if value = strings.TrimSpace(value); value != "" {
				values = append(values, value)
			}
		}
	case []interface{}:
		for _, item := range raw {
			if value, ok := item.(string); ok {
				if value = strings.TrimSpace(value); value != "" {
					values = append(values, value)
				}
			}
		}
	}
	return values
}

func authorizeProjectTool(ctx context.Context, principal authctx.Principal, db *database.DB, permission string) error {
	if !principal.HasPermission(permission) {
		return fmt.Errorf("missing permission %s", permission)
	}
	conversationID := mcpAuthorizationConversationID(ctx)
	if conversationID == "" || db == nil || !db.UserCanAccessResource(principal.UserID, principal.ScopeFor(permission), "conversation", conversationID) {
		return fmt.Errorf("no access to conversation %s", conversationID)
	}
	projectID, err := db.GetConversationProjectID(conversationID)
	if err != nil || strings.TrimSpace(projectID) == "" || !db.UserCanAccessResource(principal.UserID, principal.ScopeFor(permission), "project", projectID) {
		return fmt.Errorf("no access to project %s", projectID)
	}
	return nil
}

func mcpAuthorizationConversationID(ctx context.Context) string {
	if id := strings.TrimSpace(agent.ConversationIDFromContext(ctx)); id != "" {
		return id
	}
	return strings.TrimSpace(mcp.MCPConversationIDFromContext(ctx))
}

func mcpAuthorizationString(args map[string]interface{}, key string) string {
	value, _ := args[key].(string)
	return strings.TrimSpace(value)
}
