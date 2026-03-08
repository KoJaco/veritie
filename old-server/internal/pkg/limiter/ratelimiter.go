package limiter

import (
	"context"
	"time"
)

// RateLimiter defines the core rate limiting interface
type RateLimiter interface {
	Allow(ctx context.Context, key string) (bool, *LimitInfo, error)
	GetRemaining(ctx context.Context, key string) (*LimitInfo, error)
}

// LimitInfo contains rate limit information
type LimitInfo struct {
	Limit     int64
	Remaining int64
	Reset     int64
	Reached   bool
}

// Rate defines the rate limit configuration
type Rate struct {
	Period time.Duration
	Limit  int64
}

// NewRate creates a new rate configuration
func NewRate(period time.Duration, limit int64) Rate {
	return Rate{
		Period: period,
		Limit:  limit,
	}
}
