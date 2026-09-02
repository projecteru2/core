package meta

import (
	"context"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

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
		e.revokeLease(ctx, lease.ID)
		return nil, nil, err
	case !tx.Succeeded:
		e.revokeLease(ctx, lease.ID)
		return nil, nil, errors.Wrap(types.ErrKeyExists, path)
	}

	ctx, cancel := context.WithCancel(ctx)
	expiry := make(chan struct{})
	logger := log.WithFunc("store.etcdv3.meta.StartEphemeral")

	var wg sync.WaitGroup
	wg.Go(func() {
		defer close(expiry)

		defer func() {
			revokeCtx, revokeCancel := context.WithTimeout(context.WithoutCancel(ctx), time.Minute)
			defer revokeCancel()
			if _, err := e.cliv3.Revoke(revokeCtx, lease.ID); err != nil {
				logger.Errorf(revokeCtx, err, "revoke %d with %s failed", lease.ID, path)
			}
		}()

		_ = utils.KeepAlive(ctx, heartbeat/3, func(ctx context.Context) error {
			if _, err := e.cliv3.KeepAliveOnce(ctx, lease.ID); err != nil {
				logger.Errorf(ctx, err, "keepalive %d with %s failed", lease.ID, path)
				return err
			}
			return nil
		})
	})

	return expiry, func() {
		cancel()
		wg.Wait()
	}, nil
}
