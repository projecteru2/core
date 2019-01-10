package network

import (
	"context"

	enginetypes "github.com/projecteru2/core/engine/types"
)

// Network define network methods
type Network interface {
	// connect and disconnect
	ConnectToNetwork(ctx context.Context, containerID, networkID, ipv4 string) error
	DisconnectFromNetwork(ctx context.Context, containerID, networkID string) error
	// list networks
	ListNetworks(ctx context.Context, driver string) ([]*enginetypes.Network, error)
}
