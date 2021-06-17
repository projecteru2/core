package servicediscovery

import (
	"google.golang.org/grpc/resolver"
)

type lbResolver struct {
	cc resolver.ClientConn
}

func newLBResolver(cc resolver.ClientConn, endpoint string, updateCh <-chan []string) *lbResolver {
	r := &lbResolver{cc: cc}
	r.updateAddresses(endpoint)
	go func() {
		for {
			r.updateAddresses(<-updateCh...)
		}
	}()
	return r
}

func (r *lbResolver) updateAddresses(endpoints ...string) {
	addresses := []resolver.Address{}
	for _, ep := range endpoints {
		addresses = append(addresses, resolver.Address{Addr: ep})
	}
	r.cc.UpdateState(resolver.State{Addresses: addresses}) // nolint
}

func (r *lbResolver) ResolveNow(_ resolver.ResolveNowOptions) {}

func (r lbResolver) Close() {}
