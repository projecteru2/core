package models

import (
	"github.com/sirupsen/logrus"

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
			logrus.Errorf("[NewVolume] failed to create etcd client, err: %v", err)
			return nil, err
		}
	}
	return v, nil
}
