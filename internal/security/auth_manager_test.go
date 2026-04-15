package security

import (
	"context"
	"errors"
	"testing"
	"time"

	"cyberstrike-ai/internal/database"
)

func TestAuthManager_AuthenticateByUsername(t *testing.T) {
	db := openTestSecurityDB(t)

	roleID, err := db.CreateWebAccessRole(database.CreateWebAccessRoleInput{
		Name:        "config-reader",
		Description: "Read system configuration",
		Permissions: []string{PermissionSystemConfigRead},
		IsSystem:    false,
	})
	if err != nil {
		t.Fatalf("CreateWebAccessRole() error = %v", err)
	}

	passwordHash, err := HashPassword("ReaderPass123!")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	_, err = db.CreateWebUser(database.CreateWebUserInput{
		Username:           "reader",
		DisplayName:        "Reader",
		PasswordHash:       passwordHash,
		Enabled:            true,
		MustChangePassword: true,
		RoleIDs:            []string{roleID},
	})
	if err != nil {
		t.Fatalf("CreateWebUser() error = %v", err)
	}

	manager := NewAuthManager(db, 12)
	session, err := manager.Authenticate(context.Background(), "reader", "ReaderPass123!")
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}

	if session.Username != "reader" {
		t.Fatalf("expected session username reader, got %q", session.Username)
	}
	if !session.MustChangePassword {
		t.Fatal("expected must_change_password to be true")
	}

	if _, ok := session.Permissions[PermissionSystemConfigRead]; !ok {
		t.Fatalf("expected permission %q in session, got %#v", PermissionSystemConfigRead, session.Permissions)
	}

	delete(session.Permissions, PermissionSystemConfigRead)
	stored, ok := manager.ValidateToken(session.Token)
	if !ok {
		t.Fatal("expected token to remain valid")
	}
	if _, has := stored.Permissions[PermissionSystemConfigRead]; !has {
		t.Fatal("expected stored permissions to be immutable from caller changes")
	}

	delete(stored.Permissions, PermissionSystemConfigRead)
	storedAgain, ok := manager.ValidateToken(session.Token)
	if !ok {
		t.Fatal("expected token to remain valid after second validation")
	}
	if _, has := storedAgain.Permissions[PermissionSystemConfigRead]; !has {
		t.Fatal("expected validation result permissions to be immutable from caller changes")
	}
}

func TestAuthManager_RevokeUserSessions(t *testing.T) {
	db := openTestSecurityDB(t)

	roleID, err := db.CreateWebAccessRole(database.CreateWebAccessRoleInput{
		Name:        "viewer",
		Description: "basic viewer role",
		Permissions: []string{PermissionSystemConfigRead},
		IsSystem:    false,
	})
	if err != nil {
		t.Fatalf("CreateWebAccessRole() error = %v", err)
	}

	passwordHash, err := HashPassword("ViewerPass123!")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	user, err := db.CreateWebUser(database.CreateWebUserInput{
		Username:     "viewer",
		DisplayName:  "Viewer",
		PasswordHash: passwordHash,
		Enabled:      true,
		RoleIDs:      []string{roleID},
	})
	if err != nil {
		t.Fatalf("CreateWebUser() error = %v", err)
	}

	manager := NewAuthManager(db, 12)
	session, err := manager.Authenticate(context.Background(), "viewer", "ViewerPass123!")
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}

	manager.RevokeUserSessions(user.ID)

	if _, ok := manager.ValidateToken(session.Token); ok {
		t.Fatal("expected token to be invalid after RevokeUserSessions()")
	}
}

func TestAuthManager_Authenticate_DisabledUserCannotLogin(t *testing.T) {
	db := openTestSecurityDB(t)

	roleID, err := db.CreateWebAccessRole(database.CreateWebAccessRoleInput{
		Name:        "disabled-user-role",
		Description: "role for disabled users",
		Permissions: []string{PermissionSystemConfigRead},
		IsSystem:    false,
	})
	if err != nil {
		t.Fatalf("CreateWebAccessRole() error = %v", err)
	}

	passwordHash, err := HashPassword("DisabledPass123!")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	if _, err := db.CreateWebUser(database.CreateWebUserInput{
		Username:           "disabled-user",
		DisplayName:        "Disabled",
		PasswordHash:       passwordHash,
		Enabled:            false,
		MustChangePassword: false,
		RoleIDs:            []string{roleID},
	}); err != nil {
		t.Fatalf("CreateWebUser() error = %v", err)
	}

	manager := NewAuthManager(db, 12)
	if _, err := manager.Authenticate(context.Background(), "disabled-user", "DisabledPass123!"); !errors.Is(err, ErrUserDisabled) {
		t.Fatalf("expected ErrUserDisabled, got %v", err)
	}
}

