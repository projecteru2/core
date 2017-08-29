package etcdlock

import (
	"fmt"
	"strings"
	"sync"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/clientv3/concurrency"
	"golang.org/x/net/context"
)

const defaultTTL = 60

type Mutex struct {
	mutex   sync.Mutex
	lock    *concurrency.Mutex
	session *concurrency.Session
}

func New(cli *clientv3.Client, key string, ttl int) (*Mutex, error) {
	if key == "" {
		return nil, fmt.Errorf("Where is lock key")
	}

	if !strings.HasPrefix(key, "/") {
		key = fmt.Sprintf("/%s", key)
	}

	if ttl < 1 {
		ttl = defaultTTL
	}

	s, err := concurrency.NewSession(
		cli,
		concurrency.WithTTL(ttl),
		concurrency.WithContext(context.Background()),
	)
	if err != nil {
		return nil, err
	}
	lock := concurrency.NewMutex(s, key)
	return &Mutex{mutex: sync.Mutex{}, lock: lock, session: s}, nil
}

func (m *Mutex) Lock() error {
	m.mutex.Lock()
	return m.lock.Lock(context.TODO())
}

func (m *Mutex) Unlock() error {
	defer m.mutex.Unlock()
	defer m.session.Close()
	return m.lock.Unlock(context.TODO())
}
