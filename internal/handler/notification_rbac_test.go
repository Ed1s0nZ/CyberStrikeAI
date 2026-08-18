package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"cyberstrike-ai/internal/database"
	"cyberstrike-ai/internal/security"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func TestNotificationReadStateIsPerUser(t *testing.T) {
	db, err := database.NewDB(filepath.Join(t.TempDir(), "notification-rbac.db"), zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	h := NewNotificationHandler(db, nil, zap.NewNop())
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(security.ContextSessionKey, security.Session{UserID: "u1", Scope: database.RBACScopeAssigned})
		c.Next()
	})
	router.POST("/notifications/read", h.MarkRead)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/notifications/read", bytes.NewBufferString(`{"eventIds":["vuln:v1"]}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("mark read status = %d: %s", w.Code, w.Body.String())
	}
	u1, err := h.readStatesByIDs("u1", []string{"vuln:v1"})
	if err != nil || !u1["vuln:v1"] {
		t.Fatalf("u1 read state = %#v, err=%v", u1, err)
	}
	u2, err := h.readStatesByIDs("u2", []string{"vuln:v1"})
	if err != nil || u2["vuln:v1"] {
		t.Fatalf("u2 inherited u1 read state = %#v, err=%v", u2, err)
	}
}
