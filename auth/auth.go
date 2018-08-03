package auth

import (
	"context"

	"github.com/projecteru2/core/auth/simple"
	"github.com/projecteru2/core/types"
	"google.golang.org/grpc"
)

// Auth define auth obj
type Auth interface {
	StreamInterceptor(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error
	UnaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error)
}

// NewAuth return auth obj
func NewAuth(config types.Config) Auth {
	authConfig := config.Auth
	basicAuth := simple.NewBasicAuth(authConfig.Username, authConfig.Password)
	return basicAuth
}
