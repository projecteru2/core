package redislock

import (
	"context"
	"sync"
	"time"

	"github.com/bsm/redislock"

	"github.com/projecteru2/core/lock"
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
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				refreshCtx, refreshCancel := context.WithTimeout(ctx, interval)
				err := l.Refresh(refreshCtx, r.ttl, nil)
				refreshCancel()
				if err != nil {
					cancel()
					return
				}
			}
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
