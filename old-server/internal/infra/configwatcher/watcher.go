package configwatcher

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"schma.ai/internal/domain/configwatcher"
	"schma.ai/internal/domain/speech"
	db "schma.ai/internal/infra/db/generated"
	"schma.ai/internal/pkg/logger"
)

// Watcher is a component that tracks config changes for a session. A session in this context is an audio_start -> audio_stop cycle, a pipeline run, or a db session.

type watcher struct {
	queries *db.Queries
	state   *configwatcher.ConfigState
	mu      sync.RWMutex
}

// Inits an empty-state watcher with attached queries
func New(queries *db.Queries, appID pgtype.UUID) configwatcher.ConfigWatcher {
	return &watcher{
		queries: queries,
		state:   &configwatcher.ConfigState{},
	}
}

// helper: calculateChecksum generates a SHA256 checksum for the prompt content
func calculateChecksum(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}

// WatchSession is called with a new DB session
func (w *watcher) WatchSession(ctx context.Context, sessionID pgtype.UUID, initialConfig speech.FunctionConfig) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Convert to FunctionConfigWithoutContext to exclude PrevContext from checksum
	configWithoutContext := speech.FunctionConfigWithoutContext{
		Name:               initialConfig.Name,
		Description:        initialConfig.Description,
		ParsingConfig:    initialConfig.ParsingConfig,
		UpdateMs:  initialConfig.UpdateMs,
		Declarations:       initialConfig.Declarations,
		ParsingGuide:       initialConfig.ParsingGuide,
	}

	// Convert config declarations and parsing guide to a string to generate checksum
	configBytes, err := json.Marshal(configWithoutContext)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	checksum := configwatcher.ConfigChecksum(calculateChecksum(string(configBytes)))

	// Create a new tracked config
	initialTrackedConfig := configwatcher.TrackedConfig{
		Config:    &initialConfig,
		Checksum:  checksum,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Initialise watcher state for this session
	w.state = &configwatcher.ConfigState{
		SessionID:          sessionID,
		CurrentConfigIndex: checksum,
		TrackedConfigs:     make(map[configwatcher.ConfigChecksum]configwatcher.TrackedConfig),
		LastUpdate:         time.Now(),
	}

	w.state.TrackedConfigs[checksum] = initialTrackedConfig

	return nil
}

// TODO: How am I effectively taring down the watcher? This should maybe trigger flush?
func (w *watcher) UnwatchSession(ctx context.Context, sessionID pgtype.UUID, appID pgtype.UUID) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Flush session configs before cleanup
	if w.state != nil && w.state.SessionID == sessionID {
		if err := w.flushSessionConfigs(ctx, sessionID, appID); err != nil {
			return fmt.Errorf("failed to flush session configs: %w", err)
		}
	}
	// Clear state
	w.state = nil

	return nil
}

// CRUD - internal
func (w *watcher) UpdateSessionConfig(ctx context.Context, config speech.FunctionConfig) (configwatcher.ConfigChecksum, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.state == nil {
		return "", fmt.Errorf("no session to update")
	}

	// Convert to FunctionConfigWithoutContext to exclude PrevContext from checksum
	configWithoutContext := speech.FunctionConfigWithoutContext{
		Name:               config.Name,
		Description:        config.Description,
		ParsingConfig:    config.ParsingConfig,
		UpdateMs:  config.UpdateMs,
		Declarations:       config.Declarations,
		ParsingGuide:       config.ParsingGuide,
	}

	configBytes, err := json.Marshal(configWithoutContext)
	if err != nil {
		return "", fmt.Errorf("failed to marshal config: %w", err)
	}

	newChecksum := configwatcher.ConfigChecksum(calculateChecksum(string(configBytes)))

	// check if the checksum exists
	if _, exists := w.state.TrackedConfigs[newChecksum]; exists {
		// Update tracked config
		foundConfig := w.state.TrackedConfigs[newChecksum]
		foundConfig.UpdatedAt = time.Now()
		w.state.TrackedConfigs[newChecksum] = foundConfig
		// update state and return index
		w.state.CurrentConfigIndex = newChecksum
		w.state.LastUpdate = time.Now()
		return newChecksum, nil
	}

	trackedConfig := configwatcher.TrackedConfig{
		Config:    &config,
		Checksum:  newChecksum,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	w.state.TrackedConfigs[newChecksum] = trackedConfig
	w.state.CurrentConfigIndex = newChecksum
	w.state.LastUpdate = time.Now()

	return newChecksum, nil
}

func (w *watcher) GetCurrentConfig(ctx context.Context) (*configwatcher.TrackedConfig, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	if w.state == nil {
		return nil, fmt.Errorf("no active session, no state")
	}

	currentConfig, exists := w.state.TrackedConfigs[w.state.CurrentConfigIndex]

	if !exists {
		return nil, fmt.Errorf("current config not found")
	}

	return &currentConfig, nil
}

// FlushSessionConfigs stores all tracked configs to database
func (w *watcher) FlushSessionConfigs(ctx context.Context, sessionID pgtype.UUID, appID pgtype.UUID) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.flushSessionConfigs(ctx, sessionID, appID)
}

