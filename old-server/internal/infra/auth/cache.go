package auth

import (
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"schma.ai/internal/domain/auth"
)

type AppSettingsCache struct {
	cache *lru.Cache[string, cachedPrincipal]
}

type cachedPrincipal struct {
	principal auth.Principal
	expiresAt time.Time
}

// NewAppSettingsCache creates a n ew cache with specified size
func NewAppSettingsCache(size int) (*AppSettingsCache, error) {
	cache, err := lru.New[string, cachedPrincipal](size)
	if err != nil {
		return nil, err
	}
	return &AppSettingsCache{cache: cache}, nil
}

// Get method retrieves a principal from the cache if not expired based on a provided key
func (c *AppSettingsCache) Get(key string) (auth.Principal, bool) {
	if cached, ok := c.cache.Get(key); ok {
		if time.Now().Before(cached.expiresAt) {
			return cached.principal, true
		}
		c.cache.Remove(key)
	}
	return auth.Principal{}, false
}

// Set method adds a principal to the cahce with a given expiry
func (c *AppSettingsCache) Set(key string, principal auth.Principal, expiry time.Duration) {
	c.cache.Add(key, cachedPrincipal{
		principal: principal,
		expiresAt: time.Now().Add(expiry * time.Second),
	})
}
