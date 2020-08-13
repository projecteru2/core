package service_discovery

import (
	log "github.com/sirupsen/logrus"

	"google.golang.org/grpc/resolver"
)

type LBResolverBuilder struct {
	updateCh chan []string
}

var lbResolverBuilder *LBResolverBuilder

func init() {
	lbResolverBuilder = &LBResolverBuilder{
		updateCh: make(chan []string),
	}
	resolver.Register(lbResolverBuilder)
}

func (b *LBResolverBuilder) Scheme() string {
	return "lb"
}

func (b *LBResolverBuilder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	return newLBResolver(cc, target.Endpoint, b.updateCh), nil
}

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
