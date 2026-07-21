package app

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cyberstrike-ai/internal/authctx"
	"cyberstrike-ai/internal/database"
	"cyberstrike-ai/internal/mcp"
	"cyberstrike-ai/internal/mcp/builtin"
	"cyberstrike-ai/internal/security"

	"go.uber.org/zap"
)

func TestMCPToolAuthorizerEnforcesPermissionAndResource(t *testing.T) {
	db, err := database.NewDB(filepath.Join(t.TempDir(), "mcp-authz.db"), zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	user, err := db.CreateRBACUser("mcp-user", "MCP User", "hash", true, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"ws_allowed", "ws_hidden"} {
		if err := db.CreateWebshellConnection(&database.WebShellConnection{ID: id, URL: "http://127.0.0.1/" + id, Type: "php", Method: "post", CmdParam: "cmd", CreatedAt: time.Now()}); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.AssignResourceToUser(user.ID, "webshell", "ws_allowed"); err != nil {
		t.Fatal(err)
	}

	principal := authctx.NewPrincipal(user.ID, user.Username, database.RBACScopeAssigned, map[string]bool{"mcp:write": true, "webshell:write": true})
	ctx := authctx.WithPrincipal(context.Background(), principal)
	authorize := mcpToolAuthorizer(db)
	if err := authorize(ctx, builtin.ToolWebshellExec, map[string]interface{}{"connection_id": "ws_allowed"}); err != nil {
		t.Fatalf("allowed resource denied: %v", err)
	}
	if err := authorize(ctx, builtin.ToolWebshellExec, map[string]interface{}{"connection_id": "ws_hidden"}); err == nil {
		t.Fatal("foreign webshell resource was allowed")
	}
	if err := authorize(ctx, builtin.ToolManageWebshellDelete, map[string]interface{}{"connection_id": "ws_allowed"}); err == nil {
		t.Fatal("delete without webshell:delete was allowed")
	}
}

func TestEveryBuiltinMCPToolHasExplicitAuthorizationPolicy(t *testing.T) {
	db, err := database.NewDB(filepath.Join(t.TempDir(), "mcp-policy-inventory.db"), zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	permissions := map[string]bool{}
	for permission := range security.PermissionCatalog {
		permissions[permission] = true
	}
	ctx := authctx.WithPrincipal(context.Background(), authctx.NewPrincipal("admin", "admin", database.RBACScopeAll, permissions))
	authorize := mcpToolAuthorizer(db)
	args := map[string]interface{}{
		"action": "get", "connection_id": "x", "queue_id": "x", "listener_id": "x",
		"session_id": "x", "task_id": "x", "id": "x", "conversation_id": "x", "execution_id": "x",
	}
	for _, toolName := range builtin.GetAllBuiltinTools() {
		err := authorize(ctx, toolName, args)
		if err != nil && strings.Contains(err.Error(), "no authorization policy registered") {
			t.Errorf("builtin tool %s has no explicit policy", toolName)
		}
	}
}

func TestMCPExecutionControlAuthorizationUsesExecutionScope(t *testing.T) {
	db, err := database.NewDB(filepath.Join(t.TempDir(), "mcp-exec-authz.db"), zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	user, err := db.CreateRBACUser("exec-user", "Exec User", "hash", true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SaveToolExecution(&mcp.ToolExecution{
		ID:          "exec-owned",
		ToolName:    "lab::slow",
		Status:      "running",
		StartTime:   time.Now(),
		OwnerUserID: user.ID,
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveToolExecution(&mcp.ToolExecution{
		ID:        "exec-hidden",
		ToolName:  "lab::slow",
		Status:    "running",
		StartTime: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}

	principal := authctx.NewPrincipal(user.ID, user.Username, database.RBACScopeAssigned, map[string]bool{"monitor:read": true, "monitor:write": true})
	ctx := authctx.WithPrincipal(context.Background(), principal)
	authorize := mcpToolAuthorizer(db)
	if err := authorize(ctx, builtin.ToolWaitToolExecution, map[string]interface{}{"execution_id": "exec-owned"}); err != nil {
		t.Fatalf("owned execution denied: %v", err)
	}
	if err := authorize(ctx, builtin.ToolCancelToolExecution, map[string]interface{}{"execution_id": "exec-hidden"}); err == nil {
		t.Fatal("foreign execution was allowed")
	}
}

func TestMCPAssetToolAuthorizationUsesAssetPermissionsAndScope(t *testing.T) {
	db, err := database.NewDB(filepath.Join(t.TempDir(), "mcp-asset-authz.db"), zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	user, err := db.CreateRBACUser("asset-user", "Asset User", "hash", true, nil)
	if err != nil {
		t.Fatal(err)
	}
	owned := &database.Asset{IP: "192.0.2.10", Port: 443, Protocol: "https"}
	hidden := &database.Asset{IP: "192.0.2.20", Port: 443, Protocol: "https"}
	if _, err := db.UpsertAssets([]*database.Asset{owned}, user.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.UpsertAssets([]*database.Asset{hidden}, ""); err != nil {
		t.Fatal(err)
	}

	permissions := map[string]bool{"asset:read": true, "asset:write": true}
	ctx := authctx.WithPrincipal(context.Background(), authctx.NewPrincipal(user.ID, user.Username, database.RBACScopeAssigned, permissions))
	authorize := mcpToolAuthorizer(db)
	if err := authorize(ctx, builtin.ToolQueryAssets, nil); err != nil {
		t.Fatalf("asset query denied: %v", err)
	}
	if err := authorize(ctx, builtin.ToolGetAsset, map[string]interface{}{"id": owned.ID}); err != nil {
		t.Fatalf("owned asset denied: %v", err)
	}
	if err := authorize(ctx, builtin.ToolGetAsset, map[string]interface{}{"id": hidden.ID}); err == nil {
		t.Fatal("unassigned asset was readable")
	}
	if err := authorize(ctx, builtin.ToolUpdateAsset, map[string]interface{}{"id": owned.ID}); err != nil {
		t.Fatalf("owned asset update denied: %v", err)
	}
	if err := authorize(ctx, builtin.ToolDeleteAsset, map[string]interface{}{"id": owned.ID}); err == nil {
		t.Fatal("asset delete without asset:delete was allowed")
	}
}

func TestExternalMCPRequiresDedicatedPermission(t *testing.T) {
	authorize := externalMCPToolAuthorizer()
	ctx := authctx.WithPrincipal(context.Background(), authctx.NewPrincipal("u1", "user", database.RBACScopeAssigned, map[string]bool{"agent:execute": true}))
	if err := authorize(ctx, "server::tool", nil); err == nil {
		t.Fatal("agent:execute alone authorized an external MCP tool")
	}
	ctx = authctx.WithPrincipal(context.Background(), authctx.NewPrincipal("u1", "user", database.RBACScopeAll, map[string]bool{"mcp:external:execute": true}))
	if err := authorize(ctx, "server::tool", nil); err != nil {
		t.Fatalf("dedicated external MCP permission rejected: %v", err)
	}
}

func TestConfiguredCommandToolRequiresLocalExecutePermission(t *testing.T) {
	authorize := mcpToolAuthorizer(nil)
	agentOnly := authctx.WithPrincipal(context.Background(), authctx.NewPrincipal("u1", "user", database.RBACScopeAssigned, map[string]bool{"agent:execute": true}))
	if err := authorize(agentOnly, "nmap_scan", nil); err == nil {
		t.Fatal("agent:execute alone authorized a configured command tool")
	}
	local := authctx.WithPrincipal(context.Background(), authctx.NewPrincipal("u1", "user", database.RBACScopeAssigned, map[string]bool{"agent:local-execute": true}))
	if err := authorize(local, "nmap_scan", nil); err != nil {
		t.Fatalf("agent:local-execute rejected: %v", err)
	}
}
