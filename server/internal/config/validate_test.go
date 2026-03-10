package config

import "testing"

func validConfig() Config {
	return Config{
		Service: "api",
		App: AppConfig{
			Name:              "veritie",
			Mode:              "development",
			HTTPPort:          8080,
			WorkerConcurrency: 2,
		},
		Database: DatabaseConfig{DSN: "postgres://localhost:5432/veritie"},
		Providers: ProvidersConfig{
			STTProvider:    "deepgram",
			LLMProvider:    "gemini",
			DeepgramAPIKey: "key",
			GeminiAPIKey:   "key",
		},
		Obs: ObservabilityConfig{LogLevel: "info"},
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(cfg *Config)
		wantErr bool
	}{
		{name: "valid config", mutate: func(cfg *Config) {}, wantErr: false},
		{name: "invalid mode", mutate: func(cfg *Config) { cfg.App.Mode = "broken" }, wantErr: true},
		{name: "missing dsn", mutate: func(cfg *Config) { cfg.Database.DSN = "" }, wantErr: true},
		{name: "tracing enabled without endpoint", mutate: func(cfg *Config) { cfg.Obs.TracingEnabled = true }, wantErr: true},
		{name: "deepgram default missing key", mutate: func(cfg *Config) { cfg.Providers.DeepgramAPIKey = "" }, wantErr: true},
		{name: "provider key check bypass", mutate: func(cfg *Config) {
			cfg.Providers.DeepgramAPIKey = ""
			cfg.Providers.DisableProviderKeyChecks = true
		}, wantErr: false},
		{name: "provider key check bypass blocked in production", mutate: func(cfg *Config) {
			cfg.App.Mode = "production"
			cfg.Providers.DisableProviderKeyChecks = true
		}, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validConfig()
			tc.mutate(&cfg)
			err := cfg.Validate()
			if tc.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}
