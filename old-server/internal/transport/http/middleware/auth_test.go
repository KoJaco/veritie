package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	domain_auth "schma.ai/internal/domain/auth"
	"schma.ai/internal/pkg/limiter"
)

// Mock auth service
type MockAuthService struct {
	mock.Mock
}

// Make sure your mock methods match the interface exactly
func (m *MockAuthService) AuthenticateRequest(req *http.Request, apiKey string) (domain_auth.Principal, error) {
	args := m.Called(req, apiKey)
	return args.Get(0).(domain_auth.Principal), args.Error(1)
}

func (m *MockAuthService) CheckRateLimit(ctx context.Context, principal domain_auth.Principal) (bool, *limiter.LimitInfo, error) {
	args := m.Called(ctx, principal.AppID.String())
	return args.Bool(0), args.Get(1).(*limiter.LimitInfo), args.Error(2)
}

func (m *MockAuthService) GetRateLimitInfo(ctx context.Context, key string) (*limiter.LimitInfo, error) {
	args := m.Called(ctx, key)
	return args.Get(0).(*limiter.LimitInfo), args.Error(1)
}

func (m *MockAuthService) LookupPrincipalByAppID(ctx context.Context, appID pgtype.UUID) (domain_auth.Principal, error) {
	args := m.Called(ctx, appID)
	return args.Get(0).(domain_auth.Principal), args.Error(1)
}

func TestKeyAuthMiddleware(t *testing.T) {
	tests := []struct {
		name             string
		apiKey           string
		mockPrincipal    domain_auth.Principal
		mockError        error
		rateLimitAllowed bool
		rateLimitInfo    *limiter.LimitInfo
		rateLimitError   error
		expectedStatus   int
		expectedBody     string
		expectHeaders    map[string]string
	}{
		{
			name:   "valid request",
			apiKey: "valid-key",
			mockPrincipal: domain_auth.Principal{
				AppID:     pgtype.UUID{Bytes: [16]byte{1, 2, 3, 4}, Valid: true},
				AccountID: pgtype.UUID{Bytes: [16]byte{5, 6, 7, 8}, Valid: true},
				AppName:   "Test App",
			},
			rateLimitAllowed: true,
			rateLimitInfo: &limiter.LimitInfo{
				Limit:     10,
				Remaining: 9,
				Reset:     1234567890,
			},
			expectedStatus: http.StatusOK,
			expectHeaders: map[string]string{
				"X-RateLimit-Limit":     "10",
				"X-RateLimit-Remaining": "9",
				"X-RateLimit-Reset":     "1234567890",
			},
		},
		{
			name:           "missing api key",
			apiKey:         "",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Unauthorized: Missing API key",
		},
		{
			name:           "invalid api key",
			apiKey:         "invalid-key",
			mockError:      domain_auth.ErrInvalidApiKey,
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Unauthorized: Invalid API key",
		},
		{
			name:   "rate limit exceeded",
			apiKey: "valid-key",
			mockPrincipal: domain_auth.Principal{
				AppID: pgtype.UUID{Bytes: [16]byte{1, 2, 3, 4}, Valid: true},
			},
			rateLimitAllowed: false,
			rateLimitInfo: &limiter.LimitInfo{
				Limit:     10,
				Remaining: 0,
				Reset:     1234567890,
			},
			expectedStatus: http.StatusTooManyRequests,
			expectedBody:   "Rate limit exceeded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock
			mockService := new(MockAuthService)

			if tt.apiKey != "" {
				mockService.On("AuthenticateRequest", mock.Anything, tt.apiKey).
					Return(tt.mockPrincipal, tt.mockError)

				if tt.mockError == nil {
					mockService.On("CheckRateLimit", mock.Anything, tt.mockPrincipal.AppID.String()).
						Return(tt.rateLimitAllowed, tt.rateLimitInfo, tt.rateLimitError)
				}
			}

			// Create middleware
			middleware := KeyAuthMiddleware(mockService)

			// Create test request
			req := httptest.NewRequest("GET", "/test", nil)
			if tt.apiKey != "" {
				req.Header.Set("x-api-key", tt.apiKey)
			}

			// Create response recorder
			w := httptest.NewRecorder()

			// Create handler that checks if principal is in context
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				principal, ok := GetPrincipal(r.Context())
				if ok {
					assert.Equal(t, tt.mockPrincipal.AppName, principal.AppName)
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("success"))
				} else {
					w.WriteHeader(http.StatusInternalServerError)
				}
			})

			// Execute middleware
			middleware(handler).ServeHTTP(w, req)

			// Assertions
			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedBody != "" {
				assert.Contains(t, w.Body.String(), tt.expectedBody)
			}

			// Check headers
			for key, value := range tt.expectHeaders {
				assert.Equal(t, value, w.Header().Get(key))
			}

			// Verify mock expectations
			mockService.AssertExpectations(t)
		})
	}
}

func TestGetPrincipal(t *testing.T) {
	principal := domain_auth.Principal{
		AppName: "Test App",
	}

	ctx := context.WithValue(context.Background(), "principal", principal)

	retrieved, ok := GetPrincipal(ctx)
	assert.True(t, ok)
	assert.Equal(t, principal.AppName, retrieved.AppName)

	// Test with no principal
	emptyCtx := context.Background()
	_, ok = GetPrincipal(emptyCtx)
	assert.False(t, ok)
}
