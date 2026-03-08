package session

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"schma.ai/internal/domain/auth"
	"schma.ai/internal/domain/speech"
	db "schma.ai/internal/infra/db/generated"
)

// TODO: get rid of config tacking here and move it to the config watcher.
// the flow --
// 1. session starts and we init the session manager and the session config watcher
// 2. session manager tracks session lifecycle
// 3. config watcher tracks per-session config updates (this may be an issue for receiving config updates when audio is paused??? hmm)
// 5. config watcher flushes the configs to the database on a
// 6. session ends

// WSSessionID represents the WebSocket connection lifecycle
type WSSessionID string

// SessionID represents individual pipeline sessions within a connection
type DBSessionID pgtype.UUID

// SessionStatus tracks the state of a pipeline session
type SessionStatus string

const (
	SessionIdle       SessionStatus = "idle"
	SessionRecording  SessionStatus = "recording"
	SessionProcessing SessionStatus = "processing"
	SessionClosed     SessionStatus = "closed"
)

// Event types for progressing a session
type Event struct {
	Type string
	Data any
}

// TrackedFunctionConfig represents a function configuration at a point in time
type TrackedFunctionConfig struct {
	FunctionDeclarations []speech.FunctionDefinition
	UpdateMs             int
	ParsingGuide         string
	Timestamp            time.Time // When this config was applied
}

// SessionState represents an active pipeline session
type SessionState struct {
	ID           DBSessionID
	WSSessionID  WSSessionID
	Status       SessionStatus
	Principal    auth.Principal
	CreatedAt    time.Time
	LastActivity time.Time

	// Session configuration history
	FunctionConfigs []TrackedFunctionConfig // Multiple configs over time
	CurrentConfig   *TrackedFunctionConfig  // Current active config

	// Schema tracking
	SchemaChecksums map[string]pgtype.UUID // checksum -> schema_id
}

type SessionSnapshot struct {
	ID           DBSessionID
	WSSessionID  WSSessionID
	Status       SessionStatus
	Principal    auth.Principal
	CreatedAt    time.Time
	LastActivity time.Time
}

// Record represents the database session record
type Record struct {
	ID          DBSessionID
	WSSessionID WSSessionID
	AccountID   pgtype.UUID
	AppID       pgtype.UUID
	CreatedAt   pgtype.Timestamp
	ClosedAt    *pgtype.Timestamp
	Status      SessionStatus
}

type PartialUpdate struct {
	Status          *SessionStatus
	LastActivity    *time.Time
	FunctionConfig  *TrackedFunctionConfig
	SchemaChecksums *map[string]pgtype.UUID
}


// Domain interfaces (ports) that infrastructure will implement. Our Session Manager focuses entirely on session lifecycle and state management, NOT data accumulation.
type Manager interface {
	// Session lifecycle within connection
	StartSession(ctx context.Context, wsSessionID WSSessionID, isTest bool, kind db.SessionKindEnum, principal auth.Principal) (*SessionState, error)
	GetSession(sessionID DBSessionID) (*SessionState, bool)

	// Session State Management
	UpdateSessionStatus(ctx context.Context, sessionID DBSessionID, status SessionStatus) error
	CloseSession(ctx context.Context, sessionID DBSessionID) error

	// Session events
	UpdateSessionMetadata(ctx context.Context, sessionID DBSessionID, metadata PartialUpdate) error
	Snapshot(ctx context.Context, id DBSessionID) (SessionSnapshot, error)

	// Principal
	// LookupPrincipalByAppID(ctx context.Context, appID string) (auth.Principal, error)
}
