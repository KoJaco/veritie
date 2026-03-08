package auth

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// Session represents an authenticated websocket session
type WSSession struct {
	ID        string
	AppID     pgtype.UUID
	AccountID pgtype.UUID
	Principal Principal
	CreatedAt time.Time
	ExpiresAt time.Time
}

// SessionManager handles session creation and validation
type WSSessionManager interface {
	CreateSession(ctx context.Context, principal Principal) (*WSSession, error)
	ValidateSession(ctx context.Context, sessionToken string) (*WSSession, error)
	RevokeSession(ctx context.Context, sessionToken string) error
}

// WSSessionStore handles session persistence
type WSSessionStore interface {
	Store(ctx context.Context, session *WSSession) error
	Get(ctx context.Context, sessionToken string) (*WSSession, error)
	Delete(ctx context.Context, sessionToken string) error
	Cleanup(ctx context.Context) error // Remove expired sessions
}

// AuthRequest represents the HTTP auth request
type AuthRequest struct {
	APIKey string `json:"api_key"`
}

// AuthResponse represents the HTTP auth response
type AuthResponse struct {
	WSSessionToken string    `json:"ws_session_token"`
	ExpiresAt      time.Time `json:"expires_at"`
	AppName        string    `json:"app_name"`
}
