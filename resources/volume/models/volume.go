package models

import (
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/store/etcdv3/meta"
	coretypes "github.com/projecteru2/core/types"
)

// Volume .
type Volume struct {
	Config coretypes.Config
	store  meta.KV
}

// NewVolume .
func NewVolume(config coretypes.Config) (*Volume, error) {
	v := &Volume{Config: config}
	var err error
	if len(config.Etcd.Machines) > 0 {
		v.store, err = meta.NewETCD(config.Etcd, nil)
		if err != nil {
			log.WithFunc("resources.volume.NewVolume").Error(nil, err, "failed to create etcd client") //nolint
			return nil, err
		}
	}
	return v, nil
}
