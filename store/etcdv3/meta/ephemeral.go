package meta

import (
	"context"
	"sync"
	"time"

	"go.etcd.io/etcd/v3/clientv3"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
)

// StartEphemeral starts an empheral kv pair.
func (e *ETCD) StartEphemeral(ctx context.Context, path string, heartbeat time.Duration) (<-chan struct{}, func(), error) {
	lease, err := e.cliv3.Grant(ctx, int64(heartbeat/time.Second))
	if err != nil {
		return nil, nil, err
	}

	switch tx, err := e.cliv3.Txn(ctx).
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

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(expiry)

		tick := time.NewTicker(heartbeat / 3)
		defer tick.Stop()

		revoke := func() {
			if _, err := e.cliv3.Revoke(context.Background(), lease.ID); err != nil {
				log.Errorf("[StartEphemeral] revoke %d with %s failed: %v", lease.ID, path, err)
			}
		}

		for {
			select {
			case <-tick.C:
				if _, err := e.cliv3.KeepAliveOnce(context.Background(), lease.ID); err != nil {
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

	return expiry, func() {
		cancel()
		wg.Wait()
	}, nil
}
