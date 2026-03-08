package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"schma.ai/internal/domain/speech"
)

// HealthChecker defines dependencies for health checks
type HealthChecker struct {
	dbPool      *pgxpool.Pool
	sttClient   speech.STTClient
	geminiKey   string
	deepgramKey string
}

// NewHealthChecker creates a new health checker with required dependencies
func NewHealthChecker(db *pgxpool.Pool, stt speech.STTClient) *HealthChecker {
	return &HealthChecker{
		dbPool:      db,
		sttClient:   stt,
		geminiKey:   os.Getenv("GEMINI_API_KEY"),
		deepgramKey: os.Getenv("DEEPGRAM_API_KEY"),
	}
}

// HealthStatus represents the health status of a component
type HealthStatus struct {
	Status  string            `json:"status"` // "up", "down", "degraded"
	Message string            `json:"message,omitempty"`
	Details map[string]string `json:"details,omitempty"`
	Latency string            `json:"latency,omitempty"`
}

// HealthResponse represents the overall health response
type HealthResponse struct {
	Status     string                  `json:"status"`
	Timestamp  string                  `json:"timestamp"`
	Version    string                  `json:"version,omitempty"`
	Uptime     string                  `json:"uptime,omitempty"`
	Components map[string]HealthStatus `json:"components"`
}

var (
	serverStartTime = time.Now()
	appVersion      = "1.0.0" // TODO: inject via build flags
)

// Healthz handles liveness probe - basic server responsiveness
func (hc *HealthChecker) Healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	response := HealthResponse{
		Status:    "up",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Version:   appVersion,
		Uptime:    time.Since(serverStartTime).String(),
		Components: map[string]HealthStatus{
			"server": {
				Status:  "up",
				Message: "Server is running",
				Details: map[string]string{
					"pid":     fmt.Sprintf("%d", os.Getpid()),
					"uptime":  time.Since(serverStartTime).String(),
					"version": appVersion,
				},
			},
		},
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// Readyz handles readiness probe - checks external dependencies
func (hc *HealthChecker) Readyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	w.Header().Set("Content-Type", "application/json")

	// Run all health checks
	checks := map[string]func(context.Context) HealthStatus{
		"database":   hc.checkDatabase,
		"gemini":     hc.checkGemini,
		"deepgram":   hc.checkDeepgram,
		"filesystem": hc.checkFilesystem,
		"env":        hc.checkEnvironment,
	}

	components := make(map[string]HealthStatus)
	overallStatus := "up"

	for name, checkFn := range checks {
		start := time.Now()
		status := checkFn(ctx)
		status.Latency = time.Since(start).String()
		components[name] = status

		// If any critical component is down, mark overall as degraded
		if status.Status == "down" {
			overallStatus = "degraded"
		}
	}

	response := HealthResponse{
		Status:     overallStatus,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		Version:    appVersion,
		Uptime:     time.Since(serverStartTime).String(),
		Components: components,
	}

	// Return 503 if any critical component is down
	statusCode := http.StatusOK
	if overallStatus == "degraded" {
		statusCode = http.StatusServiceUnavailable
	}

	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}

// checkDatabase verifies database connectivity
func (hc *HealthChecker) checkDatabase(ctx context.Context) HealthStatus {
	if hc.dbPool == nil {
		return HealthStatus{
			Status:  "down",
			Message: "Database connection pool not configured",
		}
	}

	start := time.Now()
	var version string
	err := hc.dbPool.QueryRow(ctx, "SELECT version()").Scan(&version)

	if err != nil {
		return HealthStatus{
			Status:  "down",
			Message: fmt.Sprintf("Database query failed: %v", err),
			Details: map[string]string{
				"error": err.Error(),
			},
		}
	}

	return HealthStatus{
		Status:  "up",
		Message: "Database connection healthy",
		Details: map[string]string{
			"version":       version[:50] + "...", // truncate for readability
			"query_time_ms": fmt.Sprintf("%.2f", time.Since(start).Seconds()*1000),
		},
	}
}

