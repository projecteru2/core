package etcdstore

import (
	"fmt"
	"path/filepath"

	"github.com/coreos/etcd/client"
	"gitlab.ricebook.net/platform/core/lock"
	"gitlab.ricebook.net/platform/core/lock/etcdlock"
	"gitlab.ricebook.net/platform/core/types"
)

var (
	allPodsKey       = "/eru-core/pod"
	podInfoKey       = "/eru-core/pod/%s/info"
	podNodesKey      = "/eru-core/pod/%s/node"
	nodeInfoKey      = "/eru-core/pod/%s/node/%s/info"
	nodeCaKey        = "/eru-core/pod/%s/node/%s/ca.pem"
	nodeCertKey      = "/eru-core/pod/%s/node/%s/cert.pem"
	nodeKeyKey       = "/eru-core/pod/%s/node/%s/key.pem"
	nodeContainerKey = "/eru-core/pod/%s/node/%s/containers"
	containerInfoKey = "/eru-core/container/%s"
)

type krypton struct {
	etcd   client.KeysAPI
	config types.Config
}

func New(config types.Config) (*krypton, error) {
	if len(config.EtcdMachines) == 0 {
		return nil, fmt.Errorf("ETCD must be set")
	}

	cli, err := client.New(client.Config{Endpoints: config.EtcdMachines})
	if err != nil {
		return nil, err
	}

	etcd := client.NewKeysAPI(cli)
	return &krypton{etcd: etcd, config: config}, nil
}

func (k *krypton) CreateLock(key string, ttl int) (lock.DistributedLock, error) {
	lockKey := filepath.Join(k.config.EtcdLockPrefix, key)
	mutex := etcdlock.New(k.etcd, lockKey, ttl)
	if mutex == nil {
		return nil, fmt.Errorf("Error creating mutex %q %q", key, ttl)
	}
	return mutex, nil
}
