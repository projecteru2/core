package redis

import (
	"fmt"
	"time"

	"github.com/projecteru2/core/lock"
	redislock "github.com/projecteru2/core/lock/redis"
)

// CreateLock creates a redis based lock
func (r *Rediaron) CreateLock(key string, ttl time.Duration) (lock.DistributedLock, error) {
	lockKey := fmt.Sprintf("%s/%s", r.config.Redis.LockPrefix, key)
	return redislock.New(r.cli, lockKey, ttl, ttl)
}
