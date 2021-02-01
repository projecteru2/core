package meta

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.etcd.io/etcd/v3/clientv3"
	"go.etcd.io/etcd/v3/etcdserver/etcdserverpb"
	"go.etcd.io/etcd/v3/mvcc/mvccpb"

	"github.com/projecteru2/core/store/etcdv3/meta/mocks"
	"github.com/projecteru2/core/types"
)

func TestGetOneError(t *testing.T) {
	e := NewMockedETCD(t)
	expErr := fmt.Errorf("exp")
	e.cliv3.(*mocks.ETCDClientV3).On("Get", mock.Anything, mock.Anything).Return(nil, expErr).Once()
	kv, err := e.GetOne(context.Background(), "foo")
	require.Equal(t, expErr, err)
	require.Nil(t, kv)
}

func TestGetOneFailedAsRespondMore(t *testing.T) {
	e := NewMockedETCD(t)
	expResp := &clientv3.GetResponse{Count: 2}
	e.cliv3.(*mocks.ETCDClientV3).On("Get", mock.Anything, mock.Anything).Return(expResp, nil).Once()
	kv, err := e.GetOne(context.Background(), "foo")
	require.Error(t, err)
	require.Nil(t, kv)
}

func TestGetMultiWithNoKeys(t *testing.T) {
	e := NewEmbeddedETCD(t)
	defer e.TerminateEmbededStorage()
	kvs, err := e.GetMulti(context.Background(), []string{})
	require.NoError(t, err)
	require.Equal(t, 0, len(kvs))
}

func TestGetMultiFailedAsBatchGetError(t *testing.T) {
	e := NewMockedETCD(t)
	expErr := fmt.Errorf("exp")
	expTxn := &mocks.Txn{}
	expTxn.On("Then", mock.Anything).Return(expTxn).Once()
	expTxn.On("Else", mock.Anything).Return(expTxn).Once()
	expTxn.On("Commit").Return(nil, expErr).Once()
	e.cliv3.(*mocks.ETCDClientV3).On("Txn", mock.Anything).Return(expTxn)
	kvs, err := e.GetMulti(context.Background(), []string{"foo"})
	require.Equal(t, expErr, err)
	require.Nil(t, kvs)
}

func TestGrant(t *testing.T) {
	e := NewMockedETCD(t)
	expErr := fmt.Errorf("exp")
	e.cliv3.(*mocks.ETCDClientV3).On("Grant", mock.Anything, mock.Anything).Return(nil, expErr)
	resp, err := e.Grant(context.Background(), 1)
	require.Equal(t, expErr, err)
	require.Nil(t, resp)
}

func TestBindStatusFailedAsGrantError(t *testing.T) {
	e, etcd, assert := testKeepAliveETCD(t)
	defer assert()
	expErr := fmt.Errorf("exp")
	etcd.On("Grant", mock.Anything, mock.Anything).Return(nil, expErr).Once()
	require.Equal(t, expErr, e.BindStatus(context.Background(), "/entity", "/status", "status", 1))
}

func TestBindStatusFailedAsCommitError(t *testing.T) {
	e, etcd, assert := testKeepAliveETCD(t)
	defer assert()

	expErr := fmt.Errorf("exp")
	txn := &mocks.Txn{}
	defer txn.AssertExpectations(t)
	txn.On("If", mock.Anything).Return(txn).Once()
	txn.On("Then", mock.Anything).Return(txn).Once()
	txn.On("Commit").Return(nil, expErr).Once()

	etcd.On("Grant", mock.Anything, mock.Anything).Return(&clientv3.LeaseGrantResponse{}, nil).Once()
	etcd.On("Txn", mock.Anything).Return(txn).Once()
	require.Equal(t, expErr, e.BindStatus(context.Background(), "/entity", "/status", "status", 1))
}

func TestBindStatusButEntityTxnUnsuccessful(t *testing.T) {
	e, etcd, assert := testKeepAliveETCD(t)
	defer assert()

	entityTxn := &clientv3.TxnResponse{Succeeded: false}
	txn := &mocks.Txn{}
	defer txn.AssertExpectations(t)
	txn.On("If", mock.Anything).Return(txn).Once()
	txn.On("Then", mock.Anything).Return(txn).Once()
	txn.On("Commit").Return(entityTxn, nil)

	etcd.On("Grant", mock.Anything, mock.Anything).Return(&clientv3.LeaseGrantResponse{}, nil).Once()
	etcd.On("Txn", mock.Anything).Return(txn).Once()
	require.Equal(t, nil, e.BindStatus(context.Background(), "/entity", "/status", "status", 1))
}

