package meta

import (
	"context"
	"time"

	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/projecteru2/core/lock"
)

// KV is the etcd surface the store is built on.
type KV interface {
	BindStatus(ctx context.Context, entityKey, statusKey, statusValue string, ttl int64) error

	Get(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.GetResponse, error)
	GetOne(ctx context.Context, key string, opts ...clientv3.OpOption) (*mvccpb.KeyValue, error)
	GetMulti(ctx context.Context, keys []string) ([]*mvccpb.KeyValue, error)
	Watch(ctx context.Context, key string, opts ...clientv3.OpOption) clientv3.WatchChan

	Create(ctx context.Context, key, val string) (*clientv3.TxnResponse, error)
	Put(ctx context.Context, key, val string) (*clientv3.PutResponse, error)
	Delete(ctx context.Context, key string) (*clientv3.DeleteResponse, error)

	BatchCreateAndDecr(ctx context.Context, data map[string]string, decrKey string) error

	BatchCreate(ctx context.Context, data map[string]string) (*clientv3.TxnResponse, error)
	BatchUpdate(ctx context.Context, data map[string]string) (*clientv3.TxnResponse, error)
	BatchDelete(ctx context.Context, keys []string) (*clientv3.TxnResponse, error)
	BatchPut(ctx context.Context, data map[string]string) (*clientv3.TxnResponse, error)

	StartEphemeral(ctx context.Context, path string, heartbeat time.Duration) (<-chan struct{}, func(), error)
	CreateLock(key string, ttl time.Duration) (lock.DistributedLock, error)
}
