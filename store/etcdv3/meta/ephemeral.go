package meta

import (
	"context"
	"sync"
	"time"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"

	"github.com/cockroachdb/errors"
	clientv3 "go.etcd.io/etcd/client/v3"
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
		return nil, nil, errors.Wrap(types.ErrKeyExists, path)
	}

	ctx, cancel := context.WithCancel(ctx)
	expiry := make(chan struct{})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(expiry)

		tick := time.NewTicker(heartbeat / 3)
		defer tick.Stop()

		// Revokes the lease.
		defer func() {
			// It shouldn't be inheriting from the ctx.
			ctx, cancel := context.WithTimeout(context.TODO(), time.Minute)
			defer cancel()
			if _, err := e.cliv3.Revoke(ctx, lease.ID); err != nil {
				log.Errorf(ctx, err, "[StartEphemeral] revoke %d with %s failed", lease.ID, path)
			}
		}()

		for {
			select {
			case <-tick.C:
				if _, err := e.cliv3.KeepAliveOnce(ctx, lease.ID); err != nil {
					log.Errorf(ctx, err, "[StartEphemeral] keepalive %d with %s failed", lease.ID, path)
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return expiry, func() {
		cancel()
		wg.Wait()
	}, nil
}
