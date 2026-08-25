package meta

import (
	"context"
	"crypto/tls"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"go.etcd.io/etcd/api/v3/mvccpb"
	"go.etcd.io/etcd/client/pkg/v3/transport"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/namespace"

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

// ETCDClientV3 is the etcd client surface the store depends on.
type ETCDClientV3 interface {
	clientv3.KV
	clientv3.Lease
	clientv3.Watcher
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

// ETCD is the etcd backed meta store.
type ETCD struct {
	cliv3  ETCDClientV3
	config types.EtcdConfig
}

func NewETCD(config types.EtcdConfig, embeddedETCD *embedded.Cluster) (*ETCD, error) {
	var cliv3 *clientv3.Client
	var err error
	var tlsConfig *tls.Config

	switch {
	case embeddedETCD != nil:
		cliv3 = embeddedETCD.Client(config.Prefix)
		log.WithFunc("store.etcdv3.meta.NewETCD").Info(context.Background(), "use embedded cluster")
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

func (e *ETCD) CreateLock(key string, ttl time.Duration) (lock.DistributedLock, error) {
	lockKey := fmt.Sprintf("%s/%s", e.config.LockPrefix, key)
	mutex, err := etcdlock.New(e.cliv3.(*clientv3.Client), lockKey, ttl)
	return mutex, err
}

func (e *ETCD) Get(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.GetResponse, error) {
	return e.cliv3.Get(ctx, key, opts...)
}

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

func (e *ETCD) GetMulti(ctx context.Context, keys []string, _ ...clientv3.OpOption) (kvs []*mvccpb.KeyValue, err error) {
	var txnResponse *clientv3.TxnResponse
	if len(keys) == 0 {
		return kvs, err
	}
	if txnResponse, err = e.batchGet(ctx, keys); err != nil {
		return kvs, err
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
	return kvs, err
}

func (e *ETCD) Delete(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.DeleteResponse, error) {
	return e.cliv3.Delete(ctx, key, opts...)
}

func (e *ETCD) Put(ctx context.Context, key, val string, opts ...clientv3.OpOption) (*clientv3.PutResponse, error) {
	return e.cliv3.Put(ctx, key, val, opts...)
}

func (e *ETCD) Create(ctx context.Context, key, val string, opts ...clientv3.OpOption) (*clientv3.TxnResponse, error) {
	return e.BatchCreate(ctx, map[string]string{key: val}, opts...)
}

func (e *ETCD) Watch(ctx context.Context, key string, opts ...clientv3.OpOption) clientv3.WatchChan {
	return e.cliv3.Watch(ctx, key, opts...)
}

func (e *ETCD) BatchDelete(ctx context.Context, keys []string, opts ...clientv3.OpOption) (*clientv3.TxnResponse, error) {
	txn := ETCDTxn{}
	for _, key := range keys {
		txn.Then = append(txn.Then, clientv3.OpDelete(key, opts...))
	}
	return e.doBatchOp(ctx, []ETCDTxn{txn})
}

func (e *ETCD) BatchCreate(ctx context.Context, data map[string]string, opts ...clientv3.OpOption) (*clientv3.TxnResponse, error) {
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

func (e *ETCD) BatchUpdate(ctx context.Context, data map[string]string, opts ...clientv3.OpOption) (*clientv3.TxnResponse, error) {
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

func (e *ETCD) BatchPut(ctx context.Context, data map[string]string, opts ...clientv3.OpOption) (*clientv3.TxnResponse, error) {
	return e.batchPut(ctx, data, nil, opts...)
}

func (e *ETCD) BindStatus(ctx context.Context, entityKey, statusKey, statusValue string, ttl int64) error {
	if ttl == 0 {
		return e.bindStatusWithoutTTL(ctx, statusKey, statusValue)
	}
	return e.bindStatusWithTTL(ctx, entityKey, statusKey, statusValue, ttl)
}

func (e *ETCD) BatchCreateAndDecr(ctx context.Context, data map[string]string, decrKey string) (err error) {
	resp, err := e.Get(ctx, decrKey)
	if err != nil {
		return err
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

func (e *ETCD) batchGet(ctx context.Context, keys []string, opt ...clientv3.OpOption) (txnResponse *clientv3.TxnResponse, err error) {
	txn := ETCDTxn{}
	for _, key := range keys {
		txn.Then = append(txn.Then, clientv3.OpGet(key, opt...))
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

func (e *ETCD) grant(ctx context.Context, ttl int64) (*clientv3.LeaseGrantResponse, error) {
	return e.cliv3.Grant(ctx, ttl)
}

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

func (e *ETCD) bindStatusWithTTL(ctx context.Context, entityKey, statusKey, statusValue string, ttl int64) error {
	lease, err := e.grant(ctx, ttl)
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
			Then(updateStatus...).
			Commit()
	} else {
		entityTxn, err = e.cliv3.Txn(ctx).
			If(clientv3.Compare(clientv3.Version(entityKey), "!=", 0)).
			Then(
				clientv3.OpTxn(
					[]clientv3.Cmp{clientv3.Compare(clientv3.Version(statusKey), "!=", 0)},
					[]clientv3.Op{clientv3.OpTxn(
						[]clientv3.Cmp{clientv3.Compare(clientv3.LeaseValue(statusKey), "!=", 0)},
						[]clientv3.Op{clientv3.OpTxn(
							[]clientv3.Cmp{clientv3.Compare(clientv3.Value(statusKey), "=", statusValue)},
							[]clientv3.Op{clientv3.OpGet(statusKey)},
							updateStatus,
						)},
						updateStatus,
					)},
					updateStatus,
				),
			).Commit()
	}

	if err != nil {
		e.revokeLease(ctx, leaseID)
		return err
	}

	if !entityTxn.Succeeded {
		e.revokeLease(ctx, leaseID)
		return types.ErrInvaildCount
	}

	if ttlChanged {
		logger.Infof(ctx, "put: key %s value %s", statusKey, statusValue)
		return nil
	}

	statusTxn := entityTxn.Responses[0].GetResponseTxn()
	if !statusTxn.Succeeded {
		logger.Infof(ctx, "put: key %s value %s", statusKey, statusValue)
		return nil
	}

	leaseTxn := statusTxn.Responses[0].GetResponseTxn()
	if !leaseTxn.Succeeded {
		logger.Infof(ctx, "put: key %s value %s", statusKey, statusValue)
		return nil
	}

	valueTxn := leaseTxn.Responses[0].GetResponseTxn()
	if !valueTxn.Succeeded {
		logger.Infof(ctx, "put: key %s value %s", statusKey, statusValue)
		return nil
	}

	origLeaseID := clientv3.LeaseID(valueTxn.Responses[0].GetResponseRange().Kvs[0].Lease)

	if origLeaseID != leaseID {
		e.revokeLease(ctx, leaseID)
	}

	_, err = e.cliv3.KeepAliveOnce(ctx, origLeaseID)
	return err
}

// bindStatusWithoutTTL skips the entity check: an agent may report status before core records the entity.
func (e *ETCD) bindStatusWithoutTTL(ctx context.Context, statusKey, statusValue string) error {
	updateStatus := []clientv3.Op{clientv3.OpPut(statusKey, statusValue)}
	logger := log.WithFunc("store.etcdv3.etcd.bindStatusWithoutTTL")

	ttlChanged, err := e.isTTLChanged(ctx, statusKey, 0)
	if err != nil {
		return err
	}
	if ttlChanged {
		if _, err = e.Put(ctx, statusKey, statusValue); err != nil {
			return err
		}

		logger.Infof(ctx, "put: key %s value %s", statusKey, statusValue)
		return nil
	}

	resp, err := e.cliv3.Txn(ctx).
		If(clientv3.Compare(clientv3.Version(statusKey), "!=", 0)).
		Then(clientv3.OpTxn(
			[]clientv3.Cmp{clientv3.Compare(clientv3.Value(statusKey), "!=", statusValue)},
			updateStatus,
			[]clientv3.Op{},
		)).
		Else(updateStatus...).
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

func (e *ETCD) doBatchOp(ctx context.Context, transactions []ETCDTxn) (resp *clientv3.TxnResponse, err error) {
	if len(transactions) == 0 {
		return nil, types.ErrNoOps
	}

	const txnLimit = 125

	txnes := []ETCDTxn{}
	for _, txn := range transactions {
		if len(txn.Then) <= txnLimit {
			txnes = append(txnes, txn)
			continue
		}

		n, m := len(txn.Then)/txnLimit, len(txn.Then)%txnLimit
		for i := range n {
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
	commit := func(from, to int) {
		wg.Go(func() {
			conds, thens, elses := []clientv3.Cmp{}, []clientv3.Op{}, []clientv3.Op{}
			for _, txn := range txnes[from:to] {
				conds = append(conds, txn.If...)
				thens = append(thens, txn.Then...)
				elses = append(elses, txn.Else...)
			}
			txnResp, txnErr := e.cliv3.Txn(ctx).If(conds...).Then(thens...).Else(elses...).Commit()
			respChan <- ETCDTxnResp{resp: txnResp, err: txnErr}
		})
	}

	lastIdx := 0
	lenIf, lenThen, lenElse := 0, 0, 0
	for i := range txnes {
		if lenIf+len(txnes[i].If) > txnLimit ||
			lenThen+len(txnes[i].Then) > txnLimit ||
			lenElse+len(txnes[i].Else) > txnLimit {
			commit(lastIdx, i)

			lastIdx = i
			lenIf, lenThen, lenElse = 0, 0, 0
		}

		lenIf += len(txnes[i].If)
		lenThen += len(txnes[i].Then)
		lenElse += len(txnes[i].Else)
	}
	commit(lastIdx, len(txnes))

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
		return resp, err
	}

	if len(resps) == 0 {
		return &clientv3.TxnResponse{}, nil
	}

	resp = resps[0].resp
	for _, r := range resps[1:] {
		resp.Succeeded = resp.Succeeded && r.resp.Succeeded
		resp.Responses = append(resp.Responses, r.resp.Responses...)
	}
	return resp, nil
}
