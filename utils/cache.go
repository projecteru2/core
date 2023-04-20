package utils

import (
	"time"

	"github.com/projecteru2/core/engine"

	"github.com/patrickmn/go-cache"
)

// EngineCache connections
// otherwise they'll leak
type EngineCache struct {
	cache *cache.Cache
}

// NewEngineCache creates Cache instance
func NewEngineCache(expire time.Duration, cleanupInterval time.Duration) *EngineCache {
	return &EngineCache{
		cache: cache.New(expire, cleanupInterval),
	}
}

// Set connection with host
func (c *EngineCache) Set(endpoint string, client engine.API) {
	c.cache.Set(endpoint, client, cache.DefaultExpiration)
}

// Get connection by host
func (c *EngineCache) Get(endpoint string) engine.API {
	e, found := c.cache.Get(endpoint)
	if found {
		return e.(engine.API)
	}
	return nil
}

// Delete connection by host
func (c *EngineCache) Delete(host string, _ ...string) {
	c.cache.Delete(host)
}
