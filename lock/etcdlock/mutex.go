package etcdlock

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/projecteru2/core/types"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
)

// Mutex is an etcd session based distributed lock.
type Mutex struct {
	timeout   time.Duration
	mutex     *concurrency.Mutex
	session   *concurrency.Session
	locked    bool
	lockedMux sync.Mutex
}

// New creates a Mutex on key, released automatically after ttl.
func New(cli *clientv3.Client, key string, ttl time.Duration) (*Mutex, error) {
	if key == "" {
		return nil, types.ErrLockKeyInvaild
	}

	if !strings.HasPrefix(key, "/") {
		key = fmt.Sprintf("/%s", key)
	}

	session, err := concurrency.NewSession(cli, concurrency.WithTTL(int(ttl.Seconds())))
	if err != nil {
		return nil, err
	}

	mutex := &Mutex{mutex: concurrency.NewMutex(session, key), session: session}
	mutex.timeout = ttl
	return mutex, nil
}

func (m *Mutex) Lock(ctx context.Context) (context.Context, error) {
	lockCtx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	if err := m.mutex.Lock(lockCtx); err != nil {
		return nil, err
	}

	ctx, cancel = context.WithCancel(ctx)
	rCtx := &lockContext{Context: ctx}

	m.lockedMux.Lock()
	m.locked = true
	m.lockedMux.Unlock()

	go func() {
		defer cancel()

		// session.Done() fires both on a lost lock and on our own Unlock
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
			return

		case <-ctx.Done():
			return
		}
	}()

	return rCtx, nil
}

func (m *Mutex) TryLock(ctx context.Context) (context.Context, error) {
	lockCtx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	if err := m.mutex.TryLock(lockCtx); err != nil {
		return nil, err
	}

	ctx, cancel = context.WithCancel(ctx)
	rCtx := &lockContext{Context: ctx}

	m.lockedMux.Lock()
	m.locked = true
	m.lockedMux.Unlock()

	go func() {
		defer cancel()

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
			return

		case <-ctx.Done():
			return
		}
	}()

	return rCtx, nil
}

func (m *Mutex) Unlock(ctx context.Context) error {
	defer func() {
		_ = m.session.Close()
	}()

	lockCtx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()
	return m.unlock(lockCtx)
}

func (m *Mutex) unlock(ctx context.Context) error {
	m.lockedMux.Lock()
	m.locked = false
	m.lockedMux.Unlock()

	_, err := m.session.Client().Txn(ctx).If(m.mutex.IsOwner()).
		Then(clientv3.OpDelete(m.mutex.Key())).Commit()
	return err
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
