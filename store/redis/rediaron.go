package redis

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/panjf2000/ants/v2"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"

	"github.com/cockroachdb/errors"
	"github.com/redis/go-redis/v9"
)

const (
	podInfoKey       = "/pod/info/%s" // /pod/info/{podname}
	serviceStatusKey = "/services/%s" // /service/{ipv4:port}

	nodeInfoKey      = "/node/%s"              // /node/{nodename}
	nodePodKey       = "/node/%s:pod/%s"       // /node/{podname}:pod/{nodename}
	nodeCaKey        = "/node/%s:ca"           // /node/{nodename}:ca
	nodeCertKey      = "/node/%s:cert"         // /node/{nodename}:cert
	nodeKeyKey       = "/node/%s:key"          // /node/{nodename}:key
	nodeStatusPrefix = "/status:node/"         // /status:node/{nodename} -> node status key
	nodeWorkloadsKey = "/node/%s:workloads/%s" // /node/{nodename}:workloads/{workloadID}

	workloadInfoKey          = "/workloads/%s" // /workloads/{workloadID}
	workloadDeployPrefix     = "/deploy"       // /deploy/{appname}/{entrypoint}/{nodename}/{workloadID}
	workloadStatusPrefix     = "/status"       // /status/{appname}/{entrypoint}/{nodename}/{workloadID} value -> something by agent
	workloadProcessingPrefix = "/processing"   // /processing/{appname}/{entrypoint}/{nodename}/{opsIdent} value -> count

	keyNotifyPrefix = "__keyspace@%d__:%s"

	actionExpire  = "expire"
	actionExpired = "expired"
	actionSet     = "set"
	actionDel     = "del"
)

var (
	// ErrAlreadyExists indicates SETNX found the key already set.
	ErrAlreadyExists = errors.New("key already exists")
	// ErrBadCmdType indicates a redis command replied with an unexpected type.
	ErrBadCmdType = errors.New("bad redis cmd type")
	// ErrKeyNotExists indicates an update targeted a missing key, as the etcd store reports it.
	ErrKeyNotExists = errors.New("key not exists")
)

// Rediaron is a store implemented by redis
type Rediaron struct {
	cli    *redis.Client
	config types.Config
	pool   *ants.PoolWithFunc
	db     int
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
	return &Rediaron{
		cli:    cli,
		config: config,
		pool:   pool,
		db:     config.Redis.DB,
	}, nil
}

// KNotifyMessage is received when using KNotify
type KNotifyMessage struct {
	Key    string
	Action string
}

// KNotify streams key change notifications, the redis counterpart of an etcd watch.
func (r *Rediaron) KNotify(ctx context.Context, pattern string) chan *KNotifyMessage {
	ch := make(chan *KNotifyMessage)
	_ = r.pool.Invoke(func() {
		defer close(ch)

		prefix := fmt.Sprintf(keyNotifyPrefix, r.db, "")
		channel := fmt.Sprintf(keyNotifyPrefix, r.db, pattern)
		pubsub := r.cli.PSubscribe(ctx, channel)
		subC := pubsub.Channel()

		for {
			select {
			case <-ctx.Done():
				_ = pubsub.Close()
				return
			case v := <-subC:
				if v == nil {
					log.WithFunc("store.redis.KNotify").Warn(ctx, "channel closed, knotify returns")
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
	data := map[string]string{}
	fetch := func(pipe redis.Pipeliner) error {
		for _, k := range keys {
			_, err := pipe.Get(ctx, k).Result()
			if err != nil {
				return err
			}
		}
		return nil
	}
	cmders, err := r.cli.Pipelined(ctx, fetch)
	for _, cmd := range cmders {
		c, ok := cmd.(*redis.StringCmd)
		if !ok {
			return nil, ErrBadCmdType
		}

		args := c.Args()
		if len(args) != 2 {
			return nil, ErrBadCmdType
		}

		key, ok := args[1].(string)
		if !ok {
			return nil, ErrBadCmdType
		}

		if isRedisNoKeyError(c.Err()) {
			return nil, errors.Wrapf(err, "key not found: %s", key)
		}

		data[key] = c.Val()
	}
	return data, err
}

func (r *Rediaron) BatchUpdate(ctx context.Context, data map[string]string) error {
	keys := []string{}
	for k := range data {
		keys = append(keys, k)
	}

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

	cmds, err := r.cli.TxPipelined(ctx, update)
	if err != nil {
		return err
	}

	for _, cmd := range cmds {
		if err := cmd.Err(); err != nil {
			return err
		}
	}
	return nil
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
		bc, ok := cmd.(*redis.BoolCmd)
		if !ok {
			return ErrBadCmdType
		}

		created, err := bc.Result()
		if !created {
			return ErrAlreadyExists
		}
		if err != nil {
			return err
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

	cmds, err := r.cli.TxPipelined(ctx, replace)
	if err != nil {
		return err
	}

	for _, cmd := range cmds {
		if err := cmd.Err(); err != nil {
			return err
		}
	}
	return nil
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

// TerminateEmbededStorage closes the redis client; only tests call it.
func (r *Rediaron) TerminateEmbededStorage() {
	_ = r.cli.Close()
}

// go-redis does not export proto.Error, so the message is the only signal.
func isRedisNoKeyError(e error) bool {
	return e != nil && strings.Contains(e.Error(), "redis: nil")
}
