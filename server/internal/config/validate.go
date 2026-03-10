package config

import (
	"fmt"
	"strings"
)

// Validate checks Config values for correctness.
func (c Config) Validate() error {
	if c.Service == "" {
		return fmt.Errorf("service must be set")
	}

	switch c.App.Mode {
	case "development", "test", "staging", "production":
	default:
		return fmt.Errorf("invalid app mode %q", c.App.Mode)
	}

	if c.App.Name == "" {
		return fmt.Errorf("app name must be set")
	}

	if c.App.HTTPPort <= 0 || c.App.HTTPPort > 65535 {
		return fmt.Errorf("http port must be between 1 and 65535")
	}

	if c.App.WorkerConcurrency <= 0 {
		return fmt.Errorf("worker concurrency must be greater than 0")
	}

	if c.Database.DSN == "" {
		return fmt.Errorf("database dsn must be set")
	}

	switch c.Providers.STTProvider {
	case "deepgram", "speechmatics":
	default:
		return fmt.Errorf("invalid stt provider %q", c.Providers.STTProvider)
	}

	switch c.Providers.LLMProvider {
	case "gemini", "openai":
	default:
		return fmt.Errorf("invalid llm provider %q", c.Providers.LLMProvider)
	}

	switch strings.ToLower(c.Obs.LogLevel) {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("invalid log level %q", c.Obs.LogLevel)
	}

	if c.Obs.TracingEnabled && c.Obs.TracingExporterEndpoint == "" {
		return fmt.Errorf("tracing exporter endpoint must be set when tracing is enabled")
	}

	// Useful for local scaffolding while providers are not yet fully wired.
	if c.Providers.DisableProviderKeyChecks {
		if c.App.Mode == "staging" || c.App.Mode == "production" {
			return fmt.Errorf("disable provider key checks is not allowed in %s mode", c.App.Mode)
		}
		return nil
	}

	// Enforce only the selected provider credentials to keep config strict but focused.
	switch c.Providers.STTProvider {
	case "deepgram":
		if c.Providers.DeepgramAPIKey == "" {
			return fmt.Errorf("deepgram api key must be set when stt provider is deepgram")
		}
	case "speechmatics":
		if c.Providers.SpeechmaticsAPIKey == "" {
			return fmt.Errorf("speechmatics api key must be set when stt provider is speechmatics")
		}
	}

	switch c.Providers.LLMProvider {
	case "gemini":
		if c.Providers.GeminiAPIKey == "" {
			return fmt.Errorf("gemini api key must be set when llm provider is gemini")
		}
	case "openai":
		if c.Providers.OpenAIAPIKey == "" {
			return fmt.Errorf("openai api key must be set when llm provider is openai")
		}
	}

	return nil
}
