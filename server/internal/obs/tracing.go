package obs

import (
	"context"
	"fmt"

	"veritie.io/internal/config"
)

// Tracing provides tracing initialization/shutdown hooks.
type Tracing interface {
	Shutdown(ctx context.Context) error
	Enabled() bool
}

type noopTracing struct {
	enabled bool
}

// InitTracing sets up tracing hooks. In branch 05 this is a no-op backend.
func InitTracing(cfg config.ObservabilityConfig, logger *Logger) (Tracing, error) {
	if !cfg.TracingEnabled {
		logger.Info("tracing disabled")
		return noopTracing{enabled: false}, nil
	}

	if cfg.TracingExporterEndpoint == "" {
		return nil, fmt.Errorf("tracing exporter endpoint must be set when tracing is enabled")
	}

	// Branch 05 keeps tracing backend as a no-op but validates config and lifecycle hooks.
	logger.Info("tracing enabled")
	return noopTracing{enabled: true}, nil
}

func (n noopTracing) Shutdown(context.Context) error { return nil }

func (n noopTracing) Enabled() bool { return n.enabled }
