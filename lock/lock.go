package lock

import "context"

// DistributedLock is a lock based on something
type DistributedLock interface {
	Lock(ctx context.Context) (context.Context, error)
	Unlock(ctx context.Context) error
}
