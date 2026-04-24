package claude

import "sync"

// SessionStore maps CyberStrikeAI conversation IDs to Claude CLI session IDs
// so that multi-turn conversations use --resume.
type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]string // conversationID → claudeSessionID
}

// NewSessionStore creates a new SessionStore.
func NewSessionStore() *SessionStore {
	return &SessionStore{
		sessions: make(map[string]string),
	}
}

// Get returns the Claude session ID for a conversation, or empty string if not found.
func (s *SessionStore) Get(conversationID string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sessions[conversationID]
}

// Set stores the Claude session ID for a conversation.
func (s *SessionStore) Set(conversationID, claudeSessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[conversationID] = claudeSessionID
}

// Delete removes a conversation's session mapping.
func (s *SessionStore) Delete(conversationID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, conversationID)
}
