package eru

import (
	"context"
	"strings"

	"github.com/projecteru2/core/client/servicediscovery"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"google.golang.org/grpc/resolver"
)

// Resolver for target eru://{addr}
type Resolver struct {
	cc        resolver.ClientConn
	cancel    context.CancelFunc
	discovery servicediscovery.ServiceDiscovery
}

// New Resolver
func New(cc resolver.ClientConn, endpoint string, authority string) *Resolver {
	var username, password string
	if authority != "" {
		parts := strings.Split(authority, ":")
		username, password = strings.TrimLeft(parts[0], "@"), parts[1]
	}
	authConfig := types.AuthConfig{Username: username, Password: password}
	r := &Resolver{
		cc:        cc,
		discovery: servicediscovery.New(endpoint, authConfig),
	}
	cc.UpdateState(resolver.State{Addresses: []resolver.Address{{Addr: endpoint}}})
	go r.sync()
	return r
}

// ResolveNow for interface
func (r *Resolver) ResolveNow(_ resolver.ResolveNowOptions) {}

// Close for interface
func (r *Resolver) Close() {
	r.cancel()
}

func (r *Resolver) sync() {
	log.Debug("[EruResolver] start sync service discovery")
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	defer cancel()

	ch, err := r.discovery.Watch(ctx)
	if err != nil {
		log.Errorf("[EruResolver] failed to watch service status: %v", err)
		return
	}
	for {
		select {
		case <-ctx.Done():
			log.Errorf("[EruResolver] watch interrupted: %v", ctx.Err())
			return
		case endpoints, ok := <-ch:
			if !ok {
				log.Error("[EruResolver] watch closed")
				return
			}

			var addresses []resolver.Address
			log.Debugf("[EruResolver] update state: %v", endpoints)
			for _, ep := range endpoints {
				addresses = append(addresses, resolver.Address{Addr: ep})
			}
			r.cc.UpdateState(resolver.State{Addresses: addresses})
		}
	}

}
