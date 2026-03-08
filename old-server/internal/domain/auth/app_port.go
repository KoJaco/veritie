package auth

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

type AppInfo struct {
	AppID     pgtype.UUID
	AccountID pgtype.UUID

	Name      string
	CreatedAt pgtype.Timestamp
	UpdatedAt pgtype.Timestamp

	// TODO: refactored to suit
	UsageLimits UsageLimits
}

// TODO: refactored to suit database design
type UsageLimits struct {
	MaxSessionsPerMinute  int
	MaxLLMTokensPerMin    int
	MaxAudioSecondsPerMin int
	// max concurrent sessions?
}

// Implemented by infra/db/repo and used by app/session. This will be the 2nd hit to the DB after auth_port.Validate. Building out session context using this our AppInfo returned by the AppFetcher
type AppFetcher interface {
	// Validates the given API key and returns and authorized app.
	// returns ErrInvalidApiKey if not found or revoked.
	FetchAppForAPIKey(ctx context.Context, apiKey string) (AppInfo, error)

	// FetchApp retrieves config + limits for an app ID
	FetchAppByID(ctx context.Context, appID pgtype.UUID) (AppInfo, error)
}
