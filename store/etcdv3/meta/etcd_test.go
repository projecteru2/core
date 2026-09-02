package meta

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/projecteru2/core/store/etcdv3/embedded"
	"github.com/projecteru2/core/store/etcdv3/meta/mocks"
	"github.com/projecteru2/core/types"
)

func TestGetOneError(t *testing.T) {
	e := NewMockedETCD(t)
	expErr := fmt.Errorf("exp")
	e.cliv3.(*mocks.ETCDClientV3).On("Get", mock.Anything, mock.Anything).Return(nil, expErr)
	kv, err := e.GetOne(t.Context(), "foo")
	require.Equal(t, expErr, err)
	require.Nil(t, kv)
}

func TestGetOneFailedAsRespondMore(t *testing.T) {
	e := NewMockedETCD(t)
	expResp := &clientv3.GetResponse{Count: 2}
	e.cliv3.(*mocks.ETCDClientV3).On("Get", mock.Anything, mock.Anything).Return(expResp, nil)
	kv, err := e.GetOne(t.Context(), "foo")
	require.Error(t, err)
	require.Nil(t, kv)
}

func TestGetMultiWithNoKeys(t *testing.T) {
	e := NewEmbeddedETCD(t)
	kvs, err := e.GetMulti(t.Context(), []string{})
	require.NoError(t, err)
	require.Equal(t, 0, len(kvs))
}

func TestGetMultiFailedAsBatchGetError(t *testing.T) {
	e := NewMockedETCD(t)
	expErr := fmt.Errorf("exp")
	expTxn := &mocks.Txn{}
	expTxn.On("If", mock.Anything).Return(expTxn)
	expTxn.On("Then", mock.Anything).Return(expTxn)
	expTxn.On("Else", mock.Anything).Return(expTxn)
	expTxn.On("Commit").Return(nil, expErr)
	e.cliv3.(*mocks.ETCDClientV3).On("Txn", mock.Anything).Return(expTxn)
	kvs, err := e.GetMulti(t.Context(), []string{"foo"})
	require.Equal(t, expErr, err)
	require.Nil(t, kvs)
}

func TestGrant(t *testing.T) {
	e := NewMockedETCD(t)
	expErr := fmt.Errorf("exp")
	e.cliv3.(*mocks.ETCDClientV3).On("Grant", mock.Anything, mock.Anything).Return(nil, expErr)
	resp, err := e.cliv3.Grant(t.Context(), 1)
	require.Equal(t, expErr, err)
	require.Nil(t, resp)
}

func TestBindStatusFailedAsGrantError(t *testing.T) {
	e, etcd, assert := testKeepAliveETCD(t)
	defer assert()
	expErr := fmt.Errorf("exp")
	txn := &mocks.Txn{}
	defer txn.AssertExpectations(t)
	txn.On("If", mock.Anything, mock.Anything).Return(txn)
	txn.On("Then", mock.Anything).Return(txn)
	txn.On("Commit").Return(&clientv3.TxnResponse{Succeeded: false}, nil)

	etcd.On("Txn", mock.Anything).Return(txn)
	etcd.On("Grant", mock.Anything, mock.Anything).Return(nil, expErr)
	require.Equal(t, expErr, e.BindStatus(t.Context(), "/entity", "/status", "status", 1))
}

func TestBindStatusFailedAsCommitError(t *testing.T) {
	e, etcd, assert := testKeepAliveETCD(t)
	defer assert()

	expErr := fmt.Errorf("exp")
	txn := &mocks.Txn{}
	defer txn.AssertExpectations(t)
	txn.On("If", mock.Anything, mock.Anything).Return(txn).Once()
	txn.On("If", mock.Anything).Return(txn).Once()
	txn.On("Then", mock.Anything).Return(txn)
	txn.On("Commit").Return(&clientv3.TxnResponse{Succeeded: false}, nil).Once()
	txn.On("Commit").Return(nil, expErr).Once()

	etcd.On("Grant", mock.Anything, mock.Anything).Return(&clientv3.LeaseGrantResponse{}, nil)
	etcd.On("Txn", mock.Anything).Return(txn)
	require.Equal(t, expErr, e.BindStatus(t.Context(), "/entity", "/status", "status", 1))
}

