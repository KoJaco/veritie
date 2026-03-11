package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"veritie.io/internal/app/auth"
)

type noopAuthMetrics struct{}

func (noopAuthMetrics) IncCounter(string, int64, map[string]string)         {}
func (noopAuthMetrics) ObserveHistogram(string, float64, map[string]string) {}

// Middleware defines shared HTTP middleware hooks.
type Middleware struct {
	Auth func(http.Handler) http.Handler
}

// AuthzFunc can enforce additional request-level authorization checks.
type AuthzFunc func(principal auth.PrincipalSnapshot, r *http.Request) error

// NewAuthMiddleware authenticates credentials and injects principal context.
func NewAuthMiddleware(service *auth.Service, metrics auth.Metrics, authz AuthzFunc) func(http.Handler) http.Handler {
	if service == nil {
		panic("auth service is required")
	}
	if metrics == nil {
		metrics = noopAuthMetrics{}
	}

	// Local no-op to avoid leaking nil checks through handler path.
	incCounter := func(name string, v int64) {
		if metrics != nil {
			metrics.IncCounter(name, v, nil)
		}
	}
	observeMs := func(name string, start time.Time) {
		if metrics != nil {
			metrics.ObserveHistogram(name, float64(time.Since(start).Milliseconds()), nil)
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			parseStart := time.Now()
			credential, err := auth.ParseCredential(r.Header)
			observeMs("auth.parse_ms", parseStart)
			if err != nil {
				incCounter("auth.parse.error", 1)
				writeAuthError(w, err)
				return
			}

			principal, err := service.Authenticate(r.Context(), credential)
			if err != nil {
				writeAuthError(w, err)
				return
			}

			if authz != nil {
				if err := authz(principal, r); err != nil {
					writeAuthError(w, err)
					return
				}
			}

			ctx := auth.WithPrincipal(r.Context(), principal)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func writeAuthError(w http.ResponseWriter, err error) {
	status := auth.HTTPStatus(err)
	payload := map[string]string{
		"error":   "auth_error",
		"message": err.Error(),
	}

	var aerr *auth.Error
	if errors.As(err, &aerr) {
		payload["error"] = string(aerr.Code)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
