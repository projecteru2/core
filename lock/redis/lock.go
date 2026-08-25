package redislock

import (
	"context"
	"strings"
	"time"

	"github.com/bsm/redislock"

	"github.com/projecteru2/core/types"
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
}

// New creates a lock on key, waiting at most waitTimeout to acquire it and holding it for lockTTL.
func New(cli redislock.RedisClient, key string, waitTimeout, lockTTL time.Duration) (*RedisLock, error) {
	if key == "" {
		return nil, types.ErrLockKeyInvaild
	}

	if !strings.HasPrefix(key, "/") {
		key = "/" + key
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
	if err := r.lock(lockCtx, opts); err != nil {
		return nil, err
	}
	return ctx, nil
}

func (r *RedisLock) Unlock(ctx context.Context) error {
	if r.l == nil {
		return redislock.ErrLockNotHeld
	}

	lockCtx, cancel := context.WithTimeout(ctx, r.ttl)
	defer cancel()
	return r.l.Release(lockCtx)
}

func (r *RedisLock) lock(ctx context.Context, opts *redislock.Options) error {
	l, err := r.lc.Obtain(ctx, r.key, r.ttl, opts)
	if err != nil {
		return err
	}

	r.l = l
	return nil
}
