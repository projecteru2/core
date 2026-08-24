package eru

import (
	"context"
	"strings"

	"google.golang.org/grpc/resolver"

	"github.com/projecteru2/core/client/servicediscovery"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
)

// Resolver for target eru://{addr}
type Resolver struct {
	cc        resolver.ClientConn
	cancel    context.CancelFunc
	discovery servicediscovery.ServiceDiscovery
}

func New(cc resolver.ClientConn, endpoint, authority string) *Resolver {
	var username, password string
	if authority != "" {
		parts := strings.Split(authority, ":")
		username, password = strings.TrimLeft(parts[0], "@"), parts[1]
	}
	authConfig := types.AuthConfig{Username: username, Password: password}
	ctx, cancel := context.WithCancel(context.Background())
	r := &Resolver{
		cc:        cc,
		cancel:    cancel,
		discovery: servicediscovery.New(endpoint, authConfig),
	}
	cc.UpdateState(resolver.State{Addresses: []resolver.Address{{Addr: endpoint}}}) //nolint
	go r.sync(ctx)
	return r
}

func (r *Resolver) ResolveNow(_ resolver.ResolveNowOptions) {}

func (r *Resolver) Close() {
	r.cancel()
}

func (r *Resolver) sync(ctx context.Context) {
	defer r.cancel()
	logger := log.WithFunc("eru.Resolver.sync")
	logger.Debug(ctx, "start sync service discovery")

	ch, err := r.discovery.Watch(ctx)
	if err != nil {
		logger.Error(ctx, err, "watch service status")
		return
	}
	for {
		select {
		case <-ctx.Done():
			logger.Debug(ctx, "watch interrupted")
			return
		case endpoints, ok := <-ch:
			if !ok {
				logger.Info(ctx, "watch closed")
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
