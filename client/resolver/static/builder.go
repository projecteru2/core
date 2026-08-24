package static

import "google.golang.org/grpc/resolver"

type staticResolverBuilder struct{}

func (b *staticResolverBuilder) Scheme() string {
	return "static"
}

func (b *staticResolverBuilder) Build(target resolver.Target, cc resolver.ClientConn, _ resolver.BuildOptions) (resolver.Resolver, error) {
	return New(cc, target.URL.Path), nil
}

func init() { //nolint:gochecknoinits // grpc resolver registration
	resolver.Register(&staticResolverBuilder{})
}
