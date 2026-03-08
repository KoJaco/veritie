package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	domain_auth "schma.ai/internal/domain/auth"
)

// MemorySessionManager implements SessionManager with in-memory storage
type MemorySessionManager struct {
	sessions map[string]*domain_auth.WSSession
	mu       sync.RWMutex
	ttl      time.Duration
}

// NewMemorySessionManager creates a new in-memory session manager
func NewMemorySessionManager(ttl time.Duration) *MemorySessionManager {
	manager := &MemorySessionManager{
		sessions: make(map[string]*domain_auth.WSSession),
		ttl:      ttl,
	}

	// Start cleanup goroutine
	go manager.cleanupExpiredSessions()

	return manager
}

// CreateSession creates a new session for the given principal
func (m *MemorySessionManager) CreateSession(ctx context.Context, principal domain_auth.Principal) (*domain_auth.WSSession, error) {
	sessionToken, err := generateSessionToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate session token: %w", err)
	}

	now := time.Now()
	session := &domain_auth.WSSession{
		ID:        sessionToken,
		AppID:     principal.AppID,
		AccountID: principal.AccountID,
		Principal: principal,
		CreatedAt: now,
		ExpiresAt: now.Add(m.ttl),
	}

	m.mu.Lock()
	m.sessions[sessionToken] = session
	m.mu.Unlock()

	return session, nil
}

// ValidateSession validates a session token and returns the session if valid
func (m *MemorySessionManager) ValidateSession(ctx context.Context, sessionToken string) (*domain_auth.WSSession, error) {
	m.mu.RLock()
	session, exists := m.sessions[sessionToken]
	m.mu.RUnlock()

	if !exists {
		return nil, domain_auth.ErrInvalidApiKey // Reuse existing error
	}

	// Check if session is expired
	if time.Now().After(session.ExpiresAt) {
		// Remove expired session
		m.mu.Lock()
		delete(m.sessions, sessionToken)
		m.mu.Unlock()
		return nil, domain_auth.ErrInvalidApiKey
	}

	return session, nil
}

// RevokeSession revokes a session
func (m *MemorySessionManager) RevokeSession(ctx context.Context, sessionToken string) error {
	m.mu.Lock()
	delete(m.sessions, sessionToken)
	m.mu.Unlock()
	return nil
}

// cleanupExpiredSessions periodically removes expired sessions
func (m *MemorySessionManager) cleanupExpiredSessions() {
	ticker := time.NewTicker(5 * time.Minute) // Cleanup every 5 minutes
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		m.mu.Lock()

		for token, session := range m.sessions {
			if now.After(session.ExpiresAt) {
				delete(m.sessions, token)
			}
		}

		m.mu.Unlock()
	}
}

// generateSessionToken generates a cryptographically secure random session token
func generateSessionToken() (string, error) {
	bytes := make([]byte, 32) // 256-bit token
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