// checkGemini verifies Gemini API accessibility
func (hc *HealthChecker) checkGemini(ctx context.Context) HealthStatus {
	if hc.geminiKey == "" {
		return HealthStatus{
			Status:  "down",
			Message: "Gemini API key not configured",
			Details: map[string]string{
				"env_var": "GEMINI_API_KEY",
			},
		}
	}

	// Simple API check - we could make a lightweight request here
	// For now, just verify the key is present and has expected format
	if len(hc.geminiKey) < 20 {
		return HealthStatus{
			Status:  "down",
			Message: "Gemini API key appears invalid",
			Details: map[string]string{
				"key_length": fmt.Sprintf("%d", len(hc.geminiKey)),
			},
		}
	}

	return HealthStatus{
		Status:  "up",
		Message: "Gemini API key configured",
		Details: map[string]string{
			"provider":   "google",
			"key_length": fmt.Sprintf("%d", len(hc.geminiKey)),
		},
	}
}

// checkDeepgram verifies Deepgram API accessibility
func (hc *HealthChecker) checkDeepgram(ctx context.Context) HealthStatus {
	if hc.deepgramKey == "" {
		return HealthStatus{
			Status:  "degraded",
			Message: "Deepgram API key not configured (optional)",
			Details: map[string]string{
				"env_var": "DEEPGRAM_API_KEY",
				"impact":  "STT fallback unavailable",
			},
		}
	}

	if len(hc.deepgramKey) < 20 {
		return HealthStatus{
			Status:  "down",
			Message: "Deepgram API key appears invalid",
			Details: map[string]string{
				"key_length": fmt.Sprintf("%d", len(hc.deepgramKey)),
			},
		}
	}

	return HealthStatus{
		Status:  "up",
		Message: "Deepgram API key configured",
		Details: map[string]string{
			"provider":   "deepgram",
			"key_length": fmt.Sprintf("%d", len(hc.deepgramKey)),
		},
	}
}

// checkFilesystem verifies critical directories and model files
func (hc *HealthChecker) checkFilesystem(ctx context.Context) HealthStatus {
	checks := []struct {
		path        string
		description string
		required    bool
	}{
		{"/data", "Data volume mount", true},
		{"/data/models", "ML models directory", true},
		{"/tmp", "Temporary files", true},
		{"/data/models/bge/model.int8.onnx", "BGE ONNX model", false},
	}

	details := make(map[string]string)
	var issues []string

	for _, check := range checks {
		if info, err := os.Stat(check.path); err != nil {
			if check.required {
				issues = append(issues, fmt.Sprintf("%s missing", check.description))
			}
			details[check.path] = "missing"
		} else {
			var fileType string
			if info.IsDir() {
				fileType = "directory"
			} else {
				fileType = fmt.Sprintf("file (%d bytes)", info.Size())
			}
			details[check.path] = fileType
		}
	}

	if len(issues) > 0 {
		return HealthStatus{
			Status:  "down",
			Message: fmt.Sprintf("Filesystem issues: %v", issues),
			Details: details,
		}
	}

	return HealthStatus{
		Status:  "up",
		Message: "Filesystem access healthy",
		Details: details,
	}
}

// checkEnvironment verifies critical environment variables
func (hc *HealthChecker) checkEnvironment(ctx context.Context) HealthStatus {
	required := []string{
		"SUPABASE_DATABASE_URL",
		"GEMINI_API_KEY",
	}

	optional := []string{
		"DEEPGRAM_API_KEY",
		"SCHMA_STT_PROVIDER",
		"GO_ENV",
		"MODEL_DIR",
	}

	details := make(map[string]string)
	var missing []string

	// Check required vars
	for _, envVar := range required {
		if val := os.Getenv(envVar); val == "" {
			missing = append(missing, envVar)
			details[envVar] = "missing"
		} else {
			// Show length for security (don't expose actual values)
			details[envVar] = fmt.Sprintf("set (%d chars)", len(val))
		}
	}

	// Check optional vars
	for _, envVar := range optional {
		if val := os.Getenv(envVar); val == "" {
			details[envVar] = "not set (optional)"
		} else {
			details[envVar] = fmt.Sprintf("set (%d chars)", len(val))
		}
	}

	if len(missing) > 0 {
		return HealthStatus{
			Status:  "down",
			Message: fmt.Sprintf("Missing required environment variables: %v", missing),
			Details: details,
		}
	}

	return HealthStatus{
		Status:  "up",
		Message: "Environment configuration healthy",
		Details: details,
	}
}

// CreateSimpleHealthHandler creates a basic health handler for backward compatibility
func CreateSimpleHealthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}
}
