package auth

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const keyPrefixLen = 8

// ErrorCode defines stable auth failure categories.
type ErrorCode string

const (
	CodeUnauthenticated  ErrorCode = "unauthenticated"
	CodeForbidden        ErrorCode = "forbidden"
	CodeMalformed        ErrorCode = "malformed_credential"
	CodeExpiredOrRevoked ErrorCode = "expired_or_revoked_key"
	CodeInternal         ErrorCode = "internal"
)

// Error is a typed auth error with stable machine-readable code.
type Error struct {
	Code    ErrorCode
	Message string
	Err     error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Message == "" {
		return string(e.Code)
	}
	return e.Message
}

func (e *Error) Unwrap() error { return e.Err }

func newError(code ErrorCode, message string, cause error) *Error {
	return &Error{Code: code, Message: message, Err: cause}
}

func NewForbidden(message string) error {
	return newError(CodeForbidden, message, nil)
}

// HTTPStatus maps auth errors to deterministic HTTP statuses.
func HTTPStatus(err error) int {
	var ae *Error
	if !errors.As(err, &ae) {
		return http.StatusInternalServerError
	}
	switch ae.Code {
	case CodeUnauthenticated, CodeMalformed, CodeExpiredOrRevoked:
		return http.StatusUnauthorized
	case CodeForbidden:
		return http.StatusForbidden
	default:
		return http.StatusInternalServerError
	}
}

// APIKeyRecord is the DB-facing auth identity record.
type APIKeyRecord struct {
	KeyID                  uuid.UUID
	AppID                  uuid.UUID
	AccountID              uuid.UUID
	KeyHash                string
	KeyPrefix              string
	RevokedAt              *time.Time
	ExpiresAt              *time.Time
	SchemaID               uuid.UUID
	ActiveSchemaVersionID  uuid.UUID
	ActiveToolsetVersionID uuid.UUID
	ProcessingConfig       []byte
	RuntimeBehavior        []byte
	LLMConfig              []byte
}

// PrincipalSnapshot is immutable request-scoped principal context.
type PrincipalSnapshot struct {
	KeyID                  uuid.UUID
	AppID                  uuid.UUID
	AccountID              uuid.UUID
	KeyPrefix              string
	SchemaID               uuid.UUID
	ActiveSchemaVersionID  uuid.UUID
	ActiveToolsetVersionID uuid.UUID
	ProcessingConfig       []byte
	RuntimeBehavior        []byte
	LLMConfig              []byte
}

// Resolver resolves API keys and runtime bundle references.
type Resolver interface {
	LookupAPIKey(ctx context.Context, keyPrefix string) (APIKeyRecord, error)
}

// Metrics captures auth-path counters/histograms.
type Metrics interface {
	IncCounter(name string, value int64, attrs map[string]string)
	ObserveHistogram(name string, value float64, attrs map[string]string)
}

type noopMetrics struct{}

func (noopMetrics) IncCounter(string, int64, map[string]string)         {}
func (noopMetrics) ObserveHistogram(string, float64, map[string]string) {}

var ErrNotFound = errors.New("auth record not found")

type cacheEntry struct {
	principal PrincipalSnapshot
	expiresAt time.Time
}

// Service authenticates credentials and returns request principal snapshots.
type Service struct {
	resolver Resolver
	metrics  Metrics
	ttl      time.Duration
	now      func() time.Time

	mu    sync.RWMutex
	cache map[string]cacheEntry
}

type Option func(*Service)

func WithTTL(ttl time.Duration) Option {
	return func(s *Service) {
		if ttl > 0 {
			s.ttl = ttl
		}
	}
}

func WithMetrics(m Metrics) Option {
	return func(s *Service) {
		if m != nil {
			s.metrics = m
		}
	}
}

func WithNow(nowFn func() time.Time) Option {
	return func(s *Service) {
		if nowFn != nil {
			s.now = nowFn
		}
	}
}

