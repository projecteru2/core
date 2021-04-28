package redis

import (
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
)

var ephemeralValue = "__aaron__"

// StartEphemeral starts an empheral kv pair.
func (r *Rediaron) StartEphemeral(ctx context.Context, path string, heartbeat time.Duration) (<-chan struct{}, func(), error) {
	set, err := r.cli.SetNX(ctx, path, ephemeralValue, heartbeat).Result()
	if err != nil {
		return nil, nil, errors.WithStack(err)
	}
	if !set {
		return nil, nil, errors.Wrap(types.ErrKeyExists, path)
	}

	cctx, cancel := context.WithCancel(context.Background())
	expiry := make(chan struct{})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(expiry)

		tick := time.NewTicker(heartbeat / 3)
		defer tick.Stop()

		for {
			select {
			case <-tick.C:
				if err := r.refreshEphemeral(path, heartbeat); err != nil {
					r.revokeEphemeral(path)
					return
				}
			case <-cctx.Done():
				r.revokeEphemeral(path)
				return
			}
		}
	}()

	return expiry, func() {
		cancel()
		wg.Wait()
	}, nil
}

func (r *Rediaron) revokeEphemeral(path string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := r.cli.Del(ctx, path).Result(); err != nil {
		log.Errorf("[refreshEphemeral] revoke with %s failed: %v", path, err)
	}
}

func (r *Rediaron) refreshEphemeral(path string, ttl time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := r.cli.Expire(ctx, path, ttl).Result()
	return err
}
