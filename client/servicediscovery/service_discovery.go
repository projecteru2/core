package servicediscovery

import "context"

// ServiceDiscovery notifies current core service addresses
type ServiceDiscovery interface {
	Watch(context.Context) (<-chan []string, error)
}
