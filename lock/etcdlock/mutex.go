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

const SessionErrorKey = "session_error_key"

// Mutex is etcdv3 lock
type Mutex struct {
	timeout time.Duration
	mutex   *concurrency.Mutex
	session *concurrency.Session
}

type ErrorHolder struct {
	mu sync.Mutex
	err error
}

func (eh *ErrorHolder) SetError(err error) {
	eh.mu.Lock()
	eh.err = err
	eh.mu.Unlock()
}

func (eh *ErrorHolder) Error() error {
	eh.mu.Lock()
	defer eh.mu.Unlock()

	return eh.err
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

	parentCtx, parentCancel := context.WithCancel(ctx)
	userCtx := context.WithValue(parentCtx, SessionErrorKey, &ErrorHolder{})

	go func() {
		defer func() {
			userCtx.Value(SessionErrorKey).(*ErrorHolder).SetError(types.ErrLockSessionDone) // give user a chance to know its error type
			parentCancel()
		}()

		<-m.session.Done()
	}()

	select {
	case <-m.session.Done():
		// Don't return an invalid ctx
		return nil, types.ErrLockSessionDone
	default:
		return userCtx, nil
	}
}

// Unlock unlock
func (m *Mutex) Unlock(_ context.Context) error {
	defer m.session.Close()
	// release resource

	lockCtx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()
	return m.unlock(lockCtx)
}

func (m *Mutex) unlock(ctx context.Context) error {
	_, err :=m.session.Client().Txn(ctx).If(m.mutex.IsOwner()).
		Then(clientv3.OpDelete(m.mutex.Key())).Commit()
	//no way to clear it...
	//m.myKey = "\x00"
	//m.myRev = -1
	return err
}
