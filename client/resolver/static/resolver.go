package static

import (
	"strings"

	"google.golang.org/grpc/resolver"
)

// Resolver for target static://{addr1},{addr2},{addr3}
type Resolver struct{}

func New(cc resolver.ClientConn, endpoints string) *Resolver {
	var addresses []resolver.Address
	for ep := range strings.SplitSeq(endpoints, ",") {
		addresses = append(addresses, resolver.Address{Addr: ep})
	}
	cc.UpdateState(resolver.State{Addresses: addresses}) //nolint
	return &Resolver{}
}

func (r *Resolver) ResolveNow(_ resolver.ResolveNowOptions) {}

func (r *Resolver) Close() {}
