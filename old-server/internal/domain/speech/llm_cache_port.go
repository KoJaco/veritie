package speech

import (
	"context"
	"encoding/json"
	"fmt"
)

type CacheKey string 

type StaticContext struct {
	Tools []FunctionDefinition `json:"tools"`
	SystemParsingGuide string `json:"system_guide"`
	Version string `json:"version"`
}

type StaticContextStructuredOutput struct {
	Schema json.RawMessage `json:"schema"`
	SystemGuide string `json:"system_guide"`
	Version string `json:"version"`
}

// type StaticContextStructuredOutput struct {
// 	StructuredOutput genai.Schema
// 	SystemGuide string
// 	Checksum string
// }

func NewStaticContextWithVersion(tools []FunctionDefinition, systemGuide string, version string) StaticContext {
	return StaticContext{
		Tools: tools,
		SystemParsingGuide: systemGuide,
		Version: version, // version should be passed from dynamic config watcher using the same compute function
	}
}


type CacheError struct {
	Type CacheErrorType
	Message string
	Err error
}

type CacheErrorType string

const (
	CacheUnavailable CacheErrorType = "cache_unavailable"
	CacheInvalid CacheErrorType = "cache_invalid"
	CacheExpired CacheErrorType = "cache_expired"
	CacheCorrupt CacheErrorType = "cache_corrupt"
	CacheMiss CacheErrorType = "cache_miss"
	CacheHit CacheErrorType = "cache_hit"
	CacheFull CacheErrorType = "cache_full"
	CacheSizeLimitExceeded CacheErrorType = "cache_size_limit_exceeded"
	CacheInvalidationFailed CacheErrorType = "cache_invalidation_failed"
)

func (e *CacheError) Error() string {
	return fmt.Sprintf("cache error [%s]: %s", e.Type, e.Message)
}

func (e *CacheError) Unwrap() error {
	return e.Err
}

// Convenience constructors for common cache errs

func NewCacheUnavailableError(err error) *CacheError {
	return &CacheError{
		Type: CacheUnavailable,
		Message: "cache service unavailable",
		Err: err,
	}
}

// TODO: complete all errors
func NewCacheInvalidError(err error) *CacheError {
	return &CacheError{
		Type: CacheInvalid,
		Message: "cache invalid",
		Err: err,
	}
}

func NewCacheExpiredError(err error) *CacheError {
	return &CacheError{
		Type: CacheExpired,
		Message: "cache expired",
		Err: err,
	}
}

func NewCacheCorruptError(err error) *CacheError {
	return &CacheError{
		Type: CacheCorrupt,
		Message: "cache corrupt",
		Err: err,
	}
}

func NewCacheMissError(err error) *CacheError {
	return &CacheError{
		Type: CacheMiss,
		Message: "cache miss",
		Err: err,
	}
}

func NewCacheHitError(err error) *CacheError {
	return &CacheError{
		Type: CacheHit,
		Message: "cache hit",
		Err: err,
	}
}

func NewCacheFullError(err error) *CacheError {
	return &CacheError{
		Type: CacheFull,
		Message: "cache full",
		Err: err,
	}
}

func NewCacheSizeLimitExceededError(err error) *CacheError {
	return &CacheError{
		Type: CacheSizeLimitExceeded,
		Message: "cache size limit exceeded",
		Err: err,
	}
}

func NewCacheInvalidationFailedError(err error) *CacheError {
	return &CacheError{
		Type: CacheInvalidationFailed,
		Message: "cache invalidation failed",
		Err: err,
	}
}


// LLMCache defines the contract for aching LLM context
type LLMCache interface {
	Store(ctx context.Context, context StaticContext) (CacheKey, error)
	Get(ctx context.Context, key CacheKey) (StaticContext, error)
	Delete(ctx context.Context, key CacheKey) error
	Invalidate(ctx context.Context, key CacheKey) error
	IsValid(ctx context.Context, key CacheKey) bool
	Clear() error
	Close() error
	IsAvailable() bool
	IsCorrupt() bool
	IsExpired() bool
}

type CachedLLM interface {
    LLM // inherit Enrich method for backward compatibility
    EnrichWithCache(ctx context.Context, key CacheKey, prompt Prompt, partial Transcript, cfg *FunctionConfig) ([]FunctionCall, *LLMUsage, error)
}