package auth

import "errors"

var (
	ErrRateLimitExceeded = errors.New("rate limit exceeded")
	ErrInvalidAPIKey     = errors.New("invalid API key")
	ErrMissingAPIKey     = errors.New("missing API key")
	ErrUnauthorized      = errors.New("unauthorized")
)
