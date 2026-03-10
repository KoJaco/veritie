package obs

import "context"

// Metrics provides instrumentation hooks used by app/runtime layers.
type Metrics interface {
	IncCounter(name string, value int64, attrs map[string]string)
	ObserveHistogram(name string, value float64, attrs map[string]string)
	Shutdown(ctx context.Context) error
}

// NoopMetrics is a safe default before metrics backends are wired.
type NoopMetrics struct{}

// NewNoopMetrics creates a no-op metrics implementation.
func NewNoopMetrics() Metrics {
	return NoopMetrics{}
}

func (NoopMetrics) IncCounter(string, int64, map[string]string) {}

func (NoopMetrics) ObserveHistogram(string, float64, map[string]string) {}

func (NoopMetrics) Shutdown(context.Context) error { return nil }
