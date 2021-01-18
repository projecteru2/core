package meta

import (
	"context"
	"crypto/tls"
	"fmt"
	"sync"
	"time"

	"go.etcd.io/etcd/v3/clientv3"
	"go.etcd.io/etcd/v3/clientv3/namespace"
	"go.etcd.io/etcd/v3/mvcc/mvccpb"
	"go.etcd.io/etcd/v3/pkg/transport"

	"github.com/projecteru2/core/lock"
	"github.com/projecteru2/core/lock/etcdlock"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/store/etcdv3/embedded"
	"github.com/projecteru2/core/types"
)

const (
	cmpVersion = "version"
	cmpValue   = "value"
)

// ETCDClientV3 .
type ETCDClientV3 interface {
	clientv3.KV
	clientv3.Lease
	clientv3.Watcher
}

// ETCD .
type ETCD struct {
	cliv3  ETCDClientV3
	config types.EtcdConfig
}

// NewETCD initailizes a new ETCD instance.
func NewETCD(config types.EtcdConfig, embeddedStorage bool) (*ETCD, error) {
	var cliv3 *clientv3.Client
	var err error
	var tlsConfig *tls.Config

	switch {
	case embeddedStorage:
		cliv3 = embedded.NewCluster()
		log.Info("[Mercury] use embedded cluster")
	default:
		if config.Ca != "" && config.Key != "" && config.Cert != "" {
			tlsInfo := transport.TLSInfo{
				TrustedCAFile: config.Ca,
				KeyFile:       config.Key,
				CertFile:      config.Cert,
			}
			tlsConfig, err = tlsInfo.ClientConfig()
			if err != nil {
				return nil, err
			}
		}
		if cliv3, err = clientv3.New(clientv3.Config{
			Endpoints: config.Machines,
			Username:  config.Auth.Username,
			Password:  config.Auth.Password,
			TLS:       tlsConfig,
		}); err != nil {
			return nil, err
		}
	}
	cliv3.KV = namespace.NewKV(cliv3.KV, config.Prefix)
	cliv3.Watcher = namespace.NewWatcher(cliv3.Watcher, config.Prefix)
	cliv3.Lease = namespace.NewLease(cliv3.Lease, config.Prefix)
	return &ETCD{cliv3: cliv3, config: config}, nil
}

// ClientV3 gets the raw ETCD client v3.
func (e *ETCD) ClientV3() *clientv3.Client {
	return e.cliv3.(*clientv3.Client)
}

// TerminateEmbededStorage terminate embedded storage
func (e *ETCD) TerminateEmbededStorage() {
	embedded.TerminateCluster()
}

// CreateLock create a lock instance
func (e *ETCD) CreateLock(key string, ttl time.Duration) (lock.DistributedLock, error) {
	lockKey := fmt.Sprintf("%s/%s", e.config.LockPrefix, key)
	mutex, err := etcdlock.New(e.ClientV3(), lockKey, ttl)
	return mutex, err
}

// Get get results or noting
func (e *ETCD) Get(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.GetResponse, error) {
	return e.cliv3.Get(ctx, key, opts...)
}

// GetOne get one result or noting
func (e *ETCD) GetOne(ctx context.Context, key string, opts ...clientv3.OpOption) (*mvccpb.KeyValue, error) {
	resp, err := e.Get(ctx, key, opts...)
	if err != nil {
		return nil, err
	}
	if resp.Count != 1 {
		return nil, types.NewDetailedErr(types.ErrBadCount, fmt.Sprintf("key: %s", key))
	}
	return resp.Kvs[0], nil
}

// GetMulti gets several results
func (e *ETCD) GetMulti(ctx context.Context, keys []string, opts ...clientv3.OpOption) (kvs []*mvccpb.KeyValue, err error) {
	var txnResponse *clientv3.TxnResponse
	if len(keys) == 0 {
		return
	}
	if txnResponse, err = e.batchGet(ctx, keys); err != nil {
		return
	}
	for idx, responseOp := range txnResponse.Responses {
		resp := responseOp.GetResponseRange()
		if resp.Count != 1 {
			return nil, types.NewDetailedErr(types.ErrBadCount, fmt.Sprintf("key: %s", keys[idx]))
		}
		kvs = append(kvs, resp.Kvs[0])
	}
	if len(kvs) != len(keys) {
		err = types.NewDetailedErr(types.ErrBadCount, fmt.Sprintf("keys: %v", keys))
	}
	return
}

// Delete delete key
func (e *ETCD) Delete(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.DeleteResponse, error) {
	return e.cliv3.Delete(ctx, key, opts...)
}

