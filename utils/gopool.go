package utils

import (
	"context"

	"github.com/projecteru2/core/log"
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
func (p *GoroutinePool) Go(ctx context.Context, f func()) {
	if err := p.sem.Acquire(ctx, 1); err != nil {
		log.Errorf("[GoroutinePool] Go acquire failed %v", err)
		return
	}
	go func() {
		defer p.sem.Release(1)
		f()
	}()
}

// Wait is equivalent to sync.WaitGroup.Wait()
func (p *GoroutinePool) Wait(ctx context.Context) {
	if err := p.sem.Acquire(ctx, p.max); err != nil {
		log.Errorf("[GoroutinePool] Wait acquire failed %v", err)
	}
}
