package etcdlock

import (
	"context"
	"sync"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"

	"github.com/projecteru2/core/lock"
	"github.com/projecteru2/core/types"
)

const maxIdleSessions = 64

// Pool keeps etcd sessions alive between locks, so a lock cycle costs two transactions instead of a lease grant and revoke as well.
type Pool struct {
	cli  *clientv3.Client
	ttl  int
	mu   sync.Mutex
	idle []*concurrency.Session
}

// NewPool builds a pool whose sessions carry a lease of ttl.
func NewPool(cli *clientv3.Client, ttl time.Duration) *Pool {
	return &Pool{cli: cli, ttl: int(ttl.Seconds())}
}

func (p *Pool) get() (*concurrency.Session, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for len(p.idle) > 0 {
		session := p.idle[len(p.idle)-1]
		p.idle = p.idle[:len(p.idle)-1]
		select {
		case <-session.Done():
		default:
			return session, nil
		}
	}
	return concurrency.NewSession(p.cli, concurrency.WithTTL(p.ttl))
}

func (p *Pool) put(session *concurrency.Session) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.idle) >= maxIdleSessions {
		_ = session.Close()
		return
	}
	p.idle = append(p.idle, session)
}

// Mutex is an etcd session based distributed lock.
type Mutex struct {
	timeout   time.Duration
	pool      *Pool
	mutex     *concurrency.Mutex
	session   *concurrency.Session
	locked    bool
	lockedMux sync.Mutex
}

// New creates a Mutex on key over a pooled session; Lock and Unlock each give up after timeout.
func New(pool *Pool, key string, timeout time.Duration) (*Mutex, error) {
	key, err := lock.Key(key)
	if err != nil {
		return nil, err
	}

	session, err := pool.get()
	if err != nil {
		return nil, err
	}

	return &Mutex{mutex: concurrency.NewMutex(session, key), session: session, pool: pool, timeout: timeout}, nil
}

func (m *Mutex) Lock(ctx context.Context) (context.Context, error) {
	lockCtx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	if err := m.mutex.Lock(lockCtx); err != nil {
		return nil, err
	}
	return m.watchSession(ctx), nil
}

func (m *Mutex) Unlock(ctx context.Context) error {
	lockCtx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()
	err := m.unlock(lockCtx)
	if err != nil {
		_ = m.session.Close()
		return err
	}
	m.pool.put(m.session)
	return nil
}

func (m *Mutex) unlock(ctx context.Context) error {
	m.lockedMux.Lock()
	m.locked = false
	m.lockedMux.Unlock()

	_, err := m.session.Client().Txn(ctx).If(m.mutex.IsOwner()).
		Then(clientv3.OpDelete(m.mutex.Key())).Commit()
	return err
}

func (m *Mutex) watchSession(ctx context.Context) context.Context {
	ctx, cancel := context.WithCancel(ctx)
	rCtx := &lockContext{Context: ctx}

	m.lockedMux.Lock()
	m.locked = true
	m.lockedMux.Unlock()

	go func() {
		defer cancel()

		// session.Done() fires on a lost lease, which the lock outlives only while it is still held
		select {
		case <-m.session.Done():
			m.lockedMux.Lock()
			if m.locked {
				rCtx.setError(types.ErrLockSessionDone)
				m.lockedMux.Unlock()
				return
			}
			m.lockedMux.Unlock()
			<-ctx.Done()
		case <-ctx.Done():
		}
	}()

	return rCtx
}

type lockContext struct {
	err   error
	mutex sync.Mutex
	context.Context
}

func (c *lockContext) Err() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if c.err != nil {
		return c.err
	}
	return c.Context.Err()
}

func (c *lockContext) setError(err error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.err = err
}
