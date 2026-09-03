package common

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

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

func (s *Store) GetServiceStatus(ctx context.Context) ([]string, error) {
	eps, err := s.getServiceEndpoints(ctx, fmt.Sprintf(ServiceStatusKey, ""))
	if err != nil {
		return nil, err
	}
	return eps.ToSlice(), nil
}

func (s *Store) ServiceStatusStream(ctx context.Context) (chan []string, error) {
	ch := make(chan []string)
	logger := log.WithFunc("store.common.ServiceStatusStream")
	prefix := fmt.Sprintf(ServiceStatusKey, "")
	utils.SentryGo(func() {
		defer close(ch)
		retryInterval := cmp.Or(s.Config.ConnectionTimeout, time.Second)
		for ctx.Err() == nil {
			if err := s.serviceStatusStream(ctx, prefix, ch); err != nil && ctx.Err() == nil {
				logger.Error(ctx, err, "service status stream interrupted")
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(retryInterval):
			}
		}
	})
	return ch, nil
}

func (s *Store) RegisterService(ctx context.Context, serviceAddress string, expire time.Duration) (<-chan struct{}, func(), error) {
	return s.StartEphemeral(ctx, fmt.Sprintf(ServiceStatusKey, serviceAddress), expire)
}

func (s *Store) serviceStatusStream(ctx context.Context, prefix string, ch chan<- []string) error {
	watchCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	watch := s.Watch(watchCtx, prefix)

	eps, err := s.getServiceEndpoints(ctx, prefix)
	if err != nil {
		return err
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

func (s *Store) getServiceEndpoints(ctx context.Context, prefix string) (Endpoints, error) {
	data, err := s.GetPrefix(ctx, prefix, 0)
	if err != nil {
		return nil, err
	}
	eps := Endpoints{}
	for key := range data {
		eps.Add(utils.Tail(key))
	}
	return eps, nil
}
