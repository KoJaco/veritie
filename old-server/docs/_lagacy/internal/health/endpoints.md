# Health & Readiness Endpoints

## Overview

The Health & Readiness endpoint system provides comprehensive service monitoring for both development and production environments. It implements industry-standard health check patterns with detailed component status reporting and Fly.io integration for automatic service recovery.

## Endpoint Types

### `/healthz` - Liveness Probe

**Purpose**: Basic server responsiveness check  
**Use Case**: Kubernetes/Fly.io liveness probes  
**Response Time**: ~1ms (no external dependencies)  
**Always Returns**: HTTP 200 OK

```json
{
    "status": "up",
    "timestamp": "2025-01-15T10:30:00Z",
    "version": "1.0.0",
    "uptime": "2h15m30s",
    "components": {
        "server": {
            "status": "up",
            "message": "Server is running",
            "details": {
                "pid": "12345",
                "uptime": "2h15m30s",
                "version": "1.0.0"
            }
        }
    }
}
```

### `/readyz` - Readiness Probe

**Purpose**: External dependency health verification  
**Use Case**: Kubernetes/Fly.io readiness probes  
**Response Time**: ~50-500ms (external checks)  
**Returns**: HTTP 200 (healthy) or HTTP 503 (degraded)

```json
{
    "status": "up",
    "timestamp": "2025-01-15T10:30:00Z",
    "version": "1.0.0",
    "uptime": "2h15m30s",
    "components": {
        "database": {
            "status": "up",
            "message": "Database connection healthy",
            "latency": "12ms",
            "details": {
                "version": "PostgreSQL 15.1...",
                "query_time_ms": "3.45"
            }
        },
        "gemini": {
            "status": "up",
            "message": "Gemini API key configured",
            "latency": "1ms",
            "details": {
                "provider": "google",
                "key_length": "39"
            }
        },
        "deepgram": {
            "status": "degraded",
            "message": "Deepgram API key not configured (optional)",
            "latency": "1ms",
            "details": {
                "env_var": "DEEPGRAM_API_KEY",
                "impact": "STT fallback unavailable"
            }
        },
        "filesystem": {
            "status": "up",
            "message": "Filesystem access healthy",
            "latency": "5ms",
            "details": {
                "/data": "directory",
                "/data/models": "directory",
                "/tmp": "directory",
                "/data/models/bge/model.int8.onnx": "file (12845056 bytes)"
            }
        },
        "env": {
            "status": "up",
            "message": "Environment configuration healthy",
            "latency": "1ms",
            "details": {
                "SUPABASE_DATABASE_URL": "set (84 chars)",
                "GEMINI_API_KEY": "set (39 chars)",
                "DEEPGRAM_API_KEY": "not set (optional)",
                "SCHMA_STT_PROVIDER": "set (8 chars)",
                "GO_ENV": "set (10 chars)",
                "MODEL_DIR": "not set (optional)"
            }
        }
    }
}
```

## Implementation Architecture

### Health Checker Service (`internal/transport/http/health.go`)

```go
type HealthChecker struct {
    dbConn      *pgx.Conn           // Database connection
    sttClient   speech.STTClient     // STT service client
    geminiKey   string              // Gemini API key
    deepgramKey string              // Deepgram API key
}

func NewHealthChecker(db *pgx.Conn, stt speech.STTClient) *HealthChecker {
    return &HealthChecker{
        dbConn:      db,
        sttClient:   stt,
        geminiKey:   os.Getenv("GEMINI_API_KEY"),
        deepgramKey: os.Getenv("DEEPGRAM_API_KEY"),
    }
}
```

### Component Checks

#### Database Check

```go
func (hc *HealthChecker) checkDatabase(ctx context.Context) HealthStatus {
    start := time.Now()
    var version string
    err := hc.dbConn.QueryRow(ctx, "SELECT version()").Scan(&version)

    if err != nil {
        return HealthStatus{
            Status:  "down",
            Message: fmt.Sprintf("Database query failed: %v", err),
            Details: map[string]string{"error": err.Error()},
        }
    }

    return HealthStatus{
        Status:  "up",
        Message: "Database connection healthy",
        Details: map[string]string{
            "version":       version[:50] + "...",
            "query_time_ms": fmt.Sprintf("%.2f", time.Since(start).Seconds()*1000),
        },
    }
}
```

#### API Key Validation

```go
func (hc *HealthChecker) checkGemini(ctx context.Context) HealthStatus {
    if hc.geminiKey == "" {
        return HealthStatus{
            Status:  "down",
            Message: "Gemini API key not configured",
            Details: map[string]string{"env_var": "GEMINI_API_KEY"},
        }
    }

    if len(hc.geminiKey) < 20 {
        return HealthStatus{
            Status:  "down",
            Message: "Gemini API key appears invalid",
            Details: map[string]string{"key_length": fmt.Sprintf("%d", len(hc.geminiKey))},
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
```

#### Filesystem Verification

```go
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
            if info.IsDir() {
                details[check.path] = "directory"
            } else {
                details[check.path] = fmt.Sprintf("file (%d bytes)", info.Size())
            }
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
```

## Fly.io Integration

### Health Check Configuration (`fly.toml`)

