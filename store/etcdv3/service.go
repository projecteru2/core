package etcdv3

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/projecteru2/core/log"
)

type endpoints map[string]struct{}

func (e *endpoints) Add(endpoint string) (changed bool) {
	if _, ok := (*e)[endpoint]; !ok {
		(*e)[endpoint] = struct{}{}
		changed = true
	}
	return
}

func (e *endpoints) Remove(endpoint string) (changed bool) {
	if _, ok := (*e)[endpoint]; ok {
		delete(*e, endpoint)
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

// ServiceStatusStream watches /services/ --prefix
func (m *Mercury) ServiceStatusStream(ctx context.Context) (chan []string, error) {
	ch := make(chan []string)
	go func() {
		defer close(ch)

		// must watch prior to get
		watchChan := m.Watch(ctx, fmt.Sprintf(serviceStatusKey, ""), clientv3.WithPrefix())

		resp, err := m.Get(ctx, fmt.Sprintf(serviceStatusKey, ""), clientv3.WithPrefix())
		if err != nil {
			log.Errorf(ctx, "[ServiceStatusStream] failed to get current services: %v", err)
			return
		}
		eps := endpoints{}
		for _, ev := range resp.Kvs {
			eps.Add(parseServiceKey(ev.Key))
		}
		ch <- eps.ToSlice()

		for resp := range watchChan {
			if resp.Err() != nil {
				if !resp.Canceled {
					log.Errorf(ctx, "[ServiceStatusStream] watch failed %v", resp.Err())
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

// RegisterService put /services/{address}
func (m *Mercury) RegisterService(ctx context.Context, serviceAddress string, expire time.Duration) (<-chan struct{}, func(), error) {
	key := fmt.Sprintf(serviceStatusKey, serviceAddress)
	return m.StartEphemeral(ctx, key, expire)
}

func parseServiceKey(key []byte) (endpoint string) {
	parts := strings.Split(string(key), "/")
	return parts[len(parts)-1]
}
