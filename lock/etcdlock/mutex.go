package etcdlock

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/projecteru2/core/types"
	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/clientv3/concurrency"
	"golang.org/x/net/context"
)

// Mutex is etcdv3 lock
type Mutex struct {
	timeout time.Duration
	mutex   *concurrency.Mutex
	session *concurrency.Session
}

type lockContext struct {
	err   error
	mutex sync.Mutex
	context.Context
}

func (c *lockContext) setError(err error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.err = err
}

func (c *lockContext) Err() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if c.err != nil {
		return c.err
	}
	return c.Context.Err()
}

// New new a lock
func New(cli *clientv3.Client, key string, ttl time.Duration) (*Mutex, error) {
	if key == "" {
		return nil, types.ErrKeyIsEmpty
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

// Lock get locked
func (m *Mutex) Lock(ctx context.Context) (context.Context, error) {
	lockCtx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	if err := m.mutex.Lock(lockCtx); err != nil {
		return nil, err
	}

	ctx, cancel = context.WithCancel(ctx)
	rCtx := &lockContext{Context: ctx}

	go func() {
		defer cancel()

		select {
		case <-m.session.Done():
			rCtx.setError(types.ErrLockSessionDone)
		case <-ctx.Done():
		}
	}()

	return rCtx, nil
}

// Unlock unlock
func (m *Mutex) Unlock(ctx context.Context) error {
	defer m.session.Close()
	// release resource

	lockCtx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()
	return m.unlock(lockCtx)
}

func (m *Mutex) unlock(ctx context.Context) error {
	_, err := m.session.Client().Txn(ctx).If(m.mutex.IsOwner()).
		Then(clientv3.OpDelete(m.mutex.Key())).Commit()
	// no way to clear it...
	// m.myKey = "\x00"
	// m.myRev = -1
	return err
}
