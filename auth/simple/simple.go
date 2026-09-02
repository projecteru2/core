package simple

import (
	"context"
	"crypto/subtle"

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
	passwords := meta.Get(b.username)
	if len(passwords) == 0 {
		return types.ErrInvaildGRPCUsername
	}
	if subtle.ConstantTimeCompare([]byte(passwords[0]), []byte(b.password)) != 1 {
		return types.ErrInvaildGRPCPassword
	}
	return nil
}
