package etcdlock

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/coreos/etcd/client"
	"golang.org/x/net/context"
)

const (
	defaultTTL = 60
	defaultTry = 3
)

// A Mutex is a mutual exclusion lock which is distributed across a cluster.
type Mutex struct {
	key   string
	id    string // The identity of the caller
	kapi  client.KeysAPI
	ttl   time.Duration
	mutex sync.Mutex
}

// New creates a Mutex with the given key which must be the same
// across the cluster nodes.
// machines are the ectd cluster addresses
func New(c client.KeysAPI, key string, ttl int) *Mutex {
	if key == "" {
		return nil
		// return fmt.Errorf("A key must be given to create etcd lock, you give %q", key)
	}

	hostname, err := os.Hostname()
	if err != nil {
		return nil
	}

	if !strings.HasPrefix(key, "/") {
		key = fmt.Sprintf("/%s", key)
	}

	if ttl < 1 {
		ttl = defaultTTL
	}

	return &Mutex{
		key:   key,
		id:    fmt.Sprintf("%v-%v-%v", hostname, os.Getpid(), time.Now().Format("20060102-15:04:05.999999999")),
		kapi:  c,
		ttl:   time.Second * time.Duration(ttl),
		mutex: sync.Mutex{},
	}
}

// Lock locks m.
// If the lock is already in use, the calling goroutine
// blocks until the mutex is available.
func (m *Mutex) Lock() (err error) {
	m.mutex.Lock()
	for try := 1; try <= defaultTry; try++ {
		err = m.lock()
		if err == nil {
			return nil
		}
	}
	return err
}

func (m *Mutex) lock() (err error) {
	setOptions := &client.SetOptions{
		PrevExist: client.PrevNoExist,
		TTL:       m.ttl,
	}
	for {
		_, err := m.kapi.Set(context.TODO(), m.key, m.id, setOptions)
		if err == nil {
			// lock done
			log.WithFields(log.Fields{"ID": m.id, "Key": m.key}).Debugf("[ETCD Lock] Create node OK")
			return nil
		}

		log.WithFields(log.Fields{"ID": m.id, "Key": m.key, "Error": err}).Debugf("[ETCD Lock] Create node failed")
		if e, ok := err.(client.Error); !ok || e.Code != client.ErrorCodeNodeExist {
			return err
		}

		// Get the already node's value.
		resp, err := m.kapi.Get(context.TODO(), m.key, nil)
		if err != nil {
			return err
		}

		watcherOptions := &client.WatcherOptions{
			AfterIndex: resp.Index,
			Recursive:  false,
		}
		watcher := m.kapi.Watcher(m.key, watcherOptions)
		for {
			log.WithFields(log.Fields{"ID": m.id, "Key": m.key}).Debugf("[ETCD Lock] Start watching...")
			resp, err := watcher.Next(context.TODO())
			if err != nil {
				return err
			}

			log.WithFields(log.Fields{"ID": m.id, "Key": m.key}).Debugf("[ETCD Lock] Received an event")
			if resp.Action == "delete" || resp.Action == "expire" {
				// break this for-loop, and try to create the node again.
				break
			}
		}
	}
}

// Unlock unlocks m.
// It is a run-time error if m is not locked on entry to Unlock.
//
// A locked Mutex is not associated with a particular goroutine.
// It is allowed for one goroutine to lock a Mutex and then
// arrange for another goroutine to unlock it.
func (m *Mutex) Unlock() (err error) {
	defer m.mutex.Unlock()
	for i := 1; i <= defaultTry; i++ {
		_, err := m.kapi.Delete(context.TODO(), m.key, nil)
		if err == nil {
			log.WithFields(log.Fields{"ID": m.id, "Key": m.key}).Debugf("[ETCD Lock] Delete node OK")
			return nil
		}

		e, ok := err.(client.Error)
		if ok && e.Code == client.ErrorCodeKeyNotFound {
			return nil
		}

		log.WithFields(log.Fields{"ID": m.id, "Key": m.key, "Error": err}).Debugf("[ETCD Lock] Delete node failed")
	}
	return err
}
