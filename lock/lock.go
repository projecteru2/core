package lock

type DistributedLock interface {
	Lock() error
	Unlock() error
}
