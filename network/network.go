package network

import (
	"context"

	"gitlab.ricebook.net/platform/core/types"
)

type Network interface {
	// connect and disconnect
	ConnectToNetwork(ctx context.Context, containerID, networkID, ipv4 string) error
	DisconnectFromNetwork(ctx context.Context, containerID, networkID string) error
	// list networks
	ListNetworks(ctx context.Context) ([]*types.Network, error)
	// type and name to identify the network manager
	// this will determine when to call connect/disconnect
	Type() string
	Name() string
}
