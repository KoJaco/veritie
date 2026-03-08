package eventlog

import (
	"context"
	"time"
)

type Logger interface {
	Log(ctx context.Context, entry LogEntry) error
}

type LogEntry struct {
	SessionID string
	AppID     string
	Type      string // e.g. "error", "info", "debug", "warning"
	Message   string
	Timestamp time.Time
}
