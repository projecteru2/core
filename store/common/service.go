package common

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

func (s *Store) ServiceStatusStream(ctx context.Context) (chan []string, error) {
	ch := make(chan []string)
	logger := log.WithFunc("store.common.ServiceStatusStream")
	prefix := fmt.Sprintf(ServiceStatusKey, "")
	if err := s.Pool.Invoke(func() {
		defer close(ch)
		s.watchRetry(ctx, logger, func(ctx context.Context) error {
			return s.serviceStatusStream(ctx, prefix, ch)
		})
	}); err != nil {
		return nil, err
	}
	return ch, nil
}

func (s *Store) RegisterService(ctx context.Context, serviceAddress string, expire time.Duration) (<-chan struct{}, func(), error) {
	return s.StartEphemeral(ctx, fmt.Sprintf(ServiceStatusKey, serviceAddress), expire)
}

func (s *Store) serviceStatusStream(ctx context.Context, prefix string, ch chan<- []string) error {
	watchCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	watch := s.Watch(watchCtx, prefix)

	data, err := s.GetPrefix(ctx, prefix, 0)
	if err != nil {
		return err
	}
	eps := Endpoints{}
	for key := range data {
		eps.Add(utils.Tail(key))
	}
	select {
	case ch <- eps.ToSlice():
	case <-ctx.Done():
		return ctx.Err()
	}

	for event := range watch {
		endpoint := utils.Tail(event.Key)
		var changed bool
		if event.Type == EventPut {
			changed = eps.Add(endpoint)
		} else {
			changed = eps.Remove(endpoint)
		}
		if changed {
			select {
			case ch <- eps.ToSlice():
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	return types.ErrMessageChanClosed
}

type Endpoints map[string]struct{}

func (e Endpoints) Add(endpoint string) (changed bool) {
	if _, ok := e[endpoint]; !ok {
		e[endpoint] = struct{}{}
		changed = true
	}
	return changed
}

func (e Endpoints) Remove(endpoint string) (changed bool) {
	if _, ok := e[endpoint]; ok {
		delete(e, endpoint)
		changed = true
	}
	return changed
}

func (e Endpoints) ToSlice() []string {
	return slices.Collect(maps.Keys(e))
}
