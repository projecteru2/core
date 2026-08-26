package common

import (
	"cmp"
	"context"
	"iter"
	"time"

	"github.com/panjf2000/ants/v2"

	"github.com/projecteru2/core/lock"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
)

const (
	EventPut EventType = iota
	EventDelete
	EventExpire
)

type EventType int

// Event is one key change reported by a backend watch.
type Event struct {
	Key  string
	Type EventType
}

// KV is the key-value surface both store backends provide.
type KV interface {
	GetOne(ctx context.Context, key string) (string, error)
	GetMulti(ctx context.Context, keys []string) (map[string]string, error)
	GetPrefix(ctx context.Context, prefix string, limit int64) (map[string]string, error)
	ListPrefix(ctx context.Context, prefix string) ([]string, error)
	NotFound(err error) bool

	Create(ctx context.Context, data map[string]string) error
	Update(ctx context.Context, data map[string]string) error
	Put(ctx context.Context, data map[string]string) error
	Delete(ctx context.Context, keys []string) error
	CreateAndDecr(ctx context.Context, data map[string]string, decrKey string) error
	BindStatus(ctx context.Context, entityKey, statusKey, statusValue string, ttl int64) error

	// Watch registers the watch before it returns, so no event is lost between a read and the first iteration.
	Watch(ctx context.Context, prefix string) iter.Seq[Event]

	StartEphemeral(ctx context.Context, path string, heartbeat time.Duration) (<-chan struct{}, func(), error)
	CreateLock(key string, ttl time.Duration) (lock.DistributedLock, error)
}

// Store is the backend-independent half of the eru metadata store.
type Store struct {
	KV

	Config types.Config
	Pool   *ants.PoolWithFunc
}

func New(kv KV, config types.Config, pool *ants.PoolWithFunc) *Store {
	return &Store{KV: kv, Config: config, Pool: pool}
}

func (s *Store) watchRetry(ctx context.Context, logger *log.Fields, body func(context.Context) error) {
	retryInterval := cmp.Or(s.Config.ConnectionTimeout, time.Second)
	for ctx.Err() == nil {
		if err := body(ctx); err != nil && ctx.Err() == nil {
			logger.Error(ctx, err, "status stream interrupted")
		}
		if ctx.Err() != nil {
			return
		}
		timer := time.NewTimer(retryInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}
