package network

import "golang.org/x/net/context"

type Network interface {
	// connect and disconnect
	ConnectToNetwork(ctx context.Context, containerID, networkID string) error
	DisconnectFromNetwork(ctx context.Context, containerID, networkID string) error
	// type and name to identify the network manager
	// this will determine when to call connect/disconnect
	Type() string
	Name() string
}
