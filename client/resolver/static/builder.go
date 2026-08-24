package static

import "google.golang.org/grpc/resolver"

type staticResolverBuilder struct{}

// Scheme for interface
func (b *staticResolverBuilder) Scheme() string {
	return "static"
}

// Build for interface
func (b *staticResolverBuilder) Build(target resolver.Target, cc resolver.ClientConn, _ resolver.BuildOptions) (resolver.Resolver, error) {
	return New(cc, target.URL.Path), nil
}

func init() { //nolint
	resolver.Register(&staticResolverBuilder{})
}
