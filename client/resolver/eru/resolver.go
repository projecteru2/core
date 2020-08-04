package eru

import (
	"context"

	"github.com/projecteru2/core/client/service_watcher"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/resolver"
)

type eruResolver struct {
	cc      resolver.ClientConn
	cancel  context.CancelFunc
	watcher service_watcher.ServiceWatcher
}

func New(cc resolver.ClientConn, endpoints string) *eruResolver {
	r := &eruResolver{
		cc:      cc,
		watcher: service_watcher.New(),
	}
	go r.sync()
	return r
}

// ResolveNow for interface
func (r *eruResolver) ResolveNow(_ resolver.ResolveNowOptions) {}

// Close for interface
func (r *eruResolver) Close() {
	r.cancel()
}

func (r *eruResolver) sync() {
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	defer cancel()

	ch := r.watcher.Watch(ctx)
	for {
		select {
		case <-ctx.Done():
			log.Errorf("[eruResolver] watch interrupted: %v", ctx.Err())
			break
		case endpoints, ok := <-ch:
			if !ok {
				log.Error("[eruResolver] watch closed")
				break
			}

			var addresses []resolver.Address
			for _, ep := range endpoints {
				addresses = append(addresses, resolver.Address{Addr: ep})
			}
			r.cc.UpdateState(resolver.State{Addresses: addresses})
		}
	}

}
