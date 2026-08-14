package handler

import (
	"bytes"
	"cyberstrike-ai/internal/audit"
	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/database"
	"cyberstrike-ai/internal/security"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func TestRBACAssignResourceBatchIsAtomicAndLegacyCompatible(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := database.NewDB(filepath.Join(t.TempDir(), "rbac-handler.db"), zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	user, err := db.CreateRBACUser("api-member", "API Member", "hash", true, nil)
	if err != nil {
		t.Fatal(err)
	}
	p1, _ := db.CreateProject(&database.Project{Name: "p1"})
	p2, _ := db.CreateProject(&database.Project{Name: "p2"})
	p3, _ := db.CreateProject(&database.Project{Name: "p3"})

	h := NewRBACHandler(db, zap.NewNop())
	router := gin.New()
	router.POST("/api/rbac/resource-assignments", h.AssignResource)

	batch := performRBACJSONRequest(t, router, map[string]interface{}{
		"user_id": user.ID, "resource_type": "project", "resource_ids": []string{p1.ID, p2.ID},
	})
	if batch.Code != http.StatusOK {
		t.Fatalf("batch status = %d, body = %s", batch.Code, batch.Body.String())
	}
	var batchBody map[string]interface{}
	if err := json.Unmarshal(batch.Body.Bytes(), &batchBody); err != nil {
		t.Fatal(err)
	}
	if batchBody["created"] != float64(2) {
		t.Fatalf("batch response = %#v, want created=2", batchBody)
	}

	invalid := performRBACJSONRequest(t, router, map[string]interface{}{
		"user_id": user.ID, "resource_type": "project", "resource_ids": []string{p3.ID, "missing"},
	})
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid status = %d, body = %s", invalid.Code, invalid.Body.String())
	}
	rows, err := db.ListRBACResourceAssignments(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("failed batch persisted partial data: %#v", rows)
	}

	legacy := performRBACJSONRequest(t, router, map[string]interface{}{
		"user_id": user.ID, "resource_type": "project", "resource_id": p3.ID,
	})
	if legacy.Code != http.StatusOK {
		t.Fatalf("legacy status = %d, body = %s", legacy.Code, legacy.Body.String())
	}
}

func TestRBACAssignResourceAutoDetectsActualType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := database.NewDB(filepath.Join(t.TempDir(), "rbac-auto-detect.db"), zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	user, err := db.CreateRBACUser("auto-member", "Auto Member", "hash", true, nil)
	if err != nil {
		t.Fatal(err)
	}
	project, err := db.CreateProject(&database.Project{Name: "auto-project"})
	if err != nil {
		t.Fatal(err)
	}
	h := NewRBACHandler(db, zap.NewNop())
	router := gin.New()
	router.POST("/api/rbac/resource-assignments", h.AssignResource)

	response := performRBACJSONRequest(t, router, map[string]interface{}{
		"user_id": user.ID, "resource_type": "conversation", "resource_ids": []string{project.ID}, "auto_detect": true,
	})
	if response.Code != http.StatusOK {
		t.Fatalf("auto-detect status = %d, body = %s", response.Code, response.Body.String())
	}
	rows, err := db.ListRBACResourceAssignments(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].ResourceType != "project" || rows[0].ResourceID != project.ID {
		t.Fatalf("auto-detected assignments = %#v, want project/%s", rows, project.ID)
	}
}

func TestRBACAssignableResourcesArePaged(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := database.NewDB(filepath.Join(t.TempDir(), "rbac-picker.db"), zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	for _, name := range []string{"p1", "p2", "p3"} {
		if _, err := db.CreateProject(&database.Project{Name: name}); err != nil {
			t.Fatal(err)
		}
	}
	h := NewRBACHandler(db, zap.NewNop())
	router := gin.New()
	router.GET("/api/rbac/resources", h.ListAssignableResources)

	request := httptest.NewRequest(http.MethodGet, "/api/rbac/resources?type=project&limit=2&offset=0", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Resources []database.RBACResourceOption `json:"resources"`
		HasMore   bool                          `json:"has_more"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Resources) != 2 || !body.HasMore {
		t.Fatalf("page = %#v, has_more = %v; want two rows and another page", body.Resources, body.HasMore)
	}
}

func TestRBACDeleteResourceAssignmentAuditsTargetResource(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := database.NewDB(filepath.Join(t.TempDir(), "rbac-revoke-audit.db"), zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	user, err := db.CreateRBACUser("audit-member", "Audit Member", "hash", true, nil)
	if err != nil {
		t.Fatal(err)
	}
	project, err := db.CreateProject(&database.Project{Name: "audit-project"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.AssignResourcesToUser(user.ID, "project", []string{project.ID}); err != nil {
		t.Fatal(err)
	}
	assignments, err := db.ListRBACResourceAssignments(user.ID)
	if err != nil || len(assignments) != 1 {
		t.Fatalf("assignments = %#v, err = %v", assignments, err)
	}

	h := NewRBACHandler(db, zap.NewNop())
	h.SetAudit(audit.NewService(db, &config.Config{}, zap.NewNop()))
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(security.ContextUsernameKey, "operator-user")
		c.Next()
	})
	router.DELETE("/api/rbac/resource-assignments/:id", h.DeleteResourceAssignment)
	request := httptest.NewRequest(http.MethodDelete, "/api/rbac/resource-assignments/"+assignments[0].ID, nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	logs, err := db.ListAuditLogs(database.ListAuditLogsFilter{Category: "rbac", RelatedUserID: user.ID, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 1 {
		t.Fatalf("audit logs = %#v, want one member-related revoke", logs)
	}
	log := logs[0]
	if log.Action != "delete_resource_assignment" || log.Actor != "operator-user" || log.ResourceType != "project" || log.ResourceID != project.ID {
		t.Fatalf("audit log = %#v", log)
	}
	if log.Detail["user_id"] != user.ID || log.Detail["assignment_id"] != assignments[0].ID {
		t.Fatalf("audit detail = %#v", log.Detail)
	}
}

func performRBACJSONRequest(t *testing.T, router http.Handler, payload map[string]interface{}) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/rbac/resource-assignments", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}
