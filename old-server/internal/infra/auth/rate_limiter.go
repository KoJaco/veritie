package auth

import (
	"context"
	"time"

	"github.com/ulule/limiter/v3"
	"github.com/ulule/limiter/v3/drivers/store/memory"
	pkg_limiter "schma.ai/internal/pkg/limiter"
)

type RateLimiter struct {
	store   limiter.Store
	limiter *limiter.Limiter
}

// NewRateLimiter creates a new rate limiter with a memory store and a given limit
func NewRateLimiter(limit int64) *RateLimiter {
	store := memory.NewStore()
	limiter := limiter.New(store, limiter.Rate{
		Period: 1 * time.Minute,
		Limit:  limit,
	})
	return &RateLimiter{store: store, limiter: limiter}
}

// Allow checks if the key is within rate limits
func (r *RateLimiter) Allow(ctx context.Context, key string) (bool, *pkg_limiter.LimitInfo, error) {
	context, err := r.limiter.Get(ctx, key)
	if err != nil {
		return false, nil, err
	}

	limitInfo := &pkg_limiter.LimitInfo{
		Limit:     context.Limit,
		Remaining: context.Remaining,
		Reset:     context.Reset,
		Reached:   context.Reached,
	}

	return !context.Reached, limitInfo, nil
}

// GetRemaining returns remaining requests for the key
func (r *RateLimiter) GetRemaining(ctx context.Context, key string) (*pkg_limiter.LimitInfo, error) {
	context, err := r.limiter.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	return &pkg_limiter.LimitInfo{
		Limit:     context.Limit,
		Remaining: context.Remaining,
		Reset:     context.Reset,
		Reached:   context.Reached,
	}, nil
}
