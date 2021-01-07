package etcdv3

import (
	"context"
	"time"

	"go.etcd.io/etcd/clientv3"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
)

// StartEphemeral starts an empheral kv pair.
func (m *Mercury) StartEphemeral(ctx context.Context, path string, heartbeat time.Duration) (<-chan struct{}, func(), error) {
	lease, err := m.cliv3.Grant(ctx, int64(heartbeat/time.Second))
	if err != nil {
		return nil, nil, err
	}

	switch tx, err := m.cliv3.Txn(ctx).
		If(clientv3.Compare(clientv3.Version(path), "=", 0)).
		Then(clientv3.OpPut(path, "", clientv3.WithLease(lease.ID))).
		Commit(); {
	case err != nil:
		return nil, nil, err
	case !tx.Succeeded:
		return nil, nil, types.NewDetailedErr(types.ErrKeyExists, path)
	}

	ctx, cancel := context.WithCancel(context.Background())
	expiry := make(chan struct{})

	go func() {
		defer close(expiry)

		tick := time.NewTicker(heartbeat / 3)
		defer tick.Stop()

		revoke := func() {
			if _, err := m.cliv3.Revoke(context.Background(), lease.ID); err != nil {
				log.Errorf("[StartEphemeral] revoke %d with %s failed: %v", lease.ID, path, err)
			}
		}

		for {
			select {
			case <-tick.C:
				if _, err := m.cliv3.KeepAliveOnce(context.Background(), lease.ID); err != nil {
					log.Errorf("[StartEphemeral] keepalive %d with %s failed: %v", lease.ID, path, err)
					revoke()
					return
				}

			case <-ctx.Done():
				revoke()
				return
			}
		}
	}()

	return expiry, func() { cancel() }, nil
}
