package security

import (
	"context"
	"cyberstrike-ai/internal/database"
	"database/sql"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const bootstrapAdminUsername = "admin"

// Predefined errors for authentication operations.
var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidPassword    = ErrInvalidCredentials
	ErrUserDisabled       = errors.New("user is disabled")
)

// Session represents an authenticated user session.
type Session struct {
	Token              string
	UserID             string
	Username           string
	MustChangePassword bool
	Permissions        map[string]struct{}
	ExpiresAt          time.Time
}

// WebAuthStore defines the minimum persistence surface needed by AuthManager.
type WebAuthStore interface {
	GetWebUserWithPermissionsByUsername(username string) (*database.WebUserWithPermissions, error)
	GetWebUserByID(userID string) (*database.WebUser, error)
	UpdateWebUserLastLogin(userID string, at time.Time) error
	UpdateWebUserPasswordByID(userID, passwordHash string, mustChangePassword bool) error
}

// AuthManager manages account-based authentication and session lifecycle.
type AuthManager struct {
	store           WebAuthStore
	sessionDuration time.Duration

	mu           sync.RWMutex
	sessions     map[string]Session
	userSessions map[string]map[string]struct{}
}

// NewAuthManager creates a new AuthManager instance.
func NewAuthManager(store WebAuthStore, sessionDurationHours int) *AuthManager {
	if sessionDurationHours <= 0 {
		sessionDurationHours = 12
	}

	return &AuthManager{
		store:           store,
		sessionDuration: time.Duration(sessionDurationHours) * time.Hour,
		sessions:        make(map[string]Session),
		userSessions:    make(map[string]map[string]struct{}),
	}
}

// Authenticate validates a user's credentials and creates a new session.
func (a *AuthManager) Authenticate(ctx context.Context, username, password string) (Session, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return Session{}, err
	}
	if a.store == nil {
		return Session{}, errors.New("auth store is not configured")
	}

	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return Session{}, ErrInvalidCredentials
	}

	user, err := a.store.GetWebUserWithPermissionsByUsername(username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Session{}, ErrInvalidCredentials
		}
		return Session{}, err
	}
	if user == nil {
		return Session{}, ErrInvalidCredentials
	}
	if !user.Enabled {
		return Session{}, ErrUserDisabled
	}
	if !CheckPassword(user.PasswordHash, password) {
		return Session{}, ErrInvalidCredentials
	}

	lastLoginAt := time.Now().UTC()
	if err := a.store.UpdateWebUserLastLogin(user.ID, lastLoginAt); err != nil {
		return Session{}, err
	}
	if err := ctx.Err(); err != nil {
		return Session{}, err
	}

	token := uuid.NewString()
	storedSession := Session{
		Token:              token,
		UserID:             user.ID,
		Username:           user.Username,
		MustChangePassword: user.MustChangePassword,
		Permissions:        permissionMap(user.Permissions),
		ExpiresAt:          time.Now().Add(a.sessionDuration),
	}

	a.mu.Lock()
	a.sessions[token] = storedSession
	if a.userSessions[user.ID] == nil {
		a.userSessions[user.ID] = make(map[string]struct{})
	}
	a.userSessions[user.ID][token] = struct{}{}
	a.mu.Unlock()

	return cloneSession(storedSession), nil
}

// ValidateToken checks whether the provided token is still valid.
func (a *AuthManager) ValidateToken(token string) (Session, bool) {
	if strings.TrimSpace(token) == "" {
		return Session{}, false
	}

	a.mu.RLock()
	session, ok := a.sessions[token]
	a.mu.RUnlock()
	if !ok {
		return Session{}, false
	}

	if time.Now().After(session.ExpiresAt) {
		a.mu.Lock()
		a.revokeTokenLocked(token)
		a.mu.Unlock()
		return Session{}, false
	}

	return cloneSession(session), true
}

// RevokeToken invalidates the specified token.
func (a *AuthManager) RevokeToken(token string) {
	if strings.TrimSpace(token) == "" {
		return
	}

	a.mu.Lock()
	a.revokeTokenLocked(token)
	a.mu.Unlock()
}

// RevokeUserSessions revokes all sessions belonging to the user.
func (a *AuthManager) RevokeUserSessions(userID string) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	tokens, ok := a.userSessions[userID]
	if !ok {
		return
	}
	for token := range tokens {
		delete(a.sessions, token)
	}
	delete(a.userSessions, userID)
}

// RevokeAllSessions clears every active session.
func (a *AuthManager) RevokeAllSessions() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.sessions = make(map[string]Session)
	a.userSessions = make(map[string]map[string]struct{})
}

// SessionDurationHours returns the configured session duration in hours.
func (a *AuthManager) SessionDurationHours() int {
	return int(a.sessionDuration / time.Hour)
}

// ChangePassword updates a specific user's password, clears must_change_password, and revokes user sessions.
func (a *AuthManager) ChangePassword(ctx context.Context, userID, oldPassword, newPassword string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if a.store == nil {
		return errors.New("auth store is not configured")
	}

	userID = strings.TrimSpace(userID)
	if userID == "" || oldPassword == "" || newPassword == "" {
		return ErrInvalidCredentials
	}

	user, err := a.store.GetWebUserByID(userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrInvalidCredentials
		}
		return err
	}
	if user == nil {
		return ErrInvalidCredentials
	}
	if !user.Enabled {
		return ErrUserDisabled
	}
	if !CheckPassword(user.PasswordHash, oldPassword) {
		return ErrInvalidCredentials
	}

	hash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}

	if err := a.store.UpdateWebUserPasswordByID(user.ID, hash, false); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrInvalidCredentials
		}
		return err
	}

	a.RevokeUserSessions(user.ID)
	return nil
}

func (a *AuthManager) revokeTokenLocked(token string) {
	session, ok := a.sessions[token]
	if !ok {
		return
	}

	delete(a.sessions, token)
	tokens, ok := a.userSessions[session.UserID]
	if !ok {
		return
	}
	delete(tokens, token)
	if len(tokens) == 0 {
		delete(a.userSessions, session.UserID)
	}
}

func permissionMap(permissions []string) map[string]struct{} {
	set := make(map[string]struct{}, len(permissions))
	for _, permission := range permissions {
		permission = strings.TrimSpace(permission)
		if permission == "" {
			continue
		}
		set[permission] = struct{}{}
	}
	return set
}

func cloneSession(session Session) Session {
	session.Permissions = clonePermissionMap(session.Permissions)
	return session
}

func clonePermissionMap(src map[string]struct{}) map[string]struct{} {
	if len(src) == 0 {
		return map[string]struct{}{}
	}

	dst := make(map[string]struct{}, len(src))
	for permission := range src {
		dst[permission] = struct{}{}
	}
	return dst
}
