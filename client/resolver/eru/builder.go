package eru

import (
	"google.golang.org/grpc/resolver"
)

type eruResolverBuilder struct{}

func init() { //nolint
	resolver.Register(&eruResolverBuilder{})
}

// Scheme for interface
func (b *eruResolverBuilder) Scheme() string {
	return "eru"
}

// Build for interface
func (b *eruResolverBuilder) Build(target resolver.Target, cc resolver.ClientConn, _ resolver.BuildOptions) (resolver.Resolver, error) {
	return New(cc, target.URL.Path, target.URL.Host), nil
}
