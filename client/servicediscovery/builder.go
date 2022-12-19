package servicediscovery

import "google.golang.org/grpc/resolver"

// LBResolverBuilder for service discovery lb
type LBResolverBuilder struct {
	updateCh chan []string
}

var lbResolverBuilder *LBResolverBuilder

func init() { //nolint
	lbResolverBuilder = &LBResolverBuilder{
		updateCh: make(chan []string),
	}
	resolver.Register(lbResolverBuilder)
}

// Scheme for interface
func (b *LBResolverBuilder) Scheme() string {
	return "lb"
}

// Build for interface
func (b *LBResolverBuilder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	return newLBResolver(cc, target.URL.Path, b.updateCh), nil
}
