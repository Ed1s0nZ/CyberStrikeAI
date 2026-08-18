package database_test

import (
	"testing"
	"time"

	"cyberstrike-ai/internal/database"
	"cyberstrike-ai/internal/security"

	"go.uber.org/zap"
)

func TestRobotBindingCodeIsSingleUseAndPermissionsAreResolvedLive(t *testing.T) {
	db, err := database.NewDB(t.TempDir()+"/robot-identity.db", zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.BootstrapRBAC("hash", security.PermissionCatalog); err != nil {
		t.Fatal(err)
	}
	user, err := db.CreateRBACUser("bound-user", "Bound User", "hash", true, []string{database.RBACSystemRoleOperator})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.CreateRobotBindingCode(user.ID, "code-hash", time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	bound, err := db.ConsumeRobotBindingCode("LARK", "t:tenant|u:user", "code-hash")
	if err != nil || bound.ID != user.ID {
		t.Fatalf("consume binding code: user=%v err=%v", bound, err)
	}
	if _, err := db.ConsumeRobotBindingCode("lark", "t:tenant|u:other", "code-hash"); err == nil {
		t.Fatal("single-use binding code was accepted twice")
	}
	access, err := db.ResolveRobotRBACAccess("lark", "t:tenant|u:user")
	if err != nil || !access.Permissions["agent:execute"] {
		t.Fatalf("resolved access does not include live role permissions: %#v err=%v", access, err)
	}
	disabled := false
	if err := db.UpdateRBACUser(user.ID, user.DisplayName, &disabled, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ResolveRobotRBACAccess("lark", "t:tenant|u:user"); err == nil {
		t.Fatal("disabled RBAC user retained robot access")
	}
}

func TestRobotBindingCodeExpiryAndOwnerScopedRevocation(t *testing.T) {
	db, err := database.NewDB(t.TempDir()+"/robot-revoke.db", zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.BootstrapRBAC("hash", security.PermissionCatalog); err != nil {
		t.Fatal(err)
	}
	u1, _ := db.CreateRBACUser("binding-owner", "Owner", "hash", true, nil)
	u2, _ := db.CreateRBACUser("binding-other", "Other", "hash", true, nil)
	now := time.Now()
	if _, err := db.Exec(`INSERT INTO robot_binding_codes (code_hash, rbac_user_id, expires_at, created_at) VALUES (?, ?, ?, ?)`, "expired-hash", u1.ID, now.Add(-time.Minute), now.Add(-2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ConsumeRobotBindingCode("wecom", "t:corp|u:expired", "expired-hash"); err == nil {
		t.Fatal("expired binding code was accepted")
	}
	if err := db.CreateRobotBindingCode(u1.ID, "valid-hash", time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ConsumeRobotBindingCode("wecom", "t:corp|u:one", "valid-hash"); err != nil {
		t.Fatal(err)
	}
	bindings, err := db.ListRobotUserBindings(u1.ID)
	if err != nil || len(bindings) != 1 {
		t.Fatalf("bindings=%v err=%v", bindings, err)
	}
	if err := db.DeleteRobotUserBindingForUser(bindings[0].ID, u2.ID); err == nil {
		t.Fatal("another user revoked a binding they do not own")
	}
	if _, err := db.ResolveRobotRBACAccess("wecom", "t:corp|u:one"); err != nil {
		t.Fatalf("unauthorized revocation changed binding: %v", err)
	}
	if err := db.DeleteRobotUserBindingForUser(bindings[0].ID, u1.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ResolveRobotRBACAccess("wecom", "t:corp|u:one"); err == nil {
		t.Fatal("revoked binding still resolves")
	}
}
