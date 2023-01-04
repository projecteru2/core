package storage

import (
	"context"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/store/etcdv3/meta"
	coretypes "github.com/projecteru2/core/types"
)

const (
	name                = "storage"
	rate                = 8
	nodeResourceInfoKey = "/resource/storage/%s"
)

// Plugin
type Plugin struct {
	name   string
	config coretypes.Config
	store  meta.KV
}

func NewPlugin(ctx context.Context, config coretypes.Config) (*Plugin, error) {
	if len(config.Etcd.Machines) < 1 {
		return nil, coretypes.ErrConfigInvaild
	}
	var err error
	plugin := &Plugin{name: name, config: config}
	if plugin.store, err = meta.NewETCD(config.Etcd, nil); err != nil {
		log.WithFunc("resource.storage.NewPlugin").Error(ctx, err)
		return nil, err
	}
	return plugin, nil
}

func (p Plugin) Name() string {
	return p.name
}
