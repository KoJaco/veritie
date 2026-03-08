package auth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRateLimiter(t *testing.T) {
	limiter := NewRateLimiter(2) // 2 requests per minute

	t.Run("within rate limit", func(t *testing.T) {
		ctx := context.Background()

		// First request
		allowed, info, err := limiter.Allow(ctx, "test-key")
		assert.NoError(t, err)
		assert.True(t, allowed)
		assert.Equal(t, int64(2), info.Limit)
		assert.Equal(t, int64(1), info.Remaining)
		assert.False(t, info.Reached)

		// Second request
		allowed, info, err = limiter.Allow(ctx, "test-key")
		assert.NoError(t, err)
		assert.True(t, allowed)
		assert.Equal(t, int64(2), info.Limit)
		assert.Equal(t, int64(0), info.Remaining)
		assert.False(t, info.Reached)
	})

	t.Run("exceed rate limit", func(t *testing.T) {
		ctx := context.Background()

		// Third request should be rate limited
		allowed, info, err := limiter.Allow(ctx, "test-key")
		assert.NoError(t, err)
		assert.False(t, allowed)
		assert.Equal(t, int64(2), info.Limit)
		assert.Equal(t, int64(0), info.Remaining)
		assert.True(t, info.Reached)
	})

	t.Run("different keys", func(t *testing.T) {
		ctx := context.Background()

		// Each key should have its own limit
		allowed1, _, err := limiter.Allow(ctx, "key1")
		assert.NoError(t, err)
		assert.True(t, allowed1)

		allowed2, _, err := limiter.Allow(ctx, "key2")
		assert.NoError(t, err)
		assert.True(t, allowed2)
	})

	t.Run("get remaining", func(t *testing.T) {
		ctx := context.Background()

		info, err := limiter.GetRemaining(ctx, "test-key")
		assert.NoError(t, err)
		assert.Equal(t, int64(2), info.Limit)
		assert.True(t, info.Remaining >= 0)
		assert.True(t, info.Remaining <= info.Limit)
	})
}

func TestRateLimiter_Concurrent(t *testing.T) {
	limiter := NewRateLimiter(10)
	ctx := context.Background()

	// Test concurrent access
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			allowed, _, err := limiter.Allow(ctx, "concurrent-key")
			assert.NoError(t, err)
			assert.True(t, allowed)
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Check that we've hit the limit
	allowed, info, err := limiter.Allow(ctx, "concurrent-key")
	assert.NoError(t, err)
	assert.False(t, allowed)
	assert.True(t, info.Reached)
}
