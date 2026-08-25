package etcdv3

import (
	"context"
	"iter"
	"time"

	"github.com/cockroachdb/errors"
	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/projecteru2/core/lock"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/store/common"
	"github.com/projecteru2/core/store/etcdv3/meta"
	"github.com/projecteru2/core/types"
)

var _ common.KV = (*etcdKV)(nil)

type etcdKV struct {
	kv meta.KV
}

func (e *etcdKV) GetOne(ctx context.Context, key string) (string, error) {
	ev, err := e.kv.GetOne(ctx, key)
	if err != nil {
		return "", err
	}
	return string(ev.Value), nil
}

func (e *etcdKV) GetMulti(ctx context.Context, keys []string) (map[string]string, error) {
	kvs, err := e.kv.GetMulti(ctx, keys)
	if err != nil {
		return nil, err
	}
	data := make(map[string]string, len(kvs))
	for _, kv := range kvs {
		data[string(kv.Key)] = string(kv.Value)
	}
	return data, nil
}

func (e *etcdKV) GetPrefix(ctx context.Context, prefix string, limit int64) (map[string]string, error) {
	resp, err := e.kv.Get(ctx, prefix, clientv3.WithPrefix(), clientv3.WithLimit(limit))
	if err != nil {
		return nil, err
	}
	data := make(map[string]string, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		data[string(kv.Key)] = string(kv.Value)
	}
	return data, nil
}

func (e *etcdKV) ListPrefix(ctx context.Context, prefix string) ([]string, error) {
	resp, err := e.kv.Get(ctx, prefix, clientv3.WithPrefix(), clientv3.WithKeysOnly())
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		keys = append(keys, string(kv.Key))
	}
	return keys, nil
}

func (e *etcdKV) NotFound(err error) bool {
	return errors.Is(err, types.ErrInvaildCount)
}

func (e *etcdKV) Create(ctx context.Context, data map[string]string) error {
	_, err := e.kv.BatchCreate(ctx, data)
	return err
}

func (e *etcdKV) Update(ctx context.Context, data map[string]string) error {
	_, err := e.kv.BatchUpdate(ctx, data)
	return err
}

func (e *etcdKV) Put(ctx context.Context, data map[string]string) error {
	resp, err := e.kv.BatchPut(ctx, data)
	if err != nil {
		return err
	}
	if !resp.Succeeded {
		return types.ErrTxnConditionFailed
	}
	return nil
}

func (e *etcdKV) Delete(ctx context.Context, keys []string) error {
	_, err := e.kv.BatchDelete(ctx, keys)
	return err
}

func (e *etcdKV) CreateAndDecr(ctx context.Context, data map[string]string, decrKey string) error {
	return e.kv.BatchCreateAndDecr(ctx, data, decrKey)
}

func (e *etcdKV) BindStatus(ctx context.Context, entityKey, statusKey, statusValue string, ttl int64) error {
	return e.kv.BindStatus(ctx, entityKey, statusKey, statusValue, ttl)
}

func (e *etcdKV) Watch(ctx context.Context, prefix string) iter.Seq[common.Event] {
	watchChan := e.kv.Watch(ctx, prefix, clientv3.WithPrefix())
	return func(yield func(common.Event) bool) {
		logger := log.WithFunc("store.etcdv3.Watch")
		for resp := range watchChan {
			if resp.Err() != nil {
				if !resp.Canceled {
					logger.Error(ctx, resp.Err(), "watch failed")
				}
				return
			}
			for _, ev := range resp.Events {
				event := common.Event{Key: string(ev.Kv.Key), Type: common.EventPut}
				if ev.Type == clientv3.EventTypeDelete {
					event.Type = common.EventDelete
				}
				if !yield(event) {
					return
				}
			}
		}
	}
}

func (e *etcdKV) StartEphemeral(ctx context.Context, path string, heartbeat time.Duration) (<-chan struct{}, func(), error) {
	return e.kv.StartEphemeral(ctx, path, heartbeat)
}

func (e *etcdKV) CreateLock(key string, ttl time.Duration) (lock.DistributedLock, error) {
	return e.kv.CreateLock(key, ttl)
}
