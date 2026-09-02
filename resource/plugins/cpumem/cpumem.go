package cpumem

import (
	"context"

	coretypes "github.com/projecteru2/core/types"
)

const (
	name                = "cpumem"
	rate                = 8
	nodeResourceInfoKey = "/resource/cpumem/%s"
	priority            = 100
)

// Store is the slice of the cluster store that keeps the node records of this plugin.
type Store interface {
	NotFound(err error) bool
	GetMulti(ctx context.Context, keys []string) (map[string]string, error)
	Put(ctx context.Context, data map[string]string) error
	Delete(ctx context.Context, keys []string) error
}

// Plugin is the built-in cpu and memory resource plugin.
type Plugin struct {
	name   string
	config coretypes.Config
	store  Store
}

func NewPlugin(config coretypes.Config, store Store) *Plugin {
	return &Plugin{name: name, config: config, store: store}
}

func (p Plugin) Name() string {
	return p.name
}