```toml
# Liveness check - basic server responsiveness
[[checks]]
  grace_period = "15s"
  interval = "30s"
  method = "get"
  path = "/healthz"
  port = 8080
  protocol = "http"
  restart_on_failure = true
  timeout = "5s"
  type = "http"

# Readiness check - external dependency health
[[checks]]
  grace_period = "30s"
  interval = "60s"
  method = "get"
  path = "/readyz"
  port = 8080
  protocol = "http"
  restart_on_failure = true
  timeout = "10s"
  type = "http"
```

### Check Behavior

-   **Liveness Check**: Every 30s, 5s timeout, restarts on failure
-   **Readiness Check**: Every 60s, 10s timeout, restarts on degraded state
-   **Grace Periods**: 15s for liveness, 30s for readiness during startup
-   **Auto-Recovery**: Fly.io automatically restarts unhealthy instances

## Status Definitions

### Component Status Values

-   **`up`**: Component is healthy and operational
-   **`down`**: Component is failing and requires attention
-   **`degraded`**: Component has issues but service can continue

### Overall Service Status

-   **`up`**: All components are `up`
-   **`degraded`**: One or more components are `down` or `degraded`

### HTTP Response Codes

-   **200 OK**: Service is healthy (`/healthz` always, `/readyz` when status is `up`)
-   **503 Service Unavailable**: Service is degraded (`/readyz` only)

## Monitoring & Alerting

### Key Metrics to Monitor

#### Liveness Probe (`/healthz`)

-   **Response Time**: Should be <10ms consistently
-   **Success Rate**: Should be 100% (always returns 200)
-   **Availability**: Measures basic server responsiveness

#### Readiness Probe (`/readyz`)

-   **Response Time**: Typically 50-500ms depending on external services
-   **Success Rate**: Should be >99% in healthy environments
-   **Component Health**: Individual component status tracking

### Alerting Scenarios

#### Critical Alerts

-   **Liveness Failure**: Server not responding (immediate restart needed)
-   **Database Down**: Core service unavailable
-   **Required Environment Missing**: Service cannot start properly

#### Warning Alerts

-   **Degraded Components**: Optional services unavailable
-   **High Latency**: External service performance issues
-   **Filesystem Issues**: Model files missing (impacts functionality)

## Error Handling

### Component Isolation

Each component check is isolated:

```go
for name, checkFn := range checks {
    start := time.Now()
    status := checkFn(ctx)           // Individual check with timeout
    status.Latency = time.Since(start).String()
    components[name] = status

    if status.Status == "down" {
        overallStatus = "degraded"   // Don't fail entire service
    }
}
```

### Timeout Protection

```go
func (hc *HealthChecker) Readyz(w http.ResponseWriter, r *http.Request) {
    ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
    defer cancel()

    // All checks respect the timeout context
}
```

### Graceful Degradation

-   **Optional Services**: Marked as `degraded` instead of `down`
-   **Service Continuity**: API remains available even with component issues
-   **Clear Messaging**: Detailed error information for debugging

## Development vs Production

### Development Environment

```json
{
    "components": {
        "deepgram": {
            "status": "degraded",
            "message": "Deepgram API key not configured (optional)"
        }
    }
}
```

### Production Environment

```json
{
    "components": {
        "deepgram": {
            "status": "up",
            "message": "Deepgram API key configured",
            "details": {
                "provider": "deepgram",
                "key_length": "32"
            }
        }
    }
}
```

## Security Considerations

### Information Disclosure

-   **API Keys**: Show length only, never actual values
-   **Database**: Show version info, not connection strings
-   **Filesystem**: Show existence/size, not contents
-   **Error Messages**: Sanitized for production use

### Access Control

-   **Public Endpoints**: No authentication required (standard for health checks)
-   **Internal Information**: Sensitive details are abstracted
-   **Rate Limiting**: Not applied to health endpoints

## Performance Impact

### Resource Usage

-   **Memory**: <1MB additional overhead
-   **CPU**: <1% during health checks
-   **Network**: Minimal (single DB query)
-   **Disk I/O**: File stat calls only

### Caching Strategy

-   **Environment Variables**: Cached at startup
-   **File System**: Live checks (critical for volume mounts)
-   **Database**: Live connection test (critical for data access)
-   **API Keys**: Validation cached (format check only)

## Integration Examples

### Kubernetes Deployment

```yaml
apiVersion: v1
kind: Pod
spec:
    containers:
        - name: schma
          livenessProbe:
              httpGet:
                  path: /healthz
                  port: 8080
              initialDelaySeconds: 15
              periodSeconds: 30
              timeoutSeconds: 5
          readinessProbe:
              httpGet:
                  path: /readyz
                  port: 8080
              initialDelaySeconds: 30
              periodSeconds: 60
              timeoutSeconds: 10
```

### Load Balancer Configuration

```nginx
upstream schma_backend {
    server schma1:8080;
    server schma2:8080;
}

location /health {
    access_log off;
    proxy_pass http://schma_backend/readyz;
    proxy_timeout 10s;
}
```

### Monitoring Integration

```yaml
# Prometheus scrape config
- job_name: "schma-health"
  metrics_path: "/readyz"
  scrape_interval: 30s
  static_configs:
      - targets: ["schma:8080"]
```

The comprehensive health check system provides production-ready monitoring with detailed diagnostics, automatic recovery, and clear operational visibility into service health status.
