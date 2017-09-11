package etcdlock

import (
	"fmt"
	"strings"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/clientv3/concurrency"
	"golang.org/x/net/context"
)

type Mutex struct {
	timeout time.Duration
	mutex   *concurrency.Mutex
	session *concurrency.Session
}

func New(cli *clientv3.Client, key string, ttl int) (*Mutex, error) {
	if key == "" {
		return nil, fmt.Errorf("No lock key")
	}

	if !strings.HasPrefix(key, "/") {
		key = fmt.Sprintf("/%s", key)
	}

	session, err := concurrency.NewSession(cli, concurrency.WithTTL(ttl))
	if err != nil {
		return nil, err
	}

	mutex := &Mutex{mutex: concurrency.NewMutex(session, key), session: session}
	mutex.timeout = time.Duration(ttl) * time.Second
	return mutex, nil
}

func (m *Mutex) Lock() error {
	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()
	return m.mutex.Lock(ctx)
}

func (m *Mutex) Unlock() error {
	defer m.session.Close()
	// 一定要释放
	return m.mutex.Unlock(context.Background())
}
