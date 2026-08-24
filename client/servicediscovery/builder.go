package servicediscovery

import "google.golang.org/grpc/resolver"

var lbResolverBuilder *LBResolverBuilder

// LBResolverBuilder builds resolvers for the lb:// scheme.
type LBResolverBuilder struct {
	updateCh chan []string
}

func (b *LBResolverBuilder) Scheme() string {
	return "lb"
}

func (b *LBResolverBuilder) Build(target resolver.Target, cc resolver.ClientConn, _ resolver.BuildOptions) (resolver.Resolver, error) {
	return newLBResolver(cc, target.URL.Path, b.updateCh), nil
}

func init() { //nolint:gochecknoinits // grpc resolver registration
	lbResolverBuilder = &LBResolverBuilder{
		updateCh: make(chan []string),
	}
	resolver.Register(lbResolverBuilder)
}
