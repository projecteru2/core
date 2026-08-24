package simple

import "context"

// BasicCredential sends a username and password as grpc metadata.
type BasicCredential struct {
	username string
	password string
}

func NewBasicCredential(username, password string) *BasicCredential {
	return &BasicCredential{username, password}
}

func (c BasicCredential) GetRequestMetadata(_ context.Context, _ ...string) (map[string]string, error) {
	return map[string]string{
		c.username: c.password,
	}, nil
}

func (c BasicCredential) RequireTransportSecurity() bool {
	return false
}
