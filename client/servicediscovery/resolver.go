package servicediscovery

import (
	"github.com/projecteru2/core/log"

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
	log.Debugf("[lbResolver] update state: %v", endpoints)
	addresses := []resolver.Address{}
	for _, ep := range endpoints {
		addresses = append(addresses, resolver.Address{Addr: ep})
	}
	r.cc.UpdateState(resolver.State{Addresses: addresses})
}

func (r *lbResolver) ResolveNow(_ resolver.ResolveNowOptions) {}

func (r lbResolver) Close() {}
