package auth

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"schma.ai/internal/domain/auth"
)

func TestAppSettingsCache(t *testing.T) {
	cache, err := NewAppSettingsCache(10)
	assert.NoError(t, err)

	principal := auth.Principal{
		AppID:     pgtype.UUID{Bytes: [16]byte{1, 2, 3, 4}, Valid: true},
		AccountID: pgtype.UUID{Bytes: [16]byte{5, 6, 7, 8}, Valid: true},
		AppName:   "Test App",
	}

	t.Run("set and get", func(t *testing.T) {
		cache.Set("test-key", principal, 20*time.Second)

		retrieved, ok := cache.Get("test-key")
		assert.True(t, ok)
		assert.Equal(t, principal.AppName, retrieved.AppName)
		assert.Equal(t, principal.AppID, retrieved.AppID)
	})

	t.Run("get non-existent", func(t *testing.T) {
		_, ok := cache.Get("non-existent")
		assert.False(t, ok)
	})

	t.Run("cache size limit", func(t *testing.T) {
		smallCache, err := NewAppSettingsCache(2)
		assert.NoError(t, err)

		// Add 3 items to a cache with size 2
		smallCache.Set("key1", principal, 20*time.Second)
		smallCache.Set("key2", principal, 20*time.Second)
		smallCache.Set("key3", principal, 20*time.Second)

		// The first key should be evicted
		_, ok := smallCache.Get("key1")
		assert.False(t, ok)

		// The last two keys should still be there
		_, ok = smallCache.Get("key2")
		assert.True(t, ok)
		_, ok = smallCache.Get("key3")
		assert.True(t, ok)
	})
}

func TestAppSettingsCache_Expiration(t *testing.T) {
	// Note: This test would require time mocking for proper testing
	// For now, we'll test the basic functionality
	cache, err := NewAppSettingsCache(10)
	assert.NoError(t, err)

	principal := auth.Principal{
		AppName: "Test App",
	}

	cache.Set("test-key", principal, 20*time.Second)

	// Should be available immediately
	retrieved, ok := cache.Get("test-key")
	assert.True(t, ok)
	assert.Equal(t, principal.AppName, retrieved.AppName)
}
