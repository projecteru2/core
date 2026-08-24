package eru

import "google.golang.org/grpc/resolver"

type eruResolverBuilder struct{}

func (b *eruResolverBuilder) Scheme() string {
	return "eru"
}

func (b *eruResolverBuilder) Build(target resolver.Target, cc resolver.ClientConn, _ resolver.BuildOptions) (resolver.Resolver, error) {
	return New(cc, target.URL.Path, target.URL.Host), nil
}

func init() { //nolint:gochecknoinits // grpc resolver registration
	resolver.Register(&eruResolverBuilder{})
}
