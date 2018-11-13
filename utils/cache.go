package utils

import (
	"sync"

	engineapi "github.com/docker/docker/client"
)

// Cache connections
// otherwise they'll leak
type Cache struct {
	sync.Mutex
	Clients map[string]engineapi.APIClient
}

// Set connection with host
func (c *Cache) Set(host string, client engineapi.APIClient) {
	c.Lock()
	defer c.Unlock()
	c.Clients[host] = client
}

// Get connection by host
func (c *Cache) Get(host string) engineapi.APIClient {
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
