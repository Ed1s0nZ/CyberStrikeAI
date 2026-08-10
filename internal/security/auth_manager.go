package security

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"cyberstrike-ai/internal/database"

	"github.com/google/uuid"
)

// Predefined errors for authentication operations.
var (
	ErrInvalidPassword = errors.New("invalid password")
	// ErrAccountLocked 连续登录失败触发账号临时锁定（见 maxLoginFailures / loginLockDuration）。
	ErrAccountLocked = errors.New("account locked due to too many failed login attempts")
)

// 登录失败锁定：连续 maxLoginFailures 次失败锁定该账号 loginLockDuration。
// 内存实现（重启清零），用于抵御慢速暴力破解；与 /api/auth/login 的 IP 限流互补。
const (
	maxLoginFailures = 5
	loginLockDuration = 15 * time.Minute
)

// loginAttempt 记录某账号的连续失败次数与锁定起始时间。
type loginAttempt struct {
	count    int
	lockedAt time.Time
}

// Session represents an authenticated user session.
type Session struct {
	Token            string
	ExpiresAt        time.Time
	UserID           string
	Username         string
	DisplayName      string
	Roles            []string
	Permissions      map[string]bool
	PermissionScopes map[string]string
	Scope            string
}

// AuthManager manages password-based authentication and session lifecycle.
type AuthManager struct {
	sessionDuration time.Duration
	db              *database.DB

	mu       sync.RWMutex
	sessions map[string]Session
	// loginFails 登录失败计数与锁定状态（key 为小写用户名）
	loginFails map[string]*loginAttempt
}

// NewAuthManager creates a new AuthManager instance.
func NewAuthManager(sessionDurationHours int) *AuthManager {
	if sessionDurationHours <= 0 {
		sessionDurationHours = 12
	}

	return &AuthManager{
		sessionDuration: time.Duration(sessionDurationHours) * time.Hour,
		sessions:        make(map[string]Session),
		loginFails:      make(map[string]*loginAttempt),
	}
}

// AttachRBACStore enables multi-user RBAC authentication. When no users exist yet,
// it bootstraps the built-in admin account and returns the generated initial password.
func (a *AuthManager) AttachRBACStore(db *database.DB) (generatedAdminPassword string, err error) {
	if db == nil {
		return "", errors.New("database is required for authentication")
	}

	needsAdminPassword, err := db.RBACNeedsAdminPassword()
	if err != nil {
		return "", err
	}

	adminPasswordHash := ""
	if needsAdminPassword {
		generatedAdminPassword, err = GenerateStrongPassword(24)
		if err != nil {
			return "", err
		}
		adminPasswordHash, err = HashPassword(generatedAdminPassword)
		if err != nil {
			return "", err
		}
	}

	if err := db.BootstrapRBAC(adminPasswordHash, PermissionCatalog); err != nil {
		return "", err
	}

	a.mu.Lock()
	a.db = db
	a.mu.Unlock()
	return generatedAdminPassword, nil
}

// Authenticate validates the password and creates a new session.
func (a *AuthManager) Authenticate(username, password string) (string, time.Time, error) {
	username = strings.TrimSpace(strings.ToLower(username))
	if username == "" {
		username = "admin"
	}
	if locked, until := a.isLoginLocked(username); locked {
		return "", time.Time{}, fmt.Errorf("%w (until %s)", ErrAccountLocked, until.Format(time.RFC3339))
	}
	session, err := a.authenticateSession(username, password)
	if err != nil {
		a.recordLoginFailure(username)
		return "", time.Time{}, err
	}
	a.clearLoginFailures(username)
	a.mu.Lock()
	a.sessions[session.Token] = session
	a.mu.Unlock()
	return session.Token, session.ExpiresAt, nil
}

// isLoginLocked 返回账号是否处于锁定期（命中时返回解锁时间）。
func (a *AuthManager) isLoginLocked(username string) (bool, time.Time) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	at, ok := a.loginFails[username]
	if !ok || at.count < maxLoginFailures {
		return false, time.Time{}
	}
	if time.Since(at.lockedAt) >= loginLockDuration {
		return false, time.Time{}
	}
	return true, at.lockedAt.Add(loginLockDuration)
}

