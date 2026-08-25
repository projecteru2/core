package lock

import (
	"context"
	"strings"

	"github.com/projecteru2/core/types"
)

// DistributedLock is a mutual exclusion lock shared by every core instance.
type DistributedLock interface {
	Lock(ctx context.Context) (context.Context, error)
	Unlock(ctx context.Context) error
}

// Key validates a lock key and gives it the leading slash every backend indexes on.
func Key(key string) (string, error) {
	if key == "" {
		return "", types.ErrLockKeyInvaild
	}
	if strings.HasPrefix(key, "/") {
		return key, nil
	}
	return "/" + key, nil
}
