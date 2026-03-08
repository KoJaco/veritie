package integration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	app_auth "schma.ai/internal/app/auth"
	domain_auth "schma.ai/internal/domain/auth"
	infra_auth "schma.ai/internal/infra/auth"
	http_middleware "schma.ai/internal/transport/http/middleware"
)

// Mock validator for integration tests
type MockValidator struct {
	mock.Mock
}

func (m *MockValidator) ValidateToken(ctx context.Context, token string) (domain_auth.Principal, error) {
	args := m.Called(ctx, token)
	return args.Get(0).(domain_auth.Principal), args.Error(1)
}

func (m *MockValidator) GetPrincipalByAppID(ctx context.Context, appID pgtype.UUID) (domain_auth.Principal, error) {
	args := m.Called(ctx, appID)
	return args.Get(0).(domain_auth.Principal), args.Error(1)
}

func TestAuthIntegration(t *testing.T) {
	// Setup real infrastructure components
	cache, err := infra_auth.NewAppSettingsCache(100)
	assert.NoError(t, err)

	rateLimiter := infra_auth.NewRateLimiter(10)

	mockValidator := new(MockValidator)

	// Setup real service
	authService := app_auth.NewService(mockValidator, cache, rateLimiter)

	principal := domain_auth.Principal{
		AppID:     pgtype.UUID{Bytes: [16]byte{1, 2, 3, 4}, Valid: true},
		AccountID: pgtype.UUID{Bytes: [16]byte{5, 6, 7, 8}, Valid: true},
		AppName:   "Integration Test App",
	}

	mockValidator.On("ValidateToken", mock.Anything, "valid-key").
		Return(principal, nil)

	// Create middleware
	middleware := http_middleware.KeyAuthMiddleware(authService)

	t.Run("successful request", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("x-api-key", "valid-key")

		w := httptest.NewRecorder()

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := http_middleware.GetPrincipal(r.Context())
			if ok {
				assert.Equal(t, "Integration Test App", principal.AppName)
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("success"))
			} else {
				w.WriteHeader(http.StatusInternalServerError)
			}
		})

		middleware(handler).ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "success", w.Body.String())
		assert.Contains(t, w.Header().Get("X-RateLimit-Remaining"), "9")
	})

	t.Run("cached request", func(t *testing.T) {
		// Second request should use cache
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("x-api-key", "valid-key")

		w := httptest.NewRecorder()

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := http_middleware.GetPrincipal(r.Context())
			if ok {
				assert.Equal(t, "Integration Test App", principal.AppName)
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("success"))
			} else {
				w.WriteHeader(http.StatusInternalServerError)
			}
		})

		middleware(handler).ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "success", w.Body.String())

		// Should only call validator once (first request)
		mockValidator.AssertNumberOfCalls(t, "ValidateToken", 1)
	})

	t.Run("rate limiting", func(t *testing.T) {
		// Make multiple requests to test rate limiting
		for i := 0; i < 10; i++ {
			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("x-api-key", "rate-limit-key")

			w := httptest.NewRecorder()

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			middleware(handler).ServeHTTP(w, req)

			if i < 9 {
				assert.Equal(t, http.StatusOK, w.Code)
			} else {
				// 10th request should be rate limited
				assert.Equal(t, http.StatusTooManyRequests, w.Code)
			}
		}
	})
}
