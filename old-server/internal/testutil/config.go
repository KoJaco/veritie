package testutil

import (
	"os"
	"testing"
)

// TestConfig holds test configuration
type TestConfig struct {
	DatabaseURL string
	STTProvider string
	LLMProvider string
}

// LoadTestConfig loads test configuration from environment
func LoadTestConfig(t *testing.T) TestConfig {
	return TestConfig{
		DatabaseURL: getEnvOrDefault(t, "TEST_DATABASE_URL", "postgres://test:test@localhost:5432/schma_test?sslmode=disable"),
		STTProvider: getEnvOrDefault(t, "TEST_STT_PROVIDER", "mock"),
		LLMProvider: getEnvOrDefault(t, "TEST_LLM_PROVIDER", "mock"),
	}
}

func getEnvOrDefault(t *testing.T, key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
