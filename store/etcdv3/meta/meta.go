package meta

import (
	"context"
	"time"

	"go.etcd.io/etcd/v3/clientv3"
	"go.etcd.io/etcd/v3/mvcc/mvccpb"

	"github.com/projecteru2/core/lock"
)

// KV .
type KV interface {
	Grant(ctx context.Context, ttl int64) (*clientv3.LeaseGrantResponse, error)
	KeepAliveOnce(ctx context.Context, id clientv3.LeaseID) (*clientv3.LeaseKeepAliveResponse, error)
	Txn(context.Context) clientv3.Txn

	Get(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.GetResponse, error)
	GetOne(ctx context.Context, key string, opts ...clientv3.OpOption) (*mvccpb.KeyValue, error)
	GetMulti(ctx context.Context, keys []string, opts ...clientv3.OpOption) (kvs []*mvccpb.KeyValue, err error)
	Watch(ctx context.Context, key string, opts ...clientv3.OpOption) clientv3.WatchChan

	Create(ctx context.Context, key, val string, opts ...clientv3.OpOption) (*clientv3.TxnResponse, error)
	Put(ctx context.Context, key, val string, opts ...clientv3.OpOption) (*clientv3.PutResponse, error)
	Update(ctx context.Context, key, val string, opts ...clientv3.OpOption) (*clientv3.TxnResponse, error)
	Delete(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.DeleteResponse, error)

	BatchCreate(ctx context.Context, data map[string]string, opts ...clientv3.OpOption) (*clientv3.TxnResponse, error)
	BatchUpdate(ctx context.Context, data map[string]string, opts ...clientv3.OpOption) (*clientv3.TxnResponse, error)
	BatchDelete(ctx context.Context, keys []string, opts ...clientv3.OpOption) (*clientv3.TxnResponse, error)

	StartEphemeral(ctx context.Context, path string, heartbeat time.Duration) (<-chan struct{}, func(), error)
	CreateLock(key string, ttl time.Duration) (lock.DistributedLock, error)

	// This's just for testing.
	TerminateEmbededStorage()
}
