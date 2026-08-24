package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/store/common"
	"github.com/projecteru2/core/utils"
)

func (r *Rediaron) ServiceStatusStream(ctx context.Context) (chan []string, error) {
	key := fmt.Sprintf(common.ServiceStatusKey, "*")
	ch := make(chan []string)
	if err := r.pool.Invoke(func() {
		defer close(ch)

		watchC := r.KNotify(ctx, key)

		data, err := r.getByKeyPattern(ctx, key, 0)
		if err != nil {
			log.WithFunc("store.redis.ServiceStatusStream").Error(ctx, err, "failed to get current services")
			return
		}
		eps := common.Endpoints{}
		for k := range data {
			eps.Add(utils.Tail(k))
		}
		ch <- eps.ToSlice()

		for message := range watchC {
			changed := false
			endpoint := utils.Tail(message.Key)
			switch message.Action {
			case actionSet, actionExpire:
				changed = eps.Add(endpoint)
			case actionDel, actionExpired:
				changed = eps.Remove(endpoint)
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

func (r *Rediaron) RegisterService(ctx context.Context, serviceAddress string, expire time.Duration) (<-chan struct{}, func(), error) {
	key := fmt.Sprintf(common.ServiceStatusKey, serviceAddress)
	return r.StartEphemeral(ctx, key, expire)
}