func TestBindStatusButStatusTxnUnsuccessful(t *testing.T) {
	e, etcd, assert := testKeepAliveETCD(t)
	defer assert()

	entityTxn := &clientv3.TxnResponse{
		Succeeded: true,
		Responses: []*etcdserverpb.ResponseOp{
			&etcdserverpb.ResponseOp{
				Response: &etcdserverpb.ResponseOp_ResponseTxn{
					// statusTxn
					ResponseTxn: &etcdserverpb.TxnResponse{Succeeded: false},
				},
			},
		},
	}
	txn := &mocks.Txn{}
	defer txn.AssertExpectations(t)
	txn.On("If", mock.Anything).Return(txn).Once()
	txn.On("Then", mock.Anything).Return(txn).Once()
	txn.On("Commit").Return(entityTxn, nil)

	etcd.On("Grant", mock.Anything, mock.Anything).Return(&clientv3.LeaseGrantResponse{}, nil).Once()
	etcd.On("Txn", mock.Anything).Return(txn).Once()
	require.Equal(t, nil, e.BindStatus(context.Background(), "/entity", "/status", "status", 1))
}

func TestBindStatusWithZeroTTL(t *testing.T) {
	e, etcd, assert := testKeepAliveETCD(t)
	defer assert()

	entityTxn := &clientv3.TxnResponse{
		Succeeded: true,
		Responses: []*etcdserverpb.ResponseOp{
			&etcdserverpb.ResponseOp{
				Response: &etcdserverpb.ResponseOp_ResponseTxn{
					// statusTxn
					ResponseTxn: &etcdserverpb.TxnResponse{Succeeded: true},
				},
			},
		},
	}
	txn := &mocks.Txn{}
	defer txn.AssertExpectations(t)
	txn.On("If", mock.Anything).Return(txn).Once()
	txn.On("Then", mock.Anything).Return(txn).Once()
	txn.On("Commit").Return(entityTxn, nil)

	etcd.On("Txn", mock.Anything).Return(txn).Once()
	require.Equal(t, nil, e.BindStatus(context.Background(), "/entity", "/status", "status", 0))
}

func TestBindStatusButValueTxnUnsuccessful(t *testing.T) {
	e, etcd, assert := testKeepAliveETCD(t)
	defer assert()

	statusTxn := &etcdserverpb.TxnResponse{
		Succeeded: true,
		Responses: []*etcdserverpb.ResponseOp{
			&etcdserverpb.ResponseOp{
				Response: &etcdserverpb.ResponseOp_ResponseTxn{
					// valueTxn
					ResponseTxn: &etcdserverpb.TxnResponse{Succeeded: false},
				},
			},
		},
	}
	entityTxn := &clientv3.TxnResponse{
		Succeeded: true,
		Responses: []*etcdserverpb.ResponseOp{
			&etcdserverpb.ResponseOp{
				Response: &etcdserverpb.ResponseOp_ResponseTxn{
					// statusTxn
					ResponseTxn: statusTxn,
				},
			},
		},
	}
	txn := &mocks.Txn{}
	defer txn.AssertExpectations(t)
	txn.On("If", mock.Anything).Return(txn).Once()
	txn.On("Then", mock.Anything).Return(txn).Once()
	txn.On("Commit").Return(entityTxn, nil)

	etcd.On("Txn", mock.Anything).Return(txn).Once()
	require.Equal(t, nil, e.BindStatus(context.Background(), "/entity", "/status", "status", 0))
}

func TestBindStatus(t *testing.T) {
	e, etcd, assert := testKeepAliveETCD(t)
	defer assert()

	leaseID := int64(1235)
	valueTxn := &etcdserverpb.TxnResponse{
		Succeeded: true,
		Responses: []*etcdserverpb.ResponseOp{
			&etcdserverpb.ResponseOp{
				Response: &etcdserverpb.ResponseOp_ResponseRange{
					ResponseRange: &etcdserverpb.RangeResponse{
						Kvs: []*mvccpb.KeyValue{
							&mvccpb.KeyValue{Lease: leaseID},
						},
					},
				},
			},
		},
	}
	statusTxn := &etcdserverpb.TxnResponse{
		Succeeded: true,
		Responses: []*etcdserverpb.ResponseOp{
			&etcdserverpb.ResponseOp{
				Response: &etcdserverpb.ResponseOp_ResponseTxn{
					ResponseTxn: valueTxn,
				},
			},
		},
	}
	entityTxn := &clientv3.TxnResponse{
		Succeeded: true,
		Responses: []*etcdserverpb.ResponseOp{
			&etcdserverpb.ResponseOp{
				Response: &etcdserverpb.ResponseOp_ResponseTxn{
					// statusTxn
					ResponseTxn: statusTxn,
				},
			},
		},
	}
	txn := &mocks.Txn{}
	defer txn.AssertExpectations(t)
	txn.On("If", mock.Anything).Return(txn).Once()
	txn.On("Then", mock.Anything).Return(txn).Once()
	txn.On("Commit").Return(entityTxn, nil)

	etcd.On("Grant", mock.Anything, mock.Anything).Return(&clientv3.LeaseGrantResponse{}, nil).Once()
	etcd.On("Txn", mock.Anything).Return(txn).Once()
	etcd.On("KeepAliveOnce", mock.Anything, clientv3.LeaseID(leaseID)).Return(nil, nil).Once()
	require.Equal(t, nil, e.BindStatus(context.Background(), "/entity", "/status", "status", 1))
}

