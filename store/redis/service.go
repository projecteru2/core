package redis

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/projecteru2/core/log"
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
func (r *Rediaron) ServiceStatusStream(ctx context.Context) (chan []string, error) {
	key := fmt.Sprintf(serviceStatusKey, "*")
	ch := make(chan []string)
	go func() {
		defer close(ch)

		watchC := r.KNotify(ctx, key)

		data, err := r.getByKeyPattern(ctx, key, 0)
		if err != nil {
			log.Errorf(ctx, "[ServiceStatusStream] failed to get current services: %v", err)
			return
		}
		eps := endpoints{}
		for k := range data {
			eps.Add(parseServiceKey(k))
		}
		ch <- eps.ToSlice()

		for message := range watchC {
			changed := false
			endpoint := parseServiceKey(message.Key)
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
	}()
	return ch, nil
}

// RegisterService put /services/{address}
func (r *Rediaron) RegisterService(ctx context.Context, serviceAddress string, expire time.Duration) (<-chan struct{}, func(), error) {
	key := fmt.Sprintf(serviceStatusKey, serviceAddress)
	return r.StartEphemeral(ctx, key, expire)
}

func parseServiceKey(key string) (endpoint string) {
	parts := strings.Split(key, "/")
	return parts[len(parts)-1]
}
