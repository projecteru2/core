package etcdv3

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/projecteru2/core/log"

	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func (m *Mercury) ServiceStatusStream(ctx context.Context) (chan []string, error) {
	ch := make(chan []string)
	logger := log.WithFunc("store.etcdv3.ServiceStatusStream")
	if err := m.pool.Invoke(func() {
		defer close(ch)

		// must watch prior to get
		watchChan := m.Watch(ctx, fmt.Sprintf(serviceStatusKey, ""), clientv3.WithPrefix())

		resp, err := m.Get(ctx, fmt.Sprintf(serviceStatusKey, ""), clientv3.WithPrefix())
		if err != nil {
			logger.Error(ctx, err, "failed to get current services")
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
					logger.Error(ctx, resp.Err(), "watch failed")
				}
				return
			}

			changed := false
			for _, ev := range resp.Events {
				endpoint := parseServiceKey(ev.Kv.Key)
				switch ev.Type {
				case mvccpb.PUT:
					changed = eps.Add(endpoint) || changed
				case mvccpb.DELETE:
					changed = eps.Remove(endpoint) || changed
				}
			}
			if changed {
				ch <- eps.ToSlice()
			}
		}
	}); err != nil {
		return nil, err
	}
	return ch, nil
}

func (m *Mercury) RegisterService(ctx context.Context, serviceAddress string, expire time.Duration) (<-chan struct{}, func(), error) {
	key := fmt.Sprintf(serviceStatusKey, serviceAddress)
	return m.StartEphemeral(ctx, key, expire)
}

type endpoints map[string]struct{}

func (e endpoints) Add(endpoint string) (changed bool) {
	if _, ok := e[endpoint]; !ok {
		e[endpoint] = struct{}{}
		changed = true
	}
	return changed
}

func (e endpoints) Remove(endpoint string) (changed bool) {
	if _, ok := e[endpoint]; ok {
		delete(e, endpoint)
		changed = true
	}
	return changed
}

func (e endpoints) ToSlice() []string {
	return slices.Collect(maps.Keys(e))
}

func parseServiceKey(key []byte) (endpoint string) {
	parts := strings.Split(string(key), "/")
	return parts[len(parts)-1]
}
