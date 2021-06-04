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
	embedded "github.com/projecteru2/core/store/etcdv3/embedded"
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

	embededETCD *embedded.EmbededETCD
}

// NewETCD initailizes a new ETCD instance.
func NewETCD(config types.EtcdConfig, embeddedStorage bool) (*ETCD, error) {
	var cliv3 *clientv3.Client
	var embededETCD *embedded.EmbededETCD
	var err error
	var tlsConfig *tls.Config

	switch {
	case embeddedStorage:
		embededETCD = embedded.NewCluster()
		cliv3 = embededETCD.Cluster.RandClient()
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
	return &ETCD{cliv3: cliv3, config: config, embededETCD: embededETCD}, nil
}

// TerminateEmbededStorage terminate embedded storage
func (e *ETCD) TerminateEmbededStorage() {
	if e.embededETCD == nil {
		return
	}
	e.embededETCD.TerminateCluster()
}

// CreateLock create a lock instance
func (e *ETCD) CreateLock(key string, ttl time.Duration) (lock.DistributedLock, error) {
	lockKey := fmt.Sprintf("%s/%s", e.config.LockPrefix, key)
	mutex, err := etcdlock.New(e.cliv3.(*clientv3.Client), lockKey, ttl)
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

func (e *ETCD) bindStatusWithoutLease(ctx context.Context, entityKey, statusKey, statusValue string) error {
	updateStatus := []clientv3.Op{clientv3.OpPut(statusKey, statusValue)}
	_, err := e.cliv3.Txn(ctx).
		If(clientv3.Compare(clientv3.Version(entityKey), "!=", 0)).
		Then( // making sure there's an exists entity kv-pair.
			clientv3.OpTxn(
				[]clientv3.Cmp{clientv3.Compare(clientv3.Version(statusKey), "!=", 0)}, // Is the status exists?
				[]clientv3.Op{clientv3.OpTxn( // there's an exists status
					[]clientv3.Cmp{clientv3.Compare(clientv3.Value(statusKey), "!=", statusValue)},
					updateStatus,    // The status had been changed.
					[]clientv3.Op{}, // The status hasn't been changed.
				)},
				updateStatus, // there isn't a status
			),
		).Commit()
	return err
}

// bindStatusWithLease does two step:
// 1. check if entityKey exists: if not, return error
// 2. check if statusKey exists:
//    2.1 statusKey doesn't exist: create key-value pair with a newly granted lease
//    2.2 statusKey exists, we get current lease and current value, if:
//        2.2.1 current lease is 0: actually its' impossible, but for compatibility, we create key-value pair with newly granted lease
//        2.2.2 current lease is not 0, we check status value, if:
//        2.2.2.1 status value == current value: keep alive the current lease
//        2.2.2.2 status value != current value: create key-value pair with newly granted lease
func (e *ETCD) bindStatusWithLease(ctx context.Context, entityKey, statusKey, statusValue string, ttl int64) error {
	// entity doen't exist, return err
	if _, err := e.cliv3.Get(ctx, entityKey); err != nil {
		return err
	}

	statusKeyResp, err := e.cliv3.Get(ctx, statusKey)
	// statusKey doesn't exist, create and return
	if err != nil || len(statusKeyResp.Kvs) == 0 {
		return e.putKeyWithNewLease(ctx, statusKey, statusValue, ttl)
	}

	kv := statusKeyResp.Kvs[0]
	currentValue := string(kv.Value)
	currentLease := clientv3.LeaseID(kv.Lease)

	// Has current lease and value doesn't change.
	// Refresh the current lease.
	if currentLease > 0 && currentValue == statusValue {
		_, err := e.cliv3.KeepAliveOnce(ctx, currentLease)
		return err
	}

	// Otherwise create
	return e.putKeyWithNewLease(ctx, statusKey, statusValue, ttl)
}

// putKeyWithNewLease grants a new lease, and bind it with the key-value pair.
func (e *ETCD) putKeyWithNewLease(ctx context.Context, key, value string, ttl int64) error {
	lease, err := e.Grant(ctx, ttl)
	if err != nil {
		return err
	}
	_, err = e.cliv3.Put(ctx, key, value, clientv3.WithLease(lease.ID))
	return err
}

// BindStatus sets statusKey to statusValue, will create or refresh a lease if ttl > 0
func (e *ETCD) BindStatus(ctx context.Context, entityKey, statusKey, statusValue string, ttl int64) error {
	if ttl == 0 {
		return e.bindStatusWithoutLease(ctx, entityKey, statusKey, statusValue)
	}
	return e.bindStatusWithLease(ctx, entityKey, statusKey, statusValue, ttl)
}

// KeepAliveOnce keeps on a lease alive.
func (e *ETCD) BindStatus2(ctx context.Context, entityKey, statusKey, statusValue string, ttl int64) error {
	var leaseID clientv3.LeaseID
	updateStatus := []clientv3.Op{clientv3.OpPut(statusKey, statusValue)}
	if ttl != 0 {
		lease, err := e.Grant(ctx, ttl)
		if err != nil {
			return err
		}
		leaseID = lease.ID
		updateStatus = []clientv3.Op{clientv3.OpPut(statusKey, statusValue, clientv3.WithLease(lease.ID))}
	}

	entityTxn, err := e.cliv3.Txn(ctx).
		If(clientv3.Compare(clientv3.Version(entityKey), "!=", 0)).
		Then( // making sure there's an exists entity kv-pair.
			clientv3.OpTxn(
				[]clientv3.Cmp{clientv3.Compare(clientv3.Version(statusKey), "!=", 0)}, // Is the status exists?
				[]clientv3.Op{clientv3.OpTxn( // there's an exists status
					[]clientv3.Cmp{clientv3.Compare(clientv3.Value(statusKey), "=", statusValue)},
					[]clientv3.Op{clientv3.OpGet(statusKey)}, // The status hasn't been changed.
					updateStatus,                             // The status had been changed.
				)},
				updateStatus, // there isn't a status
			),
		).Commit()
	if err != nil {
		e.revokeLease(ctx, leaseID)
		return err
	}

	// There isn't the entity kv pair.
	if !entityTxn.Succeeded {
		e.revokeLease(ctx, leaseID)
		return nil
	}

	// There isn't a status bound to the entity.
	statusTxn := entityTxn.Responses[0].GetResponseTxn()
	if !statusTxn.Succeeded {
		return nil
	}

	// A zero TTL means it doesn't affect anything
	if ttl == 0 {
		return nil
	}

	// There is a status bound to the entity yet but its value isn't same as the expected one.
	valueTxn := statusTxn.Responses[0].GetResponseTxn()
	if !valueTxn.Succeeded {
		return nil
	}

	// Gets the lease ID which binds onto the status, and renew it one round.
	origLeaseID := clientv3.LeaseID(valueTxn.Responses[0].GetResponseRange().Kvs[0].Lease)

	if origLeaseID != leaseID {
		e.revokeLease(ctx, leaseID)
	}

	_, err = e.cliv3.KeepAliveOnce(ctx, origLeaseID)
	return err
}

func (e *ETCD) revokeLease(ctx context.Context, leaseID clientv3.LeaseID) {
	if leaseID == 0 {
		return
	}
	if _, err := e.cliv3.Revoke(ctx, leaseID); err != nil {
		log.Errorf(ctx, "[etcd revoke lease error] %v", err)
	}
}

// Grant creates a new lease.
func (e *ETCD) Grant(ctx context.Context, ttl int64) (*clientv3.LeaseGrantResponse, error) {
	return e.cliv3.Grant(ctx, ttl)
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
