package etcdstore

import (
	"fmt"

	"github.com/coreos/etcd/client"
	"github.com/coreos/etcd/clientv3"
	"github.com/projecteru2/core/lock"
	"github.com/projecteru2/core/lock/etcdlock"
	"github.com/projecteru2/core/types"
)

const (
	allPodsKey       = "/pod"
	podInfoKey       = "/pod/%s/info"
	podNodesKey      = "/pod/%s/node"
	nodePrefixKey    = "/pod/%s/node/%s"
	nodeInfoKey      = "/pod/%s/node/%s/info"
	nodeCaKey        = "/pod/%s/node/%s/ca.pem"
	nodeCertKey      = "/pod/%s/node/%s/cert.pem"
	nodeKeyKey       = "/pod/%s/node/%s/key.pem"
	nodeContainerKey = "/pod/%s/node/%s/containers"
	nodePodKey       = "/node/%s"

	allContainerKey          = "/container"
	containerInfoKey         = "/container/%s"
	containerDeployPrefix    = "/deploy"
	containerDeployStatusKey = "/deploy/%s/%s"
	containerDeployKey       = "/deploy/%s/%s/%s/%s"
)

type krypton struct {
	etcd   client.KeysAPI
	cliv3  *clientv3.Client
	config types.Config
}

//New for create a krypton instance
func New(config types.Config) (*krypton, error) {
	if len(config.Etcd.Machines) == 0 {
		return nil, fmt.Errorf("ETCD must be set")
	}

	cli, err := client.New(client.Config{Endpoints: config.Etcd.Machines})
	if err != nil {
		return nil, err
	}

	cliv3, err := clientv3.New(clientv3.Config{Endpoints: config.Etcd.Machines})
	if err != nil {
		return nil, err
	}

	etcd := client.NewKeysAPIWithPrefix(cli, config.Etcd.Prefix)
	return &krypton{etcd: etcd, cliv3: cliv3, config: config}, nil
}

func (k *krypton) CreateLock(key string, ttl int) (lock.DistributedLock, error) {
	lockKey := fmt.Sprintf("%s/%s", k.config.Etcd.LockPrefix, key)
	mutex, err := etcdlock.New(k.cliv3, lockKey, ttl)
	return mutex, err
}
