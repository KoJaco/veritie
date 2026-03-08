package session

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"schma.ai/internal/domain/auth"
	"schma.ai/internal/domain/session"
	"schma.ai/internal/domain/speech"
	db "schma.ai/internal/infra/db/generated"
)

type manager struct {
	queries  *db.Queries
	sessions map[session.DBSessionID]*session.SessionState
	mu       sync.RWMutex
}

func New(queries *db.Queries) session.Manager {
	return &manager{
		queries:  queries,
		sessions: make(map[session.DBSessionID]*session.SessionState),
	}
}

func (m *manager) StartSession(ctx context.Context, wsSessionID session.WSSessionID, isTest bool, kind db.SessionKindEnum, principal auth.Principal) (*session.SessionState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Create database session using existing queries
	dbSession, err := m.queries.CreateSession(ctx, db.CreateSessionParams{
		AppID:     principal.AppID,
		IsTest:    isTest,
		Kind:      kind,
		CreatedAt: pgtype.Timestamp{Time: time.Now(), Valid: true},
		ClosedAt:  pgtype.Timestamp{Valid: false},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Create session state (no pipeline components)
	sessionState := &session.SessionState{
		ID:           session.DBSessionID(dbSession.ID),
		WSSessionID:  wsSessionID,
		Status:       session.SessionIdle,
		Principal:    principal,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
	}

	m.sessions[session.DBSessionID(dbSession.ID)] = sessionState
	return sessionState, nil
}

func (m *manager) GetSession(sessionID session.DBSessionID) (*session.SessionState, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.sessions[sessionID]
	return session, exists
}

func (m *manager) UpdateSessionStatus(ctx context.Context, sessionID session.DBSessionID, status session.SessionStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	sessionState, exists := m.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %v", sessionID)
	}

	sessionState.Status = status
	sessionState.LastActivity = time.Now()

	// Update database if needed
	return nil
}

func (m *manager) CloseSession(ctx context.Context, sessionID session.DBSessionID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, exists := m.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %v", sessionID)
	}

	// Update database using existing queries with current time
	err := m.queries.UpdateSessionClosedAt(ctx, db.UpdateSessionClosedAtParams{
		ClosedAt: pgtype.Timestamp{Time: time.Now(), Valid: true},
		ID:       pgtype.UUID(sessionID),
	})
	if err != nil {
		return fmt.Errorf("failed to mark session closed: %w", err)
	}

	// Remove from memory
	delete(m.sessions, sessionID)
	return nil
}

func (m *manager) UpdateSessionMetadata(ctx context.Context, sessionID session.DBSessionID, metadata session.PartialUpdate) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	sessionState, exists := m.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %v", sessionID)
	}

	// Update session state
	if metadata.Status != nil {
		sessionState.Status = *metadata.Status
	}
	if metadata.LastActivity != nil {
		sessionState.LastActivity = *metadata.LastActivity
	}

	return nil
}

func (m *manager) Snapshot(ctx context.Context, id session.DBSessionID) (session.SessionSnapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessionState, exists := m.sessions[id]
	if !exists {
		return session.SessionSnapshot{}, fmt.Errorf("session not found: %v", id)
	}

	return session.SessionSnapshot{
		ID:           sessionState.ID,
		WSSessionID:  sessionState.WSSessionID,
		Status:       sessionState.Status,
		Principal:    sessionState.Principal,
		CreatedAt:    sessionState.CreatedAt,
		LastActivity: sessionState.LastActivity,
	}, nil
}

func (m *manager) UpdateSessionFunctionConfig(ctx context.Context, sessionID session.DBSessionID, config session.TrackedFunctionConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	sessionState, exists := m.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %v", sessionID)
	}

	// Add timestamp to config
	config.Timestamp = time.Now()

	// Track schemas in memory (will be stored later)
	for _, schema := range config.FunctionDeclarations {
		// Calculate checksum for the schema
		parametersBytes, err := json.Marshal(schema.Parameters)
		if err != nil {
			return fmt.Errorf("failed to marshal parameters for %s: %w", schema.Name, err)
		}
		checksum := fmt.Sprintf("%x", sha256.Sum256(parametersBytes))

		// Store checksum for later database storage
		sessionState.SchemaChecksums[checksum] = pgtype.UUID{} // Will be populated on flush
	}

	// Add to config history
	sessionState.FunctionConfigs = append(sessionState.FunctionConfigs, config)
	sessionState.CurrentConfig = &config
	sessionState.LastActivity = time.Now()

	return nil
}

// FlushSessionSchemas stores all tracked schemas for a session to the database
func (m *manager) FlushSessionSchemas(ctx context.Context, sessionID session.DBSessionID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	sessionState, exists := m.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %v", sessionID)
	}

	// Store all function configs in database using the store-or-get approach
	for _, config := range sessionState.FunctionConfigs {
		// Convert to speech.FunctionConfig
		functionConfig := speech.FunctionConfig{
			ParsingConfig:   speech.ParsingConfig{ParsingStrategy: "auto"},
			UpdateMs: config.UpdateMs,          // Default value since TrackedFunctionConfig doesn't have this
			Declarations:      config.FunctionDeclarations,
			ParsingGuide:      config.ParsingGuide,
		}

		// Use the function schemas repo to store the entire config
		if m.queries != nil {
			// Calculate checksum for the entire config
			configBytes, err := json.Marshal(functionConfig)
			if err != nil {
				continue
			}
			checksum := fmt.Sprintf("%x", sha256.Sum256(configBytes))

			// Marshal declarations to JSON for storage
			declarationsBytes, err := json.Marshal(functionConfig.Declarations)
			if err != nil {
				continue
			}

			description := pgtype.Text{String: "", Valid: false} // No description for whole config
			name := pgtype.Text{String: "", Valid: false}        // No name for whole config
			parsingGuide := pgtype.Text{String: functionConfig.ParsingGuide, Valid: functionConfig.ParsingGuide != ""}
			updateMS := pgtype.Int4{Int32: int32(functionConfig.UpdateMs), Valid: functionConfig.UpdateMs > 0}

			parsingStrategy := db.SchemaParsingStrategyEnum(functionConfig.ParsingConfig.ParsingStrategy)

			schemaID, err := m.queries.InsertFunctionSchemaIfNotExists(ctx, db.InsertFunctionSchemaIfNotExistsParams{
				AppID:         sessionState.Principal.AppID,
				SessionID:     pgtype.UUID(sessionID),
				Name:          name,
				Description:   description,
				ParsingGuide:  parsingGuide,
				UpdateMs:      updateMS,
				ParsingStrategy: parsingStrategy,
				Declarations:  declarationsBytes,
				Checksum:      checksum,
			})

			if err != nil {
				return fmt.Errorf("failed to store function config: %w", err)
			}

			// Store the schema ID for reference
			sessionState.SchemaChecksums[checksum] = schemaID
		}
	}

	return nil
}

func (m *manager) GetCurrentFunctionConfig(sessionID session.DBSessionID) (*session.TrackedFunctionConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessionState, exists := m.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found: %v", sessionID)
	}

	return sessionState.CurrentConfig, nil
}


