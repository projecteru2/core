package utils

import (
	"context"

	"golang.org/x/sync/semaphore"
)

type GoroutinePool struct {
	max int64
	sem *semaphore.Weighted
}

func NewGoroutinePool(max int) *GoroutinePool {
	return &GoroutinePool{
		max: int64(max),
		sem: semaphore.NewWeighted(int64(max)),
	}
}

func (p *GoroutinePool) Go(f func()) {
	p.sem.Acquire(context.Background(), 1)
	go func() {
		defer p.sem.Release(1)
		f()
	}()
}

func (p *GoroutinePool) Wait() {
	p.sem.Acquire(context.Background(), p.max)
}