// Put save a key value
func (e *ETCD) Put(ctx context.Context, key, val string, opts ...clientv3.OpOption) (*clientv3.PutResponse, error) {
	return e.cliv3.Put(ctx, key, val, opts...)
}

// Create create a key if not exists
func (e *ETCD) Create(ctx context.Context, key, val string, opts ...clientv3.OpOption) (*clientv3.TxnResponse, error) {
	return e.batchCreate(ctx, map[string]string{key: val}, opts...)
}

// Update update a key if exists
func (e *ETCD) Update(ctx context.Context, key, val string, opts ...clientv3.OpOption) (*clientv3.TxnResponse, error) {
	return e.batchUpdate(ctx, map[string]string{key: val}, opts...)
}

// Watch .
func (e *ETCD) Watch(ctx context.Context, key string, opts ...clientv3.OpOption) clientv3.WatchChan {
	return e.watch(ctx, key, opts...)
}

// Watch wath a key
func (e *ETCD) watch(ctx context.Context, key string, opts ...clientv3.OpOption) clientv3.WatchChan {
	return e.cliv3.Watch(ctx, key, opts...)
}

func (e *ETCD) batchGet(ctx context.Context, keys []string, opt ...clientv3.OpOption) (txnResponse *clientv3.TxnResponse, err error) {
	ops := []clientv3.Op{}
	for _, key := range keys {
		op := clientv3.OpGet(key, opt...)
		ops = append(ops, op)
	}
	return e.doBatchOp(ctx, nil, ops, nil)
}

// BatchDelete .
func (e *ETCD) BatchDelete(ctx context.Context, keys []string, opts ...clientv3.OpOption) (*clientv3.TxnResponse, error) {
	return e.batchDelete(ctx, keys, opts...)
}

func (e *ETCD) batchDelete(ctx context.Context, keys []string, opts ...clientv3.OpOption) (*clientv3.TxnResponse, error) {
	ops := []clientv3.Op{}
	for _, key := range keys {
		op := clientv3.OpDelete(key, opts...)
		ops = append(ops, op)
	}

	return e.doBatchOp(ctx, nil, ops, nil)
}

func (e *ETCD) batchPut(ctx context.Context, data map[string]string, limit map[string]map[string]string, opts ...clientv3.OpOption) (*clientv3.TxnResponse, error) {
	ops := []clientv3.Op{}
	failOps := []clientv3.Op{}
	conds := []clientv3.Cmp{}
	for key, val := range data {
		op := clientv3.OpPut(key, val, opts...)
		ops = append(ops, op)
		if v, ok := limit[key]; ok {
			for method, condition := range v {
				switch method {
				case cmpVersion:
					cond := clientv3.Compare(clientv3.Version(key), condition, 0)
					conds = append(conds, cond)
				case cmpValue:
					cond := clientv3.Compare(clientv3.Value(key), condition, val)
					failOps = append(failOps, clientv3.OpGet(key))
					conds = append(conds, cond)
				}
			}
		}
	}
	return e.doBatchOp(ctx, conds, ops, failOps)
}

// BatchCreate .
func (e *ETCD) BatchCreate(ctx context.Context, data map[string]string, opts ...clientv3.OpOption) (*clientv3.TxnResponse, error) {
	return e.batchCreate(ctx, data, opts...)
}

func (e *ETCD) batchCreate(ctx context.Context, data map[string]string, opts ...clientv3.OpOption) (*clientv3.TxnResponse, error) {
	limit := map[string]map[string]string{}
	for key := range data {
		limit[key] = map[string]string{cmpVersion: "="}
	}
	resp, err := e.batchPut(ctx, data, limit, opts...)
	if err != nil {
		return resp, err
	}
	if !resp.Succeeded {
		return resp, types.ErrKeyExists
	}
	return resp, nil
}

// BatchUpdate .
func (e *ETCD) BatchUpdate(ctx context.Context, data map[string]string, opts ...clientv3.OpOption) (*clientv3.TxnResponse, error) {
	return e.batchUpdate(ctx, data, opts...)
}

func (e *ETCD) batchUpdate(ctx context.Context, data map[string]string, opts ...clientv3.OpOption) (*clientv3.TxnResponse, error) {
	limit := map[string]map[string]string{}
	for key := range data {
		limit[key] = map[string]string{cmpVersion: "!=", cmpValue: "!="} // ignore same data
	}
	resp, err := e.batchPut(ctx, data, limit, opts...)
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

func (e *ETCD) doBatchOp(ctx context.Context, conds []clientv3.Cmp, ops, failOps []clientv3.Op) (*clientv3.TxnResponse, error) {
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
		txn := e.cliv3.Txn(ctx)
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
