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
	cc.UpdateState(resolver.State{Addresses: []resolver.Address{{Addr: endpoint}}}) //nolint
	go r.sync(context.TODO())
	return r
}

// ResolveNow for interface
func (r *Resolver) ResolveNow(_ resolver.ResolveNowOptions) {}

// Close for interface
func (r *Resolver) Close() {
	r.cancel()
}

func (r *Resolver) sync(ctx context.Context) {
	ctx, r.cancel = context.WithCancel(ctx)
	defer r.cancel()
	logger := log.WithFunc("resolver.sync")
	logger.Debug(ctx, "start sync service discovery")

	ch, err := r.discovery.Watch(ctx)
	if err != nil {
		logger.Error(ctx, err, "failed to watch service status")
		return
	}
	for {
		select {
		case <-ctx.Done():
			logger.Error(ctx, ctx.Err(), "watch interrupted")
			return
		case endpoints, ok := <-ch:
			if !ok {
				logger.Info(ctx, nil, "watch closed")
				return
			}

			var addresses []resolver.Address
			for _, ep := range endpoints {
				addresses = append(addresses, resolver.Address{Addr: ep})
			}
			r.cc.UpdateState(resolver.State{Addresses: addresses}) //nolint
		}
	}

}
