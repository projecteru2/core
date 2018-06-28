package lock

import "context"

type DistributedLock interface {
	Lock(ctx context.Context) error
	Unlock(ctx context.Context) error
}
