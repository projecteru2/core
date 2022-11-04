package meta

import (
	"context"
	"crypto/tls"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/lock"
	"github.com/projecteru2/core/lock/etcdlock"
	"github.com/projecteru2/core/log"
	embedded "github.com/projecteru2/core/store/etcdv3/embedded"
	"github.com/projecteru2/core/types"

	"go.etcd.io/etcd/api/v3/mvccpb"
	"go.etcd.io/etcd/client/pkg/v3/transport"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/namespace"
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

// ETCDTxn wraps a group of Cmp with Op
type ETCDTxn struct {
	If   []clientv3.Cmp
	Then []clientv3.Op
	Else []clientv3.Op
}

// ETCDTxnResp wraps etcd response with error
type ETCDTxnResp struct {
	resp *clientv3.TxnResponse
	err  error
}

// NewETCD initailizes a new ETCD instance.
func NewETCD(config types.EtcdConfig, t *testing.T) (*ETCD, error) {
	var cliv3 *clientv3.Client
	var err error
	var tlsConfig *tls.Config

	switch {
	case t != nil:
		embededETCD := embedded.NewCluster(t, config.Prefix)
		cliv3 = embededETCD.RandClient()
		log.WithFunc("store.etcdv3.meta.NewETCD").Info(nil, "use embedded cluster") //nolint
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
		cliv3.KV = namespace.NewKV(cliv3.KV, config.Prefix)
		cliv3.Watcher = namespace.NewWatcher(cliv3.Watcher, config.Prefix)
		cliv3.Lease = namespace.NewLease(cliv3.Lease, config.Prefix)
	}
	return &ETCD{cliv3: cliv3, config: config}, nil
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
		return nil, errors.Wrapf(types.ErrInvaildCount, "key: %s", key)
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
			return nil, errors.Wrapf(types.ErrInvaildCount, "key: %s", keys[idx])
		}
		kvs = append(kvs, resp.Kvs[0])
	}
	if len(kvs) != len(keys) {
		err = errors.Wrapf(types.ErrInvaildCount, "keys: %+v", keys)
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
	txn := ETCDTxn{}
	for _, key := range keys {
		op := clientv3.OpGet(key, opt...)
		txn.Then = append(txn.Then, op)
	}
	return e.doBatchOp(ctx, []ETCDTxn{txn})
}

// BatchDelete .
func (e *ETCD) BatchDelete(ctx context.Context, keys []string, opts ...clientv3.OpOption) (*clientv3.TxnResponse, error) {
	return e.batchDelete(ctx, keys, opts...)
}

func (e *ETCD) batchDelete(ctx context.Context, keys []string, opts ...clientv3.OpOption) (*clientv3.TxnResponse, error) {
	txn := ETCDTxn{}
	for _, key := range keys {
		op := clientv3.OpDelete(key, opts...)
		txn.Then = append(txn.Then, op)
	}

	return e.doBatchOp(ctx, []ETCDTxn{txn})
}

