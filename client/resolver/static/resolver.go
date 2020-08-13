package static

import (
	"strings"

	"google.golang.org/grpc/resolver"
)

type staticResolver struct {
	addresses []resolver.Address
	cc        resolver.ClientConn
}

func New(cc resolver.ClientConn, endpoints string) *staticResolver {
	var addresses []resolver.Address
	for _, ep := range strings.Split(endpoints, ",") {
		addresses = append(addresses, resolver.Address{Addr: ep})
	}
	cc.UpdateState(resolver.State{Addresses: addresses})
	return &staticResolver{
		cc:        cc,
		addresses: addresses,
	}
}

// ResolveNow for interface
func (r *staticResolver) ResolveNow(_ resolver.ResolveNowOptions) {}

// Close for interface
func (r *staticResolver) Close() {}
