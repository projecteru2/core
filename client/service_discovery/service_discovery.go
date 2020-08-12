package service_discovery

import "context"

type ServiceDiscovery interface {
	Watch(context.Context) (<-chan []string, error)
}
