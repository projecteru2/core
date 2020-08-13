package eru

import (
	"context"

	"github.com/projecteru2/core/client/service_discovery"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/resolver"
)

type eruResolver struct {
	cc        resolver.ClientConn
	cancel    context.CancelFunc
	discovery service_discovery.ServiceDiscovery
}

func New(cc resolver.ClientConn, endpoint string) *eruResolver {
	r := &eruResolver{
		cc:        cc,
		discovery: service_discovery.New(endpoint),
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
	log.Info("[eruResolver] start sync service discovery")
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	defer cancel()

	ch, err := r.discovery.Watch(ctx)
	if err != nil {
		log.Errorf("[eruResolver] failed to watch service status: %v", err)
		return
	}
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
			log.Debugf("[eruResolver] update state: %v", endpoints)
			for _, ep := range endpoints {
				addresses = append(addresses, resolver.Address{Addr: ep})
			}
			r.cc.UpdateState(resolver.State{Addresses: addresses})
		}
	}

}
