package cpumem

import (
	"context"
	"testing"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/store/etcdv3/meta"
	coretypes "github.com/projecteru2/core/types"
)

const (
	name                = "cpumem"
	rate                = 8
	nodeResourceInfoKey = "/resource/cpumem/%s"
)

// Plugin
type Plugin struct {
	name   string
	config coretypes.Config
	store  meta.KV
}

func NewPlugin(ctx context.Context, config coretypes.Config, t *testing.T) (*Plugin, error) {
	if t == nil && len(config.Etcd.Machines) < 1 {
		return nil, coretypes.ErrConfigInvaild
	}
	var err error
	plugin := &Plugin{name: name, config: config}
	if plugin.store, err = meta.NewETCD(config.Etcd, t); err != nil {
		log.WithFunc("resource.cpumem.NewPlugin").Error(ctx, err)
		return nil, err
	}
	return plugin, nil
}

func (p Plugin) Name() string {
	return p.name
}
