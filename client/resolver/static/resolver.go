package static

import (
	"strings"

	"google.golang.org/grpc/resolver"
)

// Resolver for target static://{addr1},{addr2},{addr3}
type Resolver struct {
	addresses []resolver.Address
	cc        resolver.ClientConn
}

// New Resolver
func New(cc resolver.ClientConn, endpoints string) *Resolver {
	var addresses []resolver.Address
	for _, ep := range strings.Split(endpoints, ",") {
		addresses = append(addresses, resolver.Address{Addr: ep})
	}
	cc.UpdateState(resolver.State{Addresses: addresses}) //nolint
	return &Resolver{
		cc:        cc,
		addresses: addresses,
	}
}

// ResolveNow for interface
func (r *Resolver) ResolveNow(_ resolver.ResolveNowOptions) {}

// Close for interface
func (r *Resolver) Close() {}
