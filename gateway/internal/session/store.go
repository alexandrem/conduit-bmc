package session

import (
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// WebSession represents a web console session with JWT token mapping
type WebSession struct {
	ID             string // Session cookie ID
	SOLSessionID   string // Associated SOL session (if SOL console)
	VNCSessionID   string // Associated VNC session (if VNC console)
	CustomerJWT    string // Original JWT from manager
	CreatedAt      time.Time
	LastActivityAt time.Time
	ExpiresAt      time.Time
	TokenExpiresAt time.Time // JWT expiration
	TokenRenewalAt time.Time // When to renew (before expiration)
	CustomerID     string    // Extracted from JWT
	ServerID       string    // Server this session is for
}

// Store defines the interface for session storage
type Store interface {
	Create(session *WebSession) error
	Get(sessionID string) (*WebSession, error)
	Update(session *WebSession) error
	Delete(sessionID string) error
	UpdateActivity(sessionID string) error

	// Lookup by console session ID
	GetBySOLSessionID(solSessionID string) (*WebSession, error)
	GetByVNCSessionID(vncSessionID string) (*WebSession, error)

	// For token renewal
	GetSessionsNeedingRenewal() []*WebSession

	// Cleanup
	DeleteExpired() int
}

// InMemoryStore implements Store interface with in-memory storage
type InMemoryStore struct {
	sessions map[string]*WebSession
	mu       sync.RWMutex
}

// NewInMemoryStore creates a new in-memory session store
func NewInMemoryStore() *InMemoryStore {
	store := &InMemoryStore{
		sessions: make(map[string]*WebSession),
	}

	// Start cleanup worker
	go store.cleanupWorker()

	return store
}

// Create adds a new session to the store
func (s *InMemoryStore) Create(session *WebSession) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.sessions[session.ID] = session

	log.Debug().
		Str("session_id", session.ID).
		Str("customer_id", session.CustomerID).
		Str("server_id", session.ServerID).
		Time("expires_at", session.ExpiresAt).
		Msg("Created web session")

	return nil
}

// Get retrieves a session by ID
func (s *InMemoryStore) Get(sessionID string) (*WebSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, exists := s.sessions[sessionID]
	if !exists {
		return nil, ErrSessionNotFound
	}

	// Check if session expired
	if time.Now().After(session.ExpiresAt) {
		return nil, ErrSessionExpired
	}

	return session, nil
}

// Update updates an existing session
func (s *InMemoryStore) Update(session *WebSession) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.sessions[session.ID]; !exists {
		return ErrSessionNotFound
	}

	s.sessions[session.ID] = session

	log.Debug().
		Str("session_id", session.ID).
		Msg("Updated web session")

	return nil
}

// Delete removes a session from the store
func (s *InMemoryStore) Delete(sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.sessions, sessionID)

	log.Debug().
		Str("session_id", sessionID).
		Msg("Deleted web session")

	return nil
}

// UpdateActivity updates the last activity time for a session
func (s *InMemoryStore) UpdateActivity(sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, exists := s.sessions[sessionID]
	if !exists {
		return ErrSessionNotFound
	}

	session.LastActivityAt = time.Now()
	return nil
}

// GetBySOLSessionID finds a web session by its associated SOL session ID
func (s *InMemoryStore) GetBySOLSessionID(solSessionID string) (*WebSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, session := range s.sessions {
		if session.SOLSessionID == solSessionID {
			// Check if session expired
			if time.Now().After(session.ExpiresAt) {
				return nil, ErrSessionExpired
			}
			return session, nil
		}
	}

	return nil, ErrSessionNotFound
}

// GetByVNCSessionID finds a web session by its associated VNC session ID
func (s *InMemoryStore) GetByVNCSessionID(vncSessionID string) (*WebSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, session := range s.sessions {
		if session.VNCSessionID == vncSessionID {
			// Check if session expired
			if time.Now().After(session.ExpiresAt) {
				return nil, ErrSessionExpired
			}
			return session, nil
		}
	}

	return nil, ErrSessionNotFound
}

// GetSessionsNeedingRenewal returns sessions that need token renewal
// NOTE: Prepared for Phase 2 - Token renewal background worker
func (s *InMemoryStore) GetSessionsNeedingRenewal() []*WebSession {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var needsRenewal []*WebSession
	now := time.Now()

	for _, session := range s.sessions {
		// Skip expired sessions
		if now.After(session.ExpiresAt) {
			continue
		}

		// Check if token needs renewal
		if now.After(session.TokenRenewalAt) && now.Before(session.TokenExpiresAt) {
			needsRenewal = append(needsRenewal, session)
		}
	}

	return needsRenewal
}

// DeleteExpired removes all expired sessions
func (s *InMemoryStore) DeleteExpired() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	deleted := 0

	for id, session := range s.sessions {
		if now.After(session.ExpiresAt) {
			delete(s.sessions, id)
			deleted++
		}
	}

	if deleted > 0 {
		log.Info().
			Int("count", deleted).
			Msg("Cleaned up expired web sessions")
	}

	return deleted
}

// cleanupWorker runs periodic cleanup of expired sessions
func (s *InMemoryStore) cleanupWorker() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.DeleteExpired()
	}
}

// GenerateSecureSessionID generates a cryptographically secure random session ID
func GenerateSecureSessionID() (string, error) {
	b := make([]byte, 32) // 256 bits
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// Common errors
var (
	ErrSessionNotFound = &Error{Code: "session_not_found", Message: "Session not found"}
	ErrSessionExpired  = &Error{Code: "session_expired", Message: "Session has expired"}
)

// Error represents a session-related error
type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string {
	return e.Message
}