func TestBindStatusButEntityTxnUnsuccessful(t *testing.T) {
	e, etcd, assert := testKeepAliveETCD(t)
	defer assert()

	txn := &mocks.Txn{}
	defer txn.AssertExpectations(t)
	txn.On("If", mock.Anything, mock.Anything).Return(txn).Once()
	txn.On("If", mock.Anything).Return(txn).Once()
	txn.On("Then", mock.Anything).Return(txn)
	txn.On("Commit").Return(&clientv3.TxnResponse{Succeeded: false}, nil)

	etcd.On("Grant", mock.Anything, mock.Anything).Return(&clientv3.LeaseGrantResponse{}, nil)
	etcd.On("Txn", mock.Anything).Return(txn)
	require.Equal(t, types.ErrInvaildCount, e.BindStatus(t.Context(), "/entity", "/status", "status", 1))
}

func TestBindStatusRenewsAnUnchangedStatus(t *testing.T) {
	e, etcd, assert := testKeepAliveETCD(t)
	defer assert()

	leaseID := int64(1235)
	txn := &mocks.Txn{}
	defer txn.AssertExpectations(t)
	txn.On("If", mock.Anything, mock.Anything).Return(txn)
	txn.On("Then", mock.Anything).Return(txn)
	txn.On("Commit").Return(renewTxn("status", leaseID), nil)

	etcd.On("Txn", mock.Anything).Return(txn)
	etcd.On("KeepAliveOnce", mock.Anything, clientv3.LeaseID(leaseID)).Return(&clientv3.LeaseKeepAliveResponse{TTL: 1}, nil)
	require.Equal(t, nil, e.BindStatus(t.Context(), "/entity", "/status", "status", 1))
	etcd.AssertNotCalled(t, "Grant", mock.Anything, mock.Anything)
}

func TestBindStatusRebindsWhenTheRenewLostItsEntity(t *testing.T) {
	e, etcd, assert := testKeepAliveETCD(t)
	defer assert()

	txn := &mocks.Txn{}
	defer txn.AssertExpectations(t)
	txn.On("If", mock.Anything, mock.Anything).Return(txn).Once()
	txn.On("If", mock.Anything).Return(txn).Once()
	txn.On("Then", mock.Anything).Return(txn)
	txn.On("Commit").Return(&clientv3.TxnResponse{Succeeded: false}, nil)

	etcd.On("Txn", mock.Anything).Return(txn)
	etcd.On("Grant", mock.Anything, mock.Anything).Return(&clientv3.LeaseGrantResponse{}, nil)
	require.Equal(t, types.ErrInvaildCount, e.BindStatus(t.Context(), "/entity", "/status", "status", 1))
	etcd.AssertNotCalled(t, "KeepAliveOnce", mock.Anything, mock.Anything)
}

func TestBindStatusRebindsWhenTheTTLChanged(t *testing.T) {
	e, etcd, assert := testKeepAliveETCD(t)
	defer assert()

	leaseID := int64(1235)
	txn := &mocks.Txn{}
	defer txn.AssertExpectations(t)
	txn.On("If", mock.Anything, mock.Anything).Return(txn).Once()
	txn.On("If", mock.Anything).Return(txn).Once()
	txn.On("Then", mock.Anything).Return(txn)
	txn.On("Commit").Return(renewTxn("status", leaseID), nil).Once()
	txn.On("Commit").Return(&clientv3.TxnResponse{Succeeded: true}, nil).Once()

	etcd.On("Txn", mock.Anything).Return(txn)
	etcd.On("KeepAliveOnce", mock.Anything, clientv3.LeaseID(leaseID)).Return(&clientv3.LeaseKeepAliveResponse{TTL: 5}, nil)
	etcd.On("Grant", mock.Anything, mock.Anything).Return(&clientv3.LeaseGrantResponse{}, nil)
	require.Equal(t, nil, e.BindStatus(t.Context(), "/entity", "/status", "status", 1))
}