func testKeepAliveETCD(t *testing.T) (*ETCD, *mocks.ETCDClientV3, func()) {
	e := NewMockedETCD(t)
	etcd, ok := e.cliv3.(*mocks.ETCDClientV3)
	require.True(t, ok)
	return e, etcd, func() { etcd.AssertExpectations(t) }
}

func NewMockedETCD(t *testing.T) *ETCD {
	e := NewEmbeddedETCD(t)
	e.cliv3 = &mocks.ETCDClientV3{}
	e.TerminateEmbededStorage()
	return e
}

func NewEmbeddedETCD(t *testing.T) *ETCD {
	config := types.EtcdConfig{
		Machines:   []string{"127.0.0.1:2379"},
		Prefix:     "/eru-test",
		LockPrefix: "/eru-test-lock",
	}
	e, err := NewETCD(config, true)
	require.NoError(t, err)
	return e
}

func TestETCD(t *testing.T) {
	m := NewEmbeddedETCD(t)
	defer m.TerminateEmbededStorage()
	ctx := context.Background()

	// CreateLock
	_, err := m.CreateLock("test", 5)
	require.NoError(t, err)
	// Get
	resp, err := m.Get(ctx, "test")
	require.NoError(t, err)
	require.Equal(t, resp.Count, int64(0))
	// Put
	_, err = m.Put(ctx, "test/1", "a")
	m.Put(ctx, "test/2", "a")
	require.NoError(t, err)
	// Get again
	resp, err = m.Get(ctx, "test/1")
	require.NoError(t, err)
	require.Equal(t, resp.Count, int64(len(resp.Kvs)))
	// GetOne
	_, err = m.GetOne(ctx, "test", clientv3.WithPrefix())
	require.Error(t, err)
	ev, err := m.GetOne(ctx, "test/1")
	require.NoError(t, err)
	require.Equal(t, string(ev.Value), "a")
	// Delete
	_, err = m.Delete(ctx, "test/2")
	require.NoError(t, err)
	m.Put(ctx, "d1", "a")
	m.Put(ctx, "d2", "a")
	m.Put(ctx, "d3", "a")
	// BatchDelete
	r, err := m.BatchDelete(ctx, []string{"d1", "d2", "d3"})
	require.NoError(t, err)
	require.True(t, r.Succeeded)
	// Create
	r, err = m.Create(ctx, "test/2", "a")
	require.NoError(t, err)
	require.True(t, r.Succeeded)
	// CreateFail
	r, err = m.Create(ctx, "test/2", "a")
	require.Error(t, err)
	require.False(t, r.Succeeded)
	// BatchCreate
	data := map[string]string{
		"k1": "a1",
		"k2": "a2",
	}
	r, err = m.BatchCreate(ctx, data)
	require.NoError(t, err)
	require.True(t, r.Succeeded)
	// BatchCreateFailed
	r, err = m.BatchCreate(ctx, data)
	require.Error(t, err)
	require.False(t, r.Succeeded)
	// Update
	r, err = m.Update(ctx, "test/2", "b")
	require.NoError(t, err)
	require.True(t, r.Succeeded)
	// UpdateFail
	r, err = m.Update(ctx, "test/3", "b")
	require.Error(t, err)
	require.False(t, r.Succeeded)
	// BatchUpdate
	data = map[string]string{
		"k1": "b1",
		"k2": "b2",
	}
	r, err = m.BatchUpdate(ctx, data)
	require.NoError(t, err)
	require.True(t, r.Succeeded)
	// BatchUpdateFail
	data = map[string]string{
		"k1": "c1",
		"k3": "b2",
	}
	r, err = m.BatchUpdate(ctx, data)
	require.Error(t, err)
	require.False(t, r.Succeeded)
	// Watch
	ctx2, cancel := context.WithCancel(ctx)
	ch := m.watch(ctx2, "watchkey", clientv3.WithPrefix())
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
}
