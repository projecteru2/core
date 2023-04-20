package simple

import (
	"context"

	"github.com/projecteru2/core/types"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// BasicAuth use token to auth grcp request
type BasicAuth struct {
	username string
	password string
}

// NewBasicAuth return a basicauth obj
func NewBasicAuth(username, password string) *BasicAuth {
	return &BasicAuth{username, password}
}

// StreamInterceptor define stream interceptor
func (b *BasicAuth) StreamInterceptor(srv interface{}, stream grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	ctx := stream.Context()
	if err := b.doAuth(ctx); err != nil {
		return err
	}
	return handler(srv, stream)
}

// UnaryInterceptor define unary interceptor
func (b *BasicAuth) UnaryInterceptor(ctx context.Context, req interface{}, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
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
