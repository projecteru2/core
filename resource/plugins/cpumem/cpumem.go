package cpumem

import (
	"context"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/store/etcdv3/embedded"
	"github.com/projecteru2/core/store/etcdv3/meta"
	coretypes "github.com/projecteru2/core/types"
)

const (
	name                = "cpumem"
	rate                = 8
	nodeResourceInfoKey = "/resource/cpumem/%s"
	priority            = 100
)

// Plugin is the built-in cpu and memory resource plugin.
type Plugin struct {
	name   string
	config coretypes.Config
	store  meta.KV
}

func NewPlugin(ctx context.Context, config coretypes.Config, embeddedETCD *embedded.Cluster) (*Plugin, error) {
	if embeddedETCD == nil && len(config.Etcd.Machines) < 1 {
		return nil, coretypes.ErrConfigInvaild
	}
	var err error
	plugin := &Plugin{name: name, config: config}
	if plugin.store, err = meta.NewETCD(ctx, config.Etcd, embeddedETCD); err != nil {
		log.WithFunc("resource.cpumem.NewPlugin").Error(ctx, err)
		return nil, err
	}
	return plugin, nil
}

func (p Plugin) Name() string {
	return p.name
}
