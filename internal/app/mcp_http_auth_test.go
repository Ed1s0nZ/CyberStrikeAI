package app

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/database"
	"cyberstrike-ai/internal/mcp"
	"cyberstrike-ai/internal/security"

	"go.uber.org/zap"
)

func TestStandaloneMCPPrefersUserRBACAndDisablesGlobalTokenByDefault(t *testing.T) {
	db, err := database.NewDB(filepath.Join(t.TempDir(), "mcp-http-auth.db"), zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	auth := security.NewAuthManager(12)
	if _, err := auth.AttachRBACStore(db); err != nil {
		t.Fatal(err)
	}
	hash, err := security.HashPassword("admin-secret")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.UpdateRBACAdminPassword(hash); err != nil {
		t.Fatal(err)
	}
	token, _, err := auth.Authenticate("admin", "admin-secret")
	if err != nil {
		t.Fatal(err)
	}
	server := mcp.NewServer(zap.NewNop())
	server.SetToolAuthorizer(mcpToolAuthorizer(db))
	a := &App{config: &config.Config{MCP: config.MCPConfig{AuthHeader: "X-MCP-Token", AuthHeaderValue: "static-secret"}}, auth: auth, mcpServer: server}
	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)

	userReq := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	userReq.Header.Set("Authorization", "Bearer "+token)
	userW := httptest.NewRecorder()
	a.mcpHandlerWithAuth(userW, userReq)
	if userW.Code != http.StatusOK {
		t.Fatalf("user bearer status = %d: %s", userW.Code, userW.Body.String())
	}

	staticReq := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	staticReq.Header.Set("X-MCP-Token", "static-secret")
	staticW := httptest.NewRecorder()
	a.mcpHandlerWithAuth(staticW, staticReq)
	if staticW.Code != http.StatusUnauthorized {
		t.Fatalf("global static token status = %d, want 401", staticW.Code)
	}
}