func TestBindStatusWithoutEntityCarriesALease(t *testing.T) {
	e := NewEmbeddedETCD(t)
	ctx := t.Context()

	require.NoError(t, e.BindStatus(ctx, "/entity", "/status", "gone", 0))
	kv, err := e.GetOne(ctx, "/status")
	require.NoError(t, err)
	require.NotZero(t, kv.Lease)

	_, err = e.Put(ctx, "/entity", "here")
	require.NoError(t, err)
	require.NoError(t, e.BindStatus(ctx, "/entity", "/status", "gone", 0))
	kv, err = e.GetOne(ctx, "/status")
	require.NoError(t, err)
	require.Zero(t, kv.Lease)
	require.Equal(t, "gone", string(kv.Value))

	leases, err := e.cliv3.Leases(ctx)
	require.NoError(t, err)
	require.Len(t, leases.Leases, 1)
}

func TestBindStatusOrphanPutYieldsToAnEntityThatAppeared(t *testing.T) {
	e, etcd, assert := testKeepAliveETCD(t)
	defer assert()

	txn := &mocks.Txn{}
	defer txn.AssertExpectations(t)
	txn.On("If", mock.Anything).Return(txn)
	txn.On("Then", mock.Anything).Return(txn)
	txn.On("Else", mock.Anything).Return(txn)
	txn.On("Commit").Return(&clientv3.TxnResponse{Succeeded: false}, nil)

	etcd.On("Txn", mock.Anything).Return(txn)
	etcd.On("Grant", mock.Anything, mock.Anything).Return(&clientv3.LeaseGrantResponse{ID: 7}, nil)
	etcd.On("Revoke", mock.Anything, clientv3.LeaseID(7)).Return(&clientv3.LeaseRevokeResponse{}, nil)
	require.NoError(t, e.BindStatus(t.Context(), "/entity", "/status", "status", 0))
}

func TestBindStatusWithZeroTTL(t *testing.T) {
	e, etcd, assert := testKeepAliveETCD(t)
	defer assert()

	entityTxn := &clientv3.TxnResponse{
		Succeeded: true,
		Responses: []*etcdserverpb.ResponseOp{
			{
				Response: &etcdserverpb.ResponseOp_ResponseTxn{
					ResponseTxn: &etcdserverpb.TxnResponse{Succeeded: true},
				},
			},
		},
	}
	txn := &mocks.Txn{}
	defer txn.AssertExpectations(t)
	txn.On("If", mock.Anything).Return(txn)
	txn.On("Then", mock.Anything).Return(txn)
	txn.On("Commit").Return(entityTxn, nil)

	etcd.On("Txn", mock.Anything).Return(txn)

	require.Equal(t, nil, e.BindStatus(t.Context(), "/entity", "/status", "status", 0))
}

func TestBindStatusRebindsAChangedValue(t *testing.T) {
	e, etcd, assert := testKeepAliveETCD(t)
	defer assert()

	txn := &mocks.Txn{}
	defer txn.AssertExpectations(t)
	txn.On("If", mock.Anything, mock.Anything).Return(txn).Once()
	txn.On("If", mock.Anything).Return(txn).Once()
	txn.On("Then", mock.Anything).Return(txn)
	txn.On("Commit").Return(&clientv3.TxnResponse{Succeeded: false}, nil).Once()
	txn.On("Commit").Return(&clientv3.TxnResponse{Succeeded: true}, nil).Once()

	etcd.On("Grant", mock.Anything, mock.Anything).Return(&clientv3.LeaseGrantResponse{}, nil)
	etcd.On("Txn", mock.Anything).Return(txn)
	require.Equal(t, nil, e.BindStatus(t.Context(), "/entity", "/status", "status", 1))
	etcd.AssertNotCalled(t, "KeepAliveOnce", mock.Anything, mock.Anything)
}

func TestBindStatusKeepsOneLeaseAcrossRepeatedReports(t *testing.T) {
	e := NewEmbeddedETCD(t)
	ctx := t.Context()
	_, err := e.Put(ctx, "/entity", "here")
	require.NoError(t, err)

	require.NoError(t, e.BindStatus(ctx, "/entity", "/status", "status", 5))
	first, err := e.GetOne(ctx, "/status")
	require.NoError(t, err)
	require.NotZero(t, first.Lease)

	require.NoError(t, e.BindStatus(ctx, "/entity", "/status", "status", 5))
	second, err := e.GetOne(ctx, "/status")
	require.NoError(t, err)
	require.Equal(t, first.Lease, second.Lease)

	leases, err := e.cliv3.Leases(ctx)
	require.NoError(t, err)
	require.Len(t, leases.Leases, 1)
}

