package redis

import (
	"context"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	goredis "github.com/redis/go-redis/v9"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

var (
	refreshEphemeralScript = goredis.NewScript(`
if redis.call("get", KEYS[1]) == ARGV[1] then
    return redis.call("pexpire", KEYS[1], ARGV[2])
end
return 0`)
	revokeEphemeralScript = goredis.NewScript(`
if redis.call("get", KEYS[1]) == ARGV[1] then
    return redis.call("del", KEYS[1])
end
return 0`)
)

func (r *Rediaron) StartEphemeral(ctx context.Context, path string, heartbeat time.Duration) (<-chan struct{}, func(), error) {
	token := utils.RandomID()
	set, err := r.cli.SetNX(ctx, path, token, heartbeat).Result()
	if err != nil {
		return nil, nil, err
	}
	if !set {
		return nil, nil, errors.Wrap(types.ErrKeyExists, path)
	}

	ctx, cancel := context.WithCancel(ctx)
	expiry := make(chan struct{})

	var wg sync.WaitGroup
	wg.Go(func() {
		defer log.SentryDefer()
		defer close(expiry)
		defer r.revokeEphemeral(ctx, path, token)
		_ = utils.KeepAlive(ctx, heartbeat/3, func(ctx context.Context) error {
			return r.refreshEphemeral(ctx, path, token, heartbeat)
		})
	})

	return expiry, func() {
		cancel()
		wg.Wait()
	}, nil
}

func (r *Rediaron) revokeEphemeral(ctx context.Context, path, token string) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
	defer cancel()
	if _, err := revokeEphemeralScript.Run(ctx, r.cli, []string{path}, token).Result(); err != nil {
		log.WithFunc("store.redis.revokeEphemeral").Errorf(ctx, err, "revoke %s failed", path)
	}
}

func (r *Rediaron) refreshEphemeral(ctx context.Context, path, token string, ttl time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	refreshed, err := refreshEphemeralScript.Run(ctx, r.cli, []string{path}, token, max(ttl.Milliseconds(), int64(1))).Int()
	if err != nil {
		return err
	}
	if refreshed == 0 {
		return errors.Wrap(types.ErrKeyNotExists, path)
	}
	return nil
}
