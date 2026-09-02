package meta

import (
	"context"
	"crypto/tls"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/cockroachdb/errors"
	"go.etcd.io/etcd/api/v3/mvccpb"
	"go.etcd.io/etcd/client/pkg/v3/transport"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/namespace"
	"golang.org/x/sync/errgroup"

	"github.com/projecteru2/core/lock"
	"github.com/projecteru2/core/lock/etcdlock"
	"github.com/projecteru2/core/log"
	embedded "github.com/projecteru2/core/store/etcdv3/embedded"
	"github.com/projecteru2/core/types"
)

const (
	cmpVersion = "version"
	cmpValue   = "value"

	txnLimit = 125

	orphanStatusTTL = int64(time.Hour / time.Second)
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

type txnCond struct {
	method    string
	condition string
}

// ETCD is the etcd backed meta store.
type ETCD struct {
	cliv3  ETCDClientV3
	config types.EtcdConfig
}

func NewETCD(ctx context.Context, config types.EtcdConfig, embeddedETCD *embedded.Cluster) (*ETCD, error) {
	var cliv3 *clientv3.Client
	var err error
	var tlsConfig *tls.Config

	switch {
	case embeddedETCD != nil:
		cliv3 = embeddedETCD.Client(config.Prefix)
		log.WithFunc("store.etcdv3.meta.NewETCD").Info(ctx, "use embedded cluster")
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

func (e *ETCD) GetMulti(ctx context.Context, keys []string) ([]*mvccpb.KeyValue, error) {
	if len(keys) == 0 {
		return nil, nil
	}
	txnResponse, err := e.batchGet(ctx, keys)
	if err != nil {
		return nil, err
	}
	kvs := make([]*mvccpb.KeyValue, 0, len(keys))
	for idx, responseOp := range txnResponse.Responses {
		resp := responseOp.GetResponseRange()
		if resp.Count != 1 {
			return nil, errors.Wrapf(types.ErrInvaildCount, "key: %s", keys[idx])
		}
		kvs = append(kvs, resp.Kvs[0])
	}
	return kvs, nil
}

func (e *ETCD) Delete(ctx context.Context, key string) (*clientv3.DeleteResponse, error) {
	return e.cliv3.Delete(ctx, key)
}

func (e *ETCD) Put(ctx context.Context, key, val string) (*clientv3.PutResponse, error) {
	return e.cliv3.Put(ctx, key, val)
}

func (e *ETCD) Create(ctx context.Context, key, val string) (*clientv3.TxnResponse, error) {
	return e.BatchCreate(ctx, map[string]string{key: val})
}

func (e *ETCD) Watch(ctx context.Context, key string, opts ...clientv3.OpOption) clientv3.WatchChan {
	return e.cliv3.Watch(ctx, key, opts...)
}

func (e *ETCD) BatchDelete(ctx context.Context, keys []string) (*clientv3.TxnResponse, error) {
	txn := ETCDTxn{}
	for _, key := range keys {
		txn.Then = append(txn.Then, clientv3.OpDelete(key))
	}
	return e.doBatchOp(ctx, []ETCDTxn{txn})
}

func (e *ETCD) BatchCreate(ctx context.Context, data map[string]string) (*clientv3.TxnResponse, error) {
	resp, err := e.batchPut(ctx, data, &txnCond{method: cmpVersion, condition: "="})
	if err != nil {
		return resp, err
	}
	if !resp.Succeeded {
		return resp, types.ErrKeyExists
	}
	return resp, nil
}

func (e *ETCD) BatchUpdate(ctx context.Context, data map[string]string) (*clientv3.TxnResponse, error) {
	resp, err := e.batchPut(ctx, data, &txnCond{method: cmpVersion, condition: "!="})
	if err != nil {
		return resp, err
	}
	if !resp.Succeeded {
		return resp, types.ErrKeyNotExists
	}
	return resp, nil
}

func (e *ETCD) BatchPut(ctx context.Context, data map[string]string) (*clientv3.TxnResponse, error) {
	return e.batchPut(ctx, data, nil)
}

func (e *ETCD) BindStatus(ctx context.Context, entityKey, statusKey, statusValue string, ttl int64) error {
	if ttl == 0 {
		return e.bindStatusWithoutTTL(ctx, entityKey, statusKey, statusValue)
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

func (e *ETCD) batchGet(ctx context.Context, keys []string) (*clientv3.TxnResponse, error) {
	txn := ETCDTxn{}
	for _, key := range keys {
		txn.Then = append(txn.Then, clientv3.OpGet(key))
	}
	return e.doBatchOp(ctx, []ETCDTxn{txn})
}

func (e *ETCD) batchPut(ctx context.Context, data map[string]string, cond *txnCond) (*clientv3.TxnResponse, error) {
	txnes := []ETCDTxn{}
	for key, val := range data {
		txn := ETCDTxn{Then: []clientv3.Op{clientv3.OpPut(key, val)}}
		if cond != nil {
			switch cond.method {
			case cmpVersion:
				txn.If = append(txn.If, clientv3.Compare(clientv3.Version(key), cond.condition, 0))
			case cmpValue:
				txn.If = append(txn.If, clientv3.Compare(clientv3.Value(key), cond.condition, val))
				txn.Else = append(txn.Else, clientv3.OpGet(key))
			}
		}
		txnes = append(txnes, txn)
	}
	return e.doBatchOp(ctx, txnes)
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
	lease, err := e.cliv3.Grant(ctx, ttl)
	if err != nil {
		return err
	}

	leaseID := lease.ID
	updateStatus := []clientv3.Op{clientv3.OpPut(statusKey, statusValue, clientv3.WithLease(leaseID))}
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

	valueTxn := entityTxn.Responses[0].GetResponseTxn()
	for range 2 {
		if !valueTxn.Succeeded {
			logger.Infof(ctx, "put: key %s value %s", statusKey, statusValue)
			return nil
		}
		valueTxn = valueTxn.Responses[0].GetResponseTxn()
	}
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

func (e *ETCD) bindStatusWithoutTTL(ctx context.Context, entityKey, statusKey, statusValue string) error {
	logger := log.WithFunc("store.etcdv3.meta.bindStatusWithoutTTL")

	resp, err := e.cliv3.Txn(ctx).
		If(clientv3.Compare(clientv3.Version(entityKey), "!=", 0)).
		Then(clientv3.OpTxn(
			[]clientv3.Cmp{
				clientv3.Compare(clientv3.Value(statusKey), "=", statusValue),
				clientv3.Compare(clientv3.LeaseValue(statusKey), "=", 0),
			},
			[]clientv3.Op{},
			[]clientv3.Op{clientv3.OpPut(statusKey, statusValue)},
		)).
		Commit()
	if err != nil {
		return err
	}
	if resp.Succeeded {
		if !resp.Responses[0].GetResponseTxn().Succeeded {
			logger.Infof(ctx, "put: key %s value %s", statusKey, statusValue)
		}
		return nil
	}

	lease, err := e.cliv3.Grant(ctx, orphanStatusTTL)
	if err != nil {
		return err
	}
	orphaned, err := e.cliv3.Txn(ctx).
		If(clientv3.Compare(clientv3.Version(entityKey), "=", 0)).
		Then(clientv3.OpPut(statusKey, statusValue, clientv3.WithLease(lease.ID))).
		Else(clientv3.OpPut(statusKey, statusValue)).
		Commit()
	if err != nil {
		e.revokeLease(ctx, lease.ID)
		return err
	}
	if !orphaned.Succeeded {
		e.revokeLease(ctx, lease.ID)
		logger.Infof(ctx, "put: key %s value %s", statusKey, statusValue)
		return nil
	}
	logger.Infof(ctx, "put: key %s value %s, leased %ds without %s", statusKey, statusValue, orphanStatusTTL, entityKey)
	return nil
}

func (e *ETCD) revokeLease(ctx context.Context, leaseID clientv3.LeaseID) {
	if leaseID == 0 {
		return
	}
	if _, err := e.cliv3.Revoke(ctx, leaseID); err != nil {
		log.WithFunc("store.etcdv3.meta.revokeLease").Error(ctx, err, "revoke lease failed")
	}
}

func (e *ETCD) doBatchOp(ctx context.Context, transactions []ETCDTxn) (*clientv3.TxnResponse, error) {
	if len(transactions) == 0 {
		return nil, types.ErrNoOps
	}

	txnes := []ETCDTxn{}
	for _, txn := range transactions {
		if len(txn.Then) <= txnLimit {
			txnes = append(txnes, txn)
			continue
		}

		for chunk := range slices.Chunk(txn.Then, txnLimit) {
			txnes = append(txnes, ETCDTxn{If: txn.If, Then: chunk, Else: txn.Else})
		}
	}

	type span struct{ from, to int }
	spans := []span{}
	lastIdx := 0
	lenIf, lenThen, lenElse := 0, 0, 0
	for i := range txnes {
		if lenIf+len(txnes[i].If) > txnLimit ||
			lenThen+len(txnes[i].Then) > txnLimit ||
			lenElse+len(txnes[i].Else) > txnLimit {
			spans = append(spans, span{lastIdx, i})

			lastIdx = i
			lenIf, lenThen, lenElse = 0, 0, 0
		}

		lenIf += len(txnes[i].If)
		lenThen += len(txnes[i].Then)
		lenElse += len(txnes[i].Else)
	}
	spans = append(spans, span{lastIdx, len(txnes)})

	// indexed slots keep the merged responses in request order, which GetMulti pairs with its keys
	resps := make([]*clientv3.TxnResponse, len(spans))
	g, ctx := errgroup.WithContext(ctx)
	for i, sp := range spans {
		g.Go(func() error {
			conds, thens, elses := []clientv3.Cmp{}, []clientv3.Op{}, []clientv3.Op{}
			for _, txn := range txnes[sp.from:sp.to] {
				conds = append(conds, txn.If...)
				thens = append(thens, txn.Then...)
				elses = append(elses, txn.Else...)
			}
			resp, err := e.cliv3.Txn(ctx).If(conds...).Then(thens...).Else(elses...).Commit()
			resps[i] = resp
			return err
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}

	resp := resps[0]
	for _, r := range resps[1:] {
		resp.Succeeded = resp.Succeeded && r.Succeeded
		resp.Responses = append(resp.Responses, r.Responses...)
	}
	return resp, nil
}
