package obs

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

// Logger is a thin wrapper around slog to keep logging call sites stable.
type Logger struct {
	inner *slog.Logger
}

// NewLogger initializes a structured logger from a level string.
func NewLogger(level string) (*Logger, error) {
	parsed, err := parseLevel(level)
	if err != nil {
		return nil, err
	}

	// JSON logs keep output machine-readable for CI and future log aggregation.
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: parsed})
	return &Logger{inner: slog.New(handler)}, nil
}

func parseLevel(level string) (slog.Level, error) {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("unsupported log level %q", level)
	}
}

// With returns a logger enriched with key/value attributes.
func (l *Logger) With(attrs ...any) *Logger {
	return &Logger{inner: l.inner.With(attrs...)}
}

func (l *Logger) Debug(msg string, attrs ...any) {
	l.inner.Debug(msg, attrs...)
}

func (l *Logger) Info(msg string, attrs ...any) {
	l.inner.Info(msg, attrs...)
}

func (l *Logger) Warn(msg string, attrs ...any) {
	l.inner.Warn(msg, attrs...)
}

func (l *Logger) Error(msg string, attrs ...any) {
	l.inner.Error(msg, attrs...)
}
