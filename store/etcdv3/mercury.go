package etcdv3

import (
	"context"
	"crypto/tls"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/coreos/etcd/pkg/transport"

	log "github.com/sirupsen/logrus"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/mvcc/mvccpb"
	"github.com/projecteru2/core/lock"
	"github.com/projecteru2/core/lock/etcdlock"
	"github.com/projecteru2/core/store/etcdv3/embeded"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

const (
	podInfoKey  = "/pod/info/%s"     // /pod/info/{podname}
	podNodesKey = "/pod/%s:nodes"    // /pod/{podname}:nodes -> for list pod nodes
	nodeInfoKey = "/pod/%s:nodes/%s" // /pod/{podname}:nodes/{nodenmae} -> for node info

	nodeCaKey         = "/node/%s:ca"            // /node/{nodename}:ca
	nodeCertKey       = "/node/%s:cert"          // /node/{nodename}:cert
	nodeKeyKey        = "/node/%s:key"           // /node/{nodename}:key
	nodePodKey        = "/node/%s:pod"           // /node/{nodename}:pod value -> podname
	nodeContainersKey = "/node/%s:containers/%s" // /node/{nodename}:containers/{containerID}

	containerInfoKey          = "/containers/%s" // /containers/{containerID}
	containerDeployPrefix     = "/deploy"        // /deploy/{appname}/{entrypoint}/{nodename}/{containerID} value -> something by agent
	containerStatusPrefix     = "/status"        // /status/{appname}/{entrypoint}/{nodename}/{containerID} value -> something by agent
	containerProcessingPrefix = "/processing"    // /processing/{appname}/{entrypoint}/{nodename}/{opsIdent} value -> count

	cmpVersion = "version"
	cmpValue   = "value"
)

// Mercury means store with etcdv3
type Mercury struct {
	cliv3  *clientv3.Client
	config types.Config
}

// New for create a Mercury instance
func New(config types.Config, embededStorage bool) (*Mercury, error) {
	var cliv3 *clientv3.Client
	var err error
	var tlsConfig *tls.Config

	if config.Etcd.Ca != "" && config.Etcd.Key != "" && config.Etcd.Cert != "" {
		tlsInfo := transport.TLSInfo{
			TrustedCAFile: config.Etcd.Ca,
			KeyFile:       config.Etcd.Key,
			CertFile:      config.Etcd.Cert,
		}
		tlsConfig, err = tlsInfo.ClientConfig()
		if err != nil {
			return nil, err
		}
	}

	if embededStorage {
		cliv3 = embeded.NewCluster()
		log.Info("[Mercury] use embeded cluster")
	} else if cliv3, err = clientv3.New(clientv3.Config{
		Endpoints: config.Etcd.Machines,
		Username:  config.Etcd.Auth.Username,
		Password:  config.Etcd.Auth.Password,
		TLS:       tlsConfig,
	}); err != nil {
		return nil, err
	}
	return &Mercury{cliv3: cliv3, config: config}, nil
}

// TerminateEmbededStorage terminate embeded storage
func (m *Mercury) TerminateEmbededStorage() {
	embeded.TerminateCluster()
}

// CreateLock create a lock instance
func (m *Mercury) CreateLock(key string, ttl time.Duration) (lock.DistributedLock, error) {
	lockKey := fmt.Sprintf("%s/%s", m.config.Etcd.LockPrefix, key)
	mutex, err := etcdlock.New(m.cliv3, lockKey, ttl)
	return mutex, err
}

// Get get results or noting
func (m *Mercury) Get(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.GetResponse, error) {
	return m.cliv3.Get(ctx, m.parseKey(key), opts...)
}

func (m *Mercury) batchGet(ctx context.Context, keys []string, opt ...clientv3.OpOption) (txnResponse *clientv3.TxnResponse, err error) {
	ops := []clientv3.Op{}
	for _, key := range keys {
		op := clientv3.OpGet(m.parseKey(key), opt...)
		ops = append(ops, op)
	}
	return m.doBatchOp(ctx, nil, ops, nil)
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

// GetMulti gets several results
func (m *Mercury) GetMulti(ctx context.Context, keys []string, opts ...clientv3.OpOption) (kvs []*mvccpb.KeyValue, err error) {
	var txnResponse *clientv3.TxnResponse
	if len(keys) == 0 {
		return
	}

	if txnResponse, err = m.batchGet(ctx, keys); err != nil {
		return
	}

	for idx, responseOp := range txnResponse.Responses {
		resp := responseOp.GetResponseRange()
		if resp.Count != 1 {
			err = types.NewDetailedErr(types.ErrBadCount, fmt.Sprintf("key: %s", keys[idx]))
			return
		}

		kvs = append(kvs, resp.Kvs[0])
	}

	if len(kvs) != len(keys) {
		err = types.NewDetailedErr(types.ErrBadCount, fmt.Sprintf("keys: %v", keys))
	}

	return
}

// Delete delete key
func (m *Mercury) Delete(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.DeleteResponse, error) {
	return m.cliv3.Delete(ctx, m.parseKey(key), opts...)
}

// BatchDelete batch delete keys
func (m *Mercury) batchDelete(ctx context.Context, keys []string, opts ...clientv3.OpOption) (*clientv3.TxnResponse, error) {
	ops := []clientv3.Op{}
	for _, key := range keys {
		op := clientv3.OpDelete(m.parseKey(key), opts...)
		ops = append(ops, op)
	}

	return m.doBatchOp(ctx, nil, ops, nil)
}

// Put save a key value
func (m *Mercury) Put(ctx context.Context, key, val string, opts ...clientv3.OpOption) (*clientv3.TxnResponse, error) {
	return m.batchPut(ctx, map[string]string{key: val}, nil, opts...)
}

func (m *Mercury) batchPut(ctx context.Context, data map[string]string, limit map[string]map[string]string, opts ...clientv3.OpOption) (*clientv3.TxnResponse, error) {
	ops := []clientv3.Op{}
	failOps := []clientv3.Op{}
	conds := []clientv3.Cmp{}
	for key, val := range data {
		prefixKey := m.parseKey(key)
		op := clientv3.OpPut(prefixKey, val, opts...)
		ops = append(ops, op)
		if v, ok := limit[key]; ok {
			for method, condition := range v {
				switch method {
				case cmpVersion:
					cond := clientv3.Compare(clientv3.Version(prefixKey), condition, 0)
					conds = append(conds, cond)
				case cmpValue:
					cond := clientv3.Compare(clientv3.Value(prefixKey), condition, val)
					failOp := clientv3.OpGet(prefixKey)
					failOps = append(failOps, failOp)
					conds = append(conds, cond)
				}
			}
		}
	}
	return m.doBatchOp(ctx, conds, ops, failOps)
}

// Create create a key if not exists
func (m *Mercury) Create(ctx context.Context, key, val string, opts ...clientv3.OpOption) (*clientv3.TxnResponse, error) {
	return m.BatchCreate(ctx, map[string]string{key: val}, opts...)
}

// BatchCreate create key values if not exists
func (m *Mercury) BatchCreate(ctx context.Context, data map[string]string, opts ...clientv3.OpOption) (*clientv3.TxnResponse, error) {
	limit := map[string]map[string]string{}
	for key := range data {
		limit[key] = map[string]string{cmpVersion: "="}
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
	limit := map[string]map[string]string{}
	for key := range data {
		limit[key] = map[string]string{cmpVersion: "!=", cmpValue: "!="} // ignore same data
	}
	resp, err := m.batchPut(ctx, data, limit, opts...)
	if err != nil {
		return resp, err
	}
	if !resp.Succeeded {
		for _, failResp := range resp.Responses {
			if len(failResp.GetResponseRange().Kvs) == 0 {
				return resp, types.ErrKeyNotExists
			}
		}
	}
	return resp, nil
}

// Watch wath a key
func (m *Mercury) Watch(ctx context.Context, key string, opts ...clientv3.OpOption) clientv3.WatchChan {
	key = m.parseKey(key)
	return m.cliv3.Watch(ctx, key, opts...)
}

func (m *Mercury) parseKey(key string) string {
	after := filepath.Join(m.config.Etcd.Prefix, key)
	if strings.HasSuffix(key, "/") {
		after = after + "/"
	}
	return after
}

func (m *Mercury) doBatchOp(ctx context.Context, conds []clientv3.Cmp, ops, failOps []clientv3.Op) (*clientv3.TxnResponse, error) {
	if len(ops) == 0 {
		return nil, types.ErrNoOps
	}

	const txnLimit = 125
	count := len(ops) / txnLimit // stupid etcd txn, default limit is 128
	tail := len(ops) % txnLimit
	length := count
	if tail != 0 {
		length++
	}

	resps := make([]*clientv3.TxnResponse, length)
	errs := make([]error, length)

	wg := sync.WaitGroup{}
	doOp := func(index int, ops []clientv3.Op) {
		defer wg.Done()
		txn := m.cliv3.Txn(ctx)
		if len(conds) != 0 {
			txn = txn.If(conds...)
		}
		resp, err := txn.Then(ops...).Else(failOps...).Commit()
		resps[index] = resp
		errs[index] = err
	}

	if tail != 0 {
		wg.Add(1)
		go doOp(length-1, ops[count*txnLimit:])
	}

	for i := 0; i < count; i++ {
		wg.Add(1)
		go doOp(i, ops[i*txnLimit:(i+1)*txnLimit])
	}
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}

	if len(resps) == 0 {
		return &clientv3.TxnResponse{}, nil
	}

	resp := resps[0]
	for i := 1; i < len(resps); i++ {
		resp.Succeeded = resp.Succeeded && resps[i].Succeeded
		resp.Responses = append(resp.Responses, resps[i].Responses...)
	}
	return resp, nil
}

var _cache = utils.NewEngineCache(12*time.Hour, 10*time.Minute)
