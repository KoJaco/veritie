package auth

import "errors"

var (
	ErrInvalidApiKey     = errors.New("invalid API key")
	ErrRateLimitExceeded = errors.New("rate limit exceeded")
)
