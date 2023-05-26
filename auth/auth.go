package auth

import (
	"context"

	"github.com/projecteru2/core/auth/simple"
	"github.com/projecteru2/core/types"

	"google.golang.org/grpc"
)

// Auth define auth obj
type Auth interface {
	StreamInterceptor(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error
	UnaryInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error)
}

// NewAuth return auth obj
func NewAuth(auth types.AuthConfig) Auth {
	// TODO 这里可以组装其他的方法
	return simple.NewBasicAuth(auth.Username, auth.Password)
}

// Credential for client
type Credential interface {
	GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error)
	RequireTransportSecurity() bool
}

// NewCredential return credential obj
func NewCredential(auth types.AuthConfig) Credential {
	// TODO 这里可以组装其他的方法
	return simple.NewBasicCredential(auth.Username, auth.Password)
}
