package benchmarks

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/mock"
	app_auth "schma.ai/internal/app/auth"
	domain_auth "schma.ai/internal/domain/auth"
	infra_auth "schma.ai/internal/infra/auth"
)

// Mock components for benchmarking
type BenchmarkValidator struct {
	mock.Mock
}

func (b *BenchmarkValidator) ValidateToken(ctx context.Context, token string) (domain_auth.Principal, error) {
	args := b.Called(ctx, token)
	return args.Get(0).(domain_auth.Principal), args.Error(1)
}

func (b *BenchmarkValidator) GetPrincipalByAppID(ctx context.Context, appID pgtype.UUID) (domain_auth.Principal, error) {
	args := b.Called(ctx, appID)
	return args.Get(0).(domain_auth.Principal), args.Error(1)
}

func BenchmarkAuthService_AuthenticateRequest(b *testing.B) {
	mockValidator := new(BenchmarkValidator)
	cache, _ := infra_auth.NewAppSettingsCache(1000)
	rateLimiter := infra_auth.NewRateLimiter(1000)

	req := httptest.NewRequest("GET", "/test", nil)

	principal := domain_auth.Principal{
		AppID:     pgtype.UUID{Bytes: [16]byte{1, 2, 3, 4}, Valid: true},
		AccountID: pgtype.UUID{Bytes: [16]byte{5, 6, 7, 8}, Valid: true},
		AppName:   "Benchmark App",
	}

	mockValidator.On("ValidateToken", mock.Anything, "bench-key").
		Return(principal, nil)

	service := app_auth.NewService(mockValidator, cache, rateLimiter)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := service.AuthenticateRequest(req, "bench-key")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRateLimiter_Allow(b *testing.B) {
	limiter := infra_auth.NewRateLimiter(1000)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := limiter.Allow(ctx, "bench-key")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCache_Get(b *testing.B) {
	cache, _ := infra_auth.NewAppSettingsCache(1000)

	principal := domain_auth.Principal{
		AppName: "Benchmark App",
	}

	cache.Set("bench-key", principal, 20*time.Second)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cache.Get("bench-key")
	}
}
