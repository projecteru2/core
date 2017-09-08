package etcdstore

import (
	"fmt"
	"path/filepath"

	"github.com/coreos/etcd/client"
	"github.com/coreos/etcd/clientv3"
	"github.com/projecteru2/core/lock"
	"github.com/projecteru2/core/lock/etcdlock"
	"github.com/projecteru2/core/types"
)

const (
	allPodsKey       = "/eru-core/pod"
	podInfoKey       = "/eru-core/pod/%s/info"
	podNodesKey      = "/eru-core/pod/%s/node"
	nodePrefixKey    = "/eru-core/pod/%s/node/%s"
	nodeInfoKey      = "/eru-core/pod/%s/node/%s/info"
	nodeCaKey        = "/eru-core/pod/%s/node/%s/ca.pem"
	nodeCertKey      = "/eru-core/pod/%s/node/%s/cert.pem"
	nodeKeyKey       = "/eru-core/pod/%s/node/%s/key.pem"
	nodeContainerKey = "/eru-core/pod/%s/node/%s/containers"

	allContainerKey          = "/eru-core/container"
	containerInfoKey         = "/eru-core/container/%s"
	containerDeployStatusKey = "/eru-core/deploy/%s/%s"
	containerDeployKey       = "/eru-core/deploy/%s/%s/%s/%s"
)

type krypton struct {
	etcd   client.KeysAPI
	cliv3  *clientv3.Client
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

	cliv3, err := clientv3.New(clientv3.Config{Endpoints: config.EtcdMachines})
	if err != nil {
		return nil, err
	}

	etcd := client.NewKeysAPI(cli)
	return &krypton{etcd: etcd, cliv3: cliv3, config: config}, nil
}

func (k *krypton) CreateLock(key string, ttl int) (lock.DistributedLock, error) {
	lockKey := filepath.Join(k.config.EtcdLockPrefix, key)
	mutex, err := etcdlock.New(k.cliv3, lockKey, ttl)
	return mutex, err
}
