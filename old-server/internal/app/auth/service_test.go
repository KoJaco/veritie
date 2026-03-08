package auth

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"schma.ai/internal/domain/auth"
	domain_auth "schma.ai/internal/domain/auth"
	"schma.ai/internal/pkg/limiter"
	pkg_limiter "schma.ai/internal/pkg/limiter"
)

// Define mocks

type MockValidator struct {
	mock.Mock
}

func (m *MockValidator) ValidateToken(ctx context.Context, key string) (domain_auth.Principal, error) {
	args := m.Called(ctx, key)
	return args.Get(0).(domain_auth.Principal), args.Error(1)
}

func (m *MockValidator) GetPrincipalByAppID(ctx context.Context, appID pgtype.UUID) (domain_auth.Principal, error) {
	args := m.Called(ctx, appID)
	return args.Get(0).(domain_auth.Principal), args.Error(1)
}

type MockCache struct {
	mock.Mock
}

func (m *MockCache) Get(key string) (domain_auth.Principal, bool) {
	args := m.Called(key)
	return args.Get(0).(domain_auth.Principal), args.Bool(1)
}

func (m *MockCache) Set(key string, principal domain_auth.Principal, expiry time.Duration) {
	m.Called(key, principal, expiry)
}

type MockRateLimiter struct {
	mock.Mock
}

func (m *MockRateLimiter) Allow(ctx context.Context, key string) (bool, *pkg_limiter.LimitInfo, error) {
	args := m.Called(ctx, key)
	return args.Bool(0), args.Get(1).(*pkg_limiter.LimitInfo), args.Error(2)
}

func (m *MockRateLimiter) GetRemaining(ctx context.Context, key string) (*pkg_limiter.LimitInfo, error) {
	args := m.Called(ctx, key)
	return args.Get(0).(*pkg_limiter.LimitInfo), args.Error(1)
}

// Testing

func TestAuthService_AuthenticateRequest(t *testing.T) {
	tests := []struct {
		name               string
		apiKey             string
		cacheHit           bool
		cachedPrincipal    domain_auth.Principal
		validatorPrincipal domain_auth.Principal
		validatorError     error
		expectCache        bool
		expectError        bool
	}{
		{
			name:     "cache hit",
			apiKey:   "cached-key",
			cacheHit: true,
			cachedPrincipal: auth.Principal{
				AppName: "Cached App",
			},
			expectCache: true,
			expectError: false,
		},
		{
			name:     "cache miss, valid token",
			apiKey:   "valid-key",
			cacheHit: false,
			validatorPrincipal: auth.Principal{
				AppName: "Valid App",
			},
			expectCache: true,
			expectError: false,
		},
		{
			name:           "cache miss, invalid token",
			apiKey:         "invalid-key",
			cacheHit:       false,
			validatorError: auth.ErrInvalidApiKey,
			expectCache:    false,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// setup mocks
			mockValidator := new(MockValidator)
			mockCache := new(MockCache)
			mockRateLimiter := new(MockRateLimiter)

			// setup service
			service := NewService(mockValidator, mockCache, mockRateLimiter)

			// Create HTTP request with API key
			req := httptest.NewRequest("GET", "/test", nil)
			if tt.apiKey != "" {
				req.Header.Set("x-api-key", tt.apiKey)
			}

			// setup expecrtations
			mockCache.On("Get", tt.apiKey).Return(tt.cachedPrincipal, tt.cacheHit)

			if !tt.cacheHit {
				mockValidator.On("ValidateToken", mock.Anything, tt.apiKey).Return(tt.validatorPrincipal, tt.validatorError)

				if tt.validatorError == nil {
					mockCache.On("Set", tt.apiKey, tt.validatorPrincipal, 20*time.Second).Return()
				}
			}

			// execute
			principal, err := service.AuthenticateRequest(req, tt.apiKey)

			// Assertions
			if tt.expectError {
				assert.Error(t, err)
				assert.Equal(t, auth.ErrInvalidApiKey, err)
			} else {
				assert.NoError(t, err)
				if tt.cacheHit {
					assert.Equal(t, tt.cachedPrincipal.AppName, principal.AppName)
				} else {
					assert.Equal(t, tt.validatorPrincipal.AppName, principal.AppName)
				}
			}

			// Verify expectations
			mockValidator.AssertExpectations(t)
			mockCache.AssertExpectations(t)
		})
	}
}

func TestAuthService_CheckRateLimit(t *testing.T) {
	mockValidator := new(MockValidator)
	mockCache := new(MockCache)
	mockLimiter := new(MockRateLimiter)

	service := NewService(mockValidator, mockCache, mockLimiter)

	limitInfo := &limiter.LimitInfo{
		Limit:     10,
		Remaining: 5,
		Reset:     1234567890,
		Reached:   false,
	}

	principal := domain_auth.Principal{
		AppID:     pgtype.UUID{Bytes: [16]byte{1, 2, 3, 4}, Valid: true},
		AppName:   "test-app",
		AccountID: pgtype.UUID{Bytes: [16]byte{5, 6, 7, 8}, Valid: true},
		AppConfig: domain_auth.AppConfig{
			AllowedOrigins: []string{"https://example.com"},
			PreferredLLM:   "gemini-2.0-flash",
		},
	}

	mockLimiter.On("Allow", mock.Anything, principal.AppID.String()).
		Return(true, limitInfo, nil)

	allowed, info, err := service.CheckRateLimit(context.Background(), principal)

	assert.NoError(t, err)
	assert.True(t, allowed)
	assert.Equal(t, limitInfo, info)
	assert.Equal(t, int64(10), info.Limit)
	assert.Equal(t, int64(5), info.Remaining)

	mockLimiter.AssertExpectations(t)
}