func TestAuthManager_Authenticate_WrongPasswordReturnsInvalidCredentials(t *testing.T) {
	db := openTestSecurityDB(t)

	roleID, err := db.CreateWebAccessRole(database.CreateWebAccessRoleInput{
		Name:        "wrong-pass-role",
		Description: "role for wrong password test",
		Permissions: []string{PermissionSystemConfigRead},
		IsSystem:    false,
	})
	if err != nil {
		t.Fatalf("CreateWebAccessRole() error = %v", err)
	}

	passwordHash, err := HashPassword("CorrectPass123!")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	if _, err := db.CreateWebUser(database.CreateWebUserInput{
		Username:           "wrong-pass-user",
		DisplayName:        "Wrong Pass",
		PasswordHash:       passwordHash,
		Enabled:            true,
		MustChangePassword: false,
		RoleIDs:            []string{roleID},
	}); err != nil {
		t.Fatalf("CreateWebUser() error = %v", err)
	}

	manager := NewAuthManager(db, 12)
	if _, err := manager.Authenticate(context.Background(), "wrong-pass-user", "BadPass123!"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestAuthManager_Authenticate_StoreFailureIsInternalError(t *testing.T) {
	sentinelErr := errors.New("store unavailable")
	manager := NewAuthManager(&failingLookupStore{lookupErr: sentinelErr}, 12)

	_, err := manager.Authenticate(context.Background(), "reader", "ReaderPass123!")
	if !errors.Is(err, sentinelErr) {
		t.Fatalf("expected sentinel internal error, got %v", err)
	}
	if errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected non-credential error, got %v", err)
	}
}

func TestAuthManager_ChangePassword(t *testing.T) {
	db := openTestSecurityDB(t)

	roleID, err := db.CreateWebAccessRole(database.CreateWebAccessRoleInput{
		Name:        "change-pass-role",
		Description: "role for change password test",
		Permissions: []string{PermissionSystemConfigRead},
		IsSystem:    false,
	})
	if err != nil {
		t.Fatalf("CreateWebAccessRole() error = %v", err)
	}

	passwordHash, err := HashPassword("OldPass123!")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	user, err := db.CreateWebUser(database.CreateWebUserInput{
		Username:           "change-user",
		DisplayName:        "Change User",
		PasswordHash:       passwordHash,
		Enabled:            true,
		MustChangePassword: true,
		RoleIDs:            []string{roleID},
	})
	if err != nil {
		t.Fatalf("CreateWebUser() error = %v", err)
	}

	manager := NewAuthManager(db, 12)
	session, err := manager.Authenticate(context.Background(), "change-user", "OldPass123!")
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}

	if err := manager.ChangePassword(context.Background(), user.ID, "OldPass123!", "NewPass123!"); err != nil {
		t.Fatalf("ChangePassword() error = %v", err)
	}

	if _, ok := manager.ValidateToken(session.Token); ok {
		t.Fatal("expected existing user sessions to be revoked after password change")
	}

	if _, err := manager.Authenticate(context.Background(), "change-user", "OldPass123!"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected old password to be invalid, got %v", err)
	}

	newSession, err := manager.Authenticate(context.Background(), "change-user", "NewPass123!")
	if err != nil {
		t.Fatalf("Authenticate(new password) error = %v", err)
	}
	if newSession.MustChangePassword {
		t.Fatal("expected must_change_password to be cleared after password change")
	}
}

type failingLookupStore struct {
	lookupErr error
}

func (s *failingLookupStore) GetWebUserWithPermissionsByUsername(username string) (*database.WebUserWithPermissions, error) {
	return nil, s.lookupErr
}

func (s *failingLookupStore) GetWebUserByID(userID string) (*database.WebUser, error) {
	return nil, s.lookupErr
}

func (s *failingLookupStore) UpdateWebUserLastLogin(userID string, at time.Time) error {
	return nil
}

func (s *failingLookupStore) UpdateWebUserPasswordByID(userID, passwordHash string, mustChangePassword bool) error {
	return nil
}
