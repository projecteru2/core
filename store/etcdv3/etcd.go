package etcdv3

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/mvcc/mvccpb"
	engineapi "github.com/docker/docker/client"
	"github.com/projecteru2/core/lock"
	"github.com/projecteru2/core/lock/etcdlock"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	log "github.com/sirupsen/logrus"
)

const (
	nodeTCPPrefixKey  = "tcp://"
	nodeSockPrefixKey = "unix://"

	podInfoKey  = "/pod/info/%s"
	podNodesKey = "/pod/nodes/%s"    // /pod/nodes/pod1/node1 /pod/nodes/pod1/node2
	nodeInfoKey = "/pod/nodes/%s/%s" // /pod/nodes/pod1/node1 -> info

	nodeCaKey         = "/node/ca/%s"
	nodeCertKey       = "/node/cert/%s"
	nodeKeyKey        = "/node/key/%s"
	nodePodKey        = "/node/pod/%s"           // /node/pod/node1 value -> podname
	nodeContainersKey = "/node/containers/%s/%s" // /node/containers/n1/[64]

	containerInfoKey          = "/containers/%s" // /containers/[64]
	containerDeployPrefix     = "/deploy"        // /deploy/app1/entry1/node1/[64] value -> something by agent
	containerProcessingPrefix = "/processing"    // /processing/app1/entry1/node1/optsident value -> count
)

// Mercury means store with etcdv3
type Mercury struct {
	cliv3  *clientv3.Client
	config types.Config
}

// New for create a Mercury instance
func New(config types.Config) (*Mercury, error) {
	cliv3, err := clientv3.New(clientv3.Config{Endpoints: config.Etcd.Machines})
	if err != nil {
		return nil, err
	}

	return &Mercury{cliv3: cliv3, config: config}, nil
}

// CreateLock create a lock instance
func (m *Mercury) CreateLock(key string, ttl int) (lock.DistributedLock, error) {
	lockKey := fmt.Sprintf("%s/%s", m.config.Etcd.LockPrefix, key)
	mutex, err := etcdlock.New(m.cliv3, lockKey, ttl)
	return mutex, err
}

// Get get results or noting
func (m *Mercury) Get(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.GetResponse, error) {
	return m.cliv3.Get(ctx, m.parseKey(key), opts...)
}

// GetOne get one result or noting
func (m *Mercury) GetOne(ctx context.Context, key string) (*mvccpb.KeyValue, error) {
	resp, err := m.cliv3.Get(ctx, m.parseKey(key))
	if err != nil {
		return nil, err
	}

	if resp.Count != 1 {
		return nil, types.NewDetailedErr(types.ErrBadCount, fmt.Sprintf("key: %s", key))
	}

	return resp.Kvs[0], nil
}

// Delete delete key
func (m *Mercury) Delete(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.DeleteResponse, error) {
	return m.cliv3.Delete(ctx, m.parseKey(key), opts...)
}

// Put save a key value
func (m *Mercury) Put(ctx context.Context, key, val string, opts ...clientv3.OpOption) (*clientv3.PutResponse, error) {
	return m.cliv3.Put(ctx, m.parseKey(key), val, opts...)
}

// Create create a key if not exists
func (m *Mercury) Create(ctx context.Context, key, val string, opts ...clientv3.OpOption) (*clientv3.TxnResponse, error) {
	key = m.parseKey(key)
	req := clientv3.OpPut(key, val, opts...)
	cond := clientv3.Compare(clientv3.Version(key), "=", 0)
	resp, err := m.cliv3.Txn(ctx).If(cond).Then(req).Commit()
	if err != nil {
		return nil, err
	}
	if !resp.Succeeded {
		return nil, types.NewDetailedErr(errors.New("Key exists"), key) // TODO define in types.error
	}
	return resp, nil
}

// Update update a key if exists
func (m *Mercury) Update(ctx context.Context, key, val string, opts ...clientv3.OpOption) (*clientv3.TxnResponse, error) {
	key = m.parseKey(key)
	req := clientv3.OpPut(key, val, opts...)
	cond := clientv3.Compare(clientv3.Version(key), "!=", 0)
	resp, err := m.cliv3.Txn(ctx).If(cond).Then(req).Commit()
	if err != nil {
		return nil, err
	}
	if !resp.Succeeded {
		return nil, types.NewDetailedErr(errors.New("Key not exists"), key) // TODO define in types.error
	}
	return resp, nil
}

// Watch wath a key
func (m *Mercury) Watch(ctx context.Context, key string, opts ...clientv3.OpOption) clientv3.WatchChan {
	key = m.parseKey(key)
	return m.cliv3.Watch(ctx, key, opts...)
}

func (m *Mercury) parseKey(key string) string {
	key = filepath.Join(m.config.Etcd.Prefix, key)
	log.Debugf("[parseKey] ops on %s", key)
	return key
}

var _cache = &utils.Cache{Clients: make(map[string]*engineapi.Client)}
