package auth

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
)

type fakeResolver struct {
	record APIKeyRecord
	err    error
	calls  int
}

func (f *fakeResolver) LookupAPIKey(context.Context, string) (APIKeyRecord, error) {
	f.calls++
	if f.err != nil {
		return APIKeyRecord{}, f.err
	}
	return f.record, nil
}

type testMetrics struct {
	counters map[string]int64
}

func (m *testMetrics) IncCounter(name string, value int64, _ map[string]string) {
	if m.counters == nil {
		m.counters = map[string]int64{}
	}
	m.counters[name] += value
}

func (m *testMetrics) ObserveHistogram(string, float64, map[string]string) {}

func TestParseCredential(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		_, err := ParseCredential(http.Header{})
		if err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("x-api-key", func(t *testing.T) {
		h := http.Header{}
		h.Set("X-API-Key", "abc123")
		key, err := ParseCredential(h)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if key != "abc123" {
			t.Fatalf("expected abc123, got %q", key)
		}
	})

	t.Run("bearer", func(t *testing.T) {
		h := http.Header{}
		h.Set("Authorization", "Bearer secret")
		key, err := ParseCredential(h)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if key != "secret" {
			t.Fatalf("expected secret, got %q", key)
		}
	})

	t.Run("malformed bearer", func(t *testing.T) {
		h := http.Header{}
		h.Set("Authorization", "Basic secret")
		_, err := ParseCredential(h)
		if err == nil {
			t.Fatalf("expected error")
		}
		var ae *Error
		if !errors.As(err, &ae) || ae.Code != CodeMalformed {
			t.Fatalf("expected malformed error, got %v", err)
		}
	})

	t.Run("ambiguous headers", func(t *testing.T) {
		h := http.Header{}
		h.Set("Authorization", "Bearer one")
		h.Set("X-API-Key", "two")
		_, err := ParseCredential(h)
		if err == nil {
			t.Fatalf("expected error")
		}
		var ae *Error
		if !errors.As(err, &ae) || ae.Code != CodeMalformed {
			t.Fatalf("expected malformed error, got %v", err)
		}
	})
}

func TestServiceAuthenticate(t *testing.T) {
	now := time.Date(2026, 3, 11, 12, 0, 0, 0, time.UTC)
	validKey := "vt_live_key_123456"
	rec := APIKeyRecord{
		KeyID:                  uuid.New(),
		AppID:                  uuid.New(),
		AccountID:              uuid.New(),
		KeyHash:                HashCredential(validKey),
		KeyPrefix:              KeyPrefix(validKey),
		SchemaID:               uuid.New(),
		ActiveSchemaVersionID:  uuid.New(),
		ActiveToolsetVersionID: uuid.New(),
		ProcessingConfig:       []byte(`{"a":1}`),
		RuntimeBehavior:        []byte(`{"b":2}`),
		LLMConfig:              []byte(`{"c":3}`),
	}

	t.Run("valid", func(t *testing.T) {
		resolver := &fakeResolver{record: rec}
		svc := NewService(resolver, WithNow(func() time.Time { return now }))
		principal, err := svc.Authenticate(context.Background(), validKey)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if principal.AppID != rec.AppID || principal.AccountID != rec.AccountID {
			t.Fatalf("unexpected principal ids")
		}
	})

	t.Run("invalid hash", func(t *testing.T) {
		resolver := &fakeResolver{record: rec}
		svc := NewService(resolver, WithNow(func() time.Time { return now }))
		_, err := svc.Authenticate(context.Background(), "wrong-key")
		if err == nil {
			t.Fatalf("expected error")
		}
		var ae *Error
		if !errors.As(err, &ae) || ae.Code != CodeUnauthenticated {
			t.Fatalf("expected unauthenticated, got %v", err)
		}
	})

	t.Run("revoked key", func(t *testing.T) {
		revoked := now.Add(-time.Minute)
		recCopy := rec
		recCopy.RevokedAt = &revoked
		resolver := &fakeResolver{record: recCopy}
		svc := NewService(resolver, WithNow(func() time.Time { return now }))
		_, err := svc.Authenticate(context.Background(), validKey)
		var ae *Error
		if !errors.As(err, &ae) || ae.Code != CodeExpiredOrRevoked {
			t.Fatalf("expected expired/revoked, got %v", err)
		}
	})

	t.Run("expired key", func(t *testing.T) {
		expired := now.Add(-time.Second)
		recCopy := rec
		recCopy.ExpiresAt = &expired
		resolver := &fakeResolver{record: recCopy}
		svc := NewService(resolver, WithNow(func() time.Time { return now }))
		_, err := svc.Authenticate(context.Background(), validKey)
		var ae *Error
		if !errors.As(err, &ae) || ae.Code != CodeExpiredOrRevoked {
			t.Fatalf("expected expired/revoked, got %v", err)
		}
	})

	t.Run("resolver not found", func(t *testing.T) {
		resolver := &fakeResolver{err: ErrNotFound}
		svc := NewService(resolver, WithNow(func() time.Time { return now }))
		_, err := svc.Authenticate(context.Background(), validKey)
		var ae *Error
		if !errors.As(err, &ae) || ae.Code != CodeUnauthenticated {
			t.Fatalf("expected unauthenticated, got %v", err)
		}
	})

	t.Run("cache hit and expiry", func(t *testing.T) {
		resolver := &fakeResolver{record: rec}
		current := now
		metrics := &testMetrics{}
		svc := NewService(
			resolver,
			WithNow(func() time.Time { return current }),
			WithTTL(30*time.Second),
			WithMetrics(metrics),
		)

		if _, err := svc.Authenticate(context.Background(), validKey); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, err := svc.Authenticate(context.Background(), validKey); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resolver.calls != 1 {
			t.Fatalf("expected 1 resolver call after cache hit, got %d", resolver.calls)
		}

		current = current.Add(31 * time.Second)
		if _, err := svc.Authenticate(context.Background(), validKey); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resolver.calls != 2 {
			t.Fatalf("expected resolver call after cache expiry, got %d", resolver.calls)
		}
		if metrics.counters["auth.cache.hit"] == 0 {
			t.Fatalf("expected cache hit counter")
		}
		if metrics.counters["auth.cache.miss"] == 0 {
			t.Fatalf("expected cache miss counter")
		}
	})
}

func BenchmarkAuthenticate(b *testing.B) {
	key := "vt_live_key_benchmark"
	rec := APIKeyRecord{
		KeyID:                  uuid.New(),
		AppID:                  uuid.New(),
		AccountID:              uuid.New(),
		KeyHash:                HashCredential(key),
		KeyPrefix:              KeyPrefix(key),
		SchemaID:               uuid.New(),
		ActiveSchemaVersionID:  uuid.New(),
		ActiveToolsetVersionID: uuid.New(),
	}
	resolver := &fakeResolver{record: rec}
	svc := NewService(resolver)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := svc.Authenticate(context.Background(), key); err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}
