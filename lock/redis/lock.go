package redislock

import (
	"context"
	"sync"
	"time"

	"github.com/bsm/redislock"
	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/lock"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/utils"
)

var opts = &redislock.Options{
	RetryStrategy: redislock.LinearBackoff(500 * time.Millisecond),
}

// RedisLock is a redis SET NX based lock
type RedisLock struct {
	key     string
	timeout time.Duration
	ttl     time.Duration
	lc      *redislock.Client
	l       *redislock.Lock
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// New creates a lock on key, waiting at most waitTimeout to acquire it and holding it for lockTTL.
func New(cli redislock.RedisClient, key string, waitTimeout, lockTTL time.Duration) (*RedisLock, error) {
	key, err := lock.Key(key)
	if err != nil {
		return nil, err
	}

	locker := redislock.New(cli)
	return &RedisLock{
		key:     key,
		timeout: waitTimeout,
		ttl:     lockTTL,
		lc:      locker,
	}, nil
}

func (r *RedisLock) Lock(ctx context.Context) (context.Context, error) {
	lockCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	l, err := r.lc.Obtain(lockCtx, r.key, r.ttl, opts)
	if err != nil {
		return nil, err
	}
	ctx, cancel = context.WithCancel(ctx)
	r.l = l
	r.cancel = cancel
	interval := r.ttl / 3
	r.wg.Go(func() {
		err := utils.KeepAlive(ctx, interval, func(ctx context.Context) error {
			refreshCtx, refreshCancel := context.WithTimeout(ctx, interval)
			defer refreshCancel()
			err := l.Refresh(refreshCtx, r.ttl, opts)
			if err != nil && !errors.Is(err, redislock.ErrNotObtained) && ctx.Err() == nil {
				log.WithFunc("redislock.Lock").Warnf(ctx, "refresh lock %s failed: %+v", r.key, err)
				return nil
			}
			return err
		})
		if err != nil {
			cancel()
		}
	})
	return ctx, nil
}

func (r *RedisLock) Unlock(ctx context.Context) error {
	if r.l == nil {
		return redislock.ErrLockNotHeld
	}
	if r.cancel != nil {
		r.cancel()
		r.wg.Wait()
	}

	lockCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), r.ttl)
	defer cancel()
	return r.l.Release(lockCtx)
}
