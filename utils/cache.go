package utils

import (
	"sync"

	"github.com/projecteru2/core/engine"
)

// Cache connections
// otherwise they'll leak
type Cache struct {
	sync.Mutex
	Clients map[string]engine.API
}

// Set connection with host
func (c *Cache) Set(host string, client engine.API) {
	c.Lock()
	defer c.Unlock()
	c.Clients[host] = client
}

// Get connection by host
func (c *Cache) Get(host string) engine.API {
	c.Lock()
	defer c.Unlock()
	return c.Clients[host]
}

// Delete connection by host
func (c *Cache) Delete(host string) {
	c.Lock()
	defer c.Unlock()
	delete(c.Clients, host)
}
