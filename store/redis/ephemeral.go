package redis

import (
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/projecteru2/core/log"
)

var ephemeralValue = "__aaron__"

// StartEphemeral starts an empheral kv pair.
func (r *Rediaron) StartEphemeral(ctx context.Context, path string, heartbeat time.Duration) (<-chan struct{}, func(), error) {
	set, err := r.cli.SetNX(ctx, path, ephemeralValue, heartbeat).Result()
	if err != nil {
		return nil, nil, errors.WithStack(err)
	}
	if !set {
		return nil, nil, ErrAlreadyExists
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
			if _, err := r.cli.Del(context.Background(), path).Result(); err != nil {
				log.Errorf("[StartEphemeral] revoke with %s failed: %v", path, err)
			}
		}

		for {
			select {
			case <-tick.C:
				if _, err := r.cli.Expire(context.Background(), path, heartbeat).Result(); err != nil {
					log.Errorf("[StartEphemeral] keepalive with %s failed: %v", path, err)
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
