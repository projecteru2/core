package simple

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/projecteru2/core/types"
)

// BasicAuth checks a username and password carried in grpc metadata.
type BasicAuth struct {
	username string
	password string
}

func NewBasicAuth(username, password string) *BasicAuth {
	return &BasicAuth{username, password}
}

func (b *BasicAuth) StreamInterceptor(srv any, stream grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	ctx := stream.Context()
	if err := b.doAuth(ctx); err != nil {
		return err
	}
	return handler(srv, stream)
}

func (b *BasicAuth) UnaryInterceptor(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	if err := b.doAuth(ctx); err != nil {
		return nil, err
	}
	return handler(ctx, req)
}

func (b *BasicAuth) doAuth(ctx context.Context) error {
	meta, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return types.ErrInvaildGRPCRequestMeta
	}
	passwords, ok := meta[b.username]
	if !ok {
		return types.ErrInvaildGRPCUsername
	}
	if len(passwords) < 1 || passwords[0] != b.password {
		return types.ErrInvaildGRPCPassword
	}
	return nil
}
