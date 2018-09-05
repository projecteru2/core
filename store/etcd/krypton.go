package etcdstore

import (
	"fmt"
	"sync"

	"github.com/coreos/etcd/client"
	"github.com/coreos/etcd/clientv3"
	engineapi "github.com/docker/docker/client"
	"github.com/projecteru2/core/lock"
	"github.com/projecteru2/core/lock/etcdlock"
	"github.com/projecteru2/core/types"
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

	allContainersKey         = "/containers"
	containerInfoKey         = "/containers/%s"
	containerDeployPrefix    = "/deploy"
	containerDeployStatusKey = "/deploy/%s/%s"
	containerDeployKey       = "/deploy/%s/%s/%s/%s"

	nodeConnPrefixKey = "tcp://"
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

// cache connections
// otherwise they'll leak
type cache struct {
	sync.Mutex
	clients map[string]*engineapi.Client
}

func (c *cache) set(host string, client *engineapi.Client) {
	c.Lock()
	defer c.Unlock()

	c.clients[host] = client
}

func (c *cache) get(host string) *engineapi.Client {
	c.Lock()
	defer c.Unlock()
	return c.clients[host]
}

func (c *cache) delete(host string) {
	c.Lock()
	defer c.Unlock()
	delete(c.clients, host)
}

var _cache = &cache{clients: make(map[string]*engineapi.Client)}
