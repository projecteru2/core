package redis

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	perrors "github.com/pkg/errors"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

var (
	// ErrMaxRetryExceeded indicates redis transaction failed after all the retries
	ErrMaxRetryExceeded = errors.New("[Redis transaction] Max retry exceeded")
	// ErrAlreadyExists indicates the key already exists when do redis SETNX
	ErrAlreadyExists = errors.New("[Redis setnx] Already exists")
	// ErrBadCmdType indicates command type is not correct
	// e.g. SET should be StringCmd
	ErrBadCmdType = errors.New("[Redis cmd] Bad cmd type")
	// ErrKeyNotExitsts indicates no key found
	// When do update, we need to ensure the key exists, just like the behavior of etcd client
	ErrKeyNotExitsts = errors.New("[Redis exists] Key not exists")

	_cache = utils.NewEngineCache(12*time.Hour, 10*time.Minute)
)

const (
	// storage key pattern
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

	// keyspace notification prefix pattern
	keyNotifyPrefix = "__keyspace@%d__:%s"

	// key event action
	actionExpire  = "expire"
	actionExpired = "expired"
	actionSet     = "set"
	actionDel     = "del"
)

// go-redis doesn't export its proto.Error type,
// we have to check the content in this error
func isRedisNoKeyError(e error) bool {
	return e != nil && strings.Contains(e.Error(), "redis: nil")
}

// Rediaron is a store implemented by redis
type Rediaron struct {
	cli    *redis.Client
	config types.Config
	db     int
}

// New creates a new Rediaron instance from config
// Only redis address and db is used
// db is used to separate data, by default db 0 will be used
func New(config types.Config, embeddedStorage bool) (*Rediaron, error) {
	cli := redis.NewClient(&redis.Options{
		Addr: config.Redis.Addr,
		DB:   config.Redis.DB,
	})

	return &Rediaron{
		cli:    cli,
		config: config,
		db:     config.Redis.DB,
	}, nil
}

// KNotifyMessage is received when using KNotify
type KNotifyMessage struct {
	Key    string
	Action string
}

// KNotify is like `watch` in etcd
// knotify comes from inotify, when a key is changed, notification will be published
func (r *Rediaron) KNotify(ctx context.Context, pattern string) chan *KNotifyMessage {
	ch := make(chan *KNotifyMessage)
	go func() {
		defer close(ch)

		prefix := fmt.Sprintf(keyNotifyPrefix, r.db, "")
		channel := fmt.Sprintf(keyNotifyPrefix, r.db, pattern)
		pubsub := r.cli.PSubscribe(ctx, channel)
		subC := pubsub.Channel()

		for {
			select {
			case <-ctx.Done():
				pubsub.Close()
				return
			case v := <-subC:
				if v == nil {
					log.Warnf(ctx, "[KNotify] channel already closed, knotify returns")
					return
				}
				ch <- &KNotifyMessage{
					Key:    strings.TrimPrefix(v.Channel, prefix),
					Action: strings.ToLower(v.Payload),
				}
			}
		}
	}()
	return ch
}

// GetOne is a wrapper
func (r *Rediaron) GetOne(ctx context.Context, key string) (string, error) {
	value, err := r.cli.Get(ctx, key).Result()
	if isRedisNoKeyError(err) {
		return "", perrors.WithMessage(err, fmt.Sprintf("Key not found: %s", key))
	}
	return value, err
}

// GetMulti is a wrapper
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
			return nil, perrors.WithMessage(err, fmt.Sprintf("Key not found: %s", key))
		}

		data[key] = c.Val()
	}
	return data, err
}

// BatchUpdate is wrapper to adapt etcd batch update
func (r *Rediaron) BatchUpdate(ctx context.Context, data map[string]string) error {
	keys := []string{}
	for k := range data {
		keys = append(keys, k)
	}

	// check existence of keys
	// FIXME: no transaction ensured
	e, err := r.cli.Exists(ctx, keys...).Result()
	if err != nil {
		return err
	}
	if int(e) != len(keys) {
		return ErrKeyNotExitsts
	}

	update := func(pipe redis.Pipeliner) error {
		for key, value := range data {
			_, err := r.cli.Set(ctx, key, value, 0).Result()
			if err != nil {
				return err
			}
		}

		return nil
	}
	_, err = r.cli.TxPipelined(ctx, update)
	return err
}

// BatchCreate is wrapper to adapt etcd batch create
func (r *Rediaron) BatchCreate(ctx context.Context, data map[string]string) error {
	create := func(pipe redis.Pipeliner) error {
		for key, value := range data {
			created, err := r.cli.SetNX(ctx, key, value, 0).Result()
			if !created {
				return ErrAlreadyExists
			}
			if err != nil {
				return err
			}
		}
		return nil
	}
	_, err := r.cli.TxPipelined(ctx, create)
	return err
}

// BatchCreateAndDecr decr processing and add workload
func (r *Rediaron) BatchCreateAndDecr(ctx context.Context, data map[string]string, decrKey string) (err error) {
	batchCreateAndDecr := func(pipe redis.Pipeliner) error {
		pipe.Decr(ctx, decrKey)
		for key, value := range data {
			pipe.SetNX(ctx, key, value, 0)
		}
		return nil
	}
	_, err = r.cli.TxPipelined(ctx, batchCreateAndDecr)
	return
}

// BatchDelete is wrapper to adapt etcd batch delete
func (r *Rediaron) BatchDelete(ctx context.Context, keys []string) error {
	del := func(pipe redis.Pipeliner) error {
		for _, key := range keys {
			_, err := pipe.Del(ctx, key).Result()
			if err != nil {
				return err
			}
		}
		return nil
	}
	_, err := r.cli.TxPipelined(ctx, del)
	return err
}

// BindStatus is wrapper to adapt etcd bind status
func (r *Rediaron) BindStatus(ctx context.Context, entityKey, statusKey, statusValue string, ttl int64) error {
	count, err := r.cli.Exists(ctx, entityKey).Result()
	if err != nil {
		return err
	}
	// doesn't exist, returns nil, does nothing
	// to behave just like etcd
	if count != 1 {
		return nil
	}

	_, err = r.cli.Set(ctx, statusKey, statusValue, time.Duration(ttl)*time.Second).Result()
	return err
}

// TerminateEmbededStorage terminates embedded store
// in order to implement Store interface
// we can't use embedded redis, it doesn't support keyspace notification
// never call this except running unittests
func (r *Rediaron) TerminateEmbededStorage() {
	_ = r.cli.Close()
}
