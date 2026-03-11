package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"veritie.io/internal/app/auth"
)

type resolverStub struct {
	record auth.APIKeyRecord
	err    error
}

func (r resolverStub) LookupAPIKey(context.Context, string) (auth.APIKeyRecord, error) {
	if r.err != nil {
		return auth.APIKeyRecord{}, r.err
	}
	return r.record, nil
}

func newAuthServiceForTest(t *testing.T, rawKey string) *auth.Service {
	t.Helper()
	rec := auth.APIKeyRecord{
		KeyID:                  uuid.New(),
		AppID:                  uuid.New(),
		AccountID:              uuid.New(),
		KeyHash:                auth.HashCredential(rawKey),
		KeyPrefix:              "vt_live_",
		SchemaID:               uuid.New(),
		ActiveSchemaVersionID:  uuid.New(),
		ActiveToolsetVersionID: uuid.New(),
		ProcessingConfig:       []byte(`{"mode":"test"}`),
		RuntimeBehavior:        []byte(`{"stream":"sse"}`),
		LLMConfig:              []byte(`{"provider":"gemini"}`),
	}
	return auth.NewService(
		resolverStub{record: rec},
		auth.WithTTL(1*time.Minute),
	)
}

func TestAuthMiddlewareStatuses(t *testing.T) {
	validKey := "vt_live_valid_123"
	svc := newAuthServiceForTest(t, validKey)

	mw := NewAuthMiddleware(svc, nil, nil)
	protected := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := auth.PrincipalFromContext(r.Context()); !ok {
			t.Fatalf("principal missing from context")
		}
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name     string
		headers  map[string]string
		wantCode int
	}{
		{
			name:     "missing credentials",
			headers:  map[string]string{},
			wantCode: http.StatusUnauthorized,
		},
		{
			name: "malformed authorization",
			headers: map[string]string{
				"Authorization": "Basic abc",
			},
			wantCode: http.StatusUnauthorized,
		},
		{
			name: "invalid key",
			headers: map[string]string{
				"X-API-Key": "vt_live_invalid_456",
			},
			wantCode: http.StatusUnauthorized,
		},
		{
			name: "valid key",
			headers: map[string]string{
				"Authorization": "Bearer " + validKey,
			},
			wantCode: http.StatusOK,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/v1/jobs", nil)
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}
			rr := httptest.NewRecorder()
			protected.ServeHTTP(rr, req)
			if rr.Code != tc.wantCode {
				t.Fatalf("expected status %d, got %d", tc.wantCode, rr.Code)
			}
		})
	}
}

func TestAuthMiddlewareForbidden(t *testing.T) {
	validKey := "vt_live_valid_forbidden"
	svc := newAuthServiceForTest(t, validKey)

	mw := NewAuthMiddleware(svc, nil, func(auth.PrincipalSnapshot, *http.Request) error {
		return auth.NewForbidden("forbidden principal")
	})
	protected := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/jobs", nil)
	req.Header.Set("X-API-Key", validKey)
	rr := httptest.NewRecorder()
	protected.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rr.Code)
	}
}

func TestSSEAuthParity(t *testing.T) {
	validKey := "vt_live_valid_stream"
	svc := newAuthServiceForTest(t, validKey)
	mw := NewAuthMiddleware(svc, nil, nil)
	mux := Routes(RouteDeps{
		Middleware: Middleware{Auth: mw},
	})

	endpoints := []string{
		"/v1/jobs",
		"/v1/jobs/job_123/stream",
	}

	for _, endpoint := range endpoints {
		t.Run("missing:"+endpoint, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, endpoint, nil)
			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, req)
			if rr.Code != http.StatusUnauthorized {
				t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
			}
		})

		t.Run("valid:"+endpoint, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, endpoint, nil)
			req.Header.Set("Authorization", "Bearer "+validKey)
			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, req)
			if rr.Code != http.StatusOK {
				t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
			}
		})
	}
}

func TestAuthMiddlewareNilServiceFailsClosed(t *testing.T) {
	mw := NewAuthMiddleware(nil, nil, nil)
	protected := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/jobs", nil)
	rr := httptest.NewRecorder()
	protected.ServeHTTP(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
	}
}
