package etcdstore

import (
	"fmt"

	"github.com/coreos/etcd/client"
	"github.com/coreos/etcd/clientv3"
	engineapi "github.com/docker/docker/client"
	"github.com/projecteru2/core/lock"
	"github.com/projecteru2/core/lock/etcdlock"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

const (
	allPodsKey    = "/pod"
	podInfoKey    = "/pod/%s/info"
	podNodesKey   = "/pod/%s/node"
	nodePrefixKey = "/pod/%s/node/%s"
	nodeInfoKey   = "/pod/%s/node/%s/info"
	nodeCaKey     = "/pod/%s/node/%s/ca.pem"
	nodeCertKey   = "/pod/%s/node/%s/cert.pem"
	nodeKeyKey    = "/pod/%s/node/%s/key.pem"

	nodePodKey        = "/node/%s/pod"
	nodeContainers    = "/node/%s/containers"
	nodeContainersKey = "/node/%s/containers/%s"

	allContainersKey = "/containers"
	containerInfoKey = "/containers/%s"

	containerDeployPrefix     = "/deploy"
	containerProcessingPrefix = "/processing" ///Appname/Entrypoint/Nodename

	nodeTCPPrefixKey  = "tcp://"
	nodeSockPrefixKey = "unix://"
)

//Krypton means store with etcd
type Krypton struct {
	etcd   client.KeysAPI
	cliv3  *clientv3.Client
	config types.Config
}

//New for create a krypton instance
func New(config types.Config) (*Krypton, error) {
	if len(config.Etcd.Machines) == 0 {
		return nil, types.ErrNoETCD
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
	return &Krypton{etcd: etcd, cliv3: cliv3, config: config}, nil
}

//CreateLock create a lock instance
func (k *Krypton) CreateLock(key string, ttl int) (lock.DistributedLock, error) {
	lockKey := fmt.Sprintf("%s/%s", k.config.Etcd.LockPrefix, key)
	mutex, err := etcdlock.New(k.cliv3, lockKey, ttl)
	return mutex, err
}

var _cache = &utils.Cache{Clients: make(map[string]*engineapi.Client)}
