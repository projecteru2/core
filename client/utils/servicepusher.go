package utils

import (
	"context"
	"sync"
)

// EndpointPusher fans an endpoint list out to every registered channel.
type EndpointPusher struct {
	mu    sync.Mutex
	chans []chan []string
}

func (p *EndpointPusher) Register(ch chan []string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.chans = append(p.chans, ch)
}

func (p *EndpointPusher) Push(ctx context.Context, endpoints []string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, ch := range p.chans {
		select {
		case ch <- endpoints:
		case <-ctx.Done():
			return
		}
	}
}