func (e *ETCD) batchPut(ctx context.Context, data map[string]string, limit map[string]map[string]string, opts ...clientv3.OpOption) (*clientv3.TxnResponse, error) {
	txnes := []ETCDTxn{}
	for key, val := range data {
		txn := ETCDTxn{}
		op := clientv3.OpPut(key, val, opts...)
		txn.Then = append(txn.Then, op)
		if v, ok := limit[key]; ok {
			for method, condition := range v {
				switch method {
				case cmpVersion:
					cond := clientv3.Compare(clientv3.Version(key), condition, 0)
					txn.If = append(txn.If, cond)
				case cmpValue:
					cond := clientv3.Compare(clientv3.Value(key), condition, val)
					txn.Else = append(txn.Else, clientv3.OpGet(key))
					txn.If = append(txn.If, cond)
				}
			}
		}
		txnes = append(txnes, txn)
	}
	return e.doBatchOp(ctx, txnes)
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

// BatchPut .
func (e *ETCD) BatchPut(ctx context.Context, data map[string]string, opts ...clientv3.OpOption) (*clientv3.TxnResponse, error) {
	return e.batchPut(ctx, data, nil, opts...)
}

// isTTLChanged returns true if there is a lease with a different ttl bound to the key
func (e *ETCD) isTTLChanged(ctx context.Context, key string, ttl int64) (bool, error) {
	resp, err := e.GetOne(ctx, key)
	if err != nil {
		if errors.Is(err, types.ErrInvaildCount) {
			return ttl != 0, nil
		}
		return false, err
	}

	leaseID := clientv3.LeaseID(resp.Lease)
	if leaseID == 0 {
		return ttl != 0, nil
	}

	getTTLResp, err := e.cliv3.TimeToLive(ctx, leaseID)
	if err != nil {
		return false, err
	}

	changed := getTTLResp.GrantedTTL != ttl
	if changed {
		log.WithFunc("store.etcdv3.meta.isTTLChanged").Infof(ctx, "key %+v ttl changed from %+v to %+v", key, getTTLResp.GrantedTTL, ttl)
	}

	return changed, nil
}

// BindStatus keeps on a lease alive.
func (e *ETCD) BindStatus(ctx context.Context, entityKey, statusKey, statusValue string, ttl int64) error {
	if ttl == 0 {
		return e.bindStatusWithoutTTL(ctx, statusKey, statusValue)
	}
	return e.bindStatusWithTTL(ctx, entityKey, statusKey, statusValue, ttl)
}

func (e *ETCD) bindStatusWithTTL(ctx context.Context, entityKey, statusKey, statusValue string, ttl int64) error {
	lease, err := e.Grant(ctx, ttl)
	if err != nil {
		return err
	}

	leaseID := lease.ID
	updateStatus := []clientv3.Op{clientv3.OpPut(statusKey, statusValue, clientv3.WithLease(lease.ID))}
	logger := log.WithFunc("store.etcdv3.meta.bindStatusWithTTL")

	ttlChanged, err := e.isTTLChanged(ctx, statusKey, ttl)
	if err != nil {
		return err
	}

	var entityTxn *clientv3.TxnResponse

	if ttlChanged {
		entityTxn, err = e.cliv3.Txn(ctx).
			If(clientv3.Compare(clientv3.Version(entityKey), "!=", 0)).
			Then(updateStatus...). // making sure there's an exists entity kv-pair.
			Commit()
	} else {
		entityTxn, err = e.cliv3.Txn(ctx).
			If(clientv3.Compare(clientv3.Version(entityKey), "!=", 0)).
			Then( // making sure there's an exists entity kv-pair.
				clientv3.OpTxn(
					[]clientv3.Cmp{clientv3.Compare(clientv3.Version(statusKey), "!=", 0)}, // Is the status exists?
					[]clientv3.Op{clientv3.OpTxn( // there's an exists status
						[]clientv3.Cmp{clientv3.Compare(clientv3.LeaseValue(statusKey), "!=", 0)}, //
						[]clientv3.Op{clientv3.OpTxn( // there has been a lease bound to the status
							[]clientv3.Cmp{clientv3.Compare(clientv3.Value(statusKey), "=", statusValue)}, // Is the status changed?
							[]clientv3.Op{clientv3.OpGet(statusKey)},                                      // The status hasn't been changed.
							updateStatus,                                                                  // The status had been changed.
						)},
						updateStatus, // there is no lease bound to the status
					)},
					updateStatus, // there isn't a status
				),
			).Commit()
	}

	if err != nil {
		e.revokeLease(ctx, leaseID)
		return err
	}

	// There isn't the entity kv pair.
	if !entityTxn.Succeeded {
		e.revokeLease(ctx, leaseID)
		return types.ErrInvaildCount
	}

	// if ttl is changed, replace with the new lease
	if ttlChanged {
		logger.Infof(ctx, "put: key %s value %s", statusKey, statusValue)
		return nil
	}

	// There isn't a status bound to the entity.
	statusTxn := entityTxn.Responses[0].GetResponseTxn()
	if !statusTxn.Succeeded {
		logger.Infof(ctx, "put: key %s value %s", statusKey, statusValue)
		return nil
	}

	// There is no lease bound to the status yet
	leaseTxn := statusTxn.Responses[0].GetResponseTxn()
	if !leaseTxn.Succeeded {
		logger.Infof(ctx, "put: key %s value %s", statusKey, statusValue)
		return nil
	}

	// There is a status bound to the entity yet but its value isn't same as the expected one.
	valueTxn := leaseTxn.Responses[0].GetResponseTxn()
	if !valueTxn.Succeeded {
		logger.Infof(ctx, "put: key %s value %s", statusKey, statusValue)
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

// bindStatusWithoutTTL sets status without TTL.
// When dealing with status of 0 TTL, we don't use lease,
// also we don't check the existence of the entity key since
// agent may report status earlier when core has not recorded the entity.
func (e *ETCD) bindStatusWithoutTTL(ctx context.Context, statusKey, statusValue string) error {
	updateStatus := []clientv3.Op{clientv3.OpPut(statusKey, statusValue)}
	logger := log.WithFunc("store.etcdv3.etcd.bindStatusWithoutTTL")

	ttlChanged, err := e.isTTLChanged(ctx, statusKey, 0)
	if err != nil {
		return err
	}
	if ttlChanged {
		_, err := e.Put(ctx, statusKey, statusValue)
		if err != nil {
			return err
		}

		logger.Infof(ctx, "put: key %s value %s", statusKey, statusValue)
		return nil
	}

	resp, err := e.cliv3.Txn(ctx).
		If(clientv3.Compare(clientv3.Version(statusKey), "!=", 0)). // if there's an existing status key
		Then(clientv3.OpTxn(                                        // deal with existing status key
			[]clientv3.Cmp{clientv3.Compare(clientv3.Value(statusKey), "!=", statusValue)}, // if the new value != the old value
			updateStatus,    // then the status has been changed.
			[]clientv3.Op{}, // otherwise do nothing.
		)).
		Else(updateStatus...). // otherwise deal with non-existing status key
		Commit()
	if err != nil {
		return err
	}
	if !resp.Succeeded || resp.Responses[0].GetResponseTxn().Succeeded {
		logger.Infof(ctx, "put: key %s value %s", statusKey, statusValue)
	}
	return nil
}

func (e *ETCD) revokeLease(ctx context.Context, leaseID clientv3.LeaseID) {
	if leaseID == 0 {
		return
	}
	if _, err := e.cliv3.Revoke(ctx, leaseID); err != nil {
		log.WithFunc("store.etcdv3.etcd.revokeLease").Error(ctx, err, "revoke lease failed")
	}
}

// Grant creates a new lease.
func (e *ETCD) Grant(ctx context.Context, ttl int64) (*clientv3.LeaseGrantResponse, error) {
	return e.cliv3.Grant(ctx, ttl)
}

func (e *ETCD) batchUpdate(ctx context.Context, data map[string]string, opts ...clientv3.OpOption) (*clientv3.TxnResponse, error) {
	limit := map[string]map[string]string{}
	for key := range data {
		limit[key] = map[string]string{cmpVersion: "!="} // check existence
	}
	resp, err := e.batchPut(ctx, data, limit, opts...)
	if err != nil {
		return resp, err
	}
	if !resp.Succeeded {
		return resp, types.ErrKeyNotExists
	}
	return resp, nil
}

func (e *ETCD) doBatchOp(ctx context.Context, transactions []ETCDTxn) (resp *clientv3.TxnResponse, err error) {
	if len(transactions) == 0 {
		return nil, types.ErrNoOps
	}

	const txnLimit = 125

	// split transactions into smaller pieces
	txnes := []ETCDTxn{}
	for _, txn := range transactions {
		// TODO@zc: split if and else
		if len(txn.Then) <= txnLimit {
			txnes = append(txnes, txn)
			continue
		}

		n, m := len(txn.Then)/txnLimit, len(txn.Then)%txnLimit
		for i := 0; i < n; i++ {
			txnes = append(txnes, ETCDTxn{
				If:   txn.If,
				Then: txn.Then[i*txnLimit : (i+1)*txnLimit],
				Else: txn.Else,
			})
		}
		if m > 0 {
			txnes = append(txnes, ETCDTxn{
				If:   txn.If,
				Then: txn.Then[n*txnLimit:],
				Else: txn.Else,
			})
		}
	}

	wg := sync.WaitGroup{}
	respChan := make(chan ETCDTxnResp)
	doOp := func(from, to int) {
		defer wg.Done()
		conds, thens, elses := []clientv3.Cmp{}, []clientv3.Op{}, []clientv3.Op{}
		for i := from; i < to; i++ {
			conds = append(conds, txnes[i].If...)
			thens = append(thens, txnes[i].Then...)
			elses = append(elses, txnes[i].Else...)
		}
		resp, err := e.cliv3.Txn(ctx).If(conds...).Then(thens...).Else(elses...).Commit()
		respChan <- ETCDTxnResp{resp: resp, err: err}
	}

	lastIdx := 0 // last uncommit index
	lenIf, lenThen, lenElse := 0, 0, 0
	for i := 0; i < len(txnes); i++ {
		if lenIf+len(txnes[i].If) > txnLimit ||
			lenThen+len(txnes[i].Then) > txnLimit ||
			lenElse+len(txnes[i].Else) > txnLimit {
			wg.Add(1)
			go doOp(lastIdx, i) // [lastIdx, i)

			lastIdx = i
			lenIf, lenThen, lenElse = 0, 0, 0
		}

		lenIf += len(txnes[i].If)
		lenThen += len(txnes[i].Then)
		lenElse += len(txnes[i].Else)
	}
	wg.Add(1)
	go doOp(lastIdx, len(txnes))

	go func() {
		wg.Wait()
		close(respChan)
	}()

	resps := []ETCDTxnResp{}
	for resp := range respChan {
		resps = append(resps, resp)
		if resp.err != nil {
			err = resp.err
		}
	}
	if err != nil {
		return
	}

	if len(resps) == 0 {
		return &clientv3.TxnResponse{}, nil
	}

	resp = resps[0].resp
	// TODO@zc: should rollback all for any unsucceed txn
	for i := 1; i < len(resps); i++ {
		resp.Succeeded = resp.Succeeded && resps[i].resp.Succeeded
		resp.Responses = append(resp.Responses, resps[i].resp.Responses...)
	}
	return resp, nil
}

// BatchCreateAndDecr used to decr processing and add workload
func (e *ETCD) BatchCreateAndDecr(ctx context.Context, data map[string]string, decrKey string) (err error) {
	resp, err := e.Get(ctx, decrKey)
	if err != nil {
		return
	}
	if len(resp.Kvs) == 0 {
		return errors.Wrap(types.ErrKeyNotExists, decrKey)
	}

	decrKv := resp.Kvs[0]
	putOps := []clientv3.Op{}
	for key, value := range data {
		putOps = append(putOps, clientv3.OpPut(key, value))
	}

	for {
		cnt, err := strconv.Atoi(string(decrKv.Value))
		if err != nil {
			return err
		}

		txn := ETCDTxn{
			If: []clientv3.Cmp{
				clientv3.Compare(clientv3.Value(decrKey), "=", string(decrKv.Value)),
			},
			Then: append(putOps,
				clientv3.OpPut(decrKey, strconv.Itoa(cnt-1)),
			),
			Else: []clientv3.Op{
				clientv3.OpGet(decrKey),
			},
		}
		txnResp, err := e.doBatchOp(ctx, []ETCDTxn{txn})
		if err != nil {
			return err
		}
		if txnResp.Succeeded {
			break
		}
		decrKv = txnResp.Responses[0].GetResponseRange().Kvs[0]
	}

	return nil
}