// Update flushSessionConfigs
func (w *watcher) flushSessionConfigs(ctx context.Context, sessionID pgtype.UUID, appID pgtype.UUID) error {
	if w.state == nil || w.state.SessionID != sessionID {
		return fmt.Errorf("session %v not being watched", sessionID)
	}

	for checksum, tracked := range w.state.TrackedConfigs {
		if tracked.ConfigID.Valid {
			continue // already stored
		}

		// Convert to FunctionConfigWithoutContext for storage (excludes PrevContext)
		configWithoutContext := speech.FunctionConfigWithoutContext{
			Name:               tracked.Config.Name,
			Description:        tracked.Config.Description,
			ParsingConfig:    tracked.Config.ParsingConfig,
			UpdateMs:  tracked.Config.UpdateMs,
			Declarations:       tracked.Config.Declarations,
			ParsingGuide:       tracked.Config.ParsingGuide,
		}

		// Marshal declarations to JSON for storage
		declarationsBytes, err := json.Marshal(configWithoutContext.Declarations)
		if err != nil {
			return fmt.Errorf("failed to marshal declarations for config %s: %w", checksum, err)
		}

		// Store the entire function config using the new schema structure
		functionSchemaID, err := w.queries.InsertFunctionSchemaIfNotExists(ctx, db.InsertFunctionSchemaIfNotExistsParams{
			AppID:         appID,
			SessionID:     sessionID,
			Name:          pgtype.Text{String: configWithoutContext.Name, Valid: configWithoutContext.Name != ""},
			Description:   pgtype.Text{String: configWithoutContext.Description, Valid: configWithoutContext.Description != ""},
			ParsingGuide:  pgtype.Text{String: configWithoutContext.ParsingGuide, Valid: configWithoutContext.ParsingGuide != ""},
			UpdateMs:      pgtype.Int4{Int32: int32(configWithoutContext.UpdateMs), Valid: configWithoutContext.UpdateMs > 0},
			ParsingStrategy: db.SchemaParsingStrategyEnum(configWithoutContext.ParsingConfig.ParsingStrategy),
			Declarations:  declarationsBytes,
			Checksum:      string(checksum),
			CreatedAt:     pgtype.Timestamp{Time: time.Now(), Valid: true},
		})
		if err != nil {
			return fmt.Errorf("failed to store function config %s: %w", checksum, err)
		}

		// Link schema to session
		err = w.queries.LinkFunctionSchemaToSession(ctx, db.LinkFunctionSchemaToSessionParams{
			SessionID:        sessionID,
			FunctionSchemaID: functionSchemaID,
		})
		if err != nil {
			return fmt.Errorf("failed to link function config %s to session: %w", functionSchemaID, err)
		}

		// Mark as stored in our state
		tracked.ConfigID = pgtype.UUID{
			Bytes: functionSchemaID.Bytes,
			Valid: true,
		}
		w.state.TrackedConfigs[checksum] = tracked
	}
	logger.ServiceDebugf("CONFIG_WATCHER", "Flushed %d configs for session %v", len(w.state.TrackedConfigs), sessionID)
	return nil
}
