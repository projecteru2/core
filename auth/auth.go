package auth

import (
	"context"

	"google.golang.org/grpc"

	"github.com/projecteru2/core/auth/simple"
	"github.com/projecteru2/core/types"
)

// Auth authenticates incoming grpc calls.
type Auth interface {
	StreamInterceptor(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error
	UnaryInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error)
}

func NewAuth(auth types.AuthConfig) Auth {
	return simple.NewBasicAuth(auth.Username, auth.Password)
}

// Credential supplies per-RPC auth metadata to clients.
type Credential interface {
	GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error)
	RequireTransportSecurity() bool
}

func NewCredential(auth types.AuthConfig) Credential {
	return simple.NewBasicCredential(auth.Username, auth.Password)
}
