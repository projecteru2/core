package etcdv3

import (
	"context"
	"fmt"
	"time"

	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/store/common"
	"github.com/projecteru2/core/utils"
)

func (m *Mercury) ServiceStatusStream(ctx context.Context) (chan []string, error) {
	ch := make(chan []string)
	logger := log.WithFunc("store.etcdv3.ServiceStatusStream")
	if err := m.pool.Invoke(func() {
		defer close(ch)

		// must watch prior to get
		watchChan := m.Watch(ctx, fmt.Sprintf(common.ServiceStatusKey, ""), clientv3.WithPrefix())

		resp, err := m.Get(ctx, fmt.Sprintf(common.ServiceStatusKey, ""), clientv3.WithPrefix())
		if err != nil {
			logger.Error(ctx, err, "failed to get current services")
			return
		}
		eps := common.Endpoints{}
		for _, ev := range resp.Kvs {
			eps.Add(utils.Tail(string(ev.Key)))
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
				endpoint := utils.Tail(string(ev.Kv.Key))
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
	key := fmt.Sprintf(common.ServiceStatusKey, serviceAddress)
	return m.StartEphemeral(ctx, key, expire)
}
