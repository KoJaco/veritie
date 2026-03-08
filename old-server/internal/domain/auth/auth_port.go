package auth

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"schma.ai/internal/pkg/limiter"
)

// Principal represents the authenticated identity (an account belonging to a user of role 'owner')
type Principal struct {
	AppID          pgtype.UUID
	AccountID      pgtype.UUID
	AppName        string
	AppDescription string    `json:"app_description,omitempty"`
	AppConfig      AppConfig `json:"app_config,omitempty"`
}

// Mirrors row in DB
type AppConfig struct {
	AllowedOrigins []string
	EnabledSchemas []string
	PreferredLLM   string
	// TODO: Implement these config vars, consolidate with DB schema
}

type AuthService interface {
	AuthenticateRequest(req *http.Request, apiKey string) (Principal, error)
	CheckRateLimit(ctx context.Context, principal Principal) (bool, *limiter.LimitInfo, error)
	GetRateLimitInfo(ctx context.Context, key string) (*limiter.LimitInfo, error)
	LookupPrincipalByAppID(ctx context.Context, appID pgtype.UUID) (Principal, error)
}

// Should be used in auth middleware. This is purely for identity resolution (auth)
type Validator interface {
	// ValidateToken parses and validates the token (opaque API key strat) -> returns the corresponding Principal
	ValidateToken(ctx context.Context, token string) (Principal, error)
	GetPrincipalByAppID(ctx context.Context, appID pgtype.UUID) (Principal, error)
}


// type PrincipalGetter interface {
// 	GetPrincipalByAppID(ctx context.Context, appID pgtype.UUID) (Principal, error)
// }

// Cache port for app settings
type AppSettingsCache interface {
	Get(key string) (Principal, bool)
	Set(key string, principal Principal, expiry time.Duration)
}

// RateLimiter port
type RateLimiter interface {
	Allow(ctx context.Context, key string) (bool, *limiter.LimitInfo, error)
	GetRemaining(ctx context.Context, key string) (*limiter.LimitInfo, error)
}
