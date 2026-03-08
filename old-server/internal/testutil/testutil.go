package testutil

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"schma.ai/internal/domain/auth"
)

// TestDB creates a test database connection
func TestDB(t *testing.T) *pgxpool.Pool {
	dsn := "postgres://test:test@localhost:5432/schma_test?sslmode=disable"

	pool, err := pgxpool.New(context.Background(), dsn)
	require.NoError(t, err)

	// Ping to ensure connection works
	err = pool.Ping(context.Background())
	require.NoError(t, err)

	t.Cleanup(func() {
		pool.Close()
	})

	return pool
}

// CreateTestApp creates a test app in the database
func CreateTestApp(t *testing.T, pool *pgxpool.Pool) (string, string) {
	// Implementation to create test app and return API key
	// This would insert into the apps table and return the generated API key
	return "test-api-key", "test-app-id"
}

// CleanupTestData cleans up test data
func CleanupTestData(t *testing.T, pool *pgxpool.Pool) {
	// Clean up any test data created during tests
	_, err := pool.Exec(context.Background(), "DELETE FROM usage_logs WHERE app_id IN (SELECT id FROM apps WHERE name LIKE 'test-%')")
	require.NoError(t, err)

	_, err = pool.Exec(context.Background(), "DELETE FROM apps WHERE name LIKE 'test-%'")
	require.NoError(t, err)
}

// CreateTestPrincipal creates a test principal
func CreateTestPrincipal(t *testing.T) auth.Principal {
	return auth.Principal{
		AppID:     pgtype.UUID{Bytes: [16]byte{1, 2, 3, 4}, Valid: true},
		AccountID: pgtype.UUID{Bytes: [16]byte{5, 6, 7, 8}, Valid: true},
		AppName:   "Test App",
		AppConfig: auth.AppConfig{
			AllowedOrigins: []string{"https://example.com"},
			PreferredLLM:   "gemini-2.0-flash",
		},
	}
}

// CreateTestAppInfo creates test app info
func CreateTestAppInfo(t *testing.T) auth.AppInfo {
	return auth.AppInfo{
		AppID:     pgtype.UUID{Bytes: [16]byte{1, 2, 3, 4}, Valid: true},
		AccountID: pgtype.UUID{Bytes: [16]byte{5, 6, 7, 8}, Valid: true},
		Name:      "Test App",
		UsageLimits: auth.UsageLimits{
			MaxSessionsPerMinute:  10,
			MaxLLMTokensPerMin:    1000,
			MaxAudioSecondsPerMin: 300,
		},
	}
}

// AssertPrincipalEqual asserts that two principals are equal
func AssertPrincipalEqual(t *testing.T, expected, actual auth.Principal) {
	require.Equal(t, expected.AppName, actual.AppName)
	require.Equal(t, expected.AppID, actual.AppID)
	require.Equal(t, expected.AccountID, actual.AccountID)
	require.Equal(t, expected.AppConfig.PreferredLLM, actual.AppConfig.PreferredLLM)
}