func TestETCD(t *testing.T) {
	m := NewEmbeddedETCD(t)
	ctx := t.Context()

	_, err := m.CreateLock("test", 5)
	require.NoError(t, err)
	resp, err := m.Get(ctx, "test")
	require.NoError(t, err)
	require.Equal(t, resp.Count, int64(0))
	_, err = m.Put(ctx, "test/1", "a")
	m.Put(ctx, "test/2", "a")
	require.NoError(t, err)
	resp, err = m.Get(ctx, "test/1")
	require.NoError(t, err)
	require.Equal(t, resp.Count, int64(len(resp.Kvs)))
	_, err = m.GetOne(ctx, "test", clientv3.WithPrefix())
	require.Error(t, err)
	ev, err := m.GetOne(ctx, "test/1")
	require.NoError(t, err)
	require.Equal(t, string(ev.Value), "a")
	_, err = m.Delete(ctx, "test/2")
	require.NoError(t, err)
	m.Put(ctx, "d1", "a")
	m.Put(ctx, "d2", "a")
	m.Put(ctx, "d3", "a")
	r, err := m.BatchDelete(ctx, []string{"d1", "d2", "d3"})
	require.NoError(t, err)
	require.True(t, r.Succeeded)
	r, err = m.Create(ctx, "test/2", "a")
	require.NoError(t, err)
	require.True(t, r.Succeeded)
	r, err = m.Create(ctx, "test/2", "a")
	require.Error(t, err)
	require.False(t, r.Succeeded)
	data := map[string]string{
		"k1": "a1",
		"k2": "a2",
	}
	r, err = m.BatchCreate(ctx, data)
	require.NoError(t, err)
	require.True(t, r.Succeeded)
	r, err = m.BatchCreate(ctx, data)
	require.Error(t, err)
	require.False(t, r.Succeeded)
	data = map[string]string{
		"k1": "b1",
		"k2": "b2",
	}
	r, err = m.BatchUpdate(ctx, data)
	require.NoError(t, err)
	require.True(t, r.Succeeded)
	data = map[string]string{
		"k1": "c1",
		"k3": "b2",
	}
	r, err = m.BatchUpdate(ctx, data)
	require.EqualError(t, err, "key not exists")
	require.False(t, r.Succeeded)
	ctx2, cancel := context.WithCancel(ctx)
	ch := m.Watch(ctx2, "watchkey", clientv3.WithPrefix())
	go func() {
		for r := range ch {
			require.NotEmpty(t, r.Events)
			require.Equal(t, len(r.Events), 1)
			require.Equal(t, r.Events[0].Type, clientv3.EventTypePut)
			require.Equal(t, string(r.Events[0].Kv.Value), "b")
		}
	}()
	m.Create(ctx, "watchkey/1", "b")
	cancel()

	data = map[string]string{
		"bcad_k1": "v1",
		"bcad_k2": "v1",
	}
	err = m.BatchCreateAndDecr(t.Context(), data, "bcad_process")
	require.EqualError(t, err, "bcad_process: key not exists")

	_, err = m.Put(t.Context(), "bcad_process", "a")
	require.NoError(t, err)
	err = m.BatchCreateAndDecr(t.Context(), data, "bcad_process")
	require.EqualError(t, err, "strconv.Atoi: parsing \"a\": invalid syntax")

	_, err = m.Put(t.Context(), "bcad_process", "20")
	require.NoError(t, err)
	err = m.BatchCreateAndDecr(t.Context(), data, "bcad_process")
	require.NoError(t, err)
	resp, err = m.Get(t.Context(), "bcad_process")
	require.NoError(t, err)
	processCnt, err := strconv.Atoi(string(resp.Kvs[0].Value))
	require.NoError(t, err)
	require.EqualValues(t, 19, processCnt)

	_, err = m.Put(t.Context(), "bcad_process", "200")
	require.NoError(t, err)
	wg := sync.WaitGroup{}
	for range 200 {
		wg.Go(func() {
			m.BatchCreateAndDecr(t.Context(), data, "bcad_process")
		})
	}
	wg.Wait()
	resp, err = m.Get(t.Context(), "bcad_process")
	require.NoError(t, err)
	processCnt, err = strconv.Atoi(string(resp.Kvs[0].Value))
	require.NoError(t, err)
	require.EqualValues(t, 0, processCnt)

	_, err = m.doBatchOp(t.Context(), nil)
	require.EqualError(t, err, "no txn ops")

	txnes := []ETCDTxn{}
	for range 999 {
		txnes = append(txnes, ETCDTxn{Then: []clientv3.Op{clientv3.OpGet("a")}})
	}
	txnResp, err := m.doBatchOp(t.Context(), txnes)
	require.NoError(t, err)
	require.True(t, txnResp.Succeeded)
	require.EqualValues(t, 999, len(txnResp.Responses))

	txnes = []ETCDTxn{{}, {}}
	for range 999 {
		txnes[0].Then = append(txnes[0].Then, clientv3.OpGet("a"))
		txnes[1].Then = append(txnes[1].Then, clientv3.OpGet("a"), clientv3.OpGet("b"))
	}
	txnResp, err = m.doBatchOp(t.Context(), txnes)
	require.NoError(t, err)
	require.True(t, txnResp.Succeeded)
	require.EqualValues(t, 999*3, len(txnResp.Responses))

	txnes = []ETCDTxn{{If: []clientv3.Cmp{
		clientv3.Compare(clientv3.Value("a"), "=", string("123")),
	}}}
	txnResp, err = m.doBatchOp(t.Context(), txnes)
	require.NoError(t, err)
	require.False(t, txnResp.Succeeded)
	require.EqualValues(t, 0, len(txnResp.Responses))

	_, err = m.GetMulti(t.Context(), []string{"a", "b"})
	require.EqualError(t, err, "key: a: bad `Count` value, entity count invalid")

	m.Put(t.Context(), "a", "b")
	m.Put(t.Context(), "b", "c")
	kvs, err := m.GetMulti(t.Context(), []string{"a", "b"})
	require.NoError(t, err)
	require.EqualValues(t, 2, len(kvs))

	data = map[string]string{
		"aa": "bb",
		"cc": "dd",
	}
	m.Put(t.Context(), "aa", "aa")
	m.Put(t.Context(), "cc", "cc")
	txnResp, err = m.batchPut(t.Context(), data, &txnCond{method: cmpValue, condition: "!="})
	require.NoError(t, err)
	require.True(t, txnResp.Succeeded)
}

