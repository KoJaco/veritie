package auth

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
)

/**
* Testing basics
* - Principal
* - AppInfo
* - Domain Errors
**/

func TestPrincipal(t *testing.T) {
	principal := Principal{
		AppID:     pgtype.UUID{Bytes: [16]byte{1, 2, 3, 4}, Valid: true},
		AppName:   "test-app",
		AccountID: pgtype.UUID{Bytes: [16]byte{5, 6, 7, 8}, Valid: true},
		AppConfig: AppConfig{
			AllowedOrigins: []string{"https://example.com"},
			PreferredLLM:   "gemini-2.0-flash",
		},
	}

	assert.True(t, principal.AppID.Valid)
	assert.True(t, principal.AccountID.Valid)
	assert.Equal(t, "test-app", principal.AppName)
	assert.Len(t, principal.AppConfig.AllowedOrigins, 1)
	assert.Equal(t, "gemini-2.0-flash", principal.AppConfig.PreferredLLM)
}

func TestAppInfo(t *testing.T) {
	appInfo := AppInfo{
		AppID:     pgtype.UUID{Bytes: [16]byte{1, 2, 3, 4}, Valid: true},
		AccountID: pgtype.UUID{Bytes: [16]byte{5, 6, 7, 8}, Valid: true},
		Name:      "Test App",
		UsageLimits: UsageLimits{
			MaxSessionsPerMinute:  10,
			MaxLLMTokensPerMin:    1000,
			MaxAudioSecondsPerMin: 300,
		},
	}

	assert.Equal(t, "Test App", appInfo.Name)
	assert.Equal(t, 10, appInfo.UsageLimits.MaxSessionsPerMinute)
	assert.Equal(t, 1000, appInfo.UsageLimits.MaxLLMTokensPerMin)
	assert.Equal(t, 300, appInfo.UsageLimits.MaxAudioSecondsPerMin)
}

func TestDomainErrors(t *testing.T) {
	assert.Equal(t, "invalid API key", ErrInvalidApiKey.Error())
	assert.Equal(t, "rate limit exceeded", ErrRateLimitExceeded.Error())
}
