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

func (b *BasicAuth) auth(ctx context.Context) error {
	meta, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return types.ErrBadMeta
	}
	passwords, ok := meta[b.username]
	if !ok {
		return types.ErrInvaildUsername
	}
	if passwords[0] != b.password {
		return types.ErrInvaildPassword
	}
	return nil
}

// StreamInterceptor define stream interceptor
func (b *BasicAuth) StreamInterceptor(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	ctx := stream.Context()
	if err := b.auth(ctx); err != nil {
		return err
	}
	return handler(srv, stream)
}

// UnaryInterceptor define unary interceptor
func (b *BasicAuth) UnaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	if err := b.auth(ctx); err != nil {
		return nil, err
	}
	return handler(ctx, req)
}