// recordLoginFailure 记录一次登录失败；达到阈值时进入锁定。
func (a *AuthManager) recordLoginFailure(username string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	at, ok := a.loginFails[username]
	if !ok {
		a.loginFails[username] = &loginAttempt{count: 1}
		return
	}
	if !at.lockedAt.IsZero() && time.Since(at.lockedAt) >= loginLockDuration {
		at.count = 1
		at.lockedAt = time.Time{}
		return
	}
	at.count++
	if at.count >= maxLoginFailures {
		at.lockedAt = time.Now()
	}
}

// clearLoginFailures 登录成功后清除失败计数。
func (a *AuthManager) clearLoginFailures(username string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.loginFails, username)
}

func (a *AuthManager) authenticateSession(username, password string) (Session, error) {
	token := uuid.NewString()
	expiresAt := time.Now().Add(a.sessionDuration)

	a.mu.RLock()
	db := a.db
	a.mu.RUnlock()
	if db == nil {
		return Session{}, errors.New("authentication store is not configured")
	}

	username = strings.TrimSpace(strings.ToLower(username))
	if username == "" {
		username = "admin"
	}
	user, err := db.GetRBACUserByUsername(username)
	if err != nil {
		if err == sql.ErrNoRows {
			return Session{}, ErrInvalidPassword
		}
		return Session{}, err
	}
	if !user.Enabled || !VerifyPasswordHash(password, user.PasswordHash) {
		return Session{}, ErrInvalidPassword
	}
	access, err := db.ResolveRBACAccess(user.ID)
	if err != nil {
		return Session{}, err
	}
	roleIDs := make([]string, 0, len(access.Roles))
	for _, role := range access.Roles {
		roleIDs = append(roleIDs, role.ID)
	}
	return Session{
		Token:            token,
		ExpiresAt:        expiresAt,
		UserID:           user.ID,
		Username:         user.Username,
		DisplayName:      user.DisplayName,
		Roles:            roleIDs,
		Permissions:      access.Permissions,
		PermissionScopes: access.PermissionScopes,
		Scope:            access.Scope,
	}, nil
}

func (s Session) ScopeFor(permission string) string {
	if scope := strings.TrimSpace(s.PermissionScopes[strings.TrimSpace(permission)]); scope != "" {
		return scope
	}
	return strings.TrimSpace(s.Scope)
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
		delete(a.sessions, token)
		a.mu.Unlock()
		return Session{}, false
	}

	return session, true
}

// CheckPassword verifies whether the provided password matches the current password.
func (a *AuthManager) CheckPassword(password string) bool {
	return a.CheckUserPassword("admin", password)
}

// CheckUserPassword verifies whether the provided password matches a user.
func (a *AuthManager) CheckUserPassword(username, password string) bool {
	a.mu.RLock()
	db := a.db
	a.mu.RUnlock()
	if db == nil {
		return false
	}
	user, err := db.GetRBACUserByUsername(username)
	if err != nil {
		return false
	}
	return VerifyPasswordHash(password, user.PasswordHash)
}

func (a *AuthManager) UpdateUserPassword(userID, password string) error {
	password = strings.TrimSpace(password)
	if password == "" {
		return errors.New("auth password must be configured")
	}
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	a.mu.RLock()
	db := a.db
	a.mu.RUnlock()
	if db == nil {
		return errors.New("authentication store is not configured")
	}
	if err := db.UpdateRBACUserPassword(userID, hash); err != nil {
		return err
	}
	a.mu.Lock()
	for token, session := range a.sessions {
		if session.UserID == userID {
			delete(a.sessions, token)
		}
	}
	a.mu.Unlock()
	return nil
}

// RevokeToken invalidates the specified token.
func (a *AuthManager) RevokeToken(token string) {
	if strings.TrimSpace(token) == "" {
		return
	}

	a.mu.Lock()
	delete(a.sessions, token)
	a.mu.Unlock()
}

func (a *AuthManager) RevokeUserSessions(userID string) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return
	}
	a.mu.Lock()
	for token, session := range a.sessions {
		if session.UserID == userID {
			delete(a.sessions, token)
		}
	}
	a.mu.Unlock()
}

func (a *AuthManager) RevokeAllSessions() {
	a.mu.Lock()
	a.sessions = make(map[string]Session)
	a.mu.Unlock()
}

// SessionDurationHours returns the configured session duration in hours.
func (a *AuthManager) SessionDurationHours() int {
	return int(a.sessionDuration / time.Hour)
}

func allPermissions() map[string]bool {
	out := make(map[string]bool, len(PermissionCatalog))
	for key := range PermissionCatalog {
		out[key] = true
	}
	return out
}
