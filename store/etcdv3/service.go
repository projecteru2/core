package etcdv3

import (
	"context"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"go.etcd.io/etcd/v3/clientv3"
	"go.etcd.io/etcd/v3/mvcc/mvccpb"
)

type endpoints map[string]struct{}

func (e endpoints) Add(endpoint string) (changed bool) {
	if _, ok := e[endpoint]; !ok {
		e[endpoint] = struct{}{}
		changed = true
	}
	return
}

func (e endpoints) Remove(endpoint string) (changed bool) {
	if _, ok := e[endpoint]; ok {
		delete(e, endpoint)
		changed = true
	}
	return
}

func (e endpoints) ToSlice() (eps []string) {
	for ep := range e {
		eps = append(eps, ep)
	}
	return
}

func (m *Mercury) ServiceStatusStream(ctx context.Context) (chan []string, error) {
	ch := make(chan []string)
	go func() {
		defer close(ch)
		log.Info("[ServiceStatusStream] start watching service status")
		resp, err := m.Get(ctx, serviceStatusPrefix, clientv3.WithPrefix())
		if err != nil {
			log.Errorf("[ServiceStatusStream] failed to get current services: %v", err)
			return
		}
		eps := endpoints{}
		for _, ev := range resp.Kvs {
			eps.Add(parseServiceKey(ev.Key))
		}
		ch <- eps.ToSlice()

		for resp := range m.watch(ctx, serviceStatusPrefix, clientv3.WithPrefix()) {
			if resp.Err() != nil {
				if !resp.Canceled {
					log.Errorf("[ServiceStatusStream] watch failed %v", resp.Err())
				}
				return
			}

			changed := false
			for _, ev := range resp.Events {
				endpoint := parseServiceKey(ev.Kv.Key)
				c := false
				switch ev.Type {
				case mvccpb.PUT:
					c = eps.Add(endpoint)
				case mvccpb.DELETE:
					c = eps.Remove(endpoint)
				}
				if c {
					changed = true
				}
			}
			if changed {
				ch <- eps.ToSlice()
			}
		}
	}()
	return ch, nil
}

func (m *Mercury) RegisterService(ctx context.Context, expire time.Duration) error {
	key := serviceStatusPrefix + m.config.ServiceAddress
	lease, err := m.cliv3.Grant(ctx, int64(expire/time.Second))
	if err != nil {
		return err
	}

	_, err = m.Put(ctx, key, "", clientv3.WithLease(lease.ID))
	return err
}

func (m *Mercury) UnregisterService(ctx context.Context) error {
	key := serviceStatusPrefix + m.config.ServiceAddress
	_, err := m.Delete(ctx, key)
	return err
}

func parseServiceKey(key []byte) (endpoint string) {
	parts := strings.Split(string(key), "/")
	return parts[len(parts)-1]
}
