package redis

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/panjf2000/ants/v2"
	"github.com/redis/go-redis/v9"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/store/common"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

const (
	keyNotifyPrefix = "__keyspace@%d__:%s"

	actionExpire  = "expire"
	actionExpired = "expired"
	actionSet     = "set"
	actionDel     = "del"
)

var (
	// ErrAlreadyExists indicates SETNX found the key already set.
	ErrAlreadyExists = errors.New("key already exists")
	// ErrKeyNotExists indicates an update targeted a missing key, as the etcd store reports it.
	ErrKeyNotExists = errors.New("key not exists")
)

// Rediaron is a store implemented by redis
type Rediaron struct {
	*common.Store

	cli *redis.Client
}

// New creates a Rediaron, using only the redis address and db from config.
func New(config types.Config) (*Rediaron, error) {
	cli := redis.NewClient(&redis.Options{
		Addr: config.Redis.Addr,
		DB:   config.Redis.DB,
	})
	pool, err := utils.NewPool(config.MaxConcurrency)
	if err != nil {
		return nil, err
	}
	return newRediaron(cli, config, pool), nil
}

func newRediaron(cli *redis.Client, config types.Config, pool *ants.PoolWithFunc) *Rediaron {
	r := &Rediaron{cli: cli}
	r.Store = common.New(&redisKV{r: r}, config, pool)
	return r
}

// KNotifyMessage is received when using KNotify
type KNotifyMessage struct {
	Key    string
	Action string
}

// KNotify streams key change notifications, the redis counterpart of an etcd watch.
func (r *Rediaron) KNotify(ctx context.Context, pattern string) chan *KNotifyMessage {
	ch := make(chan *KNotifyMessage)
	logger := log.WithFunc("store.redis.KNotify")
	prefix := fmt.Sprintf(keyNotifyPrefix, r.Config.Redis.DB, "")
	channel := fmt.Sprintf(keyNotifyPrefix, r.Config.Redis.DB, pattern)
	pubsub := r.cli.PSubscribe(ctx, channel)
	subC := pubsub.Channel()
	_ = r.Pool.Invoke(func() {
		defer close(ch)
		defer func() {
			_ = pubsub.Close()
		}()

		for {
			select {
			case <-ctx.Done():
				return
			case v := <-subC:
				if v == nil {
					logger.Warn(ctx, "channel closed, knotify returns")
					return
				}
				ch <- &KNotifyMessage{
					Key:    strings.TrimPrefix(v.Channel, prefix),
					Action: strings.ToLower(v.Payload),
				}
			}
		}
	})
	return ch
}

func (r *Rediaron) GetOne(ctx context.Context, key string) (string, error) {
	value, err := r.cli.Get(ctx, key).Result()
	if isRedisNoKeyError(err) {
		return "", errors.Wrapf(err, "key not found: %s", key)
	}
	return value, err
}

func (r *Rediaron) GetMulti(ctx context.Context, keys []string) (map[string]string, error) {
	cmds := make([]*redis.StringCmd, 0, len(keys))
	_, err := r.cli.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		for _, k := range keys {
			cmds = append(cmds, pipe.Get(ctx, k))
		}
		return nil
	})

	data := make(map[string]string, len(keys))
	for i, cmd := range cmds {
		if isRedisNoKeyError(cmd.Err()) {
			return nil, errors.Wrapf(cmd.Err(), "key not found: %s", keys[i])
		}
		data[keys[i]] = cmd.Val()
	}
	return data, err
}

func (r *Rediaron) BatchUpdate(ctx context.Context, data map[string]string) error {
	keys := slices.Collect(maps.Keys(data))

	// the existence check is not part of the transaction below
	e, err := r.cli.Exists(ctx, keys...).Result()
	if err != nil {
		return err
	}
	if int(e) != len(keys) {
		return ErrKeyNotExists
	}

	update := func(pipe redis.Pipeliner) error {
		for key, value := range data {
			pipe.Set(ctx, key, value, 0)
		}
		return nil
	}

	_, err = r.cli.TxPipelined(ctx, update)
	return err
}

func (r *Rediaron) BatchCreate(ctx context.Context, data map[string]string) error {
	create := func(pipe redis.Pipeliner) error {
		for key, value := range data {
			pipe.SetNX(ctx, key, value, 0)
		}
		return nil
	}

	cmds, err := r.cli.TxPipelined(ctx, create)
	if err != nil {
		return err
	}

	for _, cmd := range cmds {
		if created, _ := cmd.(*redis.BoolCmd).Result(); !created {
			return ErrAlreadyExists
		}
	}
	return nil
}

func (r *Rediaron) BatchPut(ctx context.Context, data map[string]string) error {
	replace := func(pipe redis.Pipeliner) error {
		for key, value := range data {
			pipe.Set(ctx, key, value, 0)
		}
		return nil
	}

	_, err := r.cli.TxPipelined(ctx, replace)
	return err
}

func (r *Rediaron) BatchCreateAndDecr(ctx context.Context, data map[string]string, decrKey string) (err error) {
	batchCreateAndDecr := func(pipe redis.Pipeliner) error {
		pipe.Decr(ctx, decrKey)
		for key, value := range data {
			pipe.SetNX(ctx, key, value, 0)
		}
		return nil
	}
	_, err = r.cli.TxPipelined(ctx, batchCreateAndDecr)
	return err
}

func (r *Rediaron) BatchDelete(ctx context.Context, keys []string) error {
	del := func(pipe redis.Pipeliner) error {
		for _, key := range keys {
			pipe.Del(ctx, key)
		}
		return nil
	}
	_, err := r.cli.TxPipelined(ctx, del)
	return err
}

func (r *Rediaron) BindStatus(ctx context.Context, entityKey, statusKey, statusValue string, ttl int64) error {
	count, err := r.cli.Exists(ctx, entityKey).Result()
	if err != nil {
		return err
	}
	// mirrors etcd: a missing entity key is an error
	if count != 1 {
		return types.ErrInvaildCount
	}

	_, err = r.cli.Set(ctx, statusKey, statusValue, time.Duration(ttl)*time.Second).Result()
	return err
}

// go-redis does not export proto.Error, so the message is the only signal.
func isRedisNoKeyError(e error) bool {
	return e != nil && strings.Contains(e.Error(), "redis: nil")
}
