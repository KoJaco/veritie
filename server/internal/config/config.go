package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds runtime configuration for a process.
type Config struct {
	Service   string
	App       AppConfig
	Database  DatabaseConfig
	Providers ProvidersConfig
	Obs       ObservabilityConfig
}

// AppConfig contains process-level runtime flags.
type AppConfig struct {
	Name              string
	Mode              string
	HTTPPort          int
	WorkerConcurrency int
}

// DatabaseConfig holds persistence settings.
type DatabaseConfig struct {
	DSN string
}

// ProvidersConfig holds provider selection and credentials.
type ProvidersConfig struct {
	STTProvider              string
	LLMProvider              string
	DeepgramAPIKey           string
	SpeechmaticsAPIKey       string
	GeminiAPIKey             string
	OpenAIAPIKey             string
	DisableProviderKeyChecks bool
}

// ObservabilityConfig controls logging/metrics/tracing behavior.
type ObservabilityConfig struct {
	LogLevel                string
	TracingEnabled          bool
	TracingExporterEndpoint string
}

// LoadFromEnv reads process config from environment with safe defaults.
func LoadFromEnv(service string) (Config, error) {
	// Parse strongly typed env values first so we fail early on bad process config.
	httpPort, err := envInt("HTTP_PORT", 8080)
	if err != nil {
		return Config{}, err
	}

	workerConcurrency, err := envInt("WORKER_CONCURRENCY", 4)
	if err != nil {
		return Config{}, err
	}

	tracingEnabled, err := envBool("TRACING_ENABLED", false)
	if err != nil {
		return Config{}, err
	}

	disableProviderKeyChecks, err := envBool("DISABLE_PROVIDER_KEY_CHECKS", false)
	if err != nil {
		return Config{}, err
	}

	// Assemble full runtime config with sane defaults for local/dev execution.
	cfg := Config{
		Service: strings.ToLower(strings.TrimSpace(service)),
		App: AppConfig{
			Name:              envString("APP_NAME", "veritie"),
			Mode:              strings.ToLower(envString("APP_MODE", "development")),
			HTTPPort:          httpPort,
			WorkerConcurrency: workerConcurrency,
		},
		Database: DatabaseConfig{
			DSN: os.Getenv("DATABASE_DSN"),
		},
		Providers: ProvidersConfig{
			STTProvider:              strings.ToLower(envString("STT_PROVIDER", "deepgram")),
			LLMProvider:              strings.ToLower(envString("LLM_PROVIDER", "gemini")),
			DeepgramAPIKey:           os.Getenv("DEEPGRAM_API_KEY"),
			SpeechmaticsAPIKey:       os.Getenv("SPEECHMATICS_API_KEY"),
			GeminiAPIKey:             os.Getenv("GEMINI_API_KEY"),
			OpenAIAPIKey:             os.Getenv("OPENAI_API_KEY"),
			DisableProviderKeyChecks: disableProviderKeyChecks,
		},
		Obs: ObservabilityConfig{
			LogLevel:                strings.ToLower(envString("LOG_LEVEL", "info")),
			TracingEnabled:          tracingEnabled,
			TracingExporterEndpoint: os.Getenv("TRACING_EXPORTER_ENDPOINT"),
		},
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func envString(key, fallback string) string {
	// Empty values intentionally fall back to defaults to keep env setup minimal.
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) (int, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid integer for %s: %w", key, err)
	}
	return parsed, nil
}

func envBool(key string, fallback bool) (bool, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("invalid bool for %s: %w", key, err)
	}
	return parsed, nil
}