func NewMockedETCD(t *testing.T) *ETCD {
	e := NewEmbeddedETCD(t)
	e.cliv3 = &mocks.ETCDClientV3{}
	return e
}

func NewEmbeddedETCD(t *testing.T) *ETCD {
	config := types.EtcdConfig{
		Machines:   []string{"127.0.0.1:2379"},
		Prefix:     "/eru-test",
		LockPrefix: "/eru-test-lock",
	}
	cluster, err := embedded.New(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(cluster.Close)
	e, err := NewETCD(t.Context(), config, cluster)
	require.NoError(t, err)
	return e
}

func testKeepAliveETCD(t *testing.T) (*ETCD, *mocks.ETCDClientV3, func()) {
	e := NewMockedETCD(t)
	etcd, ok := e.cliv3.(*mocks.ETCDClientV3)
	require.True(t, ok)
	return e, etcd, func() { etcd.AssertExpectations(t) }
}

func renewTxn(value string, lease int64) *clientv3.TxnResponse {
	return &clientv3.TxnResponse{
		Succeeded: true,
		Responses: []*etcdserverpb.ResponseOp{{
			Response: &etcdserverpb.ResponseOp_ResponseRange{
				ResponseRange: &etcdserverpb.RangeResponse{Kvs: []*mvccpb.KeyValue{{Value: []byte(value), Lease: lease}}},
			},
		}},
	}
}
