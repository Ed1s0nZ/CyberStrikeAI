package security

import (
	"path/filepath"
	"testing"

	"cyberstrike-ai/internal/database"

	"go.uber.org/zap"
)

func TestAuthManagerAuthenticatesCreatedRBACUser(t *testing.T) {
	db, err := database.NewDB(filepath.Join(t.TempDir(), "auth-rbac.db"), zap.NewNop())
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	manager, err := NewAuthManager("admin-secret", 12)
	if err != nil {
		t.Fatalf("NewAuthManager: %v", err)
	}
	if err := manager.AttachRBACStore(db); err != nil {
		t.Fatalf("AttachRBACStore: %v", err)
	}
	hash, err := HashPassword("operator-secret")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	user, err := db.CreateRBACUser("operator1", "Operator One", hash, true, []string{database.RBACSystemRoleViewer})
	if err != nil {
		t.Fatalf("CreateRBACUser: %v", err)
	}

	token, _, err := manager.Authenticate("operator1", "operator-secret")
	if err != nil {
		t.Fatalf("Authenticate created user: %v", err)
	}
	session, ok := manager.ValidateToken(token)
	if !ok {
		t.Fatalf("expected created user session to validate")
	}
	if session.UserID != user.ID || session.Username != "operator1" {
		t.Fatalf("session user = %s/%s, want %s/operator1", session.UserID, session.Username, user.ID)
	}
	if !session.Permissions["auth:self"] || !session.Permissions["chat:read"] {
		t.Fatalf("expected viewer permissions in session, got %#v", session.Permissions)
	}

	if _, _, err := manager.Authenticate("", "operator-secret"); err == nil {
		t.Fatalf("empty username must not authenticate non-admin user")
	}
}
