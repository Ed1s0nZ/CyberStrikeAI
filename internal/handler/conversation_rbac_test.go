package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"cyberstrike-ai/internal/database"
	"cyberstrike-ai/internal/security"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func TestCreateConversationRequiresProjectAccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, user := setupConversationRBACTest(t)
	project, err := db.CreateProject(&database.Project{Name: "hidden"})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	handler := NewConversationHandler(db, zap.NewNop())

	w := performConversationRequest(user, http.MethodPost, "/api/conversations", map[string]string{
		"title":     "blocked",
		"projectId": project.ID,
	}, handler.CreateConversation)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusForbidden, w.Body.String())
	}

	if err := db.AssignResourceToUser(user.ID, "project", project.ID); err != nil {
		t.Fatalf("AssignResourceToUser: %v", err)
	}
	w = performConversationRequest(user, http.MethodPost, "/api/conversations", map[string]string{
		"title":     "allowed",
		"projectId": project.ID,
	}, handler.CreateConversation)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestSetConversationProjectRequiresProjectAccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, user := setupConversationRBACTest(t)
	project, err := db.CreateProject(&database.Project{Name: "hidden"})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	conv, err := db.CreateConversation("owned", database.ConversationCreateMeta{})
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	if err := db.SetResourceOwner("conversation", conv.ID, user.ID); err != nil {
		t.Fatalf("SetResourceOwner: %v", err)
	}
	if err := db.AssignResourceToUser(user.ID, "conversation", conv.ID); err != nil {
		t.Fatalf("AssignResourceToUser conversation: %v", err)
	}
	handler := NewConversationHandler(db, zap.NewNop())

	w := performConversationRequest(user, http.MethodPut, "/api/conversations/"+conv.ID+"/project", map[string]string{
		"projectId": project.ID,
	}, func(c *gin.Context) {
		c.Params = gin.Params{{Key: "id", Value: conv.ID}}
		handler.SetConversationProject(c)
	})
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusForbidden, w.Body.String())
	}

	if err := db.AssignResourceToUser(user.ID, "project", project.ID); err != nil {
		t.Fatalf("AssignResourceToUser project: %v", err)
	}
	w = performConversationRequest(user, http.MethodPut, "/api/conversations/"+conv.ID+"/project", map[string]string{
		"projectId": project.ID,
	}, func(c *gin.Context) {
		c.Params = gin.Params{{Key: "id", Value: conv.ID}}
		handler.SetConversationProject(c)
	})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusOK, w.Body.String())
	}
}

func setupConversationRBACTest(t *testing.T) (*database.DB, *database.RBACUser) {
	t.Helper()
	db, err := database.NewDB(filepath.Join(t.TempDir(), "conversation-rbac.db"), zap.NewNop())
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	user, err := db.CreateRBACUser("operator1", "Operator One", "hash", true, nil)
	if err != nil {
		t.Fatalf("CreateRBACUser: %v", err)
	}
	return db, user
}

func performConversationRequest(user *database.RBACUser, method, path string, body map[string]string, handler gin.HandlerFunc) *httptest.ResponseRecorder {
	payload, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, path, bytes.NewReader(payload))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(security.ContextSessionKey, security.Session{
		UserID:      user.ID,
		Username:    user.Username,
		Permissions: map[string]bool{"chat:write": true},
		Scope:       database.RBACScopeAssigned,
	})
	handler(c)
	return w
}
