package redislock

import (
	"context"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/muroq/redislock"
	"github.com/pkg/errors"
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

// New creates a lock
// key: name of the lock
// waitTimeout: timeout before getting the lock, Lock returns error if the lock is not acquired after this time
// lockTTL: ttl of lock, after this time, lock will be released automatically
func New(cli *redis.Client, key string, waitTimeout, lockTTL time.Duration) (*RedisLock, error) {
	if key == "" {
		return nil, errors.WithStack(types.ErrKeyIsEmpty)
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

// Lock acquires the lock
// will try waitTimeout time before getting the lock
func (r *RedisLock) Lock(ctx context.Context) (context.Context, error) {
	return r.lock(ctx, opts)
}

// TryLock tries to lock
// returns error if the lock is already acquired by someone else
// will not retry to get lock
func (r *RedisLock) TryLock(ctx context.Context) (context.Context, error) {
	return r.lock(ctx, nil)
}

func (r *RedisLock) lock(ctx context.Context, opts *redislock.Options) (context.Context, error) {
	l, err := r.lc.Obtain(ctx, r.key, r.timeout, r.ttl, opts)
	if err != nil {
		return nil, err
	}

	r.l = l
	return context.Background(), nil
}

// Unlock releases the lock
// if the lock is not acquired, will return ErrLockNotHeld
func (r *RedisLock) Unlock(ctx context.Context) error {
	if r.l == nil {
		return redislock.ErrLockNotHeld
	}

	lockCtx, cancel := context.WithTimeout(ctx, r.ttl)
	defer cancel()
	return r.l.Release(lockCtx)
}