func NewService(resolver Resolver, opts ...Option) *Service {
	svc := &Service{
		resolver: resolver,
		metrics:  noopMetrics{},
		ttl:      60 * time.Second,
		now:      time.Now,
		cache:    map[string]cacheEntry{},
	}
	for _, opt := range opts {
		opt(svc)
	}
	return svc
}

// Authenticate verifies a credential and returns immutable principal context.
func (s *Service) Authenticate(ctx context.Context, credential string) (PrincipalSnapshot, error) {
	if strings.TrimSpace(credential) == "" {
		return PrincipalSnapshot{}, newError(CodeUnauthenticated, "missing credential", nil)
	}
	if s.resolver == nil {
		return PrincipalSnapshot{}, newError(CodeInternal, "auth resolver not configured", nil)
	}

	totalStart := s.now()
	credHash := HashCredential(credential)

	if principal, ok := s.cacheGet(credHash); ok {
		s.metrics.IncCounter("auth.cache.hit", 1, nil)
		s.observeMS("auth.total_ms", totalStart)
		return principal, nil
	}
	s.metrics.IncCounter("auth.cache.miss", 1, nil)

	lookupStart := s.now()
	record, err := s.resolver.LookupAPIKey(ctx, KeyPrefix(credential))
	s.observeMS("auth.lookup_ms", lookupStart)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return PrincipalSnapshot{}, newError(CodeUnauthenticated, "invalid credential", nil)
		}
		return PrincipalSnapshot{}, newError(CodeInternal, "auth lookup failed", err)
	}

	if !constantHashMatch(record.KeyHash, credHash) {
		return PrincipalSnapshot{}, newError(CodeUnauthenticated, "invalid credential", nil)
	}

	now := s.now()
	if record.RevokedAt != nil && !record.RevokedAt.IsZero() {
		return PrincipalSnapshot{}, newError(CodeExpiredOrRevoked, "credential revoked", nil)
	}
	if record.ExpiresAt != nil && !record.ExpiresAt.After(now) {
		return PrincipalSnapshot{}, newError(CodeExpiredOrRevoked, "credential expired", nil)
	}

	snapshotStart := s.now()
	principal := PrincipalSnapshot{
		KeyID:                  record.KeyID,
		AppID:                  record.AppID,
		AccountID:              record.AccountID,
		KeyPrefix:              record.KeyPrefix,
		SchemaID:               record.SchemaID,
		ActiveSchemaVersionID:  record.ActiveSchemaVersionID,
		ActiveToolsetVersionID: record.ActiveToolsetVersionID,
		ProcessingConfig:       cloneBytes(record.ProcessingConfig),
		RuntimeBehavior:        cloneBytes(record.RuntimeBehavior),
		LLMConfig:              cloneBytes(record.LLMConfig),
	}
	s.observeMS("auth.snapshot_ms", snapshotStart)

	s.cacheSet(credHash, principal, now.Add(s.ttl))
	s.observeMS("auth.total_ms", totalStart)
	return principal, nil
}

func cloneBytes(in []byte) []byte {
	if len(in) == 0 {
		return nil
	}
	out := make([]byte, len(in))
	copy(out, in)
	return out
}

func (s *Service) cacheGet(credentialHash string) (PrincipalSnapshot, bool) {
	s.mu.RLock()
	entry, ok := s.cache[credentialHash]
	s.mu.RUnlock()
	if !ok {
		return PrincipalSnapshot{}, false
	}
	if !s.now().Before(entry.expiresAt) {
		s.mu.Lock()
		delete(s.cache, credentialHash)
		s.mu.Unlock()
		return PrincipalSnapshot{}, false
	}
	return entry.principal, true
}

func (s *Service) cacheSet(credentialHash string, principal PrincipalSnapshot, expiresAt time.Time) {
	s.mu.Lock()
	s.cache[credentialHash] = cacheEntry{
		principal: principal,
		expiresAt: expiresAt,
	}
	s.mu.Unlock()
}

func (s *Service) observeMS(name string, start time.Time) {
	if start.IsZero() {
		return
	}
	elapsedMS := float64(s.now().Sub(start).Milliseconds())
	s.metrics.ObserveHistogram(name, elapsedMS, nil)
}

