package lock

import "context"

// DistributedLock is a mutual exclusion lock shared by every core instance.
type DistributedLock interface {
	Lock(ctx context.Context) (context.Context, error)
	TryLock(ctx context.Context) (context.Context, error)
	Unlock(ctx context.Context) error
}
