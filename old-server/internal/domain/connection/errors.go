package connection

import "errors"

// Connection-related errors
var (
	ErrConnectionNotFound     = errors.New("connection not found")
	ErrConnectionPoolFull     = errors.New("connection pool is full")
	ErrConnectionClosed       = errors.New("connection is closed")
	ErrConnectionTimeout      = errors.New("connection timeout")
	ErrInvalidConnectionState = errors.New("invalid connection state")
	ErrConnectionLimitExceeded = errors.New("connection limit exceeded")
	ErrConnectionNotActive    = errors.New("connection is not active")
	ErrConnectionServiceNotStarted = errors.New("connection service not started")
)
