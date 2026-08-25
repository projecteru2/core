package redis

import (
	"context"
	"iter"
	"time"

	"github.com/projecteru2/core/lock"
	"github.com/projecteru2/core/store/common"
)

var _ common.KV = (*redisKV)(nil)

type redisKV struct {
	r *Rediaron
}

func (k *redisKV) GetOne(ctx context.Context, key string) (string, error) {
	return k.r.GetOne(ctx, key)
}

func (k *redisKV) GetMulti(ctx context.Context, keys []string) (map[string]string, error) {
	return k.r.GetMulti(ctx, keys)
}

func (k *redisKV) GetPrefix(ctx context.Context, prefix string, limit int64) (map[string]string, error) {
	keys, err := k.r.scanKeys(ctx, prefix+"*", limit)
	if err != nil {
		return nil, err
	}
	return k.r.GetMulti(ctx, keys)
}

func (k *redisKV) ListPrefix(ctx context.Context, prefix string) ([]string, error) {
	return k.r.scanKeys(ctx, prefix+"*", 0)
}

func (k *redisKV) NotFound(err error) bool {
	return isRedisNoKeyError(err)
}

func (k *redisKV) Create(ctx context.Context, data map[string]string) error {
	return k.r.BatchCreate(ctx, data)
}

func (k *redisKV) Update(ctx context.Context, data map[string]string) error {
	return k.r.BatchUpdate(ctx, data)
}

func (k *redisKV) Put(ctx context.Context, data map[string]string) error {
	return k.r.BatchPut(ctx, data)
}

func (k *redisKV) Delete(ctx context.Context, keys []string) error {
	return k.r.BatchDelete(ctx, keys)
}

func (k *redisKV) CreateAndDecr(ctx context.Context, data map[string]string, decrKey string) error {
	return k.r.BatchCreateAndDecr(ctx, data, decrKey)
}

func (k *redisKV) BindStatus(ctx context.Context, entityKey, statusKey, statusValue string, ttl int64) error {
	return k.r.BindStatus(ctx, entityKey, statusKey, statusValue, ttl)
}

func (k *redisKV) Watch(ctx context.Context, prefix string) iter.Seq[common.Event] {
	messages := k.r.KNotify(ctx, prefix+"*")
	return func(yield func(common.Event) bool) {
		for message := range messages {
			event := common.Event{Key: message.Key}
			switch message.Action {
			case actionSet, actionExpire:
				event.Type = common.EventPut
			case actionDel:
				event.Type = common.EventDelete
			case actionExpired:
				event.Type = common.EventExpire
			}
			if !yield(event) {
				return
			}
		}
	}
}

func (k *redisKV) StartEphemeral(ctx context.Context, path string, heartbeat time.Duration) (<-chan struct{}, func(), error) {
	return k.r.StartEphemeral(ctx, path, heartbeat)
}

func (k *redisKV) CreateLock(key string, ttl time.Duration) (lock.DistributedLock, error) {
	return k.r.CreateLock(key, ttl)
}
