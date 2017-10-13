package network

import (
	"context"

	"github.com/projecteru2/core/types"
)

type Network interface {
	// connect and disconnect
	ConnectToNetwork(ctx context.Context, containerID, networkID, ipv4 string) error
	DisconnectFromNetwork(ctx context.Context, containerID, networkID string) error
	// list networks
	ListNetworks(ctx context.Context, driver string) ([]*types.Network, error)
}
