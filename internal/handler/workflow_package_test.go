package handler

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"cyberstrike-ai/internal/database"
	"cyberstrike-ai/internal/security"
	workflowpkg "cyberstrike-ai/internal/workflow/package"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func TestWorkflowPackageHandlerInspectionAndCreateImport(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := database.NewDB(filepath.Join(t.TempDir(), "workflow-package-handler.db"), zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	h := NewWorkflowHandler(db, zap.NewNop())
	pkg, _, err := workflowpkg.Export(workflowpkg.Document{ID: "wf-api", Name: "API workflow", Version: 4, Enabled: true, UpdatedAt: time.Now().UTC(), GraphJSON: `{"nodes":[{"id":"start-1","type":"start","label":"开始","position":{"x":0,"y":0},"config":{}},{"id":"out-1","type":"output","label":"输出","position":{"x":0,"y":120},"config":{"output_key":"result","source_binding":{"from":"inputs","field":"message"}}}],"edges":[{"id":"e1","source":"start-1","target":"out-1"}],"config":{"schema_version":1}}`})
	if err != nil {
		t.Fatal(err)
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "wf-api.csapkg.zip")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(pkg); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/workflow-package-inspections", &body)
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())
	c.Set(security.ContextSessionKey, security.Session{UserID: "user-1"})
	h.CreatePackageInspection(c)
	if w.Code != http.StatusCreated {
		t.Fatalf("inspection status=%d body=%s", w.Code, w.Body.String())
	}
	var inspected struct {
		Inspection struct {
			ID string `json:"id"`
		} `json:"inspection"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &inspected); err != nil {
		t.Fatal(err)
	}
	applyBody := bytes.NewBufferString(`{"inspection_id":"` + inspected.Inspection.ID + `","resolution":{"action":"create","new_workflow_id":""},"confirm_overwrite":false}`)
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/workflow-package-imports", applyBody)
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("Idempotency-Key", uuid.NewString())
	c.Set(security.ContextSessionKey, security.Session{UserID: "user-1"})
	h.ApplyPackageImport(c)
	if w.Code != http.StatusCreated {
		t.Fatalf("import status=%d body=%s", w.Code, w.Body.String())
	}
	saved, _ := db.GetWorkflowDefinition("wf-api")
	if saved == nil || saved.Version != 1 {
		t.Fatalf("saved=%#v", saved)
	}
}
