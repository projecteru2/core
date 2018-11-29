package etcdv3

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/mvcc/mvccpb"
	engineapi "github.com/docker/docker/client"
	"github.com/projecteru2/core/lock"
	"github.com/projecteru2/core/lock/etcdlock"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

const (
	nodeTCPPrefixKey  = "tcp://"
	nodeSockPrefixKey = "unix://"
	nodeMockPrefixKey = "mock://"

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
func (m *Mercury) GetOne(ctx context.Context, key string, opts ...clientv3.OpOption) (*mvccpb.KeyValue, error) {
	resp, err := m.Get(ctx, key, opts...)
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

// BatchDelete batch delete keys
func (m *Mercury) BatchDelete(ctx context.Context, keys []string, opts ...clientv3.OpOption) (*clientv3.TxnResponse, error) {
	txn := m.cliv3.Txn(ctx)
	ops := []clientv3.Op{}
	for _, key := range keys {
		op := clientv3.OpDelete(m.parseKey(key), opts...)
		ops = append(ops, op)
	}
	return txn.Then(ops...).Commit()
}

// Put save a key value
func (m *Mercury) Put(ctx context.Context, key, val string, opts ...clientv3.OpOption) (*clientv3.PutResponse, error) {
	return m.cliv3.Put(ctx, m.parseKey(key), val, opts...)
}

func (m *Mercury) batchPut(ctx context.Context, data map[string]string, limit map[string]map[int]string, opts ...clientv3.OpOption) (*clientv3.TxnResponse, error) {
	txn := m.cliv3.Txn(ctx)
	ops := []clientv3.Op{}
	conds := []clientv3.Cmp{}
	for key, val := range data {
		prefixKey := m.parseKey(key)
		op := clientv3.OpPut(prefixKey, val, opts...)
		ops = append(ops, op)
		if v, ok := limit[key]; ok {
			for rev, condition := range v {
				cond := clientv3.Compare(clientv3.Version(prefixKey), condition, rev)
				conds = append(conds, cond)
			}
		}
	}
	return txn.If(conds...).Then(ops...).Commit()
}

// Create create a key if not exists
func (m *Mercury) Create(ctx context.Context, key, val string, opts ...clientv3.OpOption) (*clientv3.TxnResponse, error) {
	return m.BatchCreate(ctx, map[string]string{key: val}, opts...)
}

// BatchCreate create key values if not exists
func (m *Mercury) BatchCreate(ctx context.Context, data map[string]string, opts ...clientv3.OpOption) (*clientv3.TxnResponse, error) {
	limit := map[string]map[int]string{}
	for key := range data {
		limit[key] = map[int]string{0: "="}
	}
	resp, err := m.batchPut(ctx, data, limit, opts...)
	if err != nil {
		return resp, err
	}
	if !resp.Succeeded {
		return resp, types.ErrKeyExists
	}
	return resp, nil
}

// Update update a key if exists
func (m *Mercury) Update(ctx context.Context, key, val string, opts ...clientv3.OpOption) (*clientv3.TxnResponse, error) {
	return m.BatchUpdate(ctx, map[string]string{key: val}, opts...)
}

// BatchUpdate update keys if not exists
func (m *Mercury) BatchUpdate(ctx context.Context, data map[string]string, opts ...clientv3.OpOption) (*clientv3.TxnResponse, error) {
	limit := map[string]map[int]string{}
	for key := range data {
		limit[key] = map[int]string{0: "!="}
	}
	resp, err := m.batchPut(ctx, data, limit, opts...)
	if err != nil {
		return resp, err
	}
	if !resp.Succeeded {
		return resp, types.ErrKeyExists
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
	return key
}

var _cache = &utils.Cache{Clients: make(map[string]engineapi.APIClient)}
