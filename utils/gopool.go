package utils

import (
	"context"

	"golang.org/x/sync/semaphore"
)

// GoroutinePool can spawn goroutine limited by a max number
// a limited version of sync.WaitGroup
type GoroutinePool struct {
	max int64
	sem *semaphore.Weighted
}

// NewGoroutinePool new a pool
func NewGoroutinePool(max int) *GoroutinePool {
	return &GoroutinePool{
		max: int64(max),
		sem: semaphore.NewWeighted(int64(max)),
	}
}

// Go spawns new goroutine, but may block due to max number limit
func (p *GoroutinePool) Go(f func()) {
	// there won't be error once we use background ctx
	p.sem.Acquire(context.Background(), 1) // nolint:errcheck
	go func() {
		defer p.sem.Release(1)
		f()
	}()
}

// Wait is equivalent to sync.WaitGroup.Wait()
func (p *GoroutinePool) Wait() {
	// there won't be error once we use background ctx
	p.sem.Acquire(context.Background(), p.max) // nolint:errcheck
}
