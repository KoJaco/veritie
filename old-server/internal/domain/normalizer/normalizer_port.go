package normalizer

import "context"






type Normalizer interface {
	// Normalize a single string
	Normalize(ctx context.Context, text string) (normalized string, err error)

	// Normalize a batch of strings
	NormalizeBatch(ctx context.Context, texts []string) (normalized []string, err error)

	// Health probe
	Healthy(ctx context.Context) bool
}