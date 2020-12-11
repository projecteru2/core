package etcdv3

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/projecteru2/core/log"
	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/mvcc/mvccpb"
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
		resp, err := m.Get(ctx, fmt.Sprintf(serviceStatusKey, ""), clientv3.WithPrefix())
		if err != nil {
			log.Errorf("[ServiceStatusStream] failed to get current services: %v", err)
			return
		}
		eps := endpoints{}
		for _, ev := range resp.Kvs {
			eps.Add(parseServiceKey(ev.Key))
		}
		ch <- eps.ToSlice()

		for resp := range m.watch(ctx, fmt.Sprintf(serviceStatusKey, ""), clientv3.WithPrefix()) {
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

// RegisterService put /services/{address}
func (m *Mercury) RegisterService(ctx context.Context, serviceAddress string, expire time.Duration) error {
	key := fmt.Sprintf(serviceStatusKey, serviceAddress)
	lease, err := m.cliv3.Grant(ctx, int64(expire/time.Second))
	if err != nil {
		return err
	}
	tr, err := m.cliv3.Txn(ctx).
		If(clientv3.Compare(clientv3.Version(key), "!=", 0)).
		Then(clientv3.OpGet(key)).
		Else(clientv3.OpPut(key, "", clientv3.WithLease(lease.ID))).
		Commit()
	if err != nil {
		return err
	}
	if !tr.Succeeded {
		return nil
	}
	oldLeaseID := clientv3.LeaseID(tr.Responses[0].GetResponseRange().Kvs[0].Lease)
	if _, err = m.cliv3.Revoke(ctx, lease.ID); err != nil {
		log.Warnf("[RegisterService] revoke lease failed %v", err)
	}
	_, err = m.cliv3.KeepAliveOnce(ctx, oldLeaseID)
	return err
}

// UnregisterService del /services/{address}
func (m *Mercury) UnregisterService(ctx context.Context, serviceAddress string) error {
	key := fmt.Sprintf(serviceStatusKey, serviceAddress)
	_, err := m.Delete(ctx, key)
	return err
}

func parseServiceKey(key []byte) (endpoint string) {
	parts := strings.Split(string(key), "/")
	return parts[len(parts)-1]
}