// ParseCredential extracts a credential from Authorization Bearer or X-API-Key.
func ParseCredential(h http.Header) (string, error) {
	authHeader := strings.TrimSpace(h.Get("Authorization"))
	apiKeyHeader := strings.TrimSpace(h.Get("X-API-Key"))

	var authKey string
	if authHeader != "" {
		parts := strings.Fields(authHeader)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
			return "", newError(CodeMalformed, "malformed authorization header", nil)
		}
		authKey = strings.TrimSpace(parts[1])
	}

	switch {
	case authKey == "" && apiKeyHeader == "":
		return "", newError(CodeUnauthenticated, "missing credential", nil)
	case authKey != "" && apiKeyHeader != "":
		return "", newError(CodeMalformed, "multiple credential sources provided", nil)
	case authKey != "":
		return authKey, nil
	default:
		return apiKeyHeader, nil
	}
}

func KeyPrefix(credential string) string {
	trimmed := strings.TrimSpace(credential)
	if len(trimmed) <= keyPrefixLen {
		return trimmed
	}
	return trimmed[:keyPrefixLen]
}

func HashCredential(credential string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(credential)))
	return hex.EncodeToString(sum[:])
}

func constantHashMatch(expectedHash, actualHash string) bool {
	expected := strings.ToLower(strings.TrimSpace(expectedHash))
	actual := strings.ToLower(strings.TrimSpace(actualHash))
	if len(expected) != len(actual) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(expected), []byte(actual)) == 1
}

type contextKey struct{}

// WithPrincipal stores principal snapshot on request context.
func WithPrincipal(ctx context.Context, principal PrincipalSnapshot) context.Context {
	return context.WithValue(ctx, contextKey{}, principal)
}

// PrincipalFromContext extracts principal snapshot from context.
func PrincipalFromContext(ctx context.Context) (PrincipalSnapshot, bool) {
	principal, ok := ctx.Value(contextKey{}).(PrincipalSnapshot)
	return principal, ok
}

// PostgresResolver resolves auth + app runtime context from Postgres.
type PostgresResolver struct {
	Pool *pgxpool.Pool
}

func (r *PostgresResolver) LookupAPIKey(ctx context.Context, keyPrefix string) (APIKeyRecord, error) {
	if r == nil || r.Pool == nil {
		return APIKeyRecord{}, fmt.Errorf("postgres resolver pool is nil")
	}
	if strings.TrimSpace(keyPrefix) == "" {
		return APIKeyRecord{}, ErrNotFound
	}

	const q = `
SELECT
  ak.id,
  ak.app_id,
  ak.account_id,
  ak.key_hash,
  ak.key_prefix,
  ak.revoked_at,
  ak.expires_at,
  a.schema_id,
  a.active_schema_version_id,
  a.active_toolset_version_id,
  a.processing_config,
  a.runtime_behavior,
  a.llm_config
FROM api_keys ak
JOIN apps a
  ON a.id = ak.app_id
 AND a.account_id = ak.account_id
WHERE ak.key_prefix = $1
LIMIT 1`

	var (
		record    APIKeyRecord
		revokedAt *time.Time
		expiresAt *time.Time
	)
	err := r.Pool.QueryRow(ctx, q, strings.TrimSpace(keyPrefix)).Scan(
		&record.KeyID,
		&record.AppID,
		&record.AccountID,
		&record.KeyHash,
		&record.KeyPrefix,
		&revokedAt,
		&expiresAt,
		&record.SchemaID,
		&record.ActiveSchemaVersionID,
		&record.ActiveToolsetVersionID,
		&record.ProcessingConfig,
		&record.RuntimeBehavior,
		&record.LLMConfig,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return APIKeyRecord{}, ErrNotFound
		}
		return APIKeyRecord{}, err
	}
	record.RevokedAt = revokedAt
	record.ExpiresAt = expiresAt
	return record, nil
}
