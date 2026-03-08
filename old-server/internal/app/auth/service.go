package auth

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	domain_auth "schma.ai/internal/domain/auth"
	pkg_limiter "schma.ai/internal/pkg/limiter"
)

type Service struct {
	validator   domain_auth.Validator
	// principalGetter domain_auth.PrincipalGetter
	cache       domain_auth.AppSettingsCache
	rateLimiter domain_auth.RateLimiter
}

var _ domain_auth.AuthService = (*Service)(nil)

func NewService(validator domain_auth.Validator, cache domain_auth.AppSettingsCache, rateLimiter domain_auth.RateLimiter) *Service {
	return &Service{
		validator:   validator,
		cache:       cache,
		rateLimiter: rateLimiter,
	}
}

func (s *Service) AuthenticateRequest(r *http.Request, key string) (domain_auth.Principal, error) {

	// Check cache first
	if principal, ok := s.cache.Get(key); ok {
		return principal, nil
	}

	// Validate token
	principal, err := s.validator.ValidateToken(r.Context(), key)
	if err != nil {
		return domain_auth.Principal{}, err
	}

	// Cache the result
	s.cache.Set(key, principal, 20*time.Second)
	return principal, nil
}

func (s *Service) CheckRateLimit(ctx context.Context, principal domain_auth.Principal) (bool, *pkg_limiter.LimitInfo, error) {
	return s.rateLimiter.Allow(ctx, principal.AppID.String())
}

// GetRateLimitInfo gets rate limit information for the given key
func (s *Service) GetRateLimitInfo(ctx context.Context, key string) (*pkg_limiter.LimitInfo, error) {
	return s.rateLimiter.GetRemaining(ctx, key)
}

func (s *Service) LookupPrincipalByAppID(ctx context.Context, appID pgtype.UUID) (domain_auth.Principal, error) {
	return s.validator.GetPrincipalByAppID(ctx, appID)
}

