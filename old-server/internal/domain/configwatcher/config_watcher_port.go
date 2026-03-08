package configwatcher

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"schma.ai/internal/domain/speech"
)

// TODO: As of now, we are only watching for speech.FunctionConfig changes but this will be expanded to include other config changes (like structuredOutput config)

// ConfigWatcher manages dynamic config updates during a single audio_start -> audio_stop cycle, pipeline run, or database session.
type ConfigWatcher interface {
	// Session management
	WatchSession(ctx context.Context, sessionID pgtype.UUID, initialConfig speech.FunctionConfig) error
	UnwatchSession(ctx context.Context, sessionID pgtype.UUID, appID pgtype.UUID) error

	UpdateSessionConfig(ctx context.Context, config speech.FunctionConfig) (ConfigChecksum, error)
	GetCurrentConfig(ctx context.Context) (*TrackedConfig, error)

	// LLM Session Update Trigger
	FlushSessionConfigs(ctx context.Context, sessionID pgtype.UUID, appID pgtype.UUID) error
}

type ConfigChecksum string

// TrackedConfig represents a config that's been tracked for a session
type TrackedConfig struct {
	Config    *speech.FunctionConfig
	Checksum  ConfigChecksum
	CreatedAt time.Time
	UpdatedAt time.Time
	ConfigID  pgtype.UUID // populated on flush
}

// ConfigState represents the current state of a config for a session
type ConfigState struct {
	SessionID          pgtype.UUID
	CurrentConfigIndex ConfigChecksum
	TrackedConfigs     map[ConfigChecksum]TrackedConfig // checksum -> tracked config
	LastUpdate         time.Time
}
