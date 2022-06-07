package utils

import (
	"context"

	"golang.org/x/sync/semaphore"

	"github.com/projecteru2/core/log"
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
func (p *GoroutinePool) Go(ctx context.Context, f func()) {
	if err := p.sem.Acquire(context.TODO(), 1); err != nil {
		log.Errorf(ctx, "[GoroutinePool] Go acquire failed %v", err)
		return
	}
	SentryGo(func() {
		defer p.sem.Release(1)
		f()
	})
}

// Wait is equivalent to sync.WaitGroup.Wait()
func (p *GoroutinePool) Wait(ctx context.Context) {
	if err := p.sem.Acquire(context.TODO(), p.max); err != nil {
		log.Errorf(ctx, "[GoroutinePool] Wait acquire failed %v", err)
	}
}
