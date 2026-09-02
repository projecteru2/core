package redis

import (
	"context"
	"fmt"
	"iter"
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

	actionExpired = "expired"
	actionSet     = "set"
	actionDel     = "del"

	replyExists  = "exists"
	replyMissing = "missing"
	replyChanged = "changed"

	scanCount = 1000
)

var (
	// ErrAlreadyExists indicates a create found one of its keys already set.
	ErrAlreadyExists = errors.New("key already exists")
	// ErrKeyNotExists indicates an update targeted a missing key, as the etcd store reports it.
	ErrKeyNotExists = errors.New("key not exists")

	createScript = redis.NewScript(`
if KEYS[1] ~= "" and redis.call("exists", KEYS[1]) == 0 then return "missing" end
for i = 2, #KEYS do
    if redis.call("exists", KEYS[i]) == 1 then return "exists" end
end
if KEYS[1] ~= "" then redis.call("decr", KEYS[1]) end
for i = 2, #KEYS do redis.call("set", KEYS[i], ARGV[i-1]) end
return "ok"`)
	refreshStatusScript = redis.NewScript(`
if redis.call("exists", KEYS[1]) == 0 then return "missing" end
if redis.call("get", KEYS[2]) ~= ARGV[1] then return "changed" end
if tonumber(ARGV[2]) > 0 then redis.call("expire", KEYS[2], ARGV[2]) else redis.call("persist", KEYS[2]) end
return "refreshed"`)

	globMeta = strings.NewReplacer(`\`, `\\`, "*", `\*`, "?", `\?`, "[", `\[`, "]", `\]`)
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
	r.Store = common.New(r, config, pool)
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
	subC := pubsub.ChannelWithSubscriptions()
	utils.SentryGo(func() {
		defer close(ch)
		defer func() {
			_ = pubsub.Close()
		}()

		subscribed := false
		for {
			select {
			case <-ctx.Done():
				return
			case v := <-subC:
				if v == nil {
					logger.Warn(ctx, "channel closed, knotify returns")
					return
				}
				switch v := v.(type) {
				case *redis.Subscription:
					if subscribed {
						return
					}
					subscribed = true
				case *redis.Message:
					message := &KNotifyMessage{
						Key:    strings.TrimPrefix(v.Channel, prefix),
						Action: strings.ToLower(v.Payload),
					}
					select {
					case ch <- message:
					case <-ctx.Done():
						return
					}
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

func (r *Rediaron) GetPrefix(ctx context.Context, prefix string, limit int64) (map[string]string, error) {
	keys, err := r.scanKeys(ctx, globPrefix(prefix), limit)
	if err != nil {
		return nil, err
	}
	return r.GetMulti(ctx, keys)
}

func (r *Rediaron) ListPrefix(ctx context.Context, prefix string) ([]string, error) {
	return r.scanKeys(ctx, globPrefix(prefix), 0)
}

func (r *Rediaron) NotFound(err error) bool {
	return isRedisNoKeyError(err)
}

func (r *Rediaron) Watch(ctx context.Context, prefix string) iter.Seq[common.Event] {
	messages := r.KNotify(ctx, globPrefix(prefix))
	return func(yield func(common.Event) bool) {
		for message := range messages {
			event := common.Event{Key: message.Key}
			switch message.Action {
			case actionSet:
				event.Type = common.EventPut
			case actionDel:
				event.Type = common.EventDelete
			case actionExpired:
				event.Type = common.EventExpire
			default:
				continue
			}
			if !yield(event) {
				return
			}
		}
	}
}

func (r *Rediaron) GetMulti(ctx context.Context, keys []string) (map[string]string, error) {
	data := make(map[string]string, len(keys))
	if len(keys) == 0 {
		return data, nil
	}
	vals, err := r.cli.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}
	for i, val := range vals {
		value, ok := val.(string)
		if !ok {
			return nil, errors.Wrapf(redis.Nil, "key not found: %s", keys[i])
		}
		data[keys[i]] = value
	}
	return data, nil
}

func (r *Rediaron) Update(ctx context.Context, data map[string]string) error {
	keys := slices.Collect(maps.Keys(data))

	// the existence check is not part of the transaction below
	e, err := r.cli.Exists(ctx, keys...).Result()
	if err != nil {
		return err
	}
	if int(e) != len(keys) {
		return ErrKeyNotExists
	}

	return r.Put(ctx, data)
}

func (r *Rediaron) Create(ctx context.Context, data map[string]string) error {
	return r.create(ctx, data, "")
}

func (r *Rediaron) Put(ctx context.Context, data map[string]string) error {
	replace := func(pipe redis.Pipeliner) error {
		for key, value := range data {
			pipe.Set(ctx, key, value, 0)
		}
		return nil
	}

	_, err := r.cli.TxPipelined(ctx, replace)
	return err
}

func (r *Rediaron) CreateAndDecr(ctx context.Context, data map[string]string, decrKey string) error {
	return r.create(ctx, data, decrKey)
}

func (r *Rediaron) Delete(ctx context.Context, keys []string) error {
	if len(keys) == 0 {
		return nil
	}
	return r.cli.Del(ctx, keys...).Err()
}

func (r *Rediaron) BindStatus(ctx context.Context, entityKey, statusKey, statusValue string, ttl int64) error {
	refreshed, err := refreshStatusScript.Run(ctx, r.cli, []string{entityKey, statusKey}, statusValue, ttl).Text()
	switch {
	case err != nil:
		return err
	// mirrors etcd: a missing entity key is an error
	case refreshed == replyMissing:
		return types.ErrInvaildCount
	case refreshed == replyChanged:
		return r.cli.Set(ctx, statusKey, statusValue, time.Duration(ttl)*time.Second).Err()
	}
	return nil
}

func (r *Rediaron) create(ctx context.Context, data map[string]string, decrKey string) error {
	keys := []string{decrKey}
	values := make([]any, 0, len(data))
	for _, key := range slices.Sorted(maps.Keys(data)) {
		keys = append(keys, key)
		values = append(values, data[key])
	}
	created, err := createScript.Run(ctx, r.cli, keys, values...).Text()
	if err != nil {
		return err
	}
	switch created {
	case replyExists:
		return ErrAlreadyExists
	case replyMissing:
		return errors.Wrap(ErrKeyNotExists, decrKey)
	}
	return nil
}

// go-redis does not export proto.Error, so the message is the only signal.
func isRedisNoKeyError(e error) bool {
	return e != nil && strings.Contains(e.Error(), "redis: nil")
}
