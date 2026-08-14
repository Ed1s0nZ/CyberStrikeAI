package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"cyberstrike-ai/internal/database"
	"cyberstrike-ai/internal/security"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func TestWebshellExecRequiresConnectionAccessWhenConnectionIDProvided(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, user, allowed, hidden := setupWebshellRBACTest(t)
	handler := NewWebShellHandler(zap.NewNop(), db)

	w := performWebshellJSON(user, http.MethodPost, "/api/webshell/exec", map[string]interface{}{
		"url":           hidden.URL,
		"connection_id": hidden.ID,
		"command":       "id",
	}, handler.Exec)
	if w.Code != http.StatusForbidden {
		t.Fatalf("hidden connection status = %d, want %d: %s", w.Code, http.StatusForbidden, w.Body.String())
	}

	w = performWebshellJSON(user, http.MethodPost, "/api/webshell/exec", map[string]interface{}{
		"url":           hidden.URL,
		"connection_id": allowed.ID,
		"command":       "id",
	}, handler.Exec)
	if w.Code != http.StatusForbidden {
		t.Fatalf("mismatched URL status = %d, want %d: %s", w.Code, http.StatusForbidden, w.Body.String())
	}
}

func TestWebshellExecAllowsAdHocURLWithoutConnectionID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	_, user, _, _ := setupWebshellRBACTest(t)
	handler := NewWebShellHandler(zap.NewNop(), nil)
	// Ad-hoc probe (connectivity test before save) must not be rejected as "无权访问".
	// The target URL will fail to connect; we only assert auth allows the request through.
	w := performWebshellJSON(user, http.MethodPost, "/api/webshell/exec", map[string]interface{}{
		"url": "http://127.0.0.1:1/admin", "command": "id",
	}, handler.Exec)
	if w.Code == http.StatusForbidden {
		t.Fatalf("ad-hoc URL status = %d, want non-403: %s", w.Code, w.Body.String())
	}
	var resp ExecResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v body=%s", err, w.Body.String())
	}
	if resp.OK {
		t.Fatalf("expected connection failure for closed port, got ok=true")
	}
}

func TestWebshellExecRejectsAdHocWithoutWritePermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	user := &database.RBACUser{ID: "u_ro", Username: "readonly"}
	handler := NewWebShellHandler(zap.NewNop(), nil)
	payload, _ := json.Marshal(map[string]interface{}{
		"url": "http://127.0.0.1/admin", "command": "id",
	})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/webshell/exec", bytes.NewReader(payload))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(security.ContextSessionKey, security.Session{
		UserID:      user.ID,
		Username:    user.Username,
		Permissions: map[string]bool{"webshell:read": true},
		Scope:       database.RBACScopeAssigned,
	})
	handler.Exec(c)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusForbidden, w.Body.String())
	}
}

func TestWebshellFileOpRequiresConnectionAccessWhenConnectionIDProvided(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, user, _, hidden := setupWebshellRBACTest(t)
	handler := NewWebShellHandler(zap.NewNop(), db)

	w := performWebshellJSON(user, http.MethodPost, "/api/webshell/file", map[string]interface{}{
		"url":           hidden.URL,
		"connection_id": hidden.ID,
		"action":        "list",
		"path":          ".",
	}, handler.FileOp)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusForbidden, w.Body.String())
	}
}

func setupWebshellRBACTest(t *testing.T) (*database.DB, *database.RBACUser, *database.WebShellConnection, *database.WebShellConnection) {
	t.Helper()
	db, err := database.NewDB(filepath.Join(t.TempDir(), "webshell-rbac.db"), zap.NewNop())
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	user, err := db.CreateRBACUser("operator1", "Operator One", "hash", true, nil)
	if err != nil {
		t.Fatalf("CreateRBACUser: %v", err)
	}
	allowed := &database.WebShellConnection{
		ID:        "ws_allowed",
		URL:       "http://127.0.0.1/allowed.php",
		Type:      "php",
		Method:    "post",
		CmdParam:  "cmd",
		CreatedAt: time.Now(),
	}
	hidden := &database.WebShellConnection{
		ID:        "ws_hidden",
		URL:       "http://127.0.0.1/hidden.php",
		Type:      "php",
		Method:    "post",
		CmdParam:  "cmd",
		CreatedAt: time.Now(),
	}
	if err := db.CreateWebshellConnection(allowed); err != nil {
		t.Fatalf("CreateWebshellConnection allowed: %v", err)
	}
	if err := db.CreateWebshellConnection(hidden); err != nil {
		t.Fatalf("CreateWebshellConnection hidden: %v", err)
	}
	if err := db.AssignResourceToUser(user.ID, "webshell", allowed.ID); err != nil {
		t.Fatalf("AssignResourceToUser: %v", err)
	}
	return db, user, allowed, hidden
}

func performWebshellJSON(user *database.RBACUser, method, path string, body map[string]interface{}, handler gin.HandlerFunc) *httptest.ResponseRecorder {
	payload, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, path, bytes.NewReader(payload))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(security.ContextSessionKey, security.Session{
		UserID:      user.ID,
		Username:    user.Username,
		Permissions: map[string]bool{"webshell:write": true},
		Scope:       database.RBACScopeAssigned,
	})
	handler(c)
	return w
}
