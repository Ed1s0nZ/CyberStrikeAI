package app

import (
	"context"
	"path/filepath"
	"testing"

	"cyberstrike-ai/internal/authctx"
	"cyberstrike-ai/internal/c2"
	"cyberstrike-ai/internal/database"
	"cyberstrike-ai/internal/mcp"
	"cyberstrike-ai/internal/mcp/builtin"

	"go.uber.org/zap"
)

func TestC2ListenerCreateInheritsConversationProject(t *testing.T) {
	db, err := database.NewDB(filepath.Join(t.TempDir(), "c2-tools.db"), zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	user, err := db.CreateRBACUser("c2-agent", "C2 Agent", "hash", true, nil)
	if err != nil {
		t.Fatal(err)
	}
	project, err := db.CreateProject(&database.Project{Name: "engagement"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AssignResourceToUser(user.ID, "project", project.ID); err != nil {
		t.Fatal(err)
	}
	conversation, err := db.CreateConversation("project chat", database.ConversationCreateMeta{ProjectID: project.ID})
	if err != nil {
		t.Fatal(err)
	}

	principal := authctx.NewPrincipal(user.ID, user.Username, database.RBACScopeAssigned, map[string]bool{
		"c2:read": true, "c2:write": true,
	})
	ctx := authctx.WithPrincipal(mcp.WithMCPConversationID(context.Background(), conversation.ID), principal)
	server := mcp.NewServer(zap.NewNop())
	server.SetToolAuthorizer(mcpToolAuthorizer(db))
	registerC2Tools(server, c2.NewManager(db, zap.NewNop(), t.TempDir()), zap.NewNop(), 8080)

	result, _, err := server.CallTool(ctx, builtin.ToolC2Listener, map[string]interface{}{
		"action":    "create",
		"name":      "tcp-reverse-2222",
		"type":      "tcp_reverse",
		"bind_host": "0.0.0.0",
		"bind_port": 2222,
	})
	if err != nil || result == nil || result.IsError {
		t.Fatalf("create listener result=%#v err=%v text=%q", result, err, toolResultText(result))
	}

	listeners, err := db.ListC2Listeners()
	if err != nil {
		t.Fatal(err)
	}
	if len(listeners) != 1 {
		t.Fatalf("listener count=%d, want 1", len(listeners))
	}
	if listeners[0].ProjectID != project.ID {
		t.Fatalf("listener project_id=%q, want %q", listeners[0].ProjectID, project.ID)
	}
}
