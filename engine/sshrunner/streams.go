package sshrunner

import (
	"context"
	"slices"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/sync/semaphore"
)

// streamConn is the part of an ssh client a long-lived session needs.
type streamConn interface {
	NewSession() (*ssh.Session, error)
	Close() error
}

type streamDialer func(context.Context) (streamConn, error)

// streamPool gives long-lived sessions their own connections, so followed logs never starve a node's control calls.
type streamPool struct {
	dial  streamDialer
	total *semaphore.Weighted

	mu    sync.Mutex
	conns []*streamClient
}

func newStreamPool(dial streamDialer) *streamPool {
	return &streamPool{dial: dial, total: semaphore.NewWeighted(maxStreamClients * maxSessions)}
}

func (p *streamPool) acquire(ctx context.Context) (*streamClient, error) {
	if err := p.total.Acquire(ctx, 1); err != nil {
		return nil, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, c := range p.conns {
		if c.held < maxSessions {
			c.hold()
			return c, nil
		}
	}
	conn, err := p.dial(ctx)
	if err != nil {
		p.total.Release(1)
		return nil, err
	}
	c := &streamClient{conn: conn}
	c.hold()
	p.conns = append(p.conns, c)
	return c, nil
}

func (p *streamPool) release(c *streamClient) {
	p.mu.Lock()
	defer p.mu.Unlock()
	c.held--
	p.total.Release(1)
	if c.held == 0 {
		c.idle = time.AfterFunc(streamIdle, func() { p.drop(c) })
	}
}

func (p *streamPool) drop(c *streamClient) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if c.held > 0 {
		return
	}
	p.forget(c)
}

// evict takes a dead connection out of the pool before its holder's slot goes back, so no one else can pick it up.
func (p *streamPool) evict(c *streamClient) {
	p.mu.Lock()
	c.held--
	p.forget(c)
	p.mu.Unlock()
	p.total.Release(1)
}

func (p *streamPool) close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, c := range slices.Clone(p.conns) {
		p.forget(c)
	}
}

func (p *streamPool) forget(c *streamClient) {
	if c.idle != nil {
		c.idle.Stop()
	}
	_ = c.conn.Close()
	p.conns = slices.DeleteFunc(p.conns, func(x *streamClient) bool { return x == c })
}

type streamClient struct {
	conn streamConn
	held int
	idle *time.Timer
}

func (c *streamClient) hold() {
	c.held++
	if c.idle != nil {
		c.idle.Stop()
		c.idle = nil
	}
}
