package lock

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
func NewMutex(c client.KeysAPI, key string, ttl int) *Mutex {
	hostname, err := os.Hostname()
	if err != nil || key == "" {
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

		log.Debugf("%s Lock node %v ERROR %v", m.id, m.key, err)

		if try < defaultTry {
			log.Debugf("%s Try to lock node %v again ERROR, %v", m.id, m.key, err)
		}
	}
	return err
}

func (m *Mutex) lock() (err error) {
	log.Debugf("%s Trying to create a node : key=%v", m.id, m.key)
	setOptions := &client.SetOptions{
		PrevExist: client.PrevNoExist,
		TTL:       m.ttl,
	}
	for {
		resp, err := m.kapi.Set(context.TODO(), m.key, m.id, setOptions)
		if err == nil {
			log.Debugf("%s Create node %v OK [%q]", m.id, m.key, resp)
			return nil
		}

		log.Debugf("%s Create node %v failed [%v]", m.id, m.key, err)
		e, ok := err.(client.Error)
		if !ok {
			return err
		}

		if e.Code != client.ErrorCodeNodeExist {
			return err
		}

		// Get the already node's value.
		resp, err = m.kapi.Get(context.TODO(), m.key, nil)
		if err != nil {
			return err
		}
		log.Debugf("%s, Get node %v OK", m.id, m.key)
		watcherOptions := &client.WatcherOptions{
			AfterIndex: resp.Index,
			Recursive:  false,
		}
		watcher := m.kapi.Watcher(m.key, watcherOptions)
		for {
			log.Debugf("%s Watch %v ...", m.id, m.key)
			resp, err = watcher.Next(context.TODO())
			if err != nil {
				return err
			}

			log.Debugf("%s Received an event: %q", m.id, resp)
			if resp.Action == "delete" || resp.Action == "expire" {
				// break this for-loop, and try to create the node again.
				break
			}
		}
	}
	return err
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
		var resp *client.Response
		resp, err = m.kapi.Delete(context.TODO(), m.key, nil)
		if err == nil {
			log.Debugf("%s Delete %v OK", m.id, m.key)
			return nil
		}
		log.Debugf("%s Delete %v failed: %q", m.id, m.key, resp)
		e, ok := err.(client.Error)
		if ok && e.Code == client.ErrorCodeKeyNotFound {
			return nil
		}
	}
	return err
}
